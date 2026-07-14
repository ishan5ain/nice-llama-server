# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 7 — Model List: Replace with gotui/list

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/list/list.go` — understand Model, New, SetItems, SetFilter, Selected, SelectedItem, Focus, Blur, Update, View, SetSize
2. `/Users/ishansain/Documents/repos/gotui/examples/browser/main.go` — list in a real app (file list)

### Task

Replace the custom model list rendering with `gotui/list.Model`.

**⚠️ List styling is uniform.** `list.Model` applies one `selectedStyle` to all items. "Groups bold and accent-colored" requires embedding ANSI escape codes in item strings.

#### Files to touch:
- `internal/tui/model.go` — add `list.Model` field
- `internal/tui/render.go` — replace `renderListLines()` with `list.View()`
- `internal/tui/update.go` — delegate list navigation keys to `list.Update()`
- `internal/tui/selection.go` — adapt `listItems()` to produce flat strings
- Test files — update affected tests

#### Step 1: model.go

Add import:
```go
import "github.com/ishansain/gotui/list"
```

Add `modelList list.Model` field to the `model` struct.

In `newModel()`, initialize:
```go
modelList: list.New(m.theme),
```

On `WindowSizeMsg`, call:
```go
m.modelList.SetSize(listWidth, listHeight)
```

#### Step 2: selection.go — adapt listItems()

The current `listItems()` returns a hierarchical structure. Flatten it:

```go
func (m *model) listItems() []list.Item {
    var items []list.Item
    for _, group := range m.snapshot.Groups {
        // Group header with embedded ANSI for bold accent styling
        groupLabel := "\033[1;38;2;122;162;247m▶ " + group.Label + "\033[0m"
        items = append(items, list.Item{ID: group.ID, Text: groupLabel})
        
        for _, bm := range group.Bookmarks {
            prefix := "  ▸ "
            if bm.ID == m.snapshot.Runtime.ActiveBookmarkID {
                prefix = "  ▸ "
            }
            items = append(items, list.Item{ID: bm.ID, Text: prefix + bm.Name})
        }
    }
    return items
}
```

Note: The exact `list.Item` struct may differ — study the gotui list source first.

Map the old `selectedKey` tracking to `list.Selected()`:
- When the user navigates, `m.modelList.Selected()` returns the index
- Use that index to look up the corresponding bookmark/group

#### Step 3: render.go

Replace `renderListLines()` with:
```go
func (m *model) renderModelList(width, height int) string {
    m.modelList.SetSize(width, height)
    m.modelList.SetItems(m.listItems()...)
    return m.modelList.View()
}
```

#### Step 4: update.go

Delegate up/down navigation keys to `list.Update()`:
```go
// When focus is on the model list:
var cmd tea.Cmd
m.modelList, cmd = m.modelList.Update(msg)
cmds = append(cmds, cmd)
```

#### Step 5: Update tests

Update affected tests:
- `TestListItemsGroupsBookmarksByModelPath`
- `TestListItemsUsesPathFallbackForMissingModel`
- `TestNewBookmarkUsesCurrentModelGroup`
- `TestBookmarkEditorViewFillsExactBottomRegion`
- `TestBookmarkEditorViewFillsOnNarrowWidths`

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run "TestList|TestNewBookmark|TestBookmarkEditorView"
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- Flattened hierarchy: groups and bookmarks are all flat items with distinguishing prefixes
- Group labels use embedded ANSI for bold/accent (\033[1;38;2;122;162;247m)
- list.Selected() returns original index into the flat items array
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