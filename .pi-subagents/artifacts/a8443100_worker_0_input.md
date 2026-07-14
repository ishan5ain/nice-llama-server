# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 6 — Text Area (Args Editor): Replace with gotui/textarea

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/textarea/textarea.go` — understand Model, New, SetValue, Value, Focus, Blur, Update, View, ContentHeight, Prompt, Placeholder, Cursor(), ReplaceRange()
2. `/Users/ishansain/Documents/repos/gotui/textarea/history.go` — understand `ReplaceRange(start, end Position, value string)` and `Position{Row, Col}`

### Key API changes already made to gotui

I've already added `Cursor() (row, col int)` to gotui/textarea. The existing `ReplaceRange(start, end Position, value string)` takes `Position{Row, Col}` arguments — this is DIFFERENT from the old `textBuffer.ReplaceRange(row, start, end int, text string)`.

### Task

Replace the multi-line `textBuffer` for bookmark args with `gotui/textarea.Model`.

#### Files to touch:
- `internal/tui/editor.go` — replace `args textBuffer` with `textarea.Model`
- `internal/tui/render.go` — update `renderArgsEditorLines()` to call `textarea.View()`
- `internal/tui/update.go` — delegate args key handling to `textarea.Update()`
- `internal/tui/args_completion.go` — adapt completion engine to use textarea APIs
- `internal/tui/model.go` — update `newBookmarkEditor` call chain
- Test files — update affected tests

#### Step 1: editor.go changes

Add import:
```go
import "github.com/ishansain/gotui/textarea"
```

Replace `args textBuffer` with `args textarea.Model` in the `bookmarkEditor` struct.

In `newBookmarkEditor()`:
```go
func newBookmarkEditor(b config.Bookmark, isNew bool, theme gotui.Theme) *bookmarkEditor {
    e := &bookmarkEditor{
        originalID:  b.ID,
        isNew:       isNew,
        modelPath:   b.ModelPath,
        groupKey:    b.GroupKey,
        initialName: strings.TrimSpace(b.Name),
        initialArgs: strings.TrimSpace(b.ArgsText),
        name:        textinput.New(theme),
        args:        textarea.New(theme),
    }
    e.name.SetValue(b.Name)
    e.name.Prompt = ""
    e.args.SetValue(b.ArgsText)
    e.args.Prompt = ""
    return e
}
```

Map `bookmarkEditor.Bookmark()` to read `textarea.Value()`:
```go
func (e *bookmarkEditor) Bookmark() config.Bookmark {
    return config.Bookmark{
        Name: e.name.Value(),
        Args: e.args.Value(),
    }
}
```

Remove the `textBuffer` type and all its methods from editor.go (they're no longer needed).

#### Step 2: args_completion.go — adapt completion engine

The completion engine uses:
1. `m.editor.args.TokenAtCursor()` → replace with `m.editor.args.Cursor()` + standalone token scanner
2. `m.editor.args.ReplaceRange(row, start, end, text)` → adapt to `m.editor.args.ReplaceRange(textarea.Position{Row: row, Col: start}, textarea.Position{Row: row, Col: end}, text)`

**Token scanning:** Extract `scanLineTokens()` and `scanBufferTokens()` as standalone functions that operate on a string (from `textarea.Value()`) instead of a `textBuffer`. The `tokenContext` struct stays the same shape.

```go
// Standalone token scanner for textarea
func scanLineTokens(line string) []lineToken {
    // Same logic as before but operates on a string
}

func scanBufferTokens(value string) []bufferToken {
    // Same logic but operates on the full value string
}

// Replace TokenAtCursor with:
func tokenAtCursor(value string, cursorRow, cursorCol int) tokenContext {
    // Scan the value string at the cursor position
}
```

The `argCompletionState` struct and its methods (`beginCompletion`, `applyCompletion`, `cancelCompletion`) need updating:
- `beginCompletion` should call the standalone token scanner
- `applyCompletion` should use `textarea.ReplaceRange(Position{...}, Position{...}, text)`

#### Step 3: render.go

In `renderDetailLines()` or wherever the args editor is rendered, replace:
```go
// Old: argsLines := m.editor.args.RenderLines(argsWidth, argsHeight, m.focus == focusDetailArgs)
// New:
m.editor.args.SetSize(argsWidth, argsHeight)
argsView := m.editor.args.View()
```

#### Step 4: update.go

Delegate args key events to `textarea.Update()`:
```go
// When focus is on the args field:
var cmd tea.Cmd
m.editor.args, cmd = m.editor.args.Update(msg)
cmds = append(cmds, cmd)
```

Handle Ctrl+Z via textarea's built-in undo:
```go
case msg.Text == "ctrl+z":
    m.editor.args.Undo()
```

Handle Tab/Shift+Tab before textarea (for completion).

#### Step 5: Update tests

Update all editor tests and completion tests:
- Tests that reference `textBuffer` directly
- Tests that reference `TokenAtCursor`
- Tests that reference `ReplaceRange` on textBuffer

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run "TestTab|TestCompletion|TestMMProj|TestPassive|TestTextBuffer|TestEditor"
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- textarea provides: soft-wrap, undo/redo, selections, word movement, kill ring
- Completion engine LOGIC stays untouched — only the API calls change
- `textarea.ReplaceRange` takes `(start, end Position, value string)` where `Position{Row, Col}`
- `textarea.Cursor()` returns `(row, col int)`
- Do NOT touch files other than editor.go, render.go, update.go, args_completion.go, model.go, and test files

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