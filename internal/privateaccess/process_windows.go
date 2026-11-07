package privateaccess

import (
	"os/exec"
	"strconv"
)

func configureProcess(command *exec.Cmd) {}

func terminateProcess(command *exec.Cmd) error {
	if command.Process != nil {
		return exec.Command("taskkill", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F").Run()
	}
	return nil
}

func forceProcess(command *exec.Cmd) error {
	return terminateProcess(command)
}
