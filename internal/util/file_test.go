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
