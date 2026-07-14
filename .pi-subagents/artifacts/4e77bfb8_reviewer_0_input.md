# Task for reviewer

## Review the nice-llama-server gotui migration against library conventions

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui library:** /Users/ishansain/Documents/repos/gotui (module: github.com/ishan5ain/tuiweave)  
**Agent kit:** /Users/ishansain/Documents/repos/gotui-agent-kit  

### Convention sources (read these first)

1. **Library rules:** `/Users/ishansain/Documents/repos/gotui/AGENTS.md` — hard rules, component contracts, focus, sizing, MVU discipline
2. **Design rationale:** `/Users/ishansain/Documents/repos/gotui/DESIGN.md` — architectural decisions, theme roles, package architecture
3. **Agent contract:** `/Users/ishansain/Documents/repos/gotui/AGENT-CONTRACT.md` — workflow contract, compatibility, verification
4. **Review playbook:** `/Users/ishansain/Documents/repos/gotui-agent-kit/playbooks/review.md` — audit checklist
5. **Quality gates:** `/Users/ishansain/Documents/repos/gotui-agent-kit/playbooks/quality-gates.md` — pre-handoff checks

### Audit checklist (from review playbook)

1. ✅ All colors use `tuiweave.Theme` roles — no literals
2. ✅ App code does not import Ultraviolet
3. ✅ Layout rectangles are app-owned and bounded components receive `SetSize`
4. ✅ Each component `Update` return model is reassigned and each command collected
5. ✅ Fresh component addresses are applied after copied focus-model updates
6. ✅ Wheel messages reach scrollables even while blurred and use layout bounds when multiple panes exist
7. ✅ Modal command results are delivered and visibility remains app-owned
8. ✅ Narrow, empty, focused, and scrolled states behave intentionally
9. ✅ Referenced tuiweave API names exist at the pinned revision

### Specific things to verify

**Hard rules from AGENTS.md:**
- Rule 1: Colors come from Theme roles — grep for `lipgloss.Color("#` in internal/tui/
- Rule 2: No ultraviolet imports in app code
- Rule 3: Every bounded component sizes itself only via `SetSize(w, h)`
- Rule 4: MVU discipline — always reassign model, always collect cmd
- Rule 5: Snapshot/visual changes verified (note: this app uses string-based tests, not snaptest)

**Component-specific checks:**
- `frame.Panel` usage — correct API, title in border, natural height
- `tabs.Model` — `SelectID()` used for "/" key, NOT `tabs.Update()`
- `viewport.Model` — wheel events forwarded even when blurred
- `textarea.Model` — `CursorPosition()` and `ReplaceRange()` used correctly
- `autocomplete.Model` — `SetItems()` not `SetSuggestions()`
- `dialog.Model` — value type, `Body` not `Message`, `showDialog` bool for visibility
- `scrollbar.For(theme, component)` — returns string, not a Model
- `focus.Manager` — value type, fresh addresses every time
- `focus.Scope` — `ApplyBackground` for background blurring when scope active

**Focus topology:**
- Bookmarks view: tabs → list → textinput → textarea → autocomplete (when visible)
- Logs view: tabs → viewport
- Editor scope uses `focus.Scope` for conditional registration
- Autocomplete in focus cycle when visible

**Sizing:**
- `SetSize` called on `WindowSizeMsg` for all bounded components
- No component measures the terminal directly

**Mouse routing:**
- Wheel events reach `viewport.Update()` even when blurred
- Wheel ignored when logs view not visible

### Run these commands

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/ 2>&1
go vet ./internal/tui/... 2>&1
go test ./internal/tui/... 2>&1

# Color hygiene
grep -rn 'lipgloss.Color("#[0-9A-Fa-f]' internal/tui/ 2>/dev/null || echo "No color literals"

# UV imports
grep -rn 'ultraviolet' internal/tui/ 2>/dev/null || echo "No UV imports"

# Dead code checks
grep -rn 'focusArea\|bottomView\|confirmDelete\|textBuffer' internal/tui/ 2>/dev/null || echo "No dead types"

# SetSize calls
grep -rn '\.SetSize(' internal/tui/*.go 2>/dev/null
```

### Report format

Classify findings as:
- **🔴 Blocking** — correctness defect, must fix before handoff
- **🟡 Advisory** — coverage gap, design risk, or convention deviation
- **⚪ Info** — observation, no action needed

Include file:line references for every finding. Read the actual source code to verify, don't just rely on grep.

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