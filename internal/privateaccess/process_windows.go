package privateaccess

import (
	"os/exec"
	"strconv"
)

func configureProcess(command *exec.Cmd) {}

func terminateProcess(command *exec.Cmd) {
	if command.Process != nil {
		exec.Command("taskkill", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F").Run()
	}

}

func forceProcess(command *exec.Cmd) {
	terminateProcess(command)
}
