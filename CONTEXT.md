# nice-llama-server

A lightweight TUI wrapper around `llama-server` for managing local GGUF model launch
commands. The app manages one active `llama-server` instance at a time through a
background controller and a terminal UI.

## Language

### Core Entities

**Bookmark**:
A saved launch configuration containing a model path, argument flags, and a display name.
_Avoid_: Preset, profile, launch config

**DiscoveredModel**:
A `.gguf` file found during a filesystem scan of the configured model roots. Represents
a model available to load, not a running instance.
_Avoid_: Model file, GGUF file (when the GGUF format is incidental)

**Snapshot**:
A composite read-only view of the entire system state at a point in time: config,
bookmarks, discovered models, and runtime state. Returned by the controller's state
endpoint.
_Avoid_: State dump, status

### System Architecture

**Controller**:
The background HTTP service that owns persistent state (bookmarks, config), manages
the `llama-server` process lifecycle, buffers logs, and serves a local REST API that
the TUI consumes. Binds to loopback only.
_Avoid_: Daemon, backend, server (ambiguous with llama-server)

**Proxy**:
An authenticated HTTP gateway that sits in front of a running `llama-server` instance.
Provides API-key authentication, per-user rate limiting, token budgets, model-access
filtering, and usage logging. Exposes an OpenAI-compatible chat completions API.
_Avoid_: Gateway (acceptable but less precise), middleware, relay

**User** (proxy context):
A consumer of the proxy API, identified by an API key. Has a name, RPM limit, maximum
tokens per request, and a set of allowed model names.
_Avoid_: Client, API consumer

### Lifecycle

**RuntimeState**:
The current lifecycle status of the active `llama-server` process. Transitions through
`idle → loading → ready` on success, or `idle → loading → failed` on error. The
`ActiveBookmarkID` field identifies which bookmark's model is associated with the
current or most recent runtime operation — it persists even after a failure so the UI
can show what was attempted.
_Avoid_: Process state, server status

**Load / Unload**:
The user-facing verbs for starting and stopping the `llama-server` process for a
specific bookmark. "Loading a model" is the user's mental model — the fact that it
spawns an OS process is an implementation detail of the controller.
_Avoid_: Start/stop (implementation detail), activate/deactivate

### Organization

**GroupKey**:
A derived grouping label extracted from a model's filename. Strips the file extension
and shard suffixes (e.g., `-00001-of-00005`) to produce a logical group name like
`gemma-3-4b-it-Q4_K_M`. Used to organize bookmarks and discovered models into
collapsible groups in the TUI.
_Avoid_: Category, tag, folder
