# Task for reviewer

## Task: Wave R1 — Cross-Cutting Review Pass

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

Review the current state of the migration after Wave 2 (Slices 0-5, 7, 11 are done). Check for issues across these categories:

### 1. MVU Discipline
- Does every `Update` call reassign the model? `m.component, cmd = m.component.Update(msg)`
- Are all returned `Cmd`s collected and batched? `cmds = append(cmds, cmd)`
- No `Update` returns `(m, nil)` when it should return a command

### 2. Focus Wiring
- Tab/Shift+Tab routing — is it still manual or using focus.Manager? (Focus manager comes in Slice 10)
- Focused components show visual focus state (BorderFocused, SelectionBg)
- Hidden controls are NOT in the focus cycle

### 3. Sizing Contracts
- Does every component have `SetSize(width, height)` called on `WindowSizeMsg`?
- Components render exactly within their assigned box
- No component measures the terminal directly

### 4. Mouse Routing
- Mouse wheel events reach `viewport.Update()`
- Wheel events are ignored when the view is not visible

### 5. Color Hygiene
- `grep` for `lipgloss.Color("#` — zero hits outside theme setup
- `grep` for `lipgloss.NewStyle()` with `Foreground`/`Background` that don't reference a theme role

### 6. Dead Code
- `grep` for exported symbols no longer referenced
- `grep` for the old `focusArea` type, `bottomView` type, `confirmDelete` field
- Check for unused imports

### 7. Build Health
- `go build ./cmd/nice-llama-server/`
- `go vet ./internal/tui/...`
- `go test ./internal/tui/...`

### Run these commands and report findings:

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/ 2>&1
go vet ./internal/tui/... 2>&1
go test ./internal/tui/... 2>&1

# Color hygiene
grep -rn 'lipgloss.Color("#[0-9A-Fa-f]' internal/tui/ 2>/dev/null || echo "No color literals"

# Dead code checks
grep -rn 'bottomView' internal/tui/ 2>/dev/null || echo "No bottomView references"
grep -rn 'confirmDelete' internal/tui/ 2>/dev/null || echo "No confirmDelete references"
grep -rn 'logScrollY\|logViewWidth\|logViewHeight' internal/tui/ 2>/dev/null || echo "No old log scroll fields"
grep -rn 'followTail[^E]' internal/tui/ 2>/dev/null || echo "No old followTail field (non-Enabled)"

# Check SetSize calls
grep -rn '\.SetSize(' internal/tui/ 2>/dev/null
```

### Report
Provide a structured report with:
- ✅ Pass items
- ❌ Fail items with file/line references
- Any recommendations for fixing issues found

## Acceptance Contract
Acceptance level: reviewed
Completion is not accepted from prose alone. End with a structured acceptance report.

Criteria:
- criterion-1: Implement the requested change without widening scope
- criterion-2: Return evidence sufficient for an independent acceptance review

Required evidence: changed-files, tests-added, commands-run, validation-output, residual-risks, no-staged-files

Review gate: required by reviewer.

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