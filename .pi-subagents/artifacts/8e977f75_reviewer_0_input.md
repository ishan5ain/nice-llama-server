# Task for reviewer

## Task: Wave R3 — Final Comprehensive Review

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Review Checklist

Run these commands and report findings:

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/ 2>&1
go vet ./internal/tui/... 2>&1
go test ./internal/tui/... 2>&1
```

### 1. MVU Discipline
- Does every `Update` call reassign the model? `m.component, cmd = m.component.Update(msg)`
- Are all returned `Cmd`s collected and batched? `cmds = append(cmds, cmd)`
- No `Update` returns `(m, nil)` when it should return a command

### 2. Focus Wiring
- Tab/Shift+Tab routes through `focus.Manager` and `focus.Scope`
- `Focus()` is called when a component becomes active
- `Blur()` is called when a component loses focus
- Focused components show visual focus state (BorderFocused, SelectionBg)
- Hidden controls are NOT in the focus cycle (conditional registration)
- Autocomplete IS in the focus cycle when visible

### 3. Sizing Contracts
- Every component has `SetSize(width, height)` called on `WindowSizeMsg`
- Components render exactly within their assigned box
- No component measures the terminal directly

### 4. Mouse Routing
- Mouse wheel events reach `viewport.Update()`
- Wheel events are ignored when the view is not visible

### 5. Color Hygiene
- `grep -rn 'lipgloss.Color("#[0-9A-Fa-f]' internal/tui/` — zero hits
- All Foreground/Background reference a theme role

### 6. Dead Code
- No `focusArea`, `bottomView`, `confirmDelete`, `textBuffer`, `followTail` (non-Enabled) references
- No unused imports

### 7. Scrollbar Integration
- Scrollbar rendered alongside log viewport
- Uses `scrollbar.For(theme, viewport)` which returns a string

### Report
Provide a structured report with:
- ✅ Pass items
- ❌ Fail items with file/line references
- Any recommendations for fixing issues found

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