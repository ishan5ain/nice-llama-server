//go:build windows

package runtime

import (
	"os/exec"
	"strconv"
	"syscall"
)

const (
	ctrlBreakEvent        = 1
	createNewProcessGroup = 0x00000200
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGenerateConsoleEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")
)

func prepareChildProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewProcessGroup,
	}
}

func gracefulStop(pid int) error {
	r1, _, err := procGenerateConsoleEvent.Call(uintptr(ctrlBreakEvent), uintptr(uint32(pid)))
	if r1 == 0 {
		return err
	}
	return nil
}

func forceKill(pid int) error {
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	return cmd.Run()
}
