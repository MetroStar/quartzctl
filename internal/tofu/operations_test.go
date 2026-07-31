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
	"encoding/json"
	"reflect"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
)

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
