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

## State Storage

By default, state lives under the user config directory:
- macOS: `~/Library/Application Support/nice-llama-server`
- Windows: `%AppData%\nice-llama-server`

The controller stores:
- `state.json`: bookmarks and config
- `controller.json`: active controller discovery info for the TUI

## TUI Usage

### Main view

- `n`: create bookmark
- `c`: clone selected bookmark
- `e`: edit selected bookmark
- `d`: delete selected bookmark
- `L`: load selected bookmark
- `U`: unload active runtime
- `g`: toggle log panel
- `r`: rescan model roots
- `q`: quit

### Editor

The editor has three fields:
- bookmark name
- model name
- raw `llama-server` args

Key behavior:
- `Ctrl+N`: move to next field
- `Ctrl+P`: move to previous field
- `Tab` in the model field: autocomplete from discovered model names
- `Ctrl+S`: save bookmark
- `Ctrl+L`: save bookmark and start loading it
- `Esc`: cancel editing

### Model selection behavior

You do not need to type the full absolute GGUF path in the editor.

The model field accepts a discovered model name such as:

```text
gemma-3-4b-it-Q4_K_M
```

The TUI resolves that to the full GGUF path in the background when saving.

## Bookmark Format

Each bookmark stores:
- name
- resolved full model path
- free-form multiline args text

The model field in the TUI is filename-oriented, but the saved bookmark still stores the resolved absolute path internally.

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
