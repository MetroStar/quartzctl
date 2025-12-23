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
	"bytes"
	"context"
	"testing"

	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestCmdForceCleanup(t *testing.T) {
	p := defaultTestConfig(t)

	err := ForceCleanup(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd ForceCleanup, %v", err)
	}
}

func TestCleanupTerminatingPods(t *testing.T) {
	p := defaultTestConfig(t)

	// Capture output
	var buf bytes.Buffer
	util.SetWriter(&buf)
	defer util.SetWriter(&bytes.Buffer{})

	// Test with default timeout (no pods should be found in mock)
	err := CleanupTerminatingPods(context.Background(), p, 5)
	assert.NoError(t, err)

	// Verify output contains expected message
	output := buf.String()
	assert.Contains(t, output, "Cleaning up pods stuck in Terminating state")
}

func TestCleanupTerminatingPodsZeroTimeout(t *testing.T) {
	p := defaultTestConfig(t)

	// Capture output
	var buf bytes.Buffer
	util.SetWriter(&buf)
	defer util.SetWriter(&bytes.Buffer{})

	// Test with zero timeout
	err := CleanupTerminatingPods(context.Background(), p, 0)
	assert.NoError(t, err)
}

