//go:build !windows

package runtime

import (
	"os/exec"
	"syscall"
)

func prepareChildProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func gracefulStop(pid int) error {
	return syscall.Kill(-pid, syscall.SIGINT)
}

func forceKill(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
