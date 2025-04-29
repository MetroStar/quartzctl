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
