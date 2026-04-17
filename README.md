# nice-llama-server

Lightweight TUI wrapper around `llama-server` for managing local GGUF model launch commands.

The current MVP is built for a practical workflow:
- develop and iterate on macOS
- run the compiled binary on a Windows host with the real GPU and model files
- SSH into that Windows host and use the TUI there
- use `psmux` for session persistence when SSH drops

## What It Does

The app manages one active `llama-server` instance at a time.

It provides:
- bookmark-style saved launch configurations
- recursive GGUF model discovery from one or more model roots
- model-name autocomplete in the TUI
- load / unload lifecycle management for a single active model
- live log streaming in a toggleable panel
- persistent local state for bookmarks and configuration

## Current Architecture

The binary has two modes:
- default mode: attach to or start a local background controller, then open the TUI
- `controller` mode: run only the controller HTTP service

The controller is responsible for:
- storing bookmark and config state
- scanning model roots for `.gguf` files
- launching `llama-server`
- tracking runtime state
- buffering logs for reconnects

The TUI is a client for that controller.

### Controller and TUI lifecycle

On first launch in default mode, the app effectively brings up both parts of the system:
- it checks for a healthy controller using the local `controller.json`
- if no healthy controller is running, it launches one in the background
- once the controller is reachable, it opens the TUI in the foreground and connects to it

So the normal command:

```bash
nice-llama-server [model-dir...]
```

means:
- background controller process
- foreground TUI process

If a controller is already running and healthy, the command only opens the TUI and reuses that controller.

### What the controller exposes

The controller is a local HTTP API with auth token protection. The TUI talks to it for:
- state and bookmark CRUD
- rescans
- model load and unload
- log polling

The controller currently binds to loopback only (`127.0.0.1`), so it is intentionally local to the machine where it runs.

## Current Scope

Implemented:
- one active `llama-server` process at a time
- bookmark CRUD
- filename-based model selection
- manual free-form args editing
- health-check-based readiness detection
- graceful unload with force-kill fallback
- macOS development workflow
- Windows cross-build and Windows runtime validation

Not implemented yet:
- router mode
- concurrent model instances
- Hugging Face browsing / downloads
- hardware-aware recommendations
- full native Windows session persistence strategy

For now, `psmux` is the recommended solution for SSH session persistence.

## Requirements

- Go 1.26+
- a working `llama-server` binary on the target host
- one or more directories containing `.gguf` files

Validated target workflow:
- macOS development machine
- Windows 11 host running `llama-server`
- SSH access from macOS to Windows

## Build

### Local build

```bash
go build ./cmd/nice-llama-server
```

### Build a Windows binary from macOS or Linux

```bash
./scripts/build-windows.sh
```

Default output:

```text
dist/nice-llama-server-windows-amd64.exe

### Build a macOS binary from macOS host

```bash
./scripts/build-macos.sh
```

Default output:

```text
# The script automatically detects the host architecture and names the file accordingly.
# Example: dist/nice-llama-server-macos-amd64
#          dist/nice-llama-server-macos-arm64
```

You can also specify a custom output path and/or architecture:

```bash
./scripts/build-macos.sh ./dist/custom-name
# or
./scripts/build-macos.sh ./dist/custom-name arm64
```

You can also choose a custom output path:

```bash
./scripts/build-windows.sh ./dist/custom-name.exe
```

## Copy the Windows Build to the Host

The repo includes a helper script for SCP transfer:

```bash
./scripts/scp-windows-build.sh
```

The script reads `REMOTE_HOST` and `REMOTE_USER` from the environment.
If a repo-root `.env` file exists, it is loaded automatically first.

The script removes an existing remote file at that path before uploading the new build.

Example `.env`:

```bash
REMOTE_HOST=192.168.1.1
REMOTE_USER=username
```

The remote destination path defaults to:

```text
~/nice-llama-server-windows-amd64.exe
```

You can also set everything explicitly without using `.env`:

```bash
REMOTE_HOST=your-host-or-ip \
REMOTE_USER=your-user \
REMOTE_PATH='~/nice-llama-server.exe' \
./scripts/scp-windows-build.sh
```

Or pass explicit paths:

```bash
./scripts/scp-windows-build.sh ./dist/nice-llama-server-windows-amd64.exe '~/nice-llama-server.exe'
```

Note:
- `.env` is gitignored so personal host details do not get published
- `REMOTE_PATH` remains optional

## Running the App

### Typical Windows-host usage

SSH into the Windows host and run:

```powershell
.\nice-llama-server-windows-amd64.exe `
  --llama-server-bin "C:\path\to\llama-server.exe" `
  "C:\path\to\models"
```

This starts the background controller if needed, then opens the TUI and attaches to it.

You can pass multiple model roots:

```powershell
.\nice-llama-server-windows-amd64.exe `
  --llama-server-bin "C:\path\to\llama-server.exe" `
  "D:\models" `
  "E:\more-models"
```

### CLI shape

```text
nice-llama-server [model-dir...]
nice-llama-server controller [model-dir...]
```

Supported flags:
- `--llama-server-bin <path>`: override the `llama-server` executable path
- `--state-dir <path>`: override the app state directory
- `--controller-url <url>`: attach directly to an existing controller
- `--controller-token <token>`: authenticate to an existing controller directly

## Running Only the Controller

If you want to manage the controller lifecycle yourself, run the `controller` subcommand:

```bash
./dist/nice-llama-server controller \
  --llama-server-bin /opt/homebrew/bin/llama-server \
  ~/.cache/huggingface/hub
```

On Windows:

```powershell
.\nice-llama-server.exe controller `
  --print-controller-info `
  --llama-server-bin "C:\path\to\llama-server.exe" `
  "C:\path\to\models"
```

That starts only the controller. No TUI is opened in this mode.

If you pass `--print-controller-info`, the controller prints a one-line attach hint after startup:

```text
url=http://127.0.0.1:51234 token=your-token
```

## Running Only the TUI

To run only the TUI, point it at an already running controller:

```bash
./dist/nice-llama-server \
  --controller-url http://127.0.0.1:51234 \
  --controller-token your-token
```

This works when:
- a controller is already running
- the TUI can reach that controller URL
- the TUI also has a usable controller token

If `--controller-token` is omitted, the app will still try to load a matching token from local `controller.json` when the stored URL matches exactly. If that fails, startup now exits with a clear error instead of trying an unauthenticated connection.

## State Storage

By default, state lives under the user config directory:
- macOS: `~/Library/Application Support/nice-llama-server`
- Windows: `%AppData%\nice-llama-server`

The controller stores:
- `state.json`: bookmarks and config
- `controller.json`: active controller discovery info for the TUI

`controller.json` contains:
- controller URL
- auth token
- controller PID
- controller start time

The TUI uses this file to discover and authenticate to an already running controller.

## Remote Host Controller With Local TUI

This is possible, but there is an important constraint: the controller binds to `127.0.0.1` on the remote host. That means your local TUI cannot talk to it directly over the network.

To do this today, you need:
- a controller running on the remote host
- an SSH tunnel from your local machine to the remote controller port
- the remote controller token

### Recommended simple workflow

The simplest validated workflow is still:
- SSH into the remote host
- run the normal app there
- keep the session alive with `psmux`

### Advanced workflow: remote controller, local TUI

1. Start only the controller on the remote host:

```powershell
.\nice-llama-server.exe controller `
  --print-controller-info `
  --llama-server-bin "C:\path\to\llama-server.exe" `
  "C:\path\to\models"
```

2. Note the printed controller URL and token, or read them from the remote `%AppData%\nice-llama-server\controller.json`.

3. Create an SSH tunnel from your local machine to the remote controller port:

```bash
ssh -L 51234:127.0.0.1:51234 user@remote-host
```

4. Start only the local TUI against that tunneled controller:

```bash
./dist/nice-llama-server \
  --controller-url http://127.0.0.1:51234 \
  --controller-token your-token
```

The app does not manage the SSH tunnel in this workflow. You create and keep the tunnel alive separately.

## TUI Usage

### Main view

- `/`: toggle between Bookmark Editor view and Log view
- `Ctrl+Q`: quit
- `n`: create bookmark
- `c`: clone selected bookmark
- `e`: edit selected bookmark
- `d`: delete selected bookmark
- `r`: rescan model roots
- `L`: load selected bookmark
- `U`: unload active runtime
- `Enter`: move from bookmark name to args, or insert newline in args
- `Tab` / `Shift+Tab`: complete or cycle `llama-server` args while editing args
- `Ctrl+S`: save edits
- `Esc`: discard edits and return focus to the list

### Editor

The bookmark detail pane has two editable fields:
- bookmark name
- raw `llama-server` args

Key behavior:
- `e`: enter edit mode with focus on bookmark name
- `Up` / `Down`: navigate the list or move between bookmark name and args
- `Enter`: move from name to args, or insert a newline in args
- `Tab` / `Shift+Tab`: autocomplete the current args token from the tracked `llama-server` parameter catalog; repeated presses cycle matching options forward or backward
- `Ctrl+S`: save changes and return focus to the list
- `Esc`: discard unsaved changes and return focus to the list

## Bookmark Format

Each bookmark stores:
- name
- resolved full model path
- free-form multiline args text

The TUI groups bookmarks by discovered model, but bookmark storage still keeps the resolved absolute GGUF path internally.

The args editor expects one argument fragment per line, for example:

```text
--ctx-size 8192
--host 0.0.0.0
--port 8080
--n-gpu-layers 999
--flash-attn on
```

The app rejects `-m` / `--model` in bookmark args because model selection is controller-owned.

## Notes on Persistence

The app includes a detached controller process model, but for practical SSH session persistence the currently validated approach is to run the app inside `psmux`.

That is the recommended workflow today if you want the session to survive SSH disconnects cleanly.

## Development

### Run tests

```bash
go test ./...
```

### Cross-build for Windows

```bash
GOOS=windows GOARCH=amd64 go build ./...
```

## Validation Status

Validated:
- macOS development build/test workflow
- Windows cross-build
- Windows host manual load/unload flow
- editor fixes and model-name autocomplete
- SSH persistence workflow using `psmux`

Still intended for iteration:
- richer UX polish
- better visual hierarchy
- more guided launch configuration

## Roadmap

Likely next phase:
- improve UX and interaction patterns
- better model browsing within discovered roots
- better bookmark editing ergonomics
- more runtime details in the log / status views
- future support for multiple instances and downloads
