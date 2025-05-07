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

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUtilWriteStringToFile(t *testing.T) {
	s := "my test string"
	path := filepath.Join(t.TempDir(), "string_test.txt")

	err := WriteStringToFile(s, path)
	if err != nil {
		t.Fatalf("error writing string to %s, %v", path, err)
	}

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("unexpected error reading from %s, %v", path, err)
	}

	if string(actual) != s {
		t.Errorf("file contents don't match, expected %s, found %s", s, actual)
	}
}

func TestUtilWriteBytesToFile(t *testing.T) {
	b := []byte("my test bytes")
	path := filepath.Join(t.TempDir(), "bytes_test.txt")

	err := WriteBytesToFile(b, path)
	if err != nil {
		t.Fatalf("error writing bytes to %s, %v", path, err)
	}

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("unexpected error reading from %s, %v", path, err)
	}

	if string(actual) != string(b) {
		t.Errorf("file contents don't match, expected %s, found %s", b, actual)
	}
}
