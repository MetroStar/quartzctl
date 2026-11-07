//go:build !windows

package privateaccess

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func installFakeSSM(t *testing.T, directory string) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	shim := []byte("#!/bin/sh\nexec \"$QUARTZ_TEST_BINARY\" -test.run=TestSessionManagerHelper -- \"$@\"\n")
	for _, name := range []string{"aws", "session-manager-plugin"} {
		if err := os.WriteFile(filepath.Join(directory, name), shim, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("QUARTZ_TEST_SSM_HELPER", "1")
	t.Setenv("QUARTZ_TEST_BINARY", binary)
}

func TestRemoteTerminationAfterForwardingAlreadyExited(t *testing.T) {
	for _, failure := range []bool{false, true} {
		name := "success"
		if failure {
			name = "denied"
		}
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			installFakeSSM(t, directory)
			record := filepath.Join(directory, "terminated")
			t.Setenv("QUARTZ_TEST_SSM_TERMINATED", record)
			if failure {
				t.Setenv("QUARTZ_TEST_SSM_TERMINATE_FAIL", "1")
			}
			wait := make(chan error, 1)
			wait <- nil
			session := &Session{
				forward: &exec.Cmd{}, wait: wait, region: "us-east-1",
				metadata: &sessionMetadata{id: "fixture-session"},
			}
			err := session.Close()
			if (err != nil) != failure {
				t.Fatalf("termination failure must be returned: %v", err)
			}
			if !failure {
				if content, err := os.ReadFile(record); err != nil || string(content) != "terminated" {
					t.Fatal("exited forwarding process did not trigger explicit remote cleanup", err)
				}
			}
		})
	}
}
