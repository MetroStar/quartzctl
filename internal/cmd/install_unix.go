//go:build !windows

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// minFreeSpace finds the log directory with the least free disk space.
// Unix/Linux/Darwin implementation using syscall.Statfs.
func minFreeSpace(paths []string) (string, uint64, error) {
	var minPath string
	var minFree uint64
	for _, path := range paths {
		probe := path
		for {
			if _, err := os.Stat(probe); err == nil {
				break
			} else if os.IsNotExist(err) {
				next := filepath.Dir(probe)
				if next == probe {
					break
				}
				probe = next
			} else {
				return "", 0, err
			}
		}
		var stat syscall.Statfs_t
		if err := syscall.Statfs(probe, &stat); err != nil {
			return "", 0, err
		}
		free := stat.Bavail * uint64(stat.Bsize) //nolint:gosec
		if minPath == "" || free < minFree {
			minPath = probe
			minFree = free
		}
	}
	if minPath == "" {
		return "", 0, fmt.Errorf("no log paths to inspect")
	}
	return minPath, minFree, nil
}
