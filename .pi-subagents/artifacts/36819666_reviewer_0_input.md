# Task for reviewer

## Interaction Bug Audit: nice-llama-server gotui migration

**Repo:** /Users/ishansain/Documents/repos/nice-llama-server  
**Gotui library:** /Users/ishansain/Documents/repos/gotui (module: github.com/ishan5ain/tuiweave)

### Background

This is a TUI app that manages llama-server launch configurations (bookmarks). It has been migrated from hand-rolled bubbletea to the gotui/tuiweave component library. The migration is structurally complete but may have interaction bugs.

### Source files to read

Read ALL of these files before analyzing:

1. `/Users/ishansain/Documents/repos/nice-llama-server/internal/tui/update.go` — key handling, focus management
2. `/Users/ishansain/Documents/repos/nice-llama-server/internal/tui/model.go` — model struct, Update switch, message handling
3. `/Users/ishansain/Documents/repos/nice-llama-server/internal/tui/render.go` — rendering
4. `/Users/ishansain/Documents/repos/nice-llama-server/internal/tui/selection.go` — list items, selection, editor creation
5. `/Users/ishansain/Documents/repos/nice-llama-server/internal/tui/args_completion.go` — arg completion engine
6. `/Users/ishansain/Documents/repos/nice-llama-server/internal/tui/editor.go` — bookmark editor struct
7. `/Users/ishansain/Documents/repos/nice-llama-server/internal/tui/model_test.go` — interaction tests
8. `/Users/ishansain/Documents/repos/nice-llama-server/internal/tui/render_test.go` — rendering tests

### Key Bindings (from update.go)

**Global:**
- `ctrl+q` — quit
- `tab` — cycle focus forward / trigger arg completion on args field
- `shift+tab` — cycle focus backward / trigger backward arg completion on args field

**Logs view (tabs.SelectedID() == "logs"):**
- `up/down` — scroll log by 1 line
- `pgup/pgdown` — scroll log by page
- `home/end` — goto top/bottom
- `left/right` — horizontal scroll
- `t` — enable follow-tail, goto bottom
- `T` — disable follow-tail
- `/` — toggle between bookmarks/logs views
- `shift+l` — load selected bookmark
- `shift+u` — unload

**Bookmarks view, list focused (fm.Index() == 1):**
- `up/down` — navigate list
- `e` — edit selected bookmark
- `n` — new bookmark for current group
- `c` — clone selected bookmark
- `d` — delete (opens dialog)
- `r` — rescan model roots
- `/` — toggle views
- `shift+l` — load selected bookmark
- `shift+u` — unload

**Bookmarks view, editor active (editorScope.Active()):**
- `esc` — discard editor / close autocomplete
- `ctrl+s` — save bookmark
- `ctrl+z` — undo in args field
- `up/down` — move between name/args fields
- `enter` — move from name to args / insert newline in args / accept autocomplete
- `ctrl+v` — paste
- `/` — toggle views (only when editor is nil)

**Dialog showing (showDialog == true):**
- All keys routed to dialog.Update()
- `enter` — confirm
- `esc` — cancel

### Focus topology

**Bookmarks view, no editor:** tabs → list
**Bookmarks view, editor open:** tabs → list → textinput(name) → textarea(args) → autocomplete
**Logs view:** tabs → viewport

The editor uses `focus.Scope` for conditional registration. When the editor is open, the scope is active and the background (tabs, list) is blurred via `ApplyBackground`.

### What to look for

Trace through EVERY key binding path and identify:

1. **Missing key handlers** — keys that should work but aren't handled
2. **Wrong key handlers** — keys that do the wrong thing
3. **Focus leaks** — situations where two components are focused simultaneously, or no component is focused
4. **State leaks** — stale state that persists when it shouldn't (e.g., completion state after focus change)
5. **Dead code paths** — branches that can never be reached
6. **Dialog interaction** — does the dialog properly block all other input? Does it restore focus after closing?
7. **Autocomplete interaction** — does tab on args trigger completion? Does enter accept the selection? Does esc close it?
8. **Edge cases** — what happens when the list is empty? When there are no bookmarks? When the editor is open but the user presses '/'?

### Run these commands

```bash
cd /Users/ishansain/Documents/repos/nice-llama-server
go build ./cmd/nice-llama-server/ 2>&1
go vet ./internal/tui/... 2>&1
go test ./internal/tui/... -count=1 -v 2>&1 | head -200
```

### Report format

Classify findings as:
- **🔴 Blocking** — correctness defect, must fix
- **🟡 Advisory** — design risk or edge case
- **⚪ Info** — observation

For each finding, include:
- The exact key sequence that triggers it
- What currently happens
- What should happen
- File:line reference
- Suggested fix

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