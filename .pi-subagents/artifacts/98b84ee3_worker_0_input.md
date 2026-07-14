# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 11 — Delete Confirmation Dialog: Use gotui/dialog

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/dialog/dialog.go` — understand Model, New, Update, View, ResultMsg, Title, Body, ConfirmLabel, CancelLabel
   - **Critical:** `dialog.Model` is a VALUE TYPE. Use a separate `showDialog bool` for visibility.
   - **Critical:** `dialog.Body` (not "Message") holds the prompt text.
2. `/Users/ishansain/Documents/repos/gotui/examples/demo/main.go` — dialog in a real app

### Task

Replace the manual confirm-delete flow (y/n key prompt) with `gotui/dialog.Model`.

#### Files to touch:
- `internal/tui/model.go` — add `dialog.Model` field + `showDialog bool`, remove `confirmDelete bool`
- `internal/tui/render.go` — render dialog overlay when `showDialog` is true
- `internal/tui/update.go` — delegate 'd' key to open dialog, handle `dialog.ResultMsg`
- Test files — update affected tests

#### Step 1: model.go

Add import:
```go
import "github.com/ishansain/gotui/dialog"
```

Add fields to the `model` struct:
```go
deleteDialog dialog.Model
showDialog   bool
```

Remove:
- `confirmDelete bool` field

In `newModel()`, initialize:
```go
deleteDialog: dialog.New(m.theme),
```

#### Step 2: update.go

When user presses 'd' on a bookmark:
```go
case msg.Text == "d":
    if selected := m.selectedBookmark(); selected != nil {
        m.deleteDialog.Title = "Delete Bookmark"
        m.deleteDialog.Body = fmt.Sprintf("Delete %q?", selected.Name)
        m.deleteDialog.ConfirmLabel = "Delete"
        m.deleteDialog.CancelLabel = "Cancel"
        m.showDialog = true
    }
    return m, nil
```

Handle dialog result:
```go
// In the Update switch:
case dialog.ResultMsg:
    if m.showDialog {
        m.showDialog = false
        if msg.OK {
            return m, deleteBookmarkCmd(m.ctx, m.client, m.selectedBookmark().ID)
        }
    }
    return m, nil
```

When dialog is showing, delegate ALL keys to dialog:
```go
// At the top of handleKey or the general Update:
if m.showDialog {
    var cmd tea.Cmd
    m.deleteDialog, cmd = m.deleteDialog.Update(msg)
    return m, cmd
}
```

#### Step 3: render.go

When `m.showDialog` is true, render the dialog as an overlay:
```go
func (m *model) render() string {
    mainView := m.renderMain()
    if m.showDialog {
        return overlay.Center(m.theme, m.deleteDialog.View(), m.width, m.height)
    }
    return mainView
}
```

Or simpler — just render the dialog on top:
```go
if m.showDialog {
    return m.deleteDialog.View()
}
```

Check the gotui overlay package for the recommended approach.

#### Step 4: Update tests

Remove y/n key handling from tests that check `m.confirmDelete`.

Update tests that reference `confirmDelete`:
- Replace `m.confirmDelete = true` with `m.showDialog = true`
- Update test expectations for dialog rendering

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run TestDelete|TestConfirm
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- dialog.Model is a VALUE TYPE — do NOT use *dialog.Model
- dialog.Body (not "Message") holds the prompt text
- Dialog is centered and modal (blocks all other input)
- Esc cancels, Enter confirms
- Do NOT touch files other than model.go, render.go, update.go, and test files

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