# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 10 — Focus Management: Use gotui/focus

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/focus/focus.go` — understand Manager, NewManager, Next, Prev, Apply, Focusable interface
2. `/Users/ishansain/Documents/repos/gotui/focus/scope.go` — understand Scope, Enter, Exit, Active, Apply, ApplyBackground
3. `/Users/ishansain/Documents/repos/gotui/examples/frame/main.go` — focus.Manager usage

### Focus Topology

**Bookmarks view active (no editor):**
  tabs → list → viewport (skipped, not visible)

**Bookmarks view active (editor open):**
  tabs → list → textinput → textarea → autocomplete (when visible)

**Logs view active:**
  tabs → viewport

**Dialog showing:**
  Block all input (handled separately in update.go)

Use `focus.Scope` for the editor group (textinput + textarea + autocomplete) so they are skipped when the editor is closed.

### Task

Replace manual focus tracking with `gotui/focus.Manager` + conditional scopes.

#### Files to touch:
- `internal/tui/model.go` — add `focus.Manager` and `focus.Scope` fields, remove `focusArea` enum and `focus` field
- `internal/tui/update.go` — delegate Tab/Shift+Tab to focus manager
- All test files — replace `m.focus = focusDetailName` with component `.Focus()` calls

#### Step 1: model.go

Add import:
```go
import "github.com/ishan5ain/tuiweave/focus"
```

Add fields to the `model` struct:
```go
fm      focus.Manager
editorScope focus.Scope
```

Remove:
- `focusArea` type and its constants (`focusModelList`, `focusDetailName`, `focusDetailArgs`)
- `focus focusArea` field from the model struct

In `newModel()`, initialize:
```go
fm: focus.NewManager(3), // tabs, list, viewport (max components)
editorScope: focus.NewScope(3), // textinput, textarea, autocomplete
```

#### Step 2: update.go — Tab/Shift+Tab handling

Replace the manual focus switching with focus.Manager:

```go
case msg.Text == "Tab":
    if m.editorScope.Active() {
        m.editorScope.Next()
        m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
    } else {
        m.fm.Next()
        m.applyFocus()
    }
    return m, nil

case msg.Text == "shift+tab":
    if m.editorScope.Active() {
        m.editorScope.Prev()
        m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
    } else {
        m.fm.Prev()
        m.applyFocus()
    }
    return m, nil
```

Create an `applyFocus()` helper:
```go
func (m *model) applyFocus() {
    if m.tabs.SelectedID() == "logs" {
        m.fm.Apply(&m.tabs, &m.logView)
    } else if m.editor != nil {
        m.editorScope.Enter(m.fm)
        m.fm.Apply(&m.tabs, &m.modelList)
        m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
    } else {
        m.fm.Apply(&m.tabs, &m.modelList)
    }
}
```

When entering editor mode (in selection.go or update.go):
```go
m.editorScope.Enter(m.fm)
m.applyFocus()
```

When exiting editor mode:
```go
m.editorScope.Exit(&m.fm)
m.applyFocus()
```

#### Step 3: Update tests

Replace ALL occurrences of `m.focus = focusDetailName` with `m.editor.name.Focus()`
Replace ALL occurrences of `m.focus = focusDetailArgs` with `m.editor.args.Focus()`
Replace ALL occurrences of `m.focus = focusModelList` with `m.modelList.Focus()`

Also update any test that references the `focusArea` type directly.

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run "TestFocus|TestEnter|TestUp|TestSave|TestEsc"
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- Hidden controls (log viewport when bookmarks active) are NOT in the focus cycle
- Autocomplete IS in the focus cycle when visible
- focus.Manager is a VALUE TYPE — safe to embed in model
- focus.Scope is a VALUE TYPE — safe to embed in model
- Do NOT touch files other than model.go, update.go, selection.go, and test files

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