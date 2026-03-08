//go:build windows

package process

import (
	"context"
	"os/exec"
	"strconv"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// Not creating process groups in the same way on Windows
}

func terminateProcess(cmd *exec.Cmd) error {
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	return kill.Run()
}

func killProcess(cmd *exec.Cmd) error {
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	return kill.Run()
}

func getShellCmd(cmdStr string) *exec.Cmd {
	return exec.Command("cmd", "/c", cmdStr)
}

func getShellCmdContext(ctx context.Context, cmdStr string) *exec.Cmd {
	return exec.CommandContext(ctx, "cmd", "/c", cmdStr)
}
