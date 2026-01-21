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
	assert.Len(t, cmd.Flags, 1)

	flag := cmd.Flags[0].(*cli.BoolFlag)
	assert.Equal(t, "refresh", flag.Name)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestCmdInstall(t *testing.T) {
	p := defaultTestConfig(t)

	err := Install(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd Install, %v", err)
	}
}

func TestCmdClean(t *testing.T) {
	p := defaultTestConfig(t)

	err := Clean(context.Background(), true, p)
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

func TestIsHelmReleaseError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{
			name:     "failed to delete release",
			errStr:   "Error: failed to delete release reloader",
			expected: true,
		},
		{
			name:     "release not found",
			errStr:   "Error: release: not found",
			expected: true,
		},
		{
			name:     "Kubernetes cluster unreachable",
			errStr:   "Kubernetes cluster unreachable: dial tcp timeout",
			expected: true,
		},
		{
			name:     "no endpoints available for webhook",
			errStr:   "Internal error: failed calling webhook: no endpoints available for service kyverno",
			expected: true,
		},
		{
			name:     "generic AWS error",
			errStr:   "Error: DependencyViolation: resource has dependencies",
			expected: false,
		},
		{
			name:     "empty string",
			errStr:   "",
			expected: false,
		},
		{
			name:     "unrelated error",
			errStr:   "Error: resource not found",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHelmReleaseError(tt.errStr)
			assert.Equal(t, tt.expected, result, "isHelmReleaseError(%q) = %v, want %v", tt.errStr, result, tt.expected)
		})
	}
}
