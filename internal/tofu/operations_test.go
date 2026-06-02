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

import "testing"

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
