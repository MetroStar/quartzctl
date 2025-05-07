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

package util

import "testing"

func TestRunOnce(t *testing.T) {
	count := 0
	countFunc := func() error {
		count = count + 1
		return nil
	}

	// first call, increment count from 0 -> 1
	err := RunOnce("test", countFunc)
	if err != nil {
		t.Errorf("unexpected error in RunOnce, %v", err)
	}

	if count != 1 {
		t.Errorf("unexpected run count in RunOnce first call, expected 1, found %d", count)
	}

	// second call with the same key, no action, count remains 1
	RunOnce("test", countFunc)

	if count != 1 {
		t.Errorf("unexpected run count in RunOnce second call, expected 1, found %d", count)
	}

	// third call with different key, increment count from 1 -> 2
	RunOnce("different", countFunc)

	if count != 2 {
		t.Errorf("unexpected run count in RunOnce third call, expected 2, found %d", count)
	}
}
