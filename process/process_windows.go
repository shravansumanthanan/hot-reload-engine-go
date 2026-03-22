//go:build windows

package process

import (
	"context"
	"os/exec"
	"strconv"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// On Windows, CREATE_NEW_PROCESS_GROUP allows us to send signals to the process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func terminateProcess(cmd *exec.Cmd) error {
	// Use taskkill with /T flag to kill the process tree
	kill := exec.Command("taskkill", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	return kill.Run()
}

func killProcess(cmd *exec.Cmd) error {
	// Use taskkill with /T and /F flags to forcefully kill the process tree
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	return kill.Run()
}

func getShellCmd(cmdStr string) *exec.Cmd {
	return exec.Command("cmd", "/c", cmdStr)
}

func getShellCmdContext(ctx context.Context, cmdStr string) *exec.Cmd {
	return exec.CommandContext(ctx, "cmd", "/c", cmdStr)
}
