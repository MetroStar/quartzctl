//go:build !windows

package privateaccess

import (
	"errors"
	"os/exec"
	"syscall"
)

func configureProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalProcess(command *exec.Cmd, signal syscall.Signal) error {
	if command.Process != nil {
		err := syscall.Kill(-command.Process.Pid, signal)
		if !errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	return nil
}

func terminateProcess(command *exec.Cmd) error {
	return signalProcess(command, syscall.SIGTERM)
}

func forceProcess(command *exec.Cmd) error {
	return signalProcess(command, syscall.SIGKILL)
}
