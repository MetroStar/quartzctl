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

package tofu

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"
)

func TestStageEnvPreservesKubeconfigWithManagedAutomationVariables(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "environment")
	binary := filepath.Join(dir, "tofu")
	script := `#!/bin/sh
printf '%s\n' "$KUBE_CONFIG_PATH" "$TF_PLUGIN_CACHE_DIR" "$STAGE_PROBE" > "$ENV_CAPTURE"
printf '{"terraform_version":"1.13.1","platform":"linux_amd64","provider_selections":{}}'
`
	if err := os.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TF_INPUT", "0")
	t.Setenv("TF_IN_AUTOMATION", "1")
	t.Setenv("STAGE_PROBE", "preserved")
	t.Setenv("ENV_CAPTURE", capture)
	client := TofuClient{cfg: config.Settings{Config: schema.QuartzConfig{Tmp: dir}}}
	tf, err := tfexec.NewTerraform(dir, binary)
	if err != nil {
		t.Fatal(err)
	}
	stage := schema.StageConfig{Id: "prereqs", Providers: schema.StageProvidersConfig{Kubernetes: true}}
	if err := client.setStageEnv(tf, stage); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tf.Version(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{filepath.Join(dir, "kubeconfig"), filepath.Join(dir, "plugin-cache"), "preserved", ""}, "\n")
	if string(got) != want {
		t.Fatalf("provider environment = %q, want %q", got, want)
	}
}

func TestStageEnvRejectsOtherManagedVariablesExplicitly(t *testing.T) {
	t.Setenv("TF_WORKSPACE", "unexpected")
	client := TofuClient{cfg: config.Settings{Config: schema.QuartzConfig{Tmp: t.TempDir()}}}
	tf, err := tfexec.NewTerraform(t.TempDir(), "/bin/true")
	if err != nil {
		t.Fatal(err)
	}
	err = client.setStageEnv(tf, schema.StageConfig{Id: "prereqs"})
	if err == nil || !strings.Contains(err.Error(), "prereqs") || !strings.Contains(err.Error(), "TF_WORKSPACE") {
		t.Fatalf("expected actionable stage environment error, got %v", err)
	}
}

func TestStageOperationsStopOnEnvironmentConfigurationError(t *testing.T) {
	t.Setenv("TF_WORKSPACE", "unexpected")
	ctx := context.Background()
	dir := t.TempDir()
	stage := schema.StageConfig{Id: "prereqs", Path: dir}
	client := TofuClient{
		cfg:      config.Settings{Config: schema.QuartzConfig{Tmp: dir}},
		execPath: "/bin/false", cacheMu: &sync.Mutex{},
		clientCache: make(map[string]*tfexec.Terraform),
	}
	operations := map[string]func() error{
		"init":       func() error { return client.Init(ctx, stage, TofuInitOpts{}) },
		"plan":       func() error { _, err := client.Plan(ctx, stage); return err },
		"apply":      func() error { return client.Apply(ctx, stage) },
		"saved plan": func() error { return client.Apply(ctx, stage, TofuApplyOpts{PlanFile: "saved.plan"}) },
		"destroy":    func() error { return client.Destroy(ctx, stage) },
		"refresh":    func() error { return client.Refresh(ctx, stage) },
		"unlock":     func() error { return client.ForceUnlock(ctx, stage, "lock-id") },
		"import":     func() error { return client.Import(ctx, stage, "resource.example", "id") },
		"remove":     func() error { return client.StateRemove(ctx, stage, "resource.example") },
		"output":     func() error { _, err := client.Output(ctx, stage); return err },
		"state list": func() error { _, err := client.StateList(ctx, stage); return err },
		"orphan cleanup": func() error {
			_, err := client.StateRemoveOrphanedClusterResources(ctx, stage)
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			err := operation()
			if err == nil || !strings.Contains(err.Error(), "TF_WORKSPACE") {
				t.Fatalf("expected environment configuration failure before subprocess execution, got %v", err)
			}
		})
	}
}

func TestExtractLockID(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   string
		wantOk bool
	}{
		{
			name:   "empty",
			errMsg: "",
			want:   "",
			wantOk: false,
		},
		{
			name:   "no lock info",
			errMsg: "Error: some unrelated failure",
			want:   "",
			wantOk: false,
		},
		{
			name: "canonical lock info block",
			errMsg: `Error: Error acquiring the state lock

Lock Info:
  ID:        54802a0b-4db5-819a-4f02-2827bdcf02ba
  Path:      pa-state/host.tfstate
  Operation: OperationTypeApply`,
			want:   "54802a0b-4db5-819a-4f02-2827bdcf02ba",
			wantOk: true,
		},
		{
			// Regression: the AWS DynamoDB ConditionalCheckFailed error puts a
			// "RequestID:" line BEFORE the "Lock Info: ID:" line. A naive
			// substring search for "ID:" matches "RequestID:" and returns the
			// wrong value, causing force-unlock to fail with
			// "does not match existing lock".
			name: "dynamodb request id must not shadow lock id",
			errMsg: `Error: Error acquiring the state lock

  Error message: operation error DynamoDB: PutItem, https response error
  StatusCode: 400, RequestID:
  HNRHVM001U02E6AO7SOBNUG3LRVV4KQNSO5AEMVJF66Q9ASUAAJG,
  ConditionalCheckFailedException: The conditional request failed
  Lock Info:
    ID:        54802a0b-4db5-819a-4f02-2827bdcf02ba
    Path:      pa-state/host.tfstate`,
			want:   "54802a0b-4db5-819a-4f02-2827bdcf02ba",
			wantOk: true,
		},
		{
			name: "trailing comma stripped",
			errMsg: `Lock Info:
  ID:        abcd-1234,`,
			want:   "abcd-1234",
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ExtractLockID(tt.errMsg)
			if ok != tt.wantOk {
				t.Fatalf("ExtractLockID ok = %v, want %v", ok, tt.wantOk)
			}
			if got != tt.want {
				t.Fatalf("ExtractLockID = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactStateValues(t *testing.T) {
	tests := []struct {
		name      string
		values    map[string]interface{}
		sensitive string
		want      map[string]interface{}
	}{
		{
			name:      "nil values",
			values:    nil,
			sensitive: `{}`,
			want:      nil,
		},
		{
			name:      "no sensitive attributes",
			values:    map[string]interface{}{"name": "quartz", "replicas": float64(3)},
			sensitive: `{}`,
			want:      map[string]interface{}{"name": "quartz", "replicas": float64(3)},
		},
		{
			name:      "top-level sensitive leaf redacted",
			values:    map[string]interface{}{"name": "quartz", "token": "ghp_secret"},
			sensitive: `{"token": true}`,
			want:      map[string]interface{}{"name": "quartz", "token": "[REDACTED]"},
		},
		{
			name: "nested sensitive value redacted",
			values: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":     "creds",
					"password": "p@ss",
				},
			},
			sensitive: `{"metadata": {"password": true}}`,
			want: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":     "creds",
					"password": "[REDACTED]",
				},
			},
		},
		{
			name: "sensitive list element redacted",
			values: map[string]interface{}{
				"set_sensitive": []interface{}{"keep", "secret"},
			},
			sensitive: `{"set_sensitive": [false, true]}`,
			want: map[string]interface{}{
				"set_sensitive": []interface{}{"keep", "[REDACTED]"},
			},
		},
		{
			name:      "whole subtree redacted when node is true",
			values:    map[string]interface{}{"values": map[string]interface{}{"a": "1", "b": "2"}},
			sensitive: `{"values": true}`,
			want:      map[string]interface{}{"values": "[REDACTED]"},
		},
		{
			name:      "unparseable sensitivity leaves values unchanged",
			values:    map[string]interface{}{"token": "secret"},
			sensitive: `not-json`,
			want:      map[string]interface{}{"token": "secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactStateValues(tt.values, json.RawMessage(tt.sensitive))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("redactStateValues = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCollectStateAddresses(t *testing.T) {
	t.Run("nil state", func(t *testing.T) {
		if got := collectStateAddresses(nil); got != nil {
			t.Fatalf("collectStateAddresses(nil) = %#v, want nil", got)
		}
	})

	t.Run("root and child modules", func(t *testing.T) {
		state := &tfjson.State{
			Values: &tfjson.StateValues{
				RootModule: &tfjson.StateModule{
					Resources: []*tfjson.StateResource{
						{Address: "helm_release.quartz"},
						{Address: "kubernetes_namespace_v1.this"},
					},
					ChildModules: []*tfjson.StateModule{
						{
							Resources: []*tfjson.StateResource{
								{Address: "module.eso.kubernetes_secret_v1.git"},
							},
						},
					},
				},
			},
		}

		want := []string{
			"helm_release.quartz",
			"kubernetes_namespace_v1.this",
			"module.eso.kubernetes_secret_v1.git",
		}
		if got := collectStateAddresses(state); !reflect.DeepEqual(got, want) {
			t.Fatalf("collectStateAddresses = %#v, want %#v", got, want)
		}
	})
}

func TestIsClusterResidentType(t *testing.T) {
	tests := []struct {
		resourceType string
		want         bool
	}{
		{"helm_release", true},
		{"kubernetes_namespace_v1", true},
		{"kubectl_manifest", true},
		{"aws_iam_role", false},
		{"aws_kms_key", false},
		{"random_password", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			if got := isClusterResidentType(tt.resourceType); got != tt.want {
				t.Fatalf("isClusterResidentType(%q) = %v, want %v", tt.resourceType, got, tt.want)
			}
		})
	}
}
