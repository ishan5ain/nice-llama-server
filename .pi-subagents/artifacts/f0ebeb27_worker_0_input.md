# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 8 — Split Pane Layout: Replace with gotui/splitpane

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Before implementing

Study these gotui source files:
1. `/Users/ishansain/Documents/repos/gotui/splitpane/splitpane.go` — understand Horizontal, Options, Ratio, Gap
2. `/Users/ishansain/Documents/repos/gotui/examples/frame/main.go` — splitpane usage

### Task

Replace the manual width splitting with `splitpane.Horizontal`.

#### Files to touch:
- `internal/tui/render.go` — rewrite `renderBookmarkEditorView()` using splitpane
- `internal/tui/render.go` — remove `splitBookmarkEditorWidths()` helper

#### Step 1: Study the current code

Read the current `renderBookmarkEditorView()` and `splitBookmarkEditorWidths()` functions in render.go.

#### Step 2: Rewrite renderBookmarkEditorView()

```go
func (m *model) renderBookmarkEditorView(width, height int) string {
    gap := 1
    leftWidth, rightWidth := splitpane.Horizontal(width, splitpane.Options{
        Ratio: 0.4,  // 40% left, 60% right
        Gap:   gap,
    })

    left := m.renderModelListPanel(leftWidth, height)
    right := m.renderDetailPanel(rightWidth, height)
    joined := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
    return joined
}
```

Note: The exact API may differ — study the gotui splitpane source first and adapt accordingly.

#### Step 3: Remove splitBookmarkEditorWidths()

Delete the `splitBookmarkEditorWidths()` helper function entirely.

#### Step 4: Update tests

Update `TestBookmarkEditorViewFillsExactBottomRegion` and `TestBookmarkEditorViewFillsOnNarrowWidths` if needed.

### Verification

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/... -run TestBookmarkEditorView
go test ./internal/tui/...
```

### ⚠️ Important
- Only Theme roles for colors — no literals
- Splitpane handles width allocation and divider rendering
- Do NOT touch files other than render.go and test files

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