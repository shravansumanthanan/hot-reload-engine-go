//go:build !windows

package process

import (
	"context"
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcess(cmd *exec.Cmd) error {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Kill()
	}
	return syscall.Kill(-pgid, syscall.SIGTERM)
}

func killProcess(cmd *exec.Cmd) error {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return cmd.Process.Kill()
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}

func getShellCmd(cmdStr string) *exec.Cmd {
	/* #nosec G204 */
	return exec.Command("sh", "-c", cmdStr)
}

func getShellCmdContext(ctx context.Context, cmdStr string) *exec.Cmd {
	/* #nosec G204 */
	return exec.CommandContext(ctx, "sh", "-c", cmdStr)
}
