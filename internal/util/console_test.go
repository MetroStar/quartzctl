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
	"testing"
)

func TestConsoleWrite(t *testing.T) {
	msg := "test log entry"
	msgF := "test log entry with arg %s"
	arg := "foobar"

	Hdr(msg)
	Msg(msg)
	Print(msg)
	Error(msg)

	Hdrf(msgF, arg)
	Msgf(msgF, arg)
	Printf(msgF, arg)
	Errorf(msgF, arg)
}

func TestConsoleTable(t *testing.T) {
	headers := []string{"col1", "col2"}
	rows := [][]string{
		{"cell1", "cell2"},
		{"cell3", "cell4"},
	}

	PrintTable(headers, rows)
	PrintRowStatusTable(headers, rows, func(i int, row []string) RowStatus {
		if i == 0 {
			return StatusError
		}

		return StatusOk
	})
}

func TestConsolePromptYesNoAffirmative(t *testing.T) {
	r, w, _ := os.Pipe()
	w.Write([]byte("y\n"))
	w.Close()

	// Temporarily replace os.Stdin with our buffer
	defer func(v *os.File) { os.Stdin = v }(os.Stdin)
	os.Stdin = r

	t.Setenv("SILENT", "")
	t.Setenv("ACCESSIBLE", "1")
	res := PromptYesNo("this is a test")

	if !res {
		t.Error("unexpected response from silent yes/no prompt (affirmative)")
	}
}

func TestConsolePromptYesNoNegative(t *testing.T) {
	r, w, _ := os.Pipe()
	w.Write([]byte("n\n"))
	w.Close()

	// Temporarily replace os.Stdin with our buffer
	defer func(v *os.File) { os.Stdin = v }(os.Stdin)
	os.Stdin = r

	t.Setenv("SILENT", "")
	t.Setenv("ACCESSIBLE", "1")
	res := PromptYesNo("this is a test")

	if res {
		t.Error("unexpected response from silent yes/no prompt (negative)")
	}
}

func TestConsolePromptYesNoSilent(t *testing.T) {
	t.Setenv("SILENT", "1")
	res := PromptYesNo("this is a test")
	if !res {
		t.Error("unexpected response from silent yes/no prompt (silent)")
	}
}

func TestConsolePrintBanner(t *testing.T) {
	PrintBanner()
}
