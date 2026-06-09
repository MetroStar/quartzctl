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
	"io"
	"strings"
)

// benignRefreshNoisePatterns are diagnostic summaries OpenTofu emits during a
// pre-destroy refresh that are expected and harmless while tearing an
// environment down. The subsequent destroy runs with refresh disabled and is
// unaffected, so surfacing these to the operator only makes a healthy clean
// look alarming.
//
//   - "Planned version is different from configured version": the umbrella Helm
//     release is version-stamped by Flux ("1.0.0+<sha>") once it adopts the
//     bootstrap release, so the Helm provider reports a benign drift.
//   - "replication configuration was not found" / "Object Lock configuration
//     does not exist": optional S3 sub-resources the AWS provider reads on every
//     bucket refresh that legitimately do not exist on the state bucket.
var benignRefreshNoisePatterns = []string{
	"Planned version is different from configured version",
	"replication configuration was not found",
	"Object Lock configuration does not exist",
}

// OpenTofu's human-readable diagnostics are framed in a box drawn with these
// glyphs. Color is auto-disabled because our writer is not a TTY, so the
// boundaries appear verbatim and can be matched directly.
const (
	diagBlockTop    = "╷"
	diagBlockBottom = "╵"
)

// isBenignRefreshNoise reports whether text contains any benign-teardown
// diagnostic summary.
func isBenignRefreshNoise(text string) bool {
	for _, p := range benignRefreshNoisePatterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// noiseFilterWriter wraps an io.Writer and drops whole OpenTofu diagnostic
// blocks whose contents match a known benign-during-teardown pattern, leaving
// all other output untouched. It is line-buffered so the per-block decision is
// evaluated against complete lines regardless of how the child process output
// is chunked. A diagnostic block spans from a "╷" boundary to the matching "╵"
// boundary; the whole block is suppressed when any of its lines is benign noise,
// which avoids leaving an orphaned, half-rendered warning box on the console.
//
// Callers must invoke Flush once the command completes to emit any trailing,
// non-newline-terminated content still held in the buffer.
type noiseFilterWriter struct {
	dst io.Writer

	pending  []byte   // partial line not yet terminated by a newline
	inBlock  bool     // currently inside a diagnostic block
	blockBuf []string // buffered lines of the in-progress diagnostic block
}

// newNoiseFilterWriter returns a noiseFilterWriter forwarding kept output to dst.
func newNoiseFilterWriter(dst io.Writer) *noiseFilterWriter {
	return &noiseFilterWriter{dst: dst}
}

// Write buffers input and forwards every complete line that is not part of a
// suppressed diagnostic block. It always reports len(p) consumed so it never
// stalls the upstream copy; a downstream write error is surfaced as the error.
func (w *noiseFilterWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := string(w.pending[:i+1])
		w.pending = w.pending[i+1:]
		if err := w.handleLine(line); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

// Flush emits any buffered diagnostic block and trailing partial line. It is
// used when the underlying command finishes without a final newline.
func (w *noiseFilterWriter) Flush() error {
	if w.inBlock {
		// Unterminated block: emit it verbatim rather than risk swallowing a
		// genuine, still-streaming diagnostic.
		block := strings.Join(w.blockBuf, "")
		w.blockBuf = nil
		w.inBlock = false
		if _, err := io.WriteString(w.dst, block); err != nil {
			return err
		}
	}
	if len(w.pending) == 0 {
		return nil
	}
	line := w.pending
	w.pending = nil
	_, err := w.dst.Write(line)
	return err
}

// handleLine routes a single complete line through the diagnostic-block state
// machine, forwarding or suppressing as appropriate.
func (w *noiseFilterWriter) handleLine(line string) error {
	trimmed := strings.TrimSpace(line)

	if !w.inBlock {
		if trimmed == diagBlockTop {
			w.inBlock = true
			w.blockBuf = []string{line}
			return nil
		}
		_, err := io.WriteString(w.dst, line)
		return err
	}

	// Inside a diagnostic block — keep buffering until the closing boundary.
	w.blockBuf = append(w.blockBuf, line)
	if trimmed != diagBlockBottom {
		return nil
	}

	block := w.blockBuf
	w.blockBuf = nil
	w.inBlock = false
	if isBenignRefreshNoise(strings.Join(block, "")) {
		return nil
	}
	_, err := io.WriteString(w.dst, strings.Join(block, ""))
	return err
}
