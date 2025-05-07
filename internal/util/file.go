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
	"bufio"
	"os"
	"path/filepath"
)

// WriteStringToFile writes a string to a file at the given path.
// It creates any required directories as needed.
func WriteStringToFile(s string, path string) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0740)
	if err != nil {
		return err
	}

	f, err := os.Create(path) // #nosec G304
	if err != nil {
		return err
	}

	defer f.Close()
	w := bufio.NewWriter(f)
	_, err = w.WriteString(s)
	if err != nil {
		return err
	}
	err = w.Flush()
	if err != nil {
		return err
	}

	return nil
}

// WriteBytesToFile writes bytes to a file at the given path.
// It creates any required directories as needed.
func WriteBytesToFile(b []byte, path string) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0740)
	if err != nil {
		return err
	}

	f, err := os.Create(path) // #nosec G304
	if err != nil {
		return err
	}

	defer f.Close()
	w := bufio.NewWriter(f)
	_, err = w.Write(b)
	if err != nil {
		return err
	}
	w.Flush()

	return nil
}
