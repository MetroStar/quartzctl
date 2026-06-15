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
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
)

var stillDestroyingPattern = regexp.MustCompile(`^([^:]+): Still destroying\.\.\. \[`)

// progressAnnotatingWriter forwards OpenTofu output unchanged while adding a
// short, de-duplicated explanation for provider-side destroy waits. It watches
// only the human stream OpenTofu already emits; it does not query any cloud API.
type progressAnnotatingWriter struct {
	dst     io.Writer
	pending string
	seen    map[string]bool
	mu      sync.Mutex
}

func newProgressAnnotatingWriter(dst io.Writer) *progressAnnotatingWriter {
	return &progressAnnotatingWriter{
		dst:  dst,
		seen: map[string]bool{},
	}
}

func (w *progressAnnotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.dst.Write(p); err != nil {
		return 0, err
	}

	w.pending += string(p)
	for {
		idx := strings.IndexByte(w.pending, '\n')
		if idx < 0 {
			break
		}
		line := w.pending[:idx]
		w.pending = w.pending[idx+1:]
		if err := w.annotateLine(strings.TrimRight(line, "\r")); err != nil {
			return len(p), err
		}
	}

	return len(p), nil
}

func (w *progressAnnotatingWriter) annotateLine(line string) error {
	match := stillDestroyingPattern.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	label := destroyProgressLabel(match[1])
	if label == "" || w.seen[label] {
		return nil
	}
	w.seen[label] = true

	_, err := fmt.Fprintf(w.dst, "  Note: %s deletion is still progressing provider-side; several minutes here can be normal.\n", label)
	return err
}

func destroyProgressLabel(address string) string {
	lower := strings.ToLower(address)
	switch {
	case strings.Contains(lower, "aws_eks_identity_provider_config"):
		return "EKS OIDC identity provider"
	case strings.Contains(lower, "aws_eks_node_group"):
		return "EKS managed node group"
	case strings.Contains(lower, "aws_eks_cluster"):
		return "EKS cluster"
	case strings.Contains(lower, "aws_db_instance"):
		return "RDS database"
	case strings.Contains(lower, "aws_nat_gateway"):
		return "NAT gateway"
	case strings.Contains(lower, "aws_sqs_queue"):
		return "SQS queue"
	case strings.Contains(lower, "aws_efs_file_system"):
		return "EFS file system"
	case strings.Contains(lower, "aws_efs_mount_target"):
		return "EFS mount target"
	case strings.Contains(lower, "helm_release"):
		return "Helm release"
	default:
		return ""
	}
}
