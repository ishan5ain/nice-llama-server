Now I have a thorough understanding of the entire codebase. Let me compile my findings.

---

## Interaction Bug Audit Report

### Methodology

I traced every key binding path through `handleKey` → `handleLogKey` / `handleListKey` / `handleDetailKey`, verified focus topology against the `focus.Manager` and `focus.Scope` APIs, and inspected state transitions for leaks.

---

### 🔴 Blocking Issues

#### B1. Ctrl+Q bypasses dialog guard

**Key sequence:** `ctrl+q` while dialog is showing  
**What happens:** App quits immediately  
**What should happen:** Dialog should block quit (or at minimum require confirmation)  
**File:** `update.go:10-12`  
**Root cause:** The `ctrl+q` check is evaluated before the `m.showDialog` guard at line 44.  

```go
func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
    switch {
    case msg.Keystroke() == "ctrl+q":   // ← line 10, BEFORE dialog guard
        return m, tea.Quit
    case msg.Keystroke() == "tab":      // ← line 13, also before dialog guard
    ...
    }
    if m.showDialog {                   // ← line 43, dialog guard
```

**Suggested fix:** Move the dialog guard to the top of `handleKey`, before the global shortcuts, or add a `m.showDialog` check to each global shortcut case.

---

#### B2. Tab/Shift+Tab bypasses dialog guard

**Key sequence:** `tab` or `shift+tab` while dialog is showing  
**What happens:** Focus manager cycles, changing focus state behind the dialog  
**What should happen:** Dialog should absorb all keyboard input  
**File:** `update.go:13-41`  
**Root cause:** Same as B1 — tab/shift+tab are handled before the dialog guard.  

**Suggested fix:** Guard tab/shift+tab with `!m.showDialog`.

---

#### B3. Ctrl+V in name field reads clipboard but doesn't paste

**Key sequence:** `ctrl+v` while editor is open and name field is focused (editorScope index == 0)  
**What happens:** Clipboard is read into `clipboardContent`, then the function returns early without calling `pasteIntoBuffer`  
**What should happen:** Content should be pasted into the name field (newlines stripped, ANSI codes stripped)  
**File:** `update.go:216-226`  

```go
case msg.Keystroke() == "ctrl+v":
    clipboardContent, err := readClipboard()
    if err != nil {
        m.errorMessage = "failed to read clipboard"
        return m, nil
    }
    if m.editorScope.Index() == 0 {
        return m, nil   // ← BUG: early return, pasteIntoBuffer never called
    }
    m.pasteIntoBuffer(clipboardContent)
```

Note: `pasteIntoBuffer` already handles the name-field case (newlines → spaces, ANSI stripping), so the early return is entirely unnecessary.

**Suggested fix:** Remove the early return; always call `pasteIntoBuffer`.

---

#### B4. Opening editor does not blur background components

**Key sequences:** `e` (edit), `n` (new), `c` (clone)  
**What happens:** `m.editorScope.Enter(m.fm)` saves parent index and `m.editor.name.Focus()` focuses the name field, but `m.tabs.Blur()` and `m.modelList.Blur()` are never called. The background components retain their "focused" state.  
**What should happen:** Background components should be blurred via `editorScope.ApplyBackground(m.fm, &m.tabs, &m.modelList)`  
**Files:** `selection.go:143-148` (beginEditSelected), `selection.go:158-163` (newBookmarkForCurrentGroup), `selection.go:171-176` (cloneSelectedBookmark)  

Example from `beginEditSelected`:
```go
func (m *model) beginEditSelected() error {
    ...
    m.editor = newBookmarkEditor(*selected, false, m.theme)
    m.editorScope.Enter(m.fm)
    m.editor.name.Focus()
    // Missing: m.editorScope.ApplyBackground(m.fm, &m.tabs, &m.modelList)
    // Missing: m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
    return nil
}
```

Compare with `applyFocus()` which does it correctly:
```go
func (m *model) applyFocus() {
    ...
    } else if m.editor != nil {
        m.editorScope.Enter(m.fm)
        m.editorScope.ApplyBackground(m.fm, &m.tabs, &m.modelList)
        m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
    }
}
```

**Suggested fix:** Replace the manual `Enter`+`Focus` calls with a call to `applyFocus()` after setting `m.editor`.

---

#### B5. Closing editor does not call applyFocus

**Key sequences:** `esc` (discard), `ctrl+s` followed by successful save (via `actionMsg.clearEditor`)  
**What happens:** `m.editorScope.Exit(&m.fm)` restores the parent manager index, but `m.fm.Apply(&m.tabs, &m.modelList)` is never called. Components retain stale focus/blur state.  
**What should happen:** After exiting the scope, `applyFocus()` should be called to push the restored focus state to components.  
**Files:** `update.go:149-152` (esc handler), `model.go:103-108` (actionMsg handler)  

**Suggested fix:** Call `m.applyFocus()` after `m.editorScope.Exit(&m.fm)` in both locations.

---

### 🟡 Advisory Issues

#### A1. Footer shows "/" toggle when editor is active but it doesn't work

**Observation:** When `m.editor != nil`, the footer displays `"/"` as a toggle-to-logs shortcut, but `handleDetailKey` only handles `"/"` when `m.editor == nil`. When the editor is active, `"/"` inserts a literal `/` character into the name or args field.  
**File:** `render.go:180-181` (footer segment), `update.go:128-129` (handler guard)  

```go
// render.go — footer for editor-active state includes "/" toggle
{Text: "/", Kind: statusbar.KindAccent},
{Text: " logs  ", Kind: statusbar.KindNormal},

// update.go — "/" only toggles when editor is nil
case msg.Text == "/" && m.editor == nil:
```

**Risk:** Users may be confused why "/" doesn't toggle views while editing.  
**Suggested fix:** Either remove "/" from the editor footer, or implement toggle-from-editor (perhaps requiring the user to save/discard first, or auto-discarding).

---

#### A2. Ctrl+Z doesn't work in name field

**Key sequence:** `ctrl+z` while name field is focused  
**What happens:** Silently ignored  
**What should happen:** Should undo in the name textinput (if the component supports it)  
**File:** `update.go:179-184`  

```go
case msg.Keystroke() == "ctrl+z":
    if m.editorScope.Active() && m.editorScope.Index() == 1 {
        m.editor.args.Undo()  // only args field
        ...
    }
```

**Risk:** Minor inconsistency — undo works in args but not in name.  
**Suggested fix:** Add `m.editor.name.Undo()` for the name field case, or document that undo is args-only.

---

#### A3. Focus manager created with 3 slots but only 2 used

**Observation:** `focus.NewManager(3)` creates a manager for 3 components, but `Apply` is always called with only 2 components (`&m.tabs, &m.modelList` or `&m.tabs, &m.logView`). Slot 2 is never mapped.  
**File:** `model.go:84`  

```go
m.fm = focus.NewManager(3)
```

**Risk:** If someone later adds a third component to the tab order, they may forget to update the slot count. Low practical risk.  
**Suggested fix:** Change to `focus.NewManager(2)`.

---

### ⚪ Informational Observations

#### I1. Defensive dead code in handleDetailKey

**File:** `update.go:131-134`  

```go
case m.editor == nil:
    m.editorScope.Exit(&m.fm)
    return m, nil
```

This branch is reached when `m.editorScope.Active()` is true but `m.editor == nil`. In practice, `m.editor` and `m.editorScope` are always set/cleared atomically within the same `Update` call, so this branch is unreachable. Harmless defensive code.

---

#### I2. Mouse wheel silently ignored in bookmark view

**File:** `update.go:106-110`  

```go
func (m *model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
    if m.tabs.SelectedID() != "logs" {
        return m, nil
    }
```

Intentional — mouse wheel only scrolls the log view. Could be extended to scroll the model list in the future.

---

### Test Coverage Gaps

| Area | Existing Tests | Gap |
|---|---|---|
| Dialog interaction | None | No test verifies dialog blocks global shortcuts |
| Ctrl+V in name field | `TestPasteIntoNameStripsNewlines` (uses `PasteMsg`) | No test for `ctrl+v` keybinding in name field |
| Focus restoration after editor close | `TestEscDiscardsEditorAndReturnsFocusToList` (checks `fm.Index() == 1`) | Doesn't verify components are actually Focus()/Blur()'d |
| Tab during dialog | None | No test |
| Ctrl+Q during dialog | None | No test |

---

### Commands Run

```
go build ./cmd/nice-llama-server/   → passed
go vet ./internal/tui/...           → passed
go test ./internal/tui/... -count=1 -v → 48/48 passed
```

---

### Residual Risks

1. **Focus leak severity:** B4/B5 mean that after opening/closing the editor, multiple components may simultaneously believe they are focused. In practice, Bubble Tea's rendering loop calls `View()` which reads component state, so stale focus flags could affect visual rendering (e.g., cursor visibility, border colors). The next tab press would correct the state via `applyFocus()`, but any intervening keypresses could be misrouted.

2. **Dialog bypass severity:** B1/B2 mean that during a delete confirmation dialog, `ctrl+q` quits without warning and `tab`/`shift+tab` cycles focus behind the dialog overlay. The latter is mostly cosmetic since the dialog overlay still renders on top, but quitting during a delete confirmation is a data loss risk.

3. **Ctrl+V in name field:** B3 means users cannot paste into the bookmark name field via `ctrl+v`. The `PasteMsg` path (terminal bracketed paste) still works, but `ctrl+v` clipboard paste is broken for the name field.

---