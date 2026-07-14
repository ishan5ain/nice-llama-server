Now I have all the information needed. Let me compile the comprehensive review report.

---

## Review Report: Wave R3 — Final Comprehensive Review

### Build & Verification Results

| Command | Result |
|---------|--------|
| `go build ./cmd/nice-llama-server/` | ✅ Passed (clean) |
| `go vet ./internal/tui/...` | ✅ Passed (clean) |
| `go test ./internal/tui/...` | ✅ Passed (cached) |

---

### 1. MVU Discipline — ✅ Pass

- Every `Update` call reassigns the model: `m.component, cmd = m.component.Update(msg)` pattern is used consistently for `logView`, `modelList`, `editor.name`, `editor.args`, `ac`, and `deleteDialog`.
- All returned `Cmd`s are collected and batched via `tea.Batch(...)` or appended via `cmds = append(cmds, cmd)`.
- No `return m, nil` where a command should be returned. Synchronous operations (scrolling, focus navigation, editor mutations) correctly return nil commands.
- `saveEditor()` correctly returns `saveBookmarkCmd(...)`.

### 2. Focus Wiring — ⚠️ Issue Found

**✅ Correct:**
- Tab/Shift+Tab routes through `focus.Manager` and `focus.Scope` correctly in `handleKey()`.
- `Focus()` is called when a component becomes active (e.g., `m.editor.name.Focus()`, `m.ac.Focus()`).
- `Blur()` is called when appropriate (`m.ac.Blur()`).
- Focused components show visual focus state via `panelStyleFor(focused)` using `BorderFocused` / `Success` border colors.
- Hidden controls (autocomplete when not visible) are not in the focus cycle — autocomplete is only registered when editor scope is active.

**❌ Focus Drift in `applyFocus()`** (`internal/tui/update.go:355-357`):

```go
} else if m.editor != nil {
    m.editorScope.Enter(m.fm)
    m.fm.Apply(&m.tabs, &m.modelList)   // ← BUG: focuses modelList even when scope active
    m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
}
```

When `applyFocus()` is called with `m.editor != nil` and the scope is inactive (from the `else` branch of tab/shift+tab), `Enter` activates the scope, then `m.fm.Apply(&m.tabs, &m.modelList)` calls `Focus()` on `modelList` (the parent manager's current index), while `m.editorScope.Apply(...)` calls `Focus()` on `editor.name` (scope index 0). **Two components receive `Focus()` simultaneously**, causing focus state drift.

The fix: Replace `m.fm.Apply(&m.tabs, &m.modelList)` with `m.editorScope.ApplyBackground(m.fm, &m.tabs, &m.modelList)`. The `Scope.ApplyBackground` method (defined in `focus/scope.go:66-72`) blurs all background items when the scope is active, preventing the drift.

**Note — Autocomplete focus cycle:** The autocomplete (scope slot 2) is never reached via tab navigation from args (slot 1) — tab on args triggers arg completion instead. This is a deliberate design choice, but means slot 2 is only accessible programmatically via `m.ac.Focus()`.

### 3. Sizing Contracts — ⚠️ Note

Components are sized **during render**, not eagerly on `WindowSizeMsg`:

| Component | Sized in | Location |
|-----------|----------|----------|
| `logView` | `renderLogView()` | `render.go:147` |
| `ac` | `renderArgsEditorLines()` | `render.go:313-314` |
| `deleteDialog` | `render()` | `render.go:33` |
| `editor.name` | `renderDetailLines()` | `render.go:289` |
| `editor.args` | `renderArgsEditorLines()` | `render.go:310` |

Only `footer`, `tabs`, and `modelList` are sized in `WindowSizeMsg` (`model.go:137-141`). While this works functionally (components are sized before rendering), it deviates from the strict contract "Every component has `SetSize(width, height)` called on `WindowSizeMsg`". No component measures the terminal directly.

### 4. Mouse Routing — ✅ Pass

```go
func (m *model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
    if m.tabs.SelectedID() != "logs" {
        return m, nil   // Ignored when logs view not visible
    }
    m.followTailEnabled = false
    var cmd tea.Cmd
    m.logView, cmd = m.logView.Update(msg)  // Routed to viewport
    return m, cmd
}
```

- Mouse wheel events reach `viewport.Update()` when the logs tab is selected. ✅
- Wheel events are ignored when the logs view is not visible. ✅

### 5. Color Hygiene — ✅ Pass

```bash
$ grep -rn 'lipgloss.Color("#[0-9A-Fa-f]' internal/tui/
# Zero hits
$ grep -rn 'Color("#' internal/tui/
# Zero hits
$ grep -rn '"#' internal/tui/
# Zero hits
```

All foreground/background colors reference theme roles (e.g., `theme.Accent`, `theme.Border`, `theme.Success`, `theme.TextMuted`, `theme.Danger`, `theme.Warning`, `theme.TextFaint`). No hardcoded hex colors anywhere in `internal/tui/`.

### 6. Dead Code — ✅ Pass

- `grep -rn 'focusArea\|bottomView\|confirmDelete\|textBuffer' internal/tui/` — zero hits.
- `followTailEnabled` is actively used in `model.go`, `update.go`, and `render.go` (not dead).
- All imports verified as used across all files.

### 7. Scrollbar Integration — ✅ Pass

```go
// render.go:154
bar := scrollbar.For(m.theme, m.logView)
viewContent := lipgloss.JoinHorizontal(lipgloss.Top, m.logView.View(), bar)
```

Scrollbar is rendered alongside the log viewport using `scrollbar.For(theme, viewport)` which returns a string. One column is reserved for the scrollbar (`render.go:145`).

---

### Test Coverage Assessment

**30+ tests** covering:
- Focus state transitions (enter/exit editor, tab navigation, esc)
- Tab completion (forward/backward cycling, prefix filtering, mmproj values)
- Passive arg suggestions (single/double hyphen, popularity sorting)
- Paste handling (CRLF normalization, ANSI stripping, name vs args)
- Undo (Ctrl+Z)
- Rendering (header height, footer context, bookmark editor dimensions)
- Slash toggle between bookmark/log views
- Quit behavior (plain q vs Ctrl+Q)

Tests are thorough and cover edge cases well.

---

### Recommendations

1. **Fix focus drift** in `internal/tui/update.go:356`: Replace `m.fm.Apply(&m.tabs, &m.modelList)` with `m.editorScope.ApplyBackground(m.fm, &m.tabs, &m.modelList)` to prevent dual-focus when the editor scope is active.

2. **(Optional)** Move eager sizing to `WindowSizeMsg` for `logView`, `ac`, and `deleteDialog` to satisfy the strict sizing contract. Currently they are sized lazily during render, which works but deviates from convention.

---