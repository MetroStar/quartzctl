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

func TestIsBenignRefreshNoise(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"flux release drift", "Error: Planned version is different from configured version", true},
		{"s3 replication", "Warning: replication configuration was not found", true},
		{"s3 object lock", "Object Lock configuration does not exist for this bucket", true},
		{"genuine error", "Error: AccessDenied: not authorized to perform iam:DeleteRole", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBenignRefreshNoise(tt.text); got != tt.want {
				t.Errorf("isBenignRefreshNoise(%q)=%v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// writeFiltered runs the writer over a single payload and returns the forwarded
// output after flushing.
func writeFiltered(t *testing.T, input string) string {
	t.Helper()
	var dst bytes.Buffer
	w := newNoiseFilterWriter(&dst)
	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}
	return dst.String()
}

func TestNoiseFilterWriterPassesThroughPlainLines(t *testing.T) {
	in := "module.x.aws_s3_bucket.y: Refreshing state... [id=foo]\nApply complete!\n"
	if got := writeFiltered(t, in); got != in {
		t.Errorf("plain output mutated:\n got=%q\nwant=%q", got, in)
	}
}

func TestNoiseFilterWriterSuppressesBenignBlock(t *testing.T) {
	benign := "╷\n│ Warning: replication configuration was not found\n│ \n│   with module.state.aws_s3_bucket.this,\n│   on main.tf line 1:\n╵\n"
	in := "before\n" + benign + "after\n"
	got := writeFiltered(t, in)
	want := "before\nafter\n"
	if got != want {
		t.Errorf("benign block not suppressed:\n got=%q\nwant=%q", got, want)
	}
	if strings.Contains(got, "replication configuration") {
		t.Errorf("benign block leaked through: %q", got)
	}
}

func TestNoiseFilterWriterKeepsGenuineBlock(t *testing.T) {
	genuine := "╷\n│ Error: AccessDenied\n│ \n│   not authorized to perform iam:DeleteRole\n╵\n"
	in := "before\n" + genuine + "after\n"
	got := writeFiltered(t, in)
	if got != "before\n"+genuine+"after\n" {
		t.Errorf("genuine block altered:\n got=%q", got)
	}
}

func TestNoiseFilterWriterHandlesChunkedWrites(t *testing.T) {
	benign := "╷\n│ Warning: Object Lock configuration does not exist\n╵\n"
	full := "keep1\n" + benign + "keep2\n"

	var dst bytes.Buffer
	w := newNoiseFilterWriter(&dst)
	// Feed the stream one byte at a time to exercise partial-line buffering.
	for i := 0; i < len(full); i++ {
		if _, err := w.Write([]byte{full[i]}); err != nil {
			t.Fatalf("unexpected write error: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	want := "keep1\nkeep2\n"
	if dst.String() != want {
		t.Errorf("chunked filtering failed:\n got=%q\nwant=%q", dst.String(), want)
	}
}

func TestNoiseFilterWriterFlushesTrailingPartialLine(t *testing.T) {
	// No trailing newline: the partial line must still be emitted on Flush.
	if got := writeFiltered(t, "no newline here"); got != "no newline here" {
		t.Errorf("trailing partial line dropped: %q", got)
	}
}

func TestNoiseFilterWriterFlushesUnterminatedBlock(t *testing.T) {
	// A block that never closes should be emitted verbatim on Flush rather than
	// swallowed.
	in := "╷\n│ Error: something still streaming"
	if got := writeFiltered(t, in); got != in {
		t.Errorf("unterminated block dropped:\n got=%q\nwant=%q", got, in)
	}
}
