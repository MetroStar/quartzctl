// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestNewRootInstallCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootInstallCommand(p).Command

	assert.Equal(t, "install", cmd.Name)
	assert.Equal(t, "Perform a full install/update of the system", cmd.Usage)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestNewRootCleanCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootCleanCommand(p).Command

	assert.Equal(t, "clean", cmd.Name)
	assert.Equal(t, "Perform a full cleanup/teardown of the system", cmd.Usage)
	assert.Len(t, cmd.Flags, 2)

	flag := cmd.Flags[0].(*cli.BoolFlag)
	assert.Equal(t, "refresh", flag.Name)

	yesFlag := cmd.Flags[1].(*cli.BoolFlag)
	assert.Equal(t, "yes", yesFlag.Name)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestCmdInstall(t *testing.T) {
	p := defaultTestConfig(t)

	err := Install(context.Background(), p, "")
	if err != nil {
		t.Errorf("unexpected error in cmd Install, %v", err)
	}
}

func TestCmdClean(t *testing.T) {
	p := defaultTestConfig(t)

	err := Clean(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd Clean, %v", err)
	}
}

func TestIsRetryableDestroyError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{
			name:     "DependencyViolation error",
			errStr:   "Error: DependencyViolation: resource has dependencies",
			expected: true,
		},
		{
			name:     "has a dependent object error",
			errStr:   "cannot delete: has a dependent object",
			expected: true,
		},
		{
			name:     "NetworkInterfaceInUse error",
			errStr:   "Error: NetworkInterfaceInUse: interface eni-123 is in use",
			expected: true,
		},
		{
			name:     "InvalidGroup.InUse error",
			errStr:   "Error: InvalidGroup.InUse: security group sg-123 is in use",
			expected: true,
		},
		{
			name:     "Helm failed to delete release",
			errStr:   "Error: failed to delete release: connection refused",
			expected: true,
		},
		{
			name:     "Kubernetes cluster unreachable",
			errStr:   "Kubernetes cluster unreachable: dial tcp timeout",
			expected: true,
		},
		{
			name:     "connection refused",
			errStr:   "Post https://api.cluster.local: connection refused",
			expected: true,
		},
		{
			name:     "no endpoints available",
			errStr:   "Internal error: failed calling webhook: no endpoints available",
			expected: true,
		},
		{
			name:     "i/o timeout",
			errStr:   "Post https://api.cluster.local: i/o timeout",
			expected: true,
		},
		{
			name:     "unrelated error",
			errStr:   "Error: resource not found",
			expected: false,
		},
		{
			name:     "empty string",
			errStr:   "",
			expected: false,
		},
		{
			name:     "permission denied",
			errStr:   "Error: Access Denied",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableDestroyError(tt.errStr)
			assert.Equal(t, tt.expected, result, "isRetryableDestroyError(%q) = %v, want %v", tt.errStr, result, tt.expected)
		})
	}
}

func TestIsClusterUnreachableError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{name: "resource not found", errStr: "Error: ResourceNotFoundException: No cluster found", expected: true},
		{name: "no cluster found", errStr: "No cluster found for name pa-1", expected: true},
		{name: "discovery client", errStr: "cannot create discovery client", expected: true},
		{name: "rest mapper", errStr: "Failed to get RESTMapper client", expected: true},
		{name: "config path", errStr: "provider config_path is set but file is missing", expected: true},
		{name: "cluster unreachable", errStr: "Kubernetes cluster unreachable: timeout", expected: true},
		{name: "could not find resource", errStr: "the server could not find the requested resource", expected: true},
		{name: "unrelated", errStr: "DependencyViolation: in use", expected: false},
		{name: "empty", errStr: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isClusterUnreachableError(tt.errStr))
		})
	}
}

func TestIsHelmReleaseRecordError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{name: "failed to delete release", errStr: "Error: failed to delete release: cilium", expected: true},
		{name: "unable to uninstall", errStr: "Unable to uninstall Helm release foo", expected: true},
		{name: "unrelated", errStr: "DependencyViolation: in use", expected: false},
		{name: "empty", errStr: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isHelmReleaseRecordError(tt.errStr))
		})
	}
}

func TestSplitStageError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantStage string
		wantMsg   string
	}{
		{name: "stage prefixed", err: fmt.Errorf("stage prereqs: No cluster found"), wantStage: "prereqs", wantMsg: "No cluster found"},
		{name: "no stage prefix", err: fmt.Errorf("cleanup: boom"), wantStage: "", wantMsg: "cleanup: boom"},
		{name: "stage word but no colon", err: fmt.Errorf("stage failure"), wantStage: "", wantMsg: "stage failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage, msg := splitStageError(tt.err)
			assert.Equal(t, tt.wantStage, stage)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}

func TestIsFluxOwnedReleaseDrift(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name: "helm provider version mismatch (Flux-owned bootstrap release)",
			err: errors.New(`Error: Planned version is different from configured version` +
				"\n  with helm_release.quartz,\n" +
				`The version in the configuration is "1.0.0+df9c1f7b78c1" but the planned version is "1.0.0".`),
			expected: true,
		},
		{
			name:     "unrelated plan error",
			err:      errors.New("Error: Backend initialization required"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFluxOwnedReleaseDrift(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
