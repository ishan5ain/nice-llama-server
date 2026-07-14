# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 3 — Bottom View Tabs: Replace toggle with gotui/tabs

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/tabs/tabs.go` — understand Tab, New, SetTabs, SelectID, Focus/Blur, Update, View
   - **Critical:** `tabs.Update()` does NOT handle "/". The app must map "/" to `SelectID()` directly.
2. `/Users/ishansain/Documents/repos/gotui/examples/frame/main.go` — see tabs usage in a real composition

### Task

Replace the `bottomView` enum + manual `/` toggle with `gotui/tabs.Model`.

#### Files to touch:
- `internal/tui/model.go` — add `tabs.Model` field, remove `bottomView` type and field
- `internal/tui/render.go` — replace `m.bottomView` checks with `m.tabs.SelectedID()`
- `internal/tui/update.go` — route "/" key to `m.tabs.SelectID()` (NOT `m.tabs.Update()`)
- `internal/tui/selection.go` — any references to `bottomView`
- All test files — replace `m.bottomView = bottomViewLogs` with `m.tabs.SelectID("logs")`

#### Step 1: model.go

Add import:
```go
import "github.com/ishansain/gotui/tabs"
```

Add `tabs tabs.Model` field to the `model` struct.

Remove:
- `bottomView` type definition and its constants
- The `bottomView` field from the model struct

In `newModel()`, initialize:
```go
tabs: tabs.New(m.theme, tabs.Tab{ID: "bookmarks", Label: "Bookmarks"}, tabs.Tab{ID: "logs", Label: "Logs"}),
```

#### Step 2: render.go

Replace ALL `m.bottomView` checks with `m.tabs.SelectedID()`:
- `m.bottomView == bottomViewLogs` → `m.tabs.SelectedID() == "logs"`
- `m.bottomView == bottomViewBookmarks` → `m.tabs.SelectedID() == "bookmarks"`
- `default:` cases that assumed bookmarks → check explicitly

Also update `renderBottom()` to render the tabs bar above the content area.

#### Step 3: update.go

Route the "/" key to toggle tabs:
```go
case "/":
    if m.tabs.SelectedID() == "bookmarks" {
        m.tabs.SelectID("logs")
    } else {
        m.tabs.SelectID("bookmarks")
    }
    return m, nil
```

NOT `m.tabs.Update(msg)` — tabs.Update() doesn't handle "/".

#### Step 4: selection.go

Check for any `bottomView` references and update them.

#### Step 5: Test files

Find ALL occurrences of `m.bottomView =` and replace:
- `m.bottomView = bottomViewLogs` → `m.tabs.SelectID("logs")`
- `m.bottomView = bottomViewBookmarks` → `m.tabs.SelectID("bookmarks")`

Also update any test that references the `bottomView` type directly.

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- tabs.Update() does NOT handle "/" — the app must map "/" to SelectID() directly
- Search ALL files for bottomView references — don't miss any
- Do NOT touch files other than model.go, render.go, update.go, selection.go, and test files

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