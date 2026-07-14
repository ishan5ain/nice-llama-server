# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 5 — Text Input (Name Field): Replace with gotui/textinput

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/textinput/textinput.go` — understand Model, New, SetValue, Value, Focus, Blur, Update, View, Prompt, Placeholder, SetSize, Reset
2. `/Users/ishansain/Documents/repos/gotui/examples/autocomplete/main.go` — textinput in a real app

### Task

Replace the single-line `textBuffer` for bookmark names with `gotui/textinput.Model`.

#### Files to touch:
- `internal/tui/editor.go` — replace `name textBuffer` with `textinput.Model`
- `internal/tui/render.go` — update `renderDetailLines()` to call `textinput.View()`
- `internal/tui/update.go` — delegate name-field key handling to `textinput.Update()`
- Test files — update affected tests

#### Step 1: Understand the current code

Read `internal/tui/editor.go` to understand:
- The `bookmarkEditor` struct and its `name textBuffer` field
- How `name` is used in `newBookmarkEditor()`, `Bookmark()`, and rendering
- The `textBuffer` type and its methods that are used for the name field

The `bookmarkEditor` struct likely looks like:
```go
type bookmarkEditor struct {
    name textBuffer
    args textBuffer
    creating bool
    originalName string
}
```

#### Step 2: editor.go changes

Add import:
```go
import "github.com/ishansain/gotui/textinput"
```

Replace `name textBuffer` with `name textinput.Model` in the `bookmarkEditor` struct.

In `newBookmarkEditor()`:
```go
func newBookmarkEditor(bm config.Bookmark, creating bool) *bookmarkEditor {
    e := &bookmarkEditor{
        name: textinput.New(m.theme),  // need theme — pass it as param or use a global
        args: newTextBuffer(bm.Args, true),
        creating: creating,
        originalName: bm.Name,
    }
    e.name.SetValue(bm.Name)
    e.name.SetPrompt("")  // no prompt — field label serves that role
    e.name.Focus()
    return e
}
```

**Important:** `newBookmarkEditor` is called from `model.go` where we have access to `m.theme`. Either:
- Pass `theme gotui.Theme` as a parameter to `newBookmarkEditor`, or
- Pass the theme through the call chain

Map `bookmarkEditor.Bookmark()` to read `textinput.Value()`:
```go
func (e *bookmarkEditor) Bookmark() config.Bookmark {
    return config.Bookmark{
        Name: e.name.Value(),
        Args: e.args.Value(),
    }
}
```

#### Step 3: render.go changes

In `renderDetailLines()` or wherever the name field is rendered, replace:
```go
// Old: nameLines := m.editor.name.RenderLines(nameWidth, 1, m.focus == focusDetailName)
// New:
m.editor.name.SetSize(nameWidth, 1)
nameView := m.editor.name.View()
```

#### Step 4: update.go changes

Delegate name-field key events to `textinput.Update()`:
```go
// When focus is on the name field:
var cmd tea.Cmd
m.editor.name, cmd = m.editor.name.Update(msg)
cmds = append(cmds, cmd)

// Handle Enter: move focus to args
if msg.Text == "Enter" {
    m.editor.name.Blur()
    m.focus = focusDetailArgs
}
```

Drop Ctrl+Z undo for the name field (textinput doesn't have it, and it's acceptable for single-line input).

#### Step 5: Update tests

Update affected tests:
- `TestFocusedBookmarkNameRendersCursor` — now checks textinput cursor
- `TestPasteIntoNameStripsNewlines` — textinput handles paste natively
- `TestPasteIntoNameNormalizesCRLF` — textinput handles paste natively
- `TestPasteIntoNameStripsANSIColorCodes` — textinput handles paste natively
- `TestCtrlZUndoesInEditor / TestCtrlZNoOpWithEmptyUndoStack` — may need adjustment since textinput doesn't have Ctrl+Z
- `TestEnterInNameMovesFocusToArgs` — update for textinput API

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run "TestPaste|TestCtrlZ|TestFocused|TestEnterInName"
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- textinput provides: cursor, horizontal scroll, ctrl+u/k/w editing
- Paste handling is built into textinput via tea.PasteMsg
- You may need to pass the theme to `newBookmarkEditor()` — adapt the call chain
- Do NOT touch files other than editor.go, render.go, update.go, model.go, and test files

## Acceptance Contract
Acceptance level: checked
Completion is not accepted from prose alone. End with a structured acceptance report.

Criteria:
- criterion-1: Implement the requested change without widening scope

Required evidence: changed-files, tests-added, commands-run, residual-risks, no-staged-files

Finish with a fenced JSON block tagged `acceptance-report` in this shape:
Use empty arrays when no items apply; array fields contain strings unless object entries are shown.
```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "specific proof"
    }
  ],
  "changedFiles": [
    "src/file.ts"
  ],
  "testsAddedOrUpdated": [
    "test/file.test.ts"
  ],
  "commandsRun": [
    {
      "command": "command",
      "result": "passed",
      "summary": "short result"
    }
  ],
  "validationOutput": [
    "validation output or concise summary"
  ],
  "residualRisks": [
    "none"
  ],
  "noStagedFiles": true,
  "diffSummary": "short description of the diff",
  "reviewFindings": [
    "blocker: file.ts:12 - issue found, or no blockers"
  ],
  "manualNotes": "anything else the parent should know"
}
```