# Orchestrator Prompt: gotui Migration

You are an orchestrator agent. Your job is to migrate the nice-llama-server TUI from its current hand-rolled bubbletea implementation to use the gotui component library. You will delegate implementation work to subagents, track progress, and steer based on results.

## Context

- **Repo:** `/Users/ishansain/Documents/repos/nice-llama-server`
- **gotui library:** `/Users/ishansain/Documents/repos/gotui` (local checkout)
- **Migration plan:** `docs/gotui-migration-plan.md` (read this first — it contains all 13 slices with detailed task prompts)
- **App agent conventions:** `AGENTS.md` (at repo root — encodes gotui rules for subagents)
- **Existing tests:** `internal/tui/*_test.go` (~50 tests covering all current behavior)

## Your Toolkit

You have the `pi-subagents` skill. Use it to delegate slices to subagents. Key patterns:

- **Single agent:** `{ agent: "some-agent", task: "..." }` — for a focused implementation task
- **Chain:** `{ chain: [{agent:"a", task:"..."}, {agent:"b", task:"..."}] }` — for sequential steps where one feeds the next
- **Parallel (careful):** `{ tasks: [...], concurrency: N }` — only for truly independent work in isolated worktrees
- **Async:** `{ ..., async: true }` — for long-running tasks you monitor while doing other work
- **Status:** `{ action: "status", id: "..." }` — check on an async run
- **Interrupt/Resume:** `{ action: "interrupt", id: "..." }` / `{ action: "resume", id: "...", message: "..." }` — steer a stuck subagent

## The Migration Plan (Summary)

The plan has 13 implementation slices + 1 review slice, organized into 9 waves:

| Wave | Slices | What |
|---|---|---|
| Wave 1 | Slice 0 | Foundation: add gotui dep, replace hardcoded colors with theme roles |
| Wave 2 | Slices 1–5, 7, 11 | Header, Footer, Tabs, Viewport, TextInput, List, Dialog |
| Wave R1 | Slice R | Review pass — catch regressions |
| Wave 3 | Slice 6 | TextArea + gotui API additions (most complex) |
| Wave 4 | Slices 8, 9 | SplitPane, Autocomplete |
| Wave R2 | Slice R | Review pass — verify textarea integration |
| Wave 5 | Slice 10 | Focus management with conditional scopes |
| Wave 6 | Slice 12 | Cleanup, scrollbar, final polish |
| Wave R3 | Slice R | Final comprehensive review |

**⚠️ Critical rule:** All slices touch `model.go`, `render.go`, `update.go`, and test files. **Run slices sequentially in a single worktree.** Do NOT parallelize within the same worktree. If you want speed, use isolated git worktrees with an integration merge step.

## How to Execute Each Slice

For each slice, the plan provides a complete subagent task prompt. Your job is to:

1. **Read the slice definition** from `docs/gotui-migration-plan.md`
2. **Verify the subagent has context** — pass the relevant portions of `AGENTS.md` and the slice's task prompt
3. **Launch the subagent** with the task prompt from the plan
4. **Monitor progress** — check status periodically
5. **Verify the result** — run `go build ./cmd/nice-llama-server/ && go test ./internal/tui/...`
6. **Steer if needed** — if tests fail, diagnose and either:
   - Resume the subagent with corrective guidance, or
   - Fix the issue yourself and move on
7. **Update progress** — maintain a progress log

## Progress Tracking

Maintain a file `docs/migration-progress.json` with this structure:

```json
{
  "wave": 1,
  "slice": 0,
  "status": "in-progress",  // "pending" | "in-progress" | "done" | "blocked" | "failed"
  "notes": [],
  "completed": [],
  "blocked": []
}
```

After each slice, update the status and append notes about what happened, any deviations from the plan, and test results.

## Steering Criteria

Adjust your approach when:

- **Tests fail after a slice:** Diagnose. If the failure is in the slice's own tests, resume the subagent with the failing test output. If the failure is in OTHER tests (regression), roll back and re-examine.
- **A slice is too large:** Break it into smaller steps. For example, Slice 6 (TextArea) could be split into: (a) add Cursor/ReplaceRange to gotui, (b) swap textBuffer for textarea, (c) adapt completion engine.
- **gotui API is missing something:** You may need to add methods to gotui (in `~/Documents/repos/gotui`). This is expected — document the addition and proceed.
- **A subagent goes off-track:** Interrupt it with `{ action: "interrupt", id: "..." }` and resume with clearer instructions.
- **Merge conflicts between slices:** Since slices run sequentially in one worktree, conflicts shouldn't occur. If they do (e.g., from a parallel worktree experiment), resolve them manually.

## Verification Commands

```bash
# After every slice:
go build ./cmd/nice-llama-server/
go vet ./internal/tui/...
go test ./internal/tui/...

# Narrow-terminal smoke test (manual):
echo "80 24" | go run ./cmd/nice-llama-server/

# Full project build:
go build ./...
```

## Getting Started

1. Read `docs/gotui-migration-plan.md` in full.
2. Read `AGENTS.md` to understand the conventions subagents will follow.
3. Initialize `docs/migration-progress.json`.
4. Start Wave 1 — launch a subagent for Slice 0.
5. After Slice 0 completes and tests pass, proceed to Wave 2.
6. Run Slice R (review) after Wave 2, Wave 4, and Wave 6.
7. Continue through all 9 waves.

## Important Gotchas

- `autocomplete.SetItems()` — NOT `SetSuggestions()`
- `dialog.Model.Body` — NOT `Message`
- `dialog.Model` is a **value type** — use `showDialog bool` for visibility
- `scrollbar.For(theme, scrollable)` — returns a string, NOT a Model
- `tabs.Update()` does NOT handle `/` — app maps `/` to `SelectID()`
- `textarea.Model` lacks `Cursor()` and `ReplaceRange()` — may need to add them
- `list.Model` applies uniform styling — embed ANSI for group accents
- Mouse routing needs stored `layout.Rect` per pane — use `mouse.InBounds()`
- Focus must be conditional — hidden controls skip the focus cycle
