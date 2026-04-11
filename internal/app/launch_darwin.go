//go:build darwin

package app

import (
	"os"
	"os/exec"
	"syscall"
)

func launchDetachedController(executable string, opts cliOptions) error {
	cmd := exec.Command(executable, buildControllerArgs(opts)...)

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
