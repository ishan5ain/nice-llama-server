# Task for worker

You are a delegated subagent running from a fork of the parent session. Treat the inherited conversation as reference-only context, not a live thread to continue. Do not continue or answer prior messages as if they are waiting for a reply. Your sole job is to execute the task below and return a focused result for that task using your tools.

Task:
## Task: Slice 0 — Foundation: Add gotui Dependency & Theme

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui lib:** /Users/ishansain/Documents/repos/gotui  

### Step 1: Add gotui dependency

1. Edit `go.mod` to add the replace directive. Insert this AFTER the `require (...)` block:

```
replace github.com/ishansain/gotui => /Users/ishansain/Documents/repos/gotui
```

2. Run:
```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go get github.com/ishansain/gotui@latest
```

### Step 2: Modify `internal/tui/styles.go`

Add import for gotui at the top:
```go
import (
    "github.com/ishansain/gotui"
    lipgloss "charm.land/lipgloss/v2"
)
```

Change `newStyles()` to accept a `gotui.Theme` parameter:
```go
func newStyles(theme gotui.Theme) styles {
```

Replace EVERY hardcoded `lipgloss.Color("#...")` with a derivation from theme roles. The `styles` struct shape stays the same; only color assignments change.

**Mapping guide (current hex → theme role):**

| Current hex | Theme role | Notes |
|---|---|---|
| `#4D7CFE` | `theme.Accent` | header border |
| `#F8FAFC` | `theme.Text` | header title, bookmark active fg, footer key fg |
| `#CBD5E1` | `theme.TextMuted` | header stats, bookmark items |
| `#93C5FD` | `theme.Accent` (lighter) | header status — use `theme.Accent` |
| `#FCD34D` | `theme.Warning` | header message, log timestamp system |
| `#475569` | `theme.Border` | panel base border, input blur border |
| `#38BDF8` | `theme.BorderFocused` | panel focus border |
| `#F59E0B` | `theme.Warning` | logs panel border |
| `#E2E8F0` | `theme.Text` | panel title |
| `#7DD3FC` | `theme.Accent` | group label, group selected bg |
| `#0F172A` | `theme.TextInverted` | group selected fg, bookmark selected fg |
| `#22C55E` | `theme.Success` | input focus border |
| `#94A3B8` | `theme.TextMuted` | field label, log timestamp |
| `#64748B` | `theme.TextFaint` | completion ghost, muted, tail paused |
| `#86EFAC` | `theme.Success` | log timestamp stdout |
| `#FCA5A5` | `theme.Danger` | log timestamp stderr |
| `#334155` | `theme.SurfaceRaised` | bookmark active bg, footer key bg |
| `#4ade80` | `theme.Success` | tail indicator |

**Rules:**
- No `lipgloss.Color("#...")` literals remain in styles.go
- Use `theme.Accent`, `theme.Text`, `theme.TextMuted`, `theme.TextFaint`, `theme.TextInverted`, `theme.Success`, `theme.Warning`, `theme.Danger`, `theme.Border`, `theme.BorderFocused`, `theme.SurfaceRaised`, `theme.SelectionBg`, `theme.SelectionFg`
- The `styles` struct shape stays identical — only color values change

### Step 3: Modify `internal/tui/model.go`

1. Add import: `"github.com/ishansain/gotui"`
2. Add a `theme gotui.Theme` field to the `model` struct
3. In `newModel()`, initialize with `gotui.Dark()`:
   ```go
   theme: gotui.Dark(),
   ```
4. Change `styles: newStyles()` to `styles: newStyles(gotui.Dark())`

### Step 4: Verify

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/...
```

All three must pass. If tests fail due to visual changes (golden/snapshot updates), that's expected — review each diff and update accordingly.

### ⚠️ Important
- "Zero behavioral change" is aspirational. Colors WILL shift. That's fine — this is the baseline.
- Do NOT change the `styles` struct shape or add/remove fields — only color assignments.
- Do NOT touch any other files besides `go.mod`, `styles.go`, and `model.go`.

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