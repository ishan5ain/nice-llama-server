# App Agent Instructions

This is a [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) application
built with [gotui](https://github.com/ishansain/gotui).

## Guardrail — confirm destructive actions before executing

Before running any command that modifies or destroys state — including
git revert, reset, push --force, branch delete, rm -rf, kill, mv
overwrite, or similar — you MUST:

1. Pause and inspect what you're targeting.
2. State what you're about to do and why.
3. Wait for explicit user approval before executing.

## Before coding

1. Read the gotui `AGENT-CATALOG.md` and the relevant recipe in gotui `AGENTS.md`.
2. Start from the closest gotui example.
3. Use `gotui.Theme` roles; never literal colors.
4. Size components only through `SetSize(width, height)`.
5. Reassign models and collect commands from every `Update` call.
6. Use `layout.Rect` rectangles for all sizing.
7. Add `snaptest` goldens and scenario tests for rendering or interaction changes.

## Canonical gotui examples

| Pattern | Example |
|---|---|
| App shell | `examples/demo` |
| Multi-pane app | `examples/ops` |
| File browser | `examples/browser` |
| Agent UI | `examples/chat` |
| Command palette | `examples/palette` |
| Framing + tabs + controls | `examples/frame` |
| Table + diff + scrollbars | `examples/table` |

## gotui component mapping (this app)

| App feature | gotui component | Notes |
|---|---|---|
| Header panel | `gotui/frame.Panel` + `gotui/stack.Vertical` | Title, status, stats |
| Footer / key hints | `gotui/statusbar.Model` | Left = keybindings, Right = tail indicator |
| Bottom view switch | `gotui/tabs.Model` | Bookmarks / Logs |
| Log view | `gotui/viewport.Model` | Pre-format lines; keep horiz scroll in app |
| Model list | `gotui/list.Model` | Flatten hierarchy: prefix groups with `▶`, bookmarks with `▸` |
| Bookmark name input | `gotui/textinput.Model` | Single-line, prompt "> " |
| Bookmark args editor | `gotui/textarea.Model` | Multi-line, undo/redo, soft-wrap |
| Panel decorations | `gotui/frame.Panel` | Rounded borders, title, focus styling |
| Horizontal split | `gotui/splitpane.Horizontal` | List + detail panes |
| Vertical layout | `gotui/stack.Vertical` | Header + body + footer |
| Colors | `gotui.Theme` (18 semantic roles) | Never hardcode hex |
| Focus management | `gotui/focus.Manager` | Tab-order cycling |
| Delete confirmation | `gotui/dialog.Model` | Centered modal |
| Arg completion UI | `gotui/autocomplete.Model` | Suggestion window |
| Scroll indicator | `gotui/scrollbar.Model` | Next to log viewport |
| Key hint bar | `gotui/help.Model` | Compact one-line hints |

## State ownership

- **gotui components own:** rendering, input handling, scrolling, focus, selection, text editing.
- **App owns:** controller client, HTTP polling, bookmark CRUD, llama.cpp arg catalog, completion
  engine, token scanning, clipboard integration.

## Verification

```bash
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/...
```

Do not regenerate goldens without reviewing the diff.
