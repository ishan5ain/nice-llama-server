//go:build windows

package app

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func launchDetachedController(executable string, opts cliOptions) error {
	args := buildControllerArgs(opts)
	psArgs := make([]string, 0, len(args))
	for _, arg := range args {
		psArgs = append(psArgs, "'"+escapePowerShellSingleQuoted(arg)+"'")
	}

	command := strings.Join([]string{
		"$exe = '" + escapePowerShellSingleQuoted(executable) + "'",
		"$wd = '" + escapePowerShellSingleQuoted(filepath.Dir(executable)) + "'",
		"$args = @(" + strings.Join(psArgs, ", ") + ")",
		"Start-Process -FilePath $exe -WorkingDirectory $wd -ArgumentList $args -WindowStyle Hidden",
	}, "; ")

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", command)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func escapePowerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
