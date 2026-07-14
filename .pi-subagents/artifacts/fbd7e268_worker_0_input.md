# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 9 — Arg Completion UI: Wire gotui/autocomplete

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/autocomplete/autocomplete.go` — understand Model, New, SetItems (NOT SetSuggestions), Selected, SelectedItem, Focus, Blur, Update, View, SelectedMsg
2. `/Users/ishansain/Documents/repos/gotui/examples/autocomplete/main.go` — canonical usage pattern
   - Note: autocomplete emits `SelectedMsg` on activation; the app handles insertion

### Task

Replace the inline ghost completion rendering with `gotui/autocomplete.Model` suggestion window.

#### Files to touch:
- `internal/tui/model.go` — add `autocomplete.Model` field
- `internal/tui/render.go` — replace ghost rendering with autocomplete view
- `internal/tui/update.go` — delegate tab/shift-tab to autocomplete
- `internal/tui/args_completion.go` — adapt completion engine to feed candidates to autocomplete
- `internal/tui/styles.go` — remove `completionGhost` style (handled by autocomplete)

#### Step 1: model.go

Add import:
```go
import "github.com/ishan5ain/tuiweave/autocomplete"
```

Add `ac autocomplete.Model` field to the `model` struct.

In `newModel()`, initialize:
```go
ac: autocomplete.New(m.theme),
```

#### Step 2: render.go

Replace the inline ghost completion rendering with `autocomplete.View()`:
```go
// Instead of rendering ghost text inline:
// Old: completionView := m.styles.completionGhost.Render("  " + strings.Join(labels, "  "))
// New:
if m.ac.IsActive() {
    acView := m.ac.View()
    // Position below the args editor
    ...
}
```

The autocomplete view should be positioned below the args editor. You can use `lipgloss.JoinVertical` to stack the editor and autocomplete.

#### Step 3: update.go

On Tab/Shift+Tab:
```go
case msg.Text == "Tab", msg.Text == "shift+tab":
    if m.focus == focusDetailArgs && m.editor != nil {
        // Compute candidates from completion engine
        candidates := computeCompletionCandidates(m)
        if len(candidates) > 0 {
            m.ac.SetItems(candidates...)
            m.ac.Focus()
        }
    }
    return m, nil
```

On Esc:
```go
case msg.Text == "esc":
    if m.ac.IsActive() {
        m.ac.Blur()
        return m, nil
    }
```

Handle `autocomplete.SelectedMsg`:
```go
case autocomplete.SelectedMsg:
    if m.ac.IsActive() {
        selected := m.ac.SelectedItem()
        // Insert the selected value into textarea
        insertCompletion(m, selected)
        m.ac.Blur()
    }
    return m, nil
```

#### Step 4: args_completion.go

Adapt the completion engine to return `autocomplete.Item` slices:
```go
func computeCompletionCandidates(m *model) []autocomplete.Item {
    // Existing completion logic...
    // Instead of modifying textarea inline, return autocomplete.Item slice
    var items []autocomplete.Item
    for _, candidate := range candidates {
        items = append(items, autocomplete.Item{
            ID:   candidate.Text,
            Text: candidate.Text,
        })
    }
    return items
}
```

Keep the completion engine LOGIC (catalog parsing, popularity, mmproj) — it stays as app logic. Only change the OUTPUT format.

#### Step 5: styles.go

Remove `completionGhost` style field — autocomplete handles its own styling.

#### Step 6: Update tests

Update rendering tests:
- `TestArgsCompletionRendersInlineGhostOptions` → now checks autocomplete suggestion window
- `TestMMProjCompletionRendersInlineGhostOptions` → update
- `TestPassiveSingleHyphenCompletionRendersInlineGhostOptions` → update
- All `TestTab*` / `TestShiftTab*` tests → update

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run "TestTab|TestCompletion|TestMMProj|TestPassive|TestArgs"
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- `autocomplete.SetItems()` — NOT `SetSuggestions()`
- Completion candidates appear in a suggestion window below the args editor
- User can arrow through candidates and press Enter to select
- Completion engine LOGIC stays untouched — only the output format changes
- Do NOT touch files other than model.go, render.go, update.go, args_completion.go, styles.go, and test files

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