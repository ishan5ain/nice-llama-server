# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 2 — Footer: Replace with gotui/statusbar

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/statusbar/statusbar.go` — understand Model, Segment, Kind, SetLeft, SetRight, New, View, SetSize
2. `/Users/ishansain/Documents/repos/gotui/examples/statusbar/main.go` — canonical single-component wiring

### Task

Replace the manual footer key hints with `statusbar.Model`.

#### Files to touch:
- `internal/tui/model.go` — add `statusbar.Model` field
- `internal/tui/render.go` — rewrite `renderFooter()` to delegate to statusbar
- `internal/tui/update.go` — call `statusbar.SetSize()` on `tea.WindowSizeMsg`
- `internal/tui/styles.go` — remove footer-related style fields

#### Step 1: model.go

Add import for gotui/statusbar:
```go
import "github.com/ishansain/gotui/statusbar"
```

Add a `footer statusbar.Model` field to the `model` struct.

In `newModel()`, initialize it:
```go
footer: statusbar.New(m.theme),
```

#### Step 2: render.go — rewrite renderFooter()

Instead of building the footer string manually, call `m.footer.View()`.

The footer should have:
- **Left segments:** keybinding hints (context-dependent: bookmarks view vs logs view vs editor mode)
- **Right segment:** tail indicator (green when active, muted when paused)

Use `statusbar.Segment` with appropriate `Kind`:
- Keybinding labels: `KindAccent` (bold badge)
- Key descriptions: `KindNormal`
- Tail indicator active: `KindSuccess`
- Tail indicator paused: `KindMuted`

```go
func (m *model) renderFooter(width int) string {
    m.footer.SetSize(width, 1)
    
    var leftSegments []statusbar.Segment
    
    if m.editor != nil {
        // Editor mode
        leftSegments = []statusbar.Segment{
            {Text: "Tab", Kind: statusbar.KindAccent},
            {Text: " complete  ", Kind: statusbar.KindNormal},
            {Text: "Esc", Kind: statusbar.KindAccent},
            {Text: " cancel  ", Kind: statusbar.KindNormal},
            {Text: "Enter", Kind: statusbar.KindAccent},
            {Text: " save", Kind: statusbar.KindNormal},
        }
    } else if m.bottomView == bottomViewBookmarks {
        leftSegments = []statusbar.Segment{
            {Text: "↑↓", Kind: statusbar.KindAccent},
            {Text: " navigate  ", Kind: statusbar.KindNormal},
            {Text: "n", Kind: statusbar.KindAccent},
            {Text: " new  ", Kind: statusbar.KindNormal},
            {Text: "d", Kind: statusbar.KindAccent},
            {Text: " delete  ", Kind: statusbar.KindNormal},
            {Text: "Enter", Kind: statusbar.KindAccent},
            {Text: " edit", Kind: statusbar.KindNormal},
        }
    } else {
        // Logs view
        leftSegments = []statusbar.Segment{
            {Text: "↑↓", Kind: statusbar.KindAccent},
            {Text: " scroll  ", Kind: statusbar.KindNormal},
            {Text: "t", Kind: statusbar.KindAccent},
            {Text: " tail", Kind: statusbar.KindNormal},
        }
    }
    
    m.footer.SetLeft(leftSegments...)
    
    // Right segment: tail indicator
    if m.followTail && m.followTailEnabled {
        m.footer.SetRight(statusbar.Segment{Text: "[Tail]", Kind: statusbar.KindSuccess})
    } else {
        m.footer.SetRight(statusbar.Segment{Text: "[tail]", Kind: statusbar.KindMuted})
    }
    
    return m.footer.View()
}
```

Note: The exact API may differ — study the gotui statusbar source first and adapt accordingly.

#### Step 3: update.go

In the `tea.WindowSizeMsg` handler, add:
```go
m.footer.SetSize(msg.Width, 1)
```

#### Step 4: styles.go

Remove these footer-related style fields:
- `footerPanel`
- `footerKey`

Also remove `tailIndicator` and `tailIndicatorPaused` if they're only used in the footer (statusbar handles them now).

#### Step 5: Update tests

Look for tests referencing the footer:
- `TestFooterChangesByContext`

Update them to use the statusbar API.

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run TestFooter
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- Read the gotui source files first before implementing
- The exact API of statusbar may differ from my pseudocode — adapt accordingly
- Do NOT touch files other than model.go, render.go, update.go, styles.go, and test files

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