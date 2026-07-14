Slice 6 is complete. Here's the acceptance report.

```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "Replaced textBuffer with gotui/textarea.Model in editor.go, adapted args_completion.go to use textarea APIs (Cursor(), ReplaceRange(Position, Position, string)), updated render.go and update.go for textarea delegation, removed ~380 lines of custom textBuffer code, all tests pass"
    }
  ],
  "changedFiles": [
    "go.mod",
    "go.sum",
    "internal/tui/editor.go",
    "internal/tui/args_completion.go",
    "internal/tui/render.go",
    "internal/tui/update.go",
    "internal/tui/model.go",
    "internal/tui/model_test.go",
    "internal/tui/render_test.go",
    "internal/tui/editor_test.go",
    "/Users/ishansain/Documents/repos/gotui/textarea/textarea.go"
  ],
  "testsAddedOrUpdated": [
    "internal/tui/model_test.go",
    "internal/tui/render_test.go",
    "internal/tui/editor_test.go"
  ],
  "commandsRun": [
    {
      "command": "go build ./cmd/nice-llama-server/",
      "result": "passed",
      "summary": "Build succeeded"
    },
    {
      "command": "go vet ./internal/tui/...",
      "result": "passed",
      "summary": "Vet passed"
    },
    {
      "command": "go test ./internal/tui/...",
      "result": "passed",
      "summary": "All tests passed (0.198s)"
    }
  ],
  "validationOutput": [
    "Build: exit 0",
    "Vet: exit 0",
    "Tests: 0.198s ok"
  ],
  "residualRisks": [
    "textBuffer completely removed; any remaining references would cause compilation errors (verified none)",
    "Module rename from github.com/ishansain/gotui to github.com/ishan5ain/tuiweave may affect other projects depending on the old module name"
  ],
  "noStagedFiles": true,
  "diffSummary": "Replaced ~380 lines of custom textBuffer code with gotui/textarea.Model. Added Cursor()/SetCursor() to gotui textarea. Adapted completion engine to use textarea APIs. Updated all tests for new rendering.",
  "reviewFindings": [
    "no blockers"
  ],
  "manualNotes": "The gotui module was renamed from github.com/ishansain/gotui to github.com/ishan5ain/tuiweave. All import paths and replace directives updated accordingly. The gotui/textarea package now has Cursor() and SetCursor() methods added for app integration and testability."
}
```

## Summary

**Implemented:** Slice 6 — TextArea replacement.

**Key changes:**
- Replaced `textBuffer` type (~380 lines) with `gotui/textarea.Model` in `editor.go`
- Added `Cursor()` and `SetCursor()` methods to gotui/textarea for testability
- Rewrote `args_completion.go` to use textarea APIs (standalone `tokenAtCursor()` function, `ReplaceRange(Position, Position, string)`)
- Updated `update.go` to delegate key events to `textarea.Update()` with proper Focus/Blur management
- Updated `render.go` to use `textarea.View()` instead of custom `RenderLines()`
- Fixed module rename: `github.com/ishansain/gotui` → `github.com/ishan5ain/tuiweave`
- Updated all tests for new APIs and rendering

**Next steps:** Proceed to Wave 4 — Slice 8 (SplitPane) and Slice 9 (Autocomplete).