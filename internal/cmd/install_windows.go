//go:build windows

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// minFreeSpace finds the log directory with the least free disk space.
// Windows implementation using syscall.GetDiskFreeSpaceEx.
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

		// Use GetDiskFreeSpaceEx to get free space on Windows
		freeBytesAvailable := uint64(0)
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

		pathPtr, _ := syscall.UTF16PtrFromString(probe)
		_, _, err := getDiskFreeSpaceEx.Call(
			uintptr(unsafe.Pointer(pathPtr)),
			uintptr(unsafe.Pointer(&freeBytesAvailable)),
			uintptr(0),
			uintptr(0),
		)

		if err != nil && err != syscall.Errno(0) {
			return "", 0, err
		}

		if minPath == "" || freeBytesAvailable < minFree {
			minPath = probe
			minFree = freeBytesAvailable
		}
	}

	if minPath == "" {
		return "", 0, fmt.Errorf("no log paths to inspect")
	}

	return minPath, minFree, nil
}
