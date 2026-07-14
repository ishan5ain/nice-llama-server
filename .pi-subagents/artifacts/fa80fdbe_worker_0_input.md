# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 1 — Header Panel: Replace with gotui/frame + gotui/stack

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/frame/frame.go` — understand Panel, PanelOptions, PanelContentRect
2. `/Users/ishansain/Documents/repos/gotui/stack/stack.go` — understand Vertical with Gap and Divider options
3. `/Users/ishansain/Documents/repos/gotui/examples/frame/main.go` — see how frame.Panel and stack.Vertical compose

### Task

Rewrite the header rendering to use `gotui/frame.Panel` and `gotui/stack.Vertical`.

#### Files to touch:
- `internal/tui/render.go` — rewrite `renderHeader()`
- `internal/tui/styles.go` — remove header-specific style fields that frame now provides
- `internal/tui/model.go` — may need minor adjustments

#### renderHeader() rewrite

The header currently renders:
- Row 1: title + runtime status + port
- Row 2: stats (bookmark/model/root counts)
- Row 3 (conditional): error or flash message

Replace it with:

```go
func (m *model) renderHeader(width int) string {
    // Build the body using stack.Vertical
    var sections []string
    
    // Row 1: title + status + port
    titleLine := fmt.Sprintf("%s  %s  %s",
        m.styles.headerTitle.Render("Nice Llama Server"),
        m.statusIndicator(),
        m.portInfo(),
    )
    sections = append(sections, titleLine)
    
    // Row 2: stats
    statsLine := m.styles.headerStats.Render(m.statsLine())
    sections = append(sections, statsLine)
    
    // Row 3 (conditional): error or flash message
    if m.errorMessage != "" {
        sections = append(sections, m.styles.headerMessage.Render(m.errorMessage))
    } else if m.flashMessage != "" {
        sections = append(sections, m.styles.headerMessage.Render(m.flashMessage))
    }
    
    body := stack.Vertical(sections...).String()
    
    panel := frame.New(frame.Panel{
        Title: "Nice Llama Server",
        Body:  body,
        Width: width,
    })
    panel.Active = true  // header is always "active"
    
    return panel.Render()
}
```

Note: The actual API may differ slightly. Study the gotui frame package first and adapt accordingly.

#### Key points:
- `frame.Panel` provides rounded borders, title in top border, focused styling
- `stack.Vertical` skips empty sections (no message row when no error/flash)
- Only Theme roles for colors — no literals (already done in Slice 0)
- The header panel height may change by ±2 rows due to frame borders

#### styles.go changes:
Remove these style fields that frame now provides:
- `headerPanel` — frame.Panel handles border styling
- `headerTitle` — frame handles title styling (or keep if you need custom title style)

Keep these styles (they style the BODY content, not the panel):
- `headerStats`
- `headerStatus`
- `headerMessage`

#### Update headerPanelHeight constant

Find where `headerPanelHeight` is defined (likely in render.go or a constants file) and adjust if frame borders change the height. Frame adds 2 rows (top border + bottom border).

#### Tests to update

Look for tests that reference `renderHeader()` or `TestHeader`:
- `TestHeaderRespectsFiveLineCap`
- `TestHeaderOmitsEmptyStatusRowContent`

Update them to match the new frame-based rendering.

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run TestHeader
```

Then run ALL tests:
```bash
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- Read the gotui source files first before implementing
- The exact API of frame.Panel may differ from my pseudocode — adapt accordingly
- Do NOT touch files other than render.go, styles.go, model.go, and test files

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