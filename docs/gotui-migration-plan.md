# gotui Migration Plan: Vertically Sliced Implementation

Revamping the nice-llama-server TUI with the [gotui](https://github.com/ishansain/gotui) library (local: `~/Documents/repos/gotui`).

## Methodology

This plan follows the gotui recipe-driven development pattern:

1. **App-level AGENTS.md** (`/AGENTS.md`) encodes the gotui conventions, component mapping,
   and state ownership so every subagent receives the same design grammar.
2. **Canonical examples as templates** — each subagent task names a specific gotui example
   to start from (e.g. "Implement this like `examples/frame` but with our data").
3. **Small, routed tasks** — each subagent gets an explicit task prompt with:
   - Which gotui example to study first
   - Which components to use
   - Which files to touch
   - Which tests to update
   - Verification command
4. **Second-pass review** — after implementation, a review subagent checks MVU discipline,
   focus wiring, sizing contracts, mouse routing, and narrow-width behavior.

## Overview

The current TUI (`/internal/tui/`) is ~1,800 lines of hand-rolled bubbletea v2 code across 11 files. gotui provides production-quality, tested components for nearly every subsystem. This plan breaks the migration into 13 vertically sliced implementation steps, each producing a working binary with passing tests.

**Estimated reduction:** ~900 lines of UI infrastructure eliminated (~80% of the custom UI code). ~900 lines of legitimate domain logic (controller client, arg catalog, completion engine, bookmark operations) remain in the application layer.

## Guiding Principles

Each **vertical slice**:
- Is a self-contained, end-to-end feature replacement
- Compiles and passes all tests at the end
- Can be executed by an independent subagent
- Minimizes merge conflicts with other slices

## ⚠️ Corrected Assumptions (post-review)

The following corrections from a review of the initial plan are baked into every slice below:

1. **No unsafe parallelization.** Slices 1–8 and 11 all touch `model.go`, `render.go`,
   `update.go`, and tests. They cannot safely run in parallel in one worktree. Each slice
   runs **sequentially**, or in **isolated git worktrees** with an integration agent merging.

2. **Verified API names** (checked against gotui source at `~/Documents/repos/gotui`):
   - `autocomplete.SetItems()` — not `SetSuggestions()`
   - `dialog.Model.Body` — not `Message`
   - `dialog.Model` is a **value type** — use a `showDialog bool` field for visibility
   - `scrollbar.For(theme, scrollable)` — returns a string, no `Model` or `ForViewport`
   - `tabs.Update()` does **not** handle `/` — the app maps `/` to `SelectID()` directly

3. **textarea API gap.** `textarea.Model` does not expose a public `Cursor()` or
   `ReplaceRange()`. The completion engine needs these. Before Slice 6, either:
   - Add `Cursor() (row, col int)` and `ReplaceRange(row, start, end int, text string) bool`
     to gotui/textarea, or
   - Keep completion-specific editing behind an app-owned adapter that mirrors cursor state.

4. **"Zero behavioral change" is aspirational.** Slice 0 establishes a baseline, but
   replacing colors and renderers produces intentional visual changes. Expect snapshot
   updates. Add `snaptest.Snap`, `SnapCells`, and scenario tests rather than only updating
   string assertions.

5. **Focus topology needs design.** The focus manager must handle conditional visibility
   (don't focus hidden log controls when bookmarks are active). Autocomplete must be in the
   focus plan. Use `focus.Scope` for modal groups (editor vs logs).

6. **Mouse routing needs stored rectangles.** Retain log/editor/viewport rectangles and use
   `mouse.InBounds`. Visibility checks alone are insufficient when multiple panes exist.

7. **List styling is uniform.** `list.Model` applies one `selectedStyle` to all items.
   "Groups bold and accent-colored" requires either pre-styled strings (ANSI embedded in
   item text) or extending the list component. This is a deliberate visual trade-off.

## Prerequisites

Both projects share the same Charmverse stack (bubbletea v2, lipgloss v2, ultraviolet), so compatibility is excellent.

gotui lives locally at `~/Documents/repos/gotui`. Add a `replace` directive in `go.mod`:

```bash
go get github.com/ishansain/gotui@latest
```

Then add to `go.mod`:

```
replace github.com/ishansain/gotui => /Users/ishansain/Documents/repos/gotui
```

Minor bumps: bubbletea v2.0.2 → v2.0.8, lipgloss v2.0.2 → v2.0.5.

**For subagents:** The gotui source is at `~/Documents/repos/gotui`. When a task says
"study gotui/examples/frame", read the files under
`/Users/ishansain/Documents/repos/gotui/examples/frame/`. When a task says
"check gotui/AGENTS.md", read `/Users/ishansain/Documents/repos/gotui/AGENTS.md`.

---

## Slice 0 — Foundation: Add gotui Dependency & Theme

**Goal:** Wire gotui into the project. Establish a behavioral baseline (expect visual changes).

**Files touched:**
- `go.mod` / `go.sum` — add `github.com/ishansain/gotui` + replace directive
- `internal/tui/styles.go` — replace all hardcoded `lipgloss.Color("#...")` with derivations
  from `gotui.Theme` roles. The `styles` struct shape stays; only color assignments change.
- `internal/tui/model.go` — store `gotui.Theme` on the model, thread through `newModel()`

**Subagent task:**

```
Before implementing:
1. Read gotui/theme.go — understand the 18 semantic roles.
2. Read gotui/AGENTS.md rules 1–3 (colors from theme, no UV imports, SetSize).

Task:
- Add to go.mod:
    replace github.com/ishansain/gotui => /Users/ishansain/Documents/repos/gotui
- Run: go get github.com/ishansain/gotui@latest
- In internal/tui/styles.go, replace every hardcoded lipgloss.Color("#...")
  with a derivation from gotui.Theme roles. The styles struct shape stays;
  only color assignments change.
- In internal/tui/model.go, store gotui.Dark() on the model.
- Verify go build ./cmd/nice-llama-server/ && go test ./internal/tui/...
  passes. Expect snapshot/visual test updates — review each diff.

⚠️ "Zero behavioral change" is aspirational. Colors WILL shift. Review every
   test diff. This is the baseline; later slices build on it.

Mapping guide (current hex → theme role):
  #4D7CFE → Accent
  #F8FAFC → Text
  #CBD5E1 → TextMuted
  #93C5FD → Accent (lighter variant)
  #FCD34D → Warning
  #475569 → Border
  #38BDF8 → BorderFocused
  #F59E0B → Warning (for logs panel)
  #E2E8F0 → Text
  #7DD3FC → Accent
  #0F172A → TextInverted
  #22C55E → Success
  #94A3B8 → TextMuted
  #64748B → TextFaint
  #86EFAC → Success
  #FCA5A5 → Danger
  #F8FAFC → TextInverted (for selection)
  #334155 → SurfaceRaised
  #4ade80 → Success

Requirements:
- No color literals remain in styles.go.
- All tests pass (possibly with updated golden values).

Verification:
  go build ./cmd/nice-llama-server/ && go test ./internal/tui/...
```

**Risk:** None for compilation. Expect visual test assertion updates.

---

## Slice 1 — Header Panel: Replace with gotui/frame + gotui/stack

**Goal:** Replace the custom header rendering with `frame.Panel` and `stack.Vertical`.

**Files touched:**
- `internal/tui/render.go` — rewrite `renderHeader()` using frame + stack
- `internal/tui/styles.go` — remove header-specific style fields

**Subagent task:**

```
Before implementing:
1. Study gotui/examples/frame/main.go — see how frame.Panel and stack.Vertical compose.
2. Read gotui/frame/frame.go — understand Panel, PanelOptions, PanelContentRect.
3. Read gotui/stack/stack.go — understand Vertical with Gap and Divider options.

Task:
Implement this like examples/frame, but replace the service overview with
our app header (title + status + stats + messages).

- In internal/tui/render.go, rewrite renderHeader() to use:
    - gotui/frame.Panel with Title="Nice Llama Server"
    - gotui/stack.Vertical for the title line, stats line, and message line
- The header body shows:
    - Row 1: title + runtime status + port
    - Row 2: stats (bookmark/model/root counts)
    - Row 3 (conditional): error or flash message
- Update headerPanelHeight constant if frame borders change the height.
- Remove header-specific style fields from styles.go that frame now provides.
- Update TestHeaderRespectsFiveLineCap and TestHeaderOmitsEmptyStatusRowContent.

Requirements:
- Only Theme roles for colors — no literals.
- frame.Panel provides rounded borders, title in top border, focused styling.
- stack.Vertical skips empty sections (no message row when no error/flash).

Verification:
  go test ./internal/tui/... -run TestHeader
```

**Risk:** Low. Frame panel adds 2 rows of border (top + bottom), so height constants may shift by ±1.

---

## Slice 2 — Footer: Replace with gotui/statusbar

**Goal:** Replace the manual footer key hints with `statusbar.Model`.

**Files touched:**
- `internal/tui/model.go` — add `statusbar.Model` field
- `internal/tui/render.go` — rewrite `renderFooter()` to delegate to statusbar
- `internal/tui/update.go` — call `statusbar.SetSize()` on `tea.WindowSizeMsg`
- `internal/tui/styles.go` — remove footer-related style fields

**Subagent task:**

```
Before implementing:
1. Study gotui/examples/statusbar/main.go — canonical single-component wiring.
2. Read gotui/statusbar/statusbar.go — understand Segment, Kind, SetLeft/SetRight.

Task:
Implement this like examples/statusbar, but populate segments from our
app state (keybinding hints, tail indicator).

- In internal/tui/model.go, add a statusbar.Model field.
- In newModel(), initialize it with gotui.New(theme).
- In renderFooter(), call statusbar.View() instead of building the footer
  string manually.
- The footer has:
    - Left segments: keybinding hints (context-dependent: bookmarks view vs
      logs view vs editor mode)
    - Right segment: tail indicator (green "[Tail]" or muted "[tail]")
- Use statusbar.Segment with appropriate Kind:
    - Keybinding labels: KindAccent (bold badge)
    - Key descriptions: KindNormal
    - Tail indicator active: KindSuccess
    - Tail indicator paused: KindMuted
- In update.go, call statusbar.SetSize() on tea.WindowSizeMsg.
- Remove footer-related style fields from styles.go.
- Update TestFooterChangesByContext.

Requirements:
- Only Theme roles for colors.
- Statusbar handles truncation automatically when space is tight.

Verification:
  go test ./internal/tui/... -run TestFooter
```

**Risk:** Low. Statusbar handles truncation automatically.

---

## Slice 3 — Bottom View Tabs: Replace toggle with gotui/tabs

**Goal:** Replace the `bottomView` enum + manual `/` toggle with `tabs.Model`.

**Files touched:**
- `internal/tui/model.go` — add `tabs.Model` field, remove `bottomView` enum
- `internal/tui/render.go` — use `tabs.Model` to determine which view to render
- `internal/tui/update.go` — map `/` key to `tabs.SelectID()` (NOT `tabs.Update()`)
- `internal/tui/selection.go` — any references to `bottomView`

**Subagent task:**

```
Before implementing:
1. Study gotui/tabs/tabs.go — understand Tab, New, SetTabs, SelectID, Focus/Blur.
   Note: tabs.Update() does NOT handle "/". The app must map "/" to SelectID().
2. Check gotui/examples/frame/main.go for tabs usage in a real composition.

Task:
Replace the bottomView enum with gotui/tabs.Model.

- In internal/tui/model.go:
    - Add tabs.Model field
    - Remove bottomView type and field
    - In newModel(), create tabs with two tabs:
        tabs.Tab{ID: "bookmarks", Label: "Bookmarks"}
        tabs.Tab{ID: "logs", Label: "Logs"}
- In render.go, replace m.bottomView checks with m.tabs.SelectedID().
- In update.go, route the "/" key to m.tabs.SelectID("bookmarks") or
  m.tabs.SelectID("logs") — NOT m.tabs.Update().
- In all test files, replace:
    m.bottomView = bottomViewLogs  →  m.tabs.SelectID("logs")
    m.bottomView = bottomViewBookmarks  →  m.tabs.SelectID("bookmarks")
- Remove bottomView constant block.

Requirements:
- Tabs are rendered as a 1-row strip above the content area.
- Selected tab uses focused/blurred styling from the theme.
- All tests pass with updated references.

Verification:
  go test ./internal/tui/...
```

**Risk:** Medium — touches many test files. Systematic find-and-replace.

---

## Slice 4 — Log View: Replace with gotui/viewport

**Goal:** Replace the custom log viewport with `gotui/viewport.Model`.

**Files touched:**
- `internal/tui/model.go` — add `viewport.Model` field, remove `logScrollY`,
  `logViewWidth`, `logViewHeight`, `followTail`, `followTailEnabled`
  (keep `logScrollX` for horizontal scroll)
- `internal/tui/render.go` — rewrite `renderLogView()` to format entries then set on viewport
- `internal/tui/update.go` — delegate scroll keys and mouse wheel to viewport
- `internal/tui/scroll.go` — delete entirely (viewport owns scroll logic)
- `internal/tui/styles.go` — remove log-specific timestamp styles

**Subagent task:**

```
Before implementing:
1. Study gotui/viewport/viewport.go — understand SetContent, ScrollBy, AtBottom,
   GotoBottom, Focus/Blur, mouse wheel handling.
2. Read gotui/scrollbar/scrollbar.go — understand Scrollable interface for later.

Task:
Replace the custom log viewport with gotui/viewport.Model.

- In internal/tui/model.go:
    - Add viewport.Model field
    - Remove: logScrollY, logViewWidth, logViewHeight, followTail, followTailEnabled
    - Keep: logScrollX (horizontal scroll stays in app)
    - ADD: logRect layout.Rect (store the viewport's bounding box for mouse routing)
- In render.go, rewrite renderLogView():
    - Format each log entry as "TIMESTAMP LINE" with stream-colored styling
    - Pre-slice each line horizontally by logScrollX
    - Join into a single string and call viewport.SetContent()
    - Return viewport.View()
- In update.go:
    - Delegate up/down/pgup/pgdown/home/end keys to viewport.Update()
    - Delegate mouse wheel to viewport.Update() — but ONLY when
      mouse.InBounds(msg, logRect.Min.X, logRect.Min.Y, logRect.Dx(), logRect.Dy())
    - Keep left/right for horizontal scroll (app-level)
    - Keep 't'/'T' for follow-tail toggle (app-level)
    - Follow-tail: on new logs, if viewport.AtBottom(), call viewport.GotoBottom()
- Delete scroll.go entirely.
- Remove log timestamp styles from styles.go (timestamps are pre-formatted
  in the content string).
- Rewrite all log view tests to use viewport API.

Requirements:
- Only Theme roles for colors.
- Horizontal scroll stays as app-level pre-processing.
- Follow-tail uses viewport.AtBottom() + GotoBottom().
- Mouse wheel is routed through mouse.InBounds using stored logRect.

Verification:
  go test ./internal/tui/... -run TestLog
```

**Risk:** Medium — viewport doesn't do horizontal scroll, so that stays as app logic.
Follow-tail behavior maps cleanly to `AtBottom()`.

---

## Slice 5 — Text Input (Name Field): Replace with gotui/textinput

**Goal:** Replace the single-line `textBuffer` for bookmark names with `gotui/textinput.Model`.

**Files touched:**
- `internal/tui/editor.go` — replace `name textBuffer` with `textinput.Model`
- `internal/tui/render.go` — update `renderDetailLines()` to call `textinput.View()`
- `internal/tui/update.go` — delegate name-field key handling to `textinput.Update()`

**Subagent task:**

```
Before implementing:
1. Study gotui/textinput/textinput.go — understand New, SetValue, Value, Focus/Blur,
   Update, View, Prompt, Placeholder.
2. Check gotui/examples/autocomplete/main.go for textinput in a real app.

Task:
Replace the single-line textBuffer for bookmark names with gotui/textinput.Model.

- In internal/tui/editor.go:
    - Replace name textBuffer field with textinput.Model
    - In newBookmarkEditor(), initialize with gotui.New(theme)
    - Set Prompt to "" (no prompt — the field label serves that role)
    - Map bookmarkEditor.Bookmark() to read textinput.Value()
- In render.go, renderDetailLines():
    - Call textinput.View() instead of buffer.RenderLines()
- In update.go:
    - Delegate name-field key events to textinput.Update()
    - Handle Enter specially: if textinput is focused, move focus to args
      (textinput doesn't intercept Enter by default)
- Drop Ctrl+Z undo for the name field (acceptable for single-line input).
- Update affected tests:
    - TestFocusedBookmarkNameRendersCursor
    - TestPasteIntoNameStripsNewlines
    - TestPasteIntoNameNormalizesCRLF
    - TestPasteIntoNameStripsANSIColorCodes
    - TestCtrlZUndoesInEditor / TestCtrlZNoOpWithEmptyUndoStack
    - TestEnterInNameMovesFocusToArgs

Requirements:
- Only Theme roles for colors.
- textinput provides: cursor, horizontal scroll, ctrl+u/k/w editing.
- Paste handling is built into textinput via tea.PasteMsg.

Verification:
  go test ./internal/tui/... -run "TestPaste|TestCtrlZ|TestFocused|TestEnterInName"
```

**Risk:** Medium — textinput API differs from textBuffer. Paste handling, cursor rendering,
and Enter behavior need careful mapping.

---

## Slice 6 — Text Area (Args Editor): Replace with gotui/textarea

**Goal:** Replace the multi-line `textBuffer` for bookmark args with `gotui/textarea.Model`.

**⚠️ API gap:** `textarea.Model` does not expose public `Cursor()` or `ReplaceRange()`.
The completion engine needs these. Before this slice, either:
- **(Preferred)** Add `Cursor() (row, col int)` and
  `ReplaceRange(row, start, end int, text string) bool` to `gotui/textarea/textarea.go`,
  or
- Keep completion-specific editing behind an app-owned adapter that mirrors cursor state
  from textarea.

**Files touched:**
- `gotui/textarea/textarea.go` — add `Cursor()` and `ReplaceRange()` methods
- `internal/tui/editor.go` — replace `args textBuffer` with `textarea.Model`
- `internal/tui/render.go` — update `renderArgsEditorLines()` to call `textarea.View()`
- `internal/tui/update.go` — delegate args key handling to `textarea.Update()`
- `internal/tui/args_completion.go` — use new textarea APIs instead of textBuffer methods

**Subagent task:**

```
Before implementing:
1. Study gotui/textarea/textarea.go — understand New, SetValue, Value, Focus/Blur,
   Update, View, ContentHeight, Prompt, Placeholder, undo/redo, selections.
2. Read gotui/textarea/cells.go — understand soft-wrap and cursor geometry.
3. Check if textarea has Cursor() and ReplaceRange() public methods.
   If not, ADD them to gotui/textarea/textarea.go:
    - func (m Model) Cursor() (row, col int)  — returns logical cursor position
    - func (m *Model) ReplaceRange(row, start, end int, text string) bool
      — replaces runes in a logical line, used by completion engine

Task:
Replace the multi-line textBuffer for bookmark args with gotui/textarea.Model.

- First, add Cursor() and ReplaceRange() to gotui/textarea if missing.
- In internal/tui/editor.go:
    - Replace args textBuffer field with textarea.Model
    - In newBookmarkEditor(), initialize with gotui.New(theme)
    - Set Prompt to "" (field label serves that role)
    - Map bookmarkEditor.Bookmark() to read textarea.Value()
- In render.go:
    - Call textarea.View() instead of buffer.RenderLines()
- In update.go:
    - Delegate args key events to textarea.Update()
    - Handle Tab/Shift+Tab before textarea (for completion)
    - Handle Ctrl+Z via textarea's built-in undo
- In args_completion.go:
    - Replace textBuffer.TokenAtCursor() with textarea.Cursor() + scanLineTokens()
    - Replace textBuffer.ReplaceRange() with textarea.ReplaceRange()
    - Keep scanLineTokens() and scanBufferTokens() as standalone helpers
- Update all editor tests and completion tests.

Requirements:
- Only Theme roles for colors.
- textarea provides: soft-wrap, undo/redo, selections, word movement, kill ring.
- Completion engine logic stays untouched behind adapted API calls.

Verification:
  go test ./internal/tui/... -run "TestTab|TestCompletion|TestMMProj|TestPassive|TestTextBuffer"
```

**Risk:** **High** — this is the most complex slice. The arg completion engine is tightly
coupled to `textBuffer`'s internal API. Adding `Cursor()` and `ReplaceRange()` to gotui
is the cleanest path but requires a coordinated change across two repos.

---

## Slice 7 — Model List: Replace with gotui/list

**Goal:** Replace the custom model list rendering with `gotui/list.Model`.

**⚠️ List styling is uniform.** `list.Model` applies one `selectedStyle` to all items.
"Groups bold and accent-colored" requires either:
- **(Preferred)** Embed ANSI escape codes in item strings to style groups differently
  (e.g., `"\033[1;38;2;122;162;247m▶ My Group\033[0m"` for bold accent groups)
- Accept that groups and bookmarks share the same font weight

**Files touched:**
- `internal/tui/model.go` — add `list.Model` field
- `internal/tui/render.go` — replace `renderListLines()` with `list.View()`
- `internal/tui/update.go` — delegate list navigation keys to `list.Update()`
- `internal/tui/selection.go` — adapt `listItems()` to produce flat strings

**Subagent task:**

```
Before implementing:
1. Study gotui/list/list.go — understand New, SetItems, SetFilter, Selected,
   SelectedItem, Focus/Blur, Update, View.
2. Check gotui/examples/browser/main.go for list in a real app (file list).

Task:
Implement this like examples/browser, but replace the file list with
model groups and bookmarks.

- In internal/tui/model.go, add a list.Model field.
- In render.go, replace renderListLines() with list.View().
- In update.go, delegate up/down navigation keys to list.Update().
- In selection.go, adapt listItems() to return FLAT strings:
    - Model groups: embed ANSI codes for bold accent styling:
      "\033[1;38;2;122;162;247m▶ My Group\033[0m"
    - Bookmarks: plain text with "  ▸ " prefix
- Map the old selectedKey tracking to list.Selected() (original index).
- The list provides: scrolling, selection cursor, filtering, windowing.
- Update affected tests:
    - TestListItemsGroupsBookmarksByModelPath
    - TestListItemsUsesPathFallbackForMissingModel
    - TestNewBookmarkUsesCurrentModelGroup
    - TestBookmarkEditorViewFillsExactBottomRegion
    - TestBookmarkEditorViewFillsOnNarrowWidths

Requirements:
- Only Theme roles for colors.
- Flattened hierarchy: groups and bookmarks are all flat items with
  distinguishing prefixes. Group labels use embedded ANSI for bold/accent.
- list.Selected() returns original index into the flat items array.

Verification:
  go test ./internal/tui/... -run "TestList|TestNewBookmark|TestBookmarkEditorView"
```

**Risk:** Medium — flattening the hierarchy changes the visual structure. Embedded ANSI
in list items is a workaround for uniform styling.

---

## Slice 8 — Split Pane Layout: Replace with gotui/splitpane

**Goal:** Replace the manual width splitting with `splitpane.Horizontal`.

**Files touched:**
- `internal/tui/render.go` — rewrite `renderBookmarkEditorView()` using splitpane
- `internal/tui/render.go` — remove `splitBookmarkEditorWidths()` helper

**Subagent task:**

```
Before implementing:
1. Study gotui/splitpane/splitpane.go — understand Horizontal, Options, Ratio, Gap.
2. Check gotui/examples/frame/main.go for splitpane usage.

Task:
Replace the manual width splitting with gotui/splitpane.Horizontal.

- In internal/tui/render.go, rewrite renderBookmarkEditorView():
    - Use splitpane.Horizontal with Ratio=40 (40% left, 60% right)
    - Left view: renderModelListPanel
    - Right view: renderDetailPanel
    - Gap=1 with themed vertical divider
- Remove splitBookmarkEditorWidths() helper.
- Update TestBookmarkEditorViewFillsExactBottomRegion and
  TestBookmarkEditorViewFillsOnNarrowWidths.

Requirements:
- Only Theme roles for colors (divider uses BorderMuted).
- Splitpane handles width allocation and divider rendering.

Verification:
  go test ./internal/tui/... -run TestBookmarkEditorView
```

**Risk:** Very low. Pure layout replacement.

---

## Slice 9 — Arg Completion UI: Wire gotui/autocomplete

**Goal:** Use `gotui/autocomplete.Model` for the arg completion UI (replacing inline ghost text).

**Depends on:** Slice 6 (TextArea)

**Files touched:**
- `internal/tui/model.go` — add `autocomplete.Model` field
- `internal/tui/render.go` — replace ghost rendering with autocomplete view
- `internal/tui/update.go` — delegate tab/shift-tab to autocomplete
- `internal/tui/args_completion.go` — adapt completion engine to feed candidates

**Subagent task:**

```
Before implementing:
1. Study gotui/autocomplete/autocomplete.go — understand New, SetItems (NOT
   SetSuggestions), Selected, SelectedItem, Focus/Blur, Update, View.
2. Study gotui/examples/autocomplete/main.go — canonical usage pattern.
   Note: autocomplete emits SelectedMsg on activation; the app handles insertion.

Task:
Implement this like examples/autocomplete, but replace the static
suggestion list with our dynamic arg completion engine.

- In internal/tui/model.go, add an autocomplete.Model field.
- In render.go, replace the inline ghost completion rendering with
  autocomplete.View() positioned below the args editor.
- In update.go:
    - On Tab/Shift+Tab: compute candidates from completion engine,
      feed to autocomplete via SetItems(), then focus autocomplete
    - On Esc: blur autocomplete
    - Handle autocomplete.SelectedMsg: insert the chosen value into textarea
    - Delegate autocomplete key events to autocomplete.Update()
- In args_completion.go:
    - Keep the completion engine (candidate generation, popularity
      ranking, mmproj paths) — it stays as app logic
    - Change the output: instead of modifying textarea inline,
      return autocomplete.Item slice for autocomplete to display
- Update rendering tests:
    - TestArgsCompletionRendersInlineGhostOptions → now checks
      autocomplete suggestion window
    - TestMMProjCompletionRendersInlineGhostOptions → update
    - TestPassiveSingleHyphenCompletionRendersInlineGhostOptions → update
    - All TestTab* / TestShiftTab* tests → update

Requirements:
- Only Theme roles for colors.
- Completion candidates appear in a suggestion window below the args editor.
- User can arrow through candidates and press Enter to select.
- Completion engine logic (catalog parsing, popularity, mmproj) stays untouched.

Verification:
  go test ./internal/tui/... -run "TestTab|TestCompletion|TestMMProj|TestPassive|TestArgs"
```

**Risk:** Medium — changes the visual presentation of completions from inline ghosts to a
suggestion window. The completion engine logic stays untouched.

---

## Slice 10 — Focus Management: Use gotui/focus

**Goal:** Replace manual focus tracking with `gotui/focus.Manager` + conditional scopes.

**⚠️ Focus topology design:**
- When bookmarks view is active: tabs → list → textinput → textarea → autocomplete
- When logs view is active: tabs → viewport (log controls are hidden, skip them)
- Use `focus.Scope` for modal groups: one scope for editor fields, one for log controls
- Autocomplete must be in the focus cycle when visible

**Depends on:** Slices 1–7 (all components must exist)

**Files touched:**
- `internal/tui/model.go` — add `focus.Manager` field, register components conditionally
- `internal/tui/update.go` — delegate Tab/Shift+Tab to focus manager
- `internal/tui/model.go` — remove `focusArea` enum

**Subagent task:**

```
Before implementing:
1. Study gotui/focus/focus.go — understand Manager, Scope, Stack.
2. Study gotui/focus/scope.go — understand Apply, Next, Prev.
3. Check gotui/examples/frame/main.go for focus.Manager usage.

Task:
Implement this like examples/frame, but with CONDITIONAL focus groups.

Focus topology:
  Bookmarks view active:
    tabs → list → textinput → textarea → autocomplete (when visible)
  Logs view active:
    tabs → viewport
  Use focus.Scope for the editor group (textinput + textarea + autocomplete)
  so they are skipped when the editor is closed.

- In internal/tui/model.go:
    - Add focus.Manager field
    - Remove focusArea type and focus field
    - In newModel(), create manager with capacity for all focusable components
    - Register components via fm.Apply() CONDITIONALLY based on active view
- In update.go:
    - On Tab: fm.Next() + fm.Apply() with current component set
    - On Shift+Tab: fm.Prev() + fm.Apply() with current component set
    - When switching views (via tabs), re-register the appropriate set
    - Remove manual focusArea switching logic
- In all test files:
    - Replace m.focus = focusDetailName with m.textinput.Focus()
    - Replace m.focus = focusDetailArgs with m.textarea.Focus()
    - Replace m.focus = focusModelList with m.list.Focus()
- Update all tests that reference m.focus or focusArea.

Requirements:
- Only Theme roles for colors.
- Hidden controls (log viewport when bookmarks active) are NOT in the focus cycle.
- Autocomplete IS in the focus cycle when visible.

Verification:
  go test ./internal/tui/... -run "TestFocus|TestEnter|TestUp|TestSave|TestEsc"
```

**Risk:** Medium — focus management is woven throughout the codebase. Conditional
registration adds complexity.

---

## Slice 11 — Delete Confirmation Dialog: Use gotui/dialog

**Goal:** Replace the manual confirm-delete flow (y/n key prompt) with `gotui/dialog.Model`.

**Files touched:**
- `internal/tui/model.go` — add `dialog.Model` field + `showDialog bool`
- `internal/tui/render.go` — render dialog overlay when `showDialog` is true
- `internal/tui/update.go` — delegate 'd' key to open dialog, handle `dialog.ResultMsg`

**Subagent task:**

```
Before implementing:
1. Study gotui/dialog/dialog.go — understand New, Update, View, ResultMsg.
   Note: dialog.Model is a VALUE TYPE. Use a separate showDialog bool for visibility.
   Note: dialog.Body (not "Message") holds the prompt text.
2. Check gotui/examples/demo/main.go for dialog in a real app.

Task:
Replace the manual confirm-delete flow (y/n key prompt) with gotui/dialog.Model.

- In internal/tui/model.go:
    - Add dialog.Model field (always initialized, not a pointer)
    - Add showDialog bool field for visibility
    - Remove confirmDelete bool field
- When user presses 'd' on a bookmark:
    - Set m.dialog.Title = "Delete Bookmark"
    - Set m.dialog.Body = "Delete \"<bookmark name>\"?"
    - Set m.dialog.ConfirmLabel = "Delete"
    - Set m.dialog.CancelLabel = "Cancel"
    - Set m.showDialog = true
- In render.go:
    - If m.showDialog, render dialog.View() as an overlay on top of the normal view
    - Use gotui/overlay.Center() to position the dialog
- In update.go:
    - If m.showDialog, delegate all keys to dialog.Update()
    - Collect dialog.ResultMsg from the returned command:
        - If OK: call deleteBookmarkCmd()
        - Set m.showDialog = false
- Remove y/n key handling from handleKey().
- Update tests that check m.confirmDelete.

Requirements:
- Only Theme roles for colors.
- Dialog is centered and modal (blocks all other input).
- Esc cancels, Enter confirms.
- dialog.Model is a value type — do not use *dialog.Model.

Verification:
  go test ./internal/tui/... -run TestDelete|TestConfirm
```

**Risk:** Low. Self-contained feature.

---

## Slice 12 — Cleanup & Polish

**Goal:** Remove all dead code, add scrollbar to log view, final verification.

**Files touched:**
- `internal/tui/editor.go` — remove `textBuffer` type if fully replaced
- `internal/tui/scroll.go` — confirm deleted
- `internal/tui/styles.go` — remove unused style fields
- `internal/tui/render.go` — add `scrollbar.For()` alongside log viewport
- All test files — final cleanup

**Subagent task:**

```
Before implementing:
1. Study gotui/scrollbar/scrollbar.go — understand For(theme, scrollable).
   Note: scrollbar.For() returns a STRING, not a Model. Use:
     layout.Horizontal(layout.Fill(1), layout.Len(1)).Split(area)
       .Assign(&contentRect, &barRect)
     m.viewport.SetSize(contentRect.Dx(), contentRect.Dy())
     ...
     lipgloss.JoinHorizontal(lipgloss.Top,
       m.viewport.View(),
       scrollbar.For(theme, m.viewport),
     )
2. Review the full internal/tui/ directory for dead code.

Task:
Final cleanup: remove all dead code, add scrollbar to log view.

- Remove all dead code: unused types, functions, style fields.
  Candidates for removal:
    - textBuffer type (if fully replaced by textinput + textarea)
    - scroll.go (if viewport handles scrolling)
    - unused style fields in styles.go
    - bottomView enum (if replaced by tabs)
    - focusArea enum (if replaced by focus.Manager)
    - confirmDelete field (if replaced by dialog)
- Add scrollbar to the log view:
    - In renderLogView(), allocate 1 column for the scrollbar
    - Use layout.Horizontal(layout.Fill(1), layout.Len(1)).Split(...)
    - Join viewport.View() + scrollbar.For(theme, m.viewport)
- Verify go build ./cmd/nice-llama-server/ && go test ./internal/tui/...
  passes cleanly.
- Run go vet ./internal/tui/... — zero warnings.

Requirements:
- Scrollbar uses the Scrollable interface via viewport.
- Only Theme roles for colors — no literals.
- Do NOT regenerate snaptest goldens without reviewing the diff.

Verification:
  go build ./cmd/nice-llama-server/ && go vet ./internal/tui/... && go test ./internal/tui/...
```

**Risk:** Low.

---

## Slice R — Second-Pass Review (Cross-Cutting)

**Goal:** After each wave of implementation, run a review pass to catch MVU discipline
violations, focus wiring bugs, sizing contract mismatches, mouse routing errors, and
narrow-width regressions.

**When to run:** After Wave 2, after Wave 4, and after Wave 6 (final).

**Subagent task:**

```
Before reviewing:
1. Read gotui/AGENTS.md — understand the hard rules (colors, SetSize, MVU).
2. Read gotui/DESIGN.md §4 (Component Contract) — understand the contract.

Review checklist:

MVU Discipline:
- Every Update call reassigns the model: m.component, cmd = m.component.Update(msg)
- Every returned Cmd is collected and batched: cmds = append(cmds, cmd)
- No Update returns (m, nil) when it should return a command

Focus Wiring:
- Tab/Shift+Tab routes through focus.Manager
- Focus() is called when a component becomes active
- Blur() is called when a component loses focus
- Focused components show visual focus state (BorderFocused, SelectionBg)
- Hidden controls are NOT in the focus cycle (conditional registration)
- Autocomplete IS in the focus cycle when visible

Sizing Contracts:
- Every component has SetSize(width, height) called on WindowSizeMsg
- Components render exactly within their assigned box
- No component measures the terminal directly
- Stored layout rectangles (logRect, editorRect) are updated on resize

Mouse Routing:
- Mouse wheel events reach viewport.Update()
- Wheel events are routed through mouse.InBounds() using stored rectangles
- Wheel events are ignored when the view is not visible

Narrow-Width Behavior:
- Test with width=60, height=16 (minimums from current code)
- No panic, no infinite layout, no content overflow
- Statusbar drops right segments when tight
- List truncates items with ellipsis
- Splitpane handles very narrow widths gracefully

Color Hygiene:
- grep for lipgloss.Color("# — zero hits outside theme setup
- grep for lipgloss.NewStyle() with Foreground/Background that don't
  reference a theme role

Dead Code:
- grep for exported symbols no longer referenced
- grep for the old focusArea type, bottomView type, confirmDelete field

Verification:
  go build ./cmd/nice-llama-server/
  go vet ./internal/tui/...
  go test ./internal/tui/...
  # Manual narrow-terminal smoke test:
  echo "80 24" | go run ./cmd/nice-llama-server/  # if it accepts stdin size
```

**Risk:** None (read-only analysis). Catches issues before they compound.

---

## Dependency Graph

```
Slice 0 (Theme) ─┬─► Slice 1 (Header) ──► Slice 8 (SplitPane)
                 │
                 ├─► Slice 2 (Footer)
                 │
                 ├─► Slice 3 (Tabs)
                 │
                 ├─► Slice 4 (Viewport)
                 │
                 ├─► Slice 5 (TextInput)
                 │
                 ├─► Slice 6 (TextArea)──► Slice 9 (Autocomplete)
                 │
                 ├─► Slice 7 (List)
                 │
                 └─► Slice 11 (Dialog)

Slice 10 (Focus) ──► depends on Slices 1–7 (all components must exist)
Slice 12 (Cleanup) ──► final pass after everything
Slice R (Review) ──► runs after Waves 2, 4, and 6
```

**⚠️ All slices run SEQUENTIALLY in a single worktree.** They all touch `model.go`,
`render.go`, `update.go`, and test files. Parallel execution requires isolated git
worktrees plus an integration agent to merge.

---

## Subagent Execution Strategy

| Wave | Slices | Subagents | Rationale |
|---|---|---|---|
| Wave 1 | 0 (Foundation) | 1 | Prerequisite for everything |
| Wave 2 | 1, 2, 3, 4, 5, 7, 11 | **1 (sequential)** | All touch shared files — cannot parallelize safely in one worktree |
| Wave R1 | R (Review) | 1 | Catch regressions after bulk changes |
| Wave 3 | 6 (TextArea) + gotui API additions | 1 | Complex, needs focus; may need gotui changes |
| Wave 4 | 8, 9 | 1 (sequential) | 9 depends on 6; 8 is trivial and can precede 9 |
| Wave R2 | R (Review) | 1 | Verify textarea integration |
| Wave 5 | 10 (Focus) | 1 | Depends on all components existing |
| Wave 6 | 12 (Cleanup) | 1 | Final pass |
| Wave R3 | R (Review) | 1 | Final comprehensive review |

**Estimated total: 9 waves, ~12 subagent tasks (sequential).** Each wave produces a working
binary with passing tests.

**Alternative (faster):** Use isolated git worktrees for Waves 2–3, then an integration
agent merges and resolves conflicts. This allows true parallelism but adds merge overhead.

---

## Key Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| **textarea API gap** (Slice 6) | High | High | Add `Cursor()` and `ReplaceRange()` to gotui before Slice 6 |
| **Hierarchical list flattening** (Slice 7) | Medium | Medium | Accept uniform styling or embed ANSI in item strings |
| **No horizontal scroll in viewport** (Slice 4) | Medium | Low | Keep horizontal scroll as app-level pre-processing |
| **Focus topology complexity** (Slice 10) | Medium | Medium | Use conditional registration + focus.Scope for modal groups |
| **Sequential execution is slow** (all) | High | Medium | Consider git worktrees for parallelism |
| **Test assertion fragility** (all) | Low | Medium | Use `ansi.Strip()` in test comparisons; review each golden diff |
| **Mouse routing without stored rects** (Slice 4+) | Medium | Medium | Store logRect, editorRect on model; update on resize |

---

## gotui Component Mapping

| nice-llama feature | gotui component | Confidence | Notes |
|---|---|---|---|
| Header panel | `gotui/frame.Panel` + `gotui/stack.Vertical` | ✅ High | |
| Model list | `gotui/list.Model` (flattened) | 🔄 Medium | Uniform styling; embed ANSI for group accents |
| Log view | `gotui/viewport.Model` (+ app-level horiz scroll) | ✅ High | |
| Footer / key hints | `gotui/statusbar.Model` | ✅ High | |
| Bottom view switching | `gotui/tabs.Model` | ✅ High | App maps `/` to `SelectID()`, not `Update()` |
| Text input (name) | `gotui/textinput.Model` | ✅ High | |
| Multi-line editor (args) | `gotui/textarea.Model` | 🔄 Medium | Needs `Cursor()` + `ReplaceRange()` added |
| Panel decorations | `gotui/frame.Panel` | ✅ High | |
| Horizontal split | `gotui/splitpane.Horizontal` | ✅ High | |
| Vertical composition | `gotui/stack.Vertical` | ✅ High | |
| Semantic colors | `gotui.Theme` (18 roles) | ✅ High | |
| Focus management | `gotui/focus.Manager` + `focus.Scope` | ✅ High | Conditional registration per view |
| Confirm delete dialog | `gotui/dialog.Model` (+ `showDialog bool`) | ✅ High | Value type; use bool for visibility |
| Arg completion UI | `gotui/autocomplete.Model` | 🔄 Medium | Uses `SetItems()`, not `SetSuggestions()` |
| Scroll indicator | `gotui/scrollbar.For()` | ✅ High | Returns string, not a Model |
| Key hint bar | `gotui/help.Model` | ✅ High | |
| Mouse routing | `gotui/mouse.InBounds()` | ✅ High | Needs stored `layout.Rect` per pane |

**Legend:** ✅ = direct replacement, 🔄 = adaptation needed
