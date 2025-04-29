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
