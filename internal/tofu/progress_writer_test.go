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
	"bytes"
	"strings"
	"testing"
)

func TestDestroyProgressLabel(t *testing.T) {
	tests := []struct {
		address string
		want    string
	}{
		{"module.cluster.aws_eks_cluster.this[0]", "EKS cluster"},
		{"module.cluster.aws_eks_node_group.default", "EKS managed node group"},
		{"module.host.aws_db_instance.sonarqube", "RDS database"},
		{"module.karpenter.aws_sqs_queue.this", "SQS queue"},
		{"module.other.random_resource.this", ""},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			if got := destroyProgressLabel(tt.address); got != tt.want {
				t.Fatalf("destroyProgressLabel()=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestProgressAnnotatingWriterAddsDestroyNoteOnce(t *testing.T) {
	var dst bytes.Buffer
	w := newProgressAnnotatingWriter(&dst)
	line := "module.cluster.aws_eks_node_group.default: Still destroying... [id=pa:ng, 10s elapsed]\n"

	if _, err := w.Write([]byte(line + line)); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	got := dst.String()
	if strings.Count(got, "EKS managed node group deletion is still progressing") != 1 {
		t.Fatalf("expected one managed node group note, got:\n%s", got)
	}
	if strings.Count(got, line) != 2 {
		t.Fatalf("expected original lines to pass through unchanged, got:\n%s", got)
	}
}

func TestProgressAnnotatingWriterAddsHelmReleaseNote(t *testing.T) {
	var dst bytes.Buffer
	w := newProgressAnnotatingWriter(&dst)
	line := "helm_release.external_secrets: Still destroying... [id=external-secrets, 10s elapsed]\n"

	if _, err := w.Write([]byte(line)); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	got := dst.String()
	if !strings.Contains(got, "Helm release deletion is still unwinding in-cluster") {
		t.Fatalf("expected helm release destroy note, got:\n%s", got)
	}
	if strings.Contains(got, "provider-side") {
		t.Fatalf("helm release note should not describe the wait as provider-side, got:\n%s", got)
	}
}

func TestProgressAnnotatingWriterPassesPlainOutput(t *testing.T) {
	var dst bytes.Buffer
	w := newProgressAnnotatingWriter(&dst)
	input := "module.foo: Refreshing state...\nApply complete!\n"

	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if got := dst.String(); got != input {
		t.Fatalf("plain output changed:\n got=%q\nwant=%q", got, input)
	}
}
