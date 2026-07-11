# Plan: Detached Controller Launch on Windows

## Problem

When SSH-ing into a Windows PC and launching `nice-llama-server`, both the TUI and
controller processes terminate when the SSH connection drops or the terminal window
closes. The controller is launched via `Start-Process -WindowStyle Hidden`, which
hides the window but keeps the process in the same console session. When the console
closes, Windows sends a console close event to all processes in that session.

The TUI terminating is expected (it's a terminal-attached app). The controller
should survive — it needs to be truly detached from the console session.

## Solution

Replace the PowerShell `Start-Process` approach in `internal/app/launch_windows.go`
with a direct `CreateProcessW` call using the `DETACHED_PROCESS` (0x00000008)
creation flag. This creates a process with no console and no session inheritance.

## Dependency Check

`golang.org/x/sys/windows` is already an indirect dependency (`v0.42.0` in `go.mod`).
No new dependency required.

## Tasks

### Task 1: Rewrite `internal/app/launch_windows.go`

Replace the PowerShell-based approach with a direct `CreateProcessW` call.

**Before (current):**
```go
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
```

**After (target):**
```go
//go:build windows

package app

import (
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func launchDetachedController(executable string, opts cliOptions) error {
	args := buildControllerArgs(opts)
	cmdLine := buildCmdLine(executable, args)

	cmdLineUTF16, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return err
	}

	dirUTF16, err := windows.UTF16PtrFromString(filepath.Dir(executable))
	if err != nil {
		return err
	}

	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW
	si.ShowWindow = 0 // SW_HIDE
	si.HStdInput = windows.InvalidHandle
	si.HStdOutput = windows.InvalidHandle
	si.HStdError = windows.InvalidHandle

	var pi windows.ProcessInformation
	err = windows.CreateProcess(
		nil,
		cmdLineUTF16,
		nil,   // process attributes
		nil,   // thread attributes
		false, // inherit handles — must be false for DETACHED_PROCESS
		windows.DETACHED_PROCESS,
		nil,   // environment
		dirUTF16,
		&si,
		&pi,
	)
	if err != nil {
		return wrapErr("launch controller", err)
	}

	// Close handles immediately — the process is orphaned and doesn't need them
	windows.CloseHandle(pi.HProcess)
	windows.CloseHandle(pi.HThread)
	return nil
}

// buildCmdLine constructs a Windows command line string from executable + args.
// Windows expects a single string where the executable path is quoted, followed
// by each argument quoted individually (handles spaces, special chars).
func buildCmdLine(executable string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, "\""+executable+"\"")
	for _, a := range args {
		parts = append(parts, "\""+a+"\"")
	}
	return joinCmdLine(parts)
}

func joinCmdLine(parts []string) string {
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " " + parts[i]
	}
	return result
}
```

**Key implementation details:**
- `DETACHED_PROCESS` — creates a process with no console, no session inheritance
- `false` for `bInheritHandles` — required; DETACHED_PROCESS fails if inherit handles is true
- `InvalidHandle` for std handles — routes stdin/stdout/stderr to NUL
- `CloseHandle` on both `HProcess` and `HThread` — critical for full orphaning
- `SW_HIDE` — extra safety to suppress any window creation attempt
- `STARTF_USESTDHANDLES | STARTF_USESHOWWINDOW` — tells CreateProcess to use the handle and show-window values from STARTUPINFO
- Remove `escapePowerShellSingleQuoted` — no longer needed
- Remove `os/exec` and `strings` imports, add `golang.org/x/sys/windows` and `unsafe`

**Verification:** `GOOS=windows GOARCH=amd64 go build ./cmd/nice-llama-server/` succeeds

---

### Task 2: Confirm `wrapErr` is available

**File:** `internal/app/run.go:279-284`

`wrapErr` exists in the same package (`app`) with no build tag, so it compiles for
all platforms including Windows. No action needed — just confirm.

---

### Task 3: Verify Unix/Darwin launch files are untouched

**Files:**
- `internal/app/launch_darwin.go` (build tag: `darwin`)
- `internal/app/launch_other_unix.go` (build tag: `!windows && !darwin`)

These are excluded from the Windows build. Confirm they still compile:

```bash
GOOS=darwin go build ./cmd/nice-llama-server/
GOOS=linux go build ./cmd/nice-llama-server/
```

---

### Task 4: Run existing tests

```bash
go test ./...
```

The test suite (`internal/app/run_test.go` and others) doesn't cover
`launchDetachedController` directly, but confirms nothing else broke.

---

### Task 5: Cross-compile verification

Since the build machine is macOS, verify the Windows binary compiles:

```bash
GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/nice-llama-server/
```

## Execution Order

```
Task 1 (rewrite launch_windows.go)
    → Task 2 (confirm wrapErr availability)
    → Task 5 (cross-compile for Windows)
    → Task 3 (verify Unix/Darwin still compile)
    → Task 4 (run go test ./...)
```

Tasks 2-5 are verification gates after Task 1.

## Files Changed

| File | Action |
|------|--------|
| `internal/app/launch_windows.go` | **Rewrite** — replace PowerShell with CreateProcessW |

## Files Unchanged

| File | Reason |
|------|--------|
| `internal/app/run.go` | `launchDetachedController` signature unchanged |
| `internal/app/launch_darwin.go` | Darwin-specific, unaffected |
| `internal/app/launch_other_unix.go` | Unix-specific, unaffected |
| `go.mod` | `golang.org/x/sys` already a dependency |
| `internal/runtime/process_windows.go` | Handles llama-server child processes, not controller launch |

## Acceptance Criteria

1. `GOOS=windows GOARCH=amd64 go build ./cmd/nice-llama-server/` compiles without errors
2. `GOOS=darwin go build ./cmd/nice-llama-server/` compiles without errors
3. `GOOS=linux go build ./cmd/nice-llama-server/` compiles without errors
4. `go test ./...` passes all existing tests
5. On Windows: controller process survives SSH disconnect (manual verification required on a Windows machine)
