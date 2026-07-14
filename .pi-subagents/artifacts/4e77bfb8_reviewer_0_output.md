Here is the complete review report.

---

## Review: nice-llama-server gotui migration against library conventions

### 🔴 Blocking

**B1 — Autocomplete `SelectedMsg` handler checks wrong scope index (model.go:204)**

The `autocomplete.SelectedMsg` handler guards on `m.editorScope.Index() == 2`, but when autocomplete is activated from the args field via `handleArgCompletionTab` (args_completion.go:49-53), the scope index remains **1** (args). The function calls `m.ac.Focus()` directly without advancing the scope index to 2. This means every autocomplete selection is silently dropped — the `ReplaceRange` call inside the guard is never reached.

**Impact:** Users can see autocomplete suggestions and navigate them, but pressing enter to accept a suggestion has no effect. The completion is lost.

**Fix:** Change the guard to `m.editorScope.Index() == 1` (or remove the scope-index check entirely since autocomplete is only active when triggered from args):

```go
// model.go:204
case autocomplete.SelectedMsg:
    if m.ac.Focused() && m.editor != nil && m.editorScope.Active() {
        m.editor.args.ReplaceRange(
            textarea.Position{Row: m.editor.completion.row, Column: m.editor.completion.start},
            textarea.Position{Row: m.editor.completion.row, Column: m.editor.completion.end},
            msg.Value,
        )
        m.editor.completion = argCompletionState{}
        m.ac.Blur()
    }
    return m, nil
```

---

**B2 — Hardcoded ANSI color literal in `listItemsFlat` (selection.go:88)**

```go
flat = append(flat, "\033[1;38;2;122;162;247m▶ "+label+"\033[0m")
```

This embeds a literal RGB color (122, 162, 247) as a raw ANSI escape sequence instead of deriving it from `tuiweave.Theme` roles. Violates **Rule 1** ("Colors come from Theme roles — never literals").

**Impact:** The group-header accent color is fixed and does not respond to theme changes. If the user switches to a light theme or a different preset, the color remains the same blue tone.

**Fix:** Use `lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render(...)` to style the group header through the theme:

```go
// selection.go:86-89
case listItemModelGroup:
    label := item.label
    if item.degraded {
        label += " (missing)"
    }
    style := lipgloss.NewStyle().Bold(true).Foreground(m.theme.Accent)
    flat = append(flat, style.Render("▶ "+label))
```

---

### 🟡 Advisory

**A1 — Magic `width+6` in `renderBottom` (render.go:83)**

```go
content = m.renderBookmarkEditorView(width+6, contentHeight)
```

The `+6` is a magic-number compensation that makes the bookmark-editor splitpane wider than the surrounding chrome (header, footer, tabs). The rendered bottom section ends up 6 columns wider than the header/footer, causing horizontal misalignment when the bookmarks tab is active.

**Recommendation:** Remove the `+6` and adjust the panel sizing to fit naturally within the available width. The splitpane with `Gap: 1` and two bordered panels should fit within `width` without extra padding.

---

**A2 — `SetSize` called inconsistently on `WindowSizeMsg` (model.go:136-140)**

Only `m.footer`, `m.tabs`, and `m.modelList` receive `SetSize` on `WindowSizeMsg`. The viewport (`m.logView`), textinput (`m.editor.name`), textarea (`m.editor.args`), and autocomplete (`m.ac`) are sized only in render functions. Per the convention ("Every bounded component sizes itself only via `SetSize(w, h)`"), all bounded components should be sized on `WindowSizeMsg`.

Additionally, `m.modelList.SetSize(msg.Width/2, msg.Height-10)` hardcodes a 50% width and a magic height, but the actual layout uses `splitpane.Horizontal` with `Ratio: 40` (40% list). The WindowSizeMsg sizing is immediately overridden by `renderModelListPanel`, making it dead code.

**Recommendation:** Either size all bounded components in `WindowSizeMsg` using proper layout calculations, or remove the premature `SetSize` calls and let the render functions own sizing entirely. The latter is simpler and avoids duplication.

---

**A3 — Dead branch in `applyFocus` (update.go:328-331)**

```go
} else if m.editor != nil {
    m.editorScope.Enter(m.fm)
    m.editorScope.ApplyBackground(m.fm, &m.tabs, &m.modelList)
    m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
}
```

This branch is unreachable because `applyFocus()` is only called when `!m.editorScope.Active()` (update.go:63), and whenever `m.editor` is set, `m.editorScope.Enter()` is also called (making it active). Conversely, when the scope is exited, `m.editor` is set to nil. So `m.editor != nil` implies `m.editorScope.Active()` is true, making this branch dead code.

**Recommendation:** Remove the dead branch or convert it to a defensive assertion.

---

**A4 — Inconsistent focus application pattern**

When entering the editor, `beginEditSelected` (selection.go:130-135) calls:
```go
m.editorScope.Enter(m.fm)
m.editor.name.Focus()
```

But `applyFocus` (update.go:328-331, dead branch) and `handleDetailKey` (update.go:257-259) use:
```go
m.editorScope.Next()
m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
```

The first pattern manually focuses the name field without applying blur to args/ac. The second uses `Apply` which handles focus/blur for all three components. These should be consistent.

**Recommendation:** Standardize on the `Apply` pattern everywhere, including in `beginEditSelected`, `newBookmarkForCurrentGroup`, and `cloneSelectedBookmark`.

---

### ⚪ Info

| Check | Status | Evidence |
|---|---|---|
| No `lipgloss.Color("#...")` literals | ✅ Pass | Grep returned no matches |
| No ultraviolet imports in app code | ✅ Pass | Grep returned no matches |
| No dead types (`focusArea`, `bottomView`, etc.) | ✅ Pass | Grep returned no matches |
| MVU discipline (model reassigned, cmd collected) | ✅ Pass | Every `Update` call reassigns model and collects cmd |
| `dialog.Body` used (not `Message`) | ✅ Pass | `m.deleteDialog.Body = ...` at update.go:115 |
| `autocomplete.SetItems` used (not `SetSuggestions`) | ✅ Pass | `m.ac.SetItems(items...)` at args_completion.go:50 |
| `scrollbar.For` returns string (not a Model) | ✅ Pass | `scrollbar.For(m.theme, m.logView)` at render.go:155 |
| `focus.Manager` is a value type | ✅ Pass | `fm focus.Manager` in model struct |
| `focus.Scope` value-type semantics | ✅ Pass | `editorScope focus.Scope` in model struct |
| Wheel events reach viewport even when blurred | ✅ Pass | `handleMouseWheel` forwards unconditionally (no Focused check) |
| Wheel ignored when logs view not visible | ✅ Pass | Guarded by `m.tabs.SelectedID() != "logs"` |
| Build succeeds | ✅ Pass | `go build ./cmd/nice-llama-server/` exits clean |
| Vet passes | ✅ Pass | `go vet ./internal/tui/...` exits clean |
| All tests pass | ✅ Pass | 47/47 tests pass |
| No staged files | ✅ Pass | `git status --short` shows no staged changes |

---

### Commands run

```bash
go build ./cmd/nice-llama-server/        # BUILD OK
go vet ./internal/tui/...                # VET OK
go test ./internal/tui/... -count=1      # TESTS OK (47/47)
grep -rn 'lipgloss.Color("#' internal/tui/  # No color literals
grep -rn 'ultraviolet' internal/tui/     # No UV imports
grep -rn 'focusArea\|bottomView\|confirmDelete\|textBuffer' internal/tui/  # No dead types
grep -rn '\.SetSize(' internal/tui/*.go  # 10 SetSize calls audited
```

### Residual risks

1. The `autocomplete.SelectedMsg` bug (B1) means arg completion via autocomplete is completely broken — users see suggestions but cannot accept them. This is the highest-priority fix.
2. The hardcoded ANSI color (B2) breaks theme switching for group headers.
3. The `width+6` magic number (A1) causes visual misalignment on the bookmarks tab.
4. No snapshot tests exist for this app (the app uses string-based tests, not `snaptest` goldens). This is acceptable per the AGENTS.md note ("this app uses string-based tests, not snaptest").

---