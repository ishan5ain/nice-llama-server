# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 4 — Log View: Replace with gotui/viewport

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/viewport/viewport.go` — understand Model, New, SetContent, ScrollBy, AtBottom, GotoBottom, Focus/Blur, Update, View, SetSize, LineDown, LineUp, HalfViewDown, HalfViewUp, PageDown, PageUp
2. `/Users/ishansain/Documents/repos/gotui/scrollbar/scrollbar.go` — understand Scrollable interface (for later, not this slice)
3. `/Users/ishansain/Documents/repos/gotui/mouse/mouse.go` — understand InBounds

### Task

Replace the custom log viewport with `gotui/viewport.Model`.

#### Files to touch:
- `internal/tui/model.go` — add `viewport.Model` field, remove `logScrollY`, `logViewWidth`, `logViewHeight`, `followTail`, `followTailEnabled` (keep `logScrollX` for horizontal scroll)
- `internal/tui/render.go` — rewrite `renderLogView()` to format entries then set on viewport
- `internal/tui/update.go` — delegate scroll keys and mouse wheel to viewport
- `internal/tui/scroll.go` — DELETE entirely (viewport owns scroll logic)
- `internal/tui/styles.go` — remove log timestamp styles (timestamps are pre-formatted in content string)

#### Step 1: model.go

Add import:
```go
import "github.com/ishansain/gotui/viewport"
```

Add `logView viewport.Model` field to the `model` struct.

Add `logRect layout.Rect` field (store the viewport's bounding box for mouse routing).

Remove these fields:
- `logScrollY`
- `logViewWidth`
- `logViewHeight`
- `followTail` (viewport tracks this via AtBottom())
- `followTailEnabled` (keep this as app-level preference)

Keep:
- `logScrollX` (horizontal scroll stays as app-level pre-processing)

In `newModel()`, initialize:
```go
logView: viewport.New(m.theme),
```

In `Init()`, no changes needed (viewport starts empty).

#### Step 2: render.go — rewrite renderLogView()

```go
func (m *model) renderLogView(width, height int) string {
    // Format each log entry as "TIMESTAMP LINE" with stream-colored styling
    var lines []string
    for _, entry := range m.logs {
        tsStyle := m.logTimestampStyle(entry.Stream)
        line := tsStyle.Render(entry.Timestamp.Format("15:04:05.000")) + " " + entry.Line
        lines = append(lines, line)
    }
    
    // Apply horizontal scroll (app-level preprocessing)
    content := ansi.Wordwrap(strings.Join(lines, "\n"), width)
    if m.logScrollX > 0 {
        // Truncate each line by logScrollX
        // (existing logic from the current renderLogView)
    }
    
    m.logView.SetSize(width, height)
    m.logView.SetContent(content)
    
    // Follow-tail: if at bottom, auto-scroll
    if m.followTailEnabled && m.logView.AtBottom() {
        m.logView.GotoBottom()
    }
    
    return m.logView.View()
}
```

Note: The exact API may differ — study the gotui viewport source first and adapt accordingly.

#### Step 3: update.go

Delegate scroll keys to viewport:
```go
// In handleLogKey:
case msg.Text == "↑" || msg.Text == "k":
    m.logView.LineUp()
    m.followTailEnabled = false
    return m, nil
case msg.Text == "↓" || msg.Text == "j":
    m.logView.LineDown()
    m.followTailEnabled = false
    return m, nil
case msg.Text == "PgUp":
    m.logView.PageUp()
    m.followTailEnabled = false
    return m, nil
case msg.Text == "PgDown":
    m.logView.PageDown()
    m.followTailEnabled = false
    return m, nil
case msg.Text == "Home":
    m.logView.GotoTop()
    m.followTailEnabled = false
    return m, nil
case msg.Text == "End":
    m.logView.GotoBottom()
    m.followTailEnabled = true
    return m, nil
case msg.Text == "t":
    m.followTailEnabled = !m.followTailEnabled
    if m.followTailEnabled {
        m.logView.GotoBottom()
    }
    return m, nil
```

Mouse wheel routing:
```go
func (m *model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
    if m.tabs.SelectedID() != "logs" {
        return m, nil
    }
    // Route wheel to viewport
    var cmd tea.Cmd
    m.logView, cmd = m.logView.Update(msg)
    m.followTailEnabled = false
    return m, cmd
}
```

On `tea.WindowSizeMsg`, call `m.logView.SetSize(width, height)`.

#### Step 4: scroll.go

DELETE this file entirely. Viewport owns all scroll logic.

#### Step 5: styles.go

Remove log timestamp styles:
- `logTimestamp`
- `logTimestampStdout`
- `logTimestampStderr`
- `logTimestampSystem`
- `logTimestampStreamed`

Keep the `logTimestampStyle(stream)` helper method if it exists (it selects the right style based on stream type).

#### Step 6: Update tests

Update all log view tests to use viewport API:
- Find tests that reference `m.logScrollY`, `m.logViewWidth`, `m.logViewHeight`
- Find tests that reference `renderLogLines()` or `renderLogView()`
- Update them to use `m.logView` API

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run TestLog
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- Horizontal scroll stays as app-level pre-processing (viewport doesn't support it natively)
- Follow-tail uses viewport.AtBottom() + GotoBottom()
- Mouse wheel is routed through viewport.Update()
- Do NOT touch files other than model.go, render.go, update.go, scroll.go, styles.go, and test files

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