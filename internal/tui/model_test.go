package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"nice-llama-server/internal/config"
)

func TestSlashTogglesBetweenBookmarkAndLogViews(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	if m.bottomView != bottomViewBookmarks {
		t.Fatalf("unexpected default bottom view: %v", m.bottomView)
	}

	next, _ := m.Update(tea.KeyPressMsg{Text: "/"})
	got := next.(*model)
	if got.bottomView != bottomViewLogs {
		t.Fatalf("expected log view after slash, got %v", got.bottomView)
	}

	next, _ = got.Update(tea.KeyPressMsg{Text: "/"})
	got = next.(*model)
	if got.bottomView != bottomViewBookmarks {
		t.Fatalf("expected bookmark view after second slash, got %v", got.bottomView)
	}
}

func TestPlainQDoesNotQuitButCtrlQDoes(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	next, cmd := m.Update(tea.KeyPressMsg{Text: "q"})
	if next == nil {
		t.Fatalf("plain q should not quit")
	}
	if cmd == nil {
		return
	}
	if msg := cmd(); msg != nil {
		t.Fatalf("plain q should not emit a quit message, got %#v", msg)
	}

	_, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'q', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatalf("ctrl+q should return quit command")
	}
	if msg := cmd(); msg == nil {
		t.Fatalf("ctrl+q should emit quit message")
	}
}

func TestCtrlSSavesEditorAndReturnsFocusToList(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{
		ID:        "bookmark-1",
		Name:      "Gemma",
		ModelPath: "/models/gemma.gguf",
		GroupKey:  "gemma",
	}, false)

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	got := next.(*model)
	if cmd == nil {
		t.Fatalf("ctrl+s should trigger save command")
	}
	if got.focus != focusDetailName {
		t.Fatalf("focus should remain in detail mode until save completes")
	}

	next, _ = got.Update(actionMsg{
		snapshot: config.Snapshot{
			Bookmarks: []config.Bookmark{{
				ID:        "bookmark-1",
				Name:      "Gemma",
				ModelPath: "/models/gemma.gguf",
				GroupKey:  "gemma",
			}},
		},
		selectedKey: listItem{kind: listItemBookmark, bookmarkID: "bookmark-1"}.key(),
		clearEditor: true,
		focus:       focusModelList,
	})
	got = next.(*model)
	if got.editor != nil {
		t.Fatalf("save result should clear the editor")
	}
	if got.focus != focusModelList {
		t.Fatalf("save result should return focus to the list, got %v", got.focus)
	}
}

func TestEnterInNameMovesFocusToArgs(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("enter in name field should not trigger a command")
	}
	if got.focus != focusDetailArgs {
		t.Fatalf("enter in name field should move focus to args, got %v", got.focus)
	}
}

func TestEnterInArgsInsertsNewLine(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx-size 8192"}, false)
	m.editor.args.MoveEnd()

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("enter in args should not trigger a command")
	}
	if got.editor.args.Value() != "--ctx-size 8192\n" {
		t.Fatalf("enter in args should insert a newline, got %q", got.editor.args.Value())
	}
	if got.focus != focusDetailArgs {
		t.Fatalf("focus should stay in args, got %v", got.focus)
	}
}

func TestTabCompletesArgsFromLlamaServerCatalog(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx"}, false)
	m.editor.args.MoveEnd()

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("tab completion should not trigger a command")
	}
	if got.editor.args.Value() != "--ctx-size" {
		t.Fatalf("expected --ctx to complete to --ctx-size, got %q", got.editor.args.Value())
	}
	if !got.editor.completion.active {
		t.Fatalf("completion state should remain active for cycling")
	}
}

func TestTabCyclesArgsCompletions(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx"}, false)
	m.editor.args.MoveEnd()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	first := got.editor.args.Value()
	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got = next.(*model)
	second := got.editor.args.Value()

	if first == second {
		t.Fatalf("second tab should cycle to a different completion, still got %q", second)
	}
	if !strings.HasPrefix(second, "--ctx") {
		t.Fatalf("cycled completion should keep the original prefix, got %q", second)
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	got = next.(*model)
	if got.editor.args.Value() != first {
		t.Fatalf("shift+tab should cycle back to the previous completion, got %q want %q", got.editor.args.Value(), first)
	}
}

func TestTabCompletionExcludesAlreadyUsedArgs(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx-size 8192\n--ctx"}, false)
	_ = m.editor.args.MoveDown()
	m.editor.args.MoveEnd()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if got.editor.args.Value() == "--ctx-size 8192\n--ctx-size" {
		t.Fatalf("completion reused an already-present arg: %q", got.editor.args.Value())
	}
	if strings.Contains(got.editor.args.Value(), "\n--ctx-size") {
		t.Fatalf("already-present --ctx-size should not be suggested again: %q", got.editor.args.Value())
	}
}

func TestTabCompletesMMProjValueAfterShortFlag(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "-mm ",
	}, false)
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
		MMProjPaths: []string{"/models/mmproj-a.gguf", "/models/mmproj-b.gguf"},
	}}
	m.editor.args.MoveEnd()

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("tab completion should not trigger a command")
	}
	if got.editor.args.Value() != "-mm /models/mmproj-a.gguf" {
		t.Fatalf("expected first mmproj candidate, got %q", got.editor.args.Value())
	}
	if !got.editor.completion.active {
		t.Fatalf("completion state should remain active for cycling")
	}
}

func TestTabCompletesMMProjValueAfterLongFlag(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "--mmproj ",
	}, false)
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
		MMProjPaths: []string{"/models/mmproj-a.gguf"},
	}}
	m.editor.args.MoveEnd()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if got.editor.args.Value() != "--mmproj /models/mmproj-a.gguf" {
		t.Fatalf("expected mmproj completion after long flag, got %q", got.editor.args.Value())
	}
}

func TestTabCyclesMMProjValueCompletions(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "-mm ",
	}, false)
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
		MMProjPaths: []string{"/models/mmproj-a.gguf", "/models/mmproj-b.gguf"},
	}}
	m.editor.args.MoveEnd()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	first := got.editor.args.Value()

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got = next.(*model)
	second := got.editor.args.Value()
	if first == second {
		t.Fatalf("expected second tab to cycle mmproj candidates, still got %q", second)
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	got = next.(*model)
	if got.editor.args.Value() != first {
		t.Fatalf("expected shift+tab to cycle back, got %q want %q", got.editor.args.Value(), first)
	}
}

func TestMMProjCompletionFiltersByTypedPrefix(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "-mm mmproj-m",
	}, false)
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
		MMProjPaths: []string{"/models/mmproj-extra.gguf", "/models/mmproj-model.gguf"},
	}}
	m.editor.args.MoveEnd()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if got.editor.args.Value() != "-mm /models/mmproj-model.gguf" {
		t.Fatalf("expected prefix-filtered mmproj completion, got %q", got.editor.args.Value())
	}
}

func TestMMProjCompletionDoesNotTriggerForOtherFlags(t *testing.T) {
	t.Parallel()

	tests := []string{
		"-mmu ",
		"--mmproj-auto ",
		"--no-mmproj ",
		"--temp ",
	}

	for _, args := range tests {
		m := newModel(context.Background(), nil)
		m.focus = focusDetailArgs
		m.editor = newBookmarkEditor(config.Bookmark{
			ModelPath: "/models/vision.gguf",
			ArgsText:  args,
		}, false)
		m.snapshot.Models = []config.DiscoveredModel{{
			Path:        "/models/vision.gguf",
			DisplayName: "vision",
			MMProjPaths: []string{"/models/mmproj-a.gguf"},
		}}
		m.editor.args.MoveEnd()

		next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
		got := next.(*model)
		if strings.Contains(got.editor.args.Value(), "/models/mmproj-a.gguf") {
			t.Fatalf("unexpected mmproj completion for %q: %q", args, got.editor.args.Value())
		}
		if completionTextsContain(got.editor.completion.candidates, "/models/mmproj-a.gguf") {
			t.Fatalf("unexpected mmproj candidate for %q", args)
		}
	}
}

func TestMMProjCompletionDoesNotActivateWithoutCandidates(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "-mm ",
	}, false)
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
	}}
	m.editor.args.MoveEnd()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if got.editor.args.Value() != "-mm " {
		t.Fatalf("expected args to remain unchanged, got %q", got.editor.args.Value())
	}
	if got.editor.completion.active {
		t.Fatalf("did not expect active completion state")
	}
}

func TestWindowsMMProjFormattingAndPrefixMatching(t *testing.T) {
	t.Parallel()

	path := `C:\Models\Vision Path\MMProj-F16.gguf`
	if got, want := formatMMProjCompletionPathForOS(path, "windows"), `'`+path+`'`; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if !matchesMMProjPrefix(`'c:\models\vision path\mmp`, path, "windows") {
		t.Fatalf("expected case-insensitive full-path match on Windows")
	}
	if !matchesMMProjPrefix("mmproj-f", path, "windows") {
		t.Fatalf("expected case-insensitive basename match on Windows")
	}
}

func TestTypingSingleHyphenShowsPassiveArgSuggestions(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{}, false)

	next, cmd := m.Update(tea.KeyPressMsg{Text: "-"})
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("typing hyphen should not trigger a command")
	}
	if got.editor.args.Value() != "-" {
		t.Fatalf("passive suggestions should not mutate text, got %q", got.editor.args.Value())
	}
	if !got.editor.completion.active || !got.editor.completion.passive {
		t.Fatalf("expected passive completion state after single hyphen")
	}
	if !completionTextsContain(got.editor.completion.candidates, "-h") {
		t.Fatalf("single hyphen suggestions should include short aliases: %#v", completionTexts(got.editor.completion.candidates[:min(8, len(got.editor.completion.candidates))]))
	}
}

func TestTypingDoubleHyphenShowsPassiveLongArgSuggestions(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{}, false)

	next, _ := m.Update(tea.KeyPressMsg{Text: "-"})
	got := next.(*model)
	next, _ = got.Update(tea.KeyPressMsg{Text: "-"})
	got = next.(*model)

	if got.editor.args.Value() != "--" {
		t.Fatalf("passive suggestions should not mutate text, got %q", got.editor.args.Value())
	}
	if !got.editor.completion.active || !got.editor.completion.passive {
		t.Fatalf("expected passive completion state after double hyphen")
	}
	for _, candidate := range got.editor.completion.candidates {
		if !strings.HasPrefix(candidate.Text, "--") {
			t.Fatalf("double hyphen suggestions should only include long aliases, got %q", candidate.Text)
		}
	}
}

func TestPassiveArgSuggestionsUseOtherBookmarkPopularity(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ID: "current", ArgsText: "--"}, false)
	m.snapshot.Bookmarks = []config.Bookmark{
		{ID: "other-1", ArgsText: "--temp 0.7"},
		{ID: "other-2", ArgsText: "--temp 0.8\n--ctx-size 4096"},
		{ID: "other-3", ArgsText: "--temp 0.9"},
	}
	m.editor.args.MoveEnd()
	m.refreshPassiveArgCompletion()

	if len(m.editor.completion.candidates) == 0 {
		t.Fatalf("expected passive candidates")
	}
	if got := m.editor.completion.candidates[0].Text; got != "--temp" {
		t.Fatalf("expected most common arg first, got %q", got)
	}
}

func TestPassiveArgPopularityExcludesCurrentBookmark(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ID: "current", ArgsText: "--"}, false)
	m.snapshot.Bookmarks = []config.Bookmark{
		{ID: "current", ArgsText: "--temp 0.7"},
		{ID: "other", ArgsText: "--ctx-size 8192"},
	}
	m.editor.args.MoveEnd()
	m.refreshPassiveArgCompletion()

	if len(m.editor.completion.candidates) == 0 {
		t.Fatalf("expected passive candidates")
	}
	if got := m.editor.completion.candidates[0].Text; got != "--ctx-size" {
		t.Fatalf("expected current bookmark popularity to be excluded, got %q", got)
	}
}

func TestShiftTabFromPassiveSuggestionsAppliesLastCandidate(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{}, false)

	next, _ := m.Update(tea.KeyPressMsg{Text: "-"})
	got := next.(*model)
	candidates := append([]argCompletionCandidate(nil), got.editor.completion.candidates...)
	if len(candidates) == 0 {
		t.Fatalf("expected passive candidates")
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	got = next.(*model)
	want := candidates[len(candidates)-1].Text
	if got.editor.args.Value() != want {
		t.Fatalf("shift+tab from passive suggestions should apply last candidate, got %q want %q", got.editor.args.Value(), want)
	}
}

func TestTabFromPassiveSuggestionsThenCyclesForward(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{}, false)

	next, _ := m.Update(tea.KeyPressMsg{Text: "-"})
	got := next.(*model)
	candidates := append([]argCompletionCandidate(nil), got.editor.completion.candidates...)
	if len(candidates) < 2 {
		t.Fatalf("expected at least two passive candidates")
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got = next.(*model)
	if got.editor.args.Value() != candidates[0].Text {
		t.Fatalf("tab from passive suggestions should apply first candidate, got %q want %q", got.editor.args.Value(), candidates[0].Text)
	}
	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got = next.(*model)
	if got.editor.args.Value() != candidates[1].Text {
		t.Fatalf("second tab should cycle forward, got %q want %q", got.editor.args.Value(), candidates[1].Text)
	}
}

func completionTextsContain(candidates []argCompletionCandidate, text string) bool {
	for _, candidate := range candidates {
		if candidate.Text == text {
			return true
		}
	}
	return false
}

func completionTexts(candidates []argCompletionCandidate) []string {
	texts := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		texts = append(texts, candidate.Text)
	}
	return texts
}

func TestUpInNameKeepsFocusInName(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	got := next.(*model)
	if got.focus != focusDetailName {
		t.Fatalf("up in name field should keep focus in name, got %v", got.focus)
	}
	if got.editor == nil {
		t.Fatalf("editor should remain active")
	}
}

func TestEscDiscardsEditorAndReturnsFocusToList(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	got := next.(*model)
	if got.editor != nil {
		t.Fatalf("esc should discard the editor")
	}
	if got.focus != focusModelList {
		t.Fatalf("esc should return focus to model list, got %v", got.focus)
	}
}

func TestNewBookmarkUsesCurrentModelGroup(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.snapshot.Models = []config.DiscoveredModel{
		{
			Path:        "/models/gemma.gguf",
			DisplayName: "gemma-3-4b-it-Q4_K_M",
			GroupKey:    "gemma-3-4b-it-Q4_K_M",
		},
	}
	m.selectedKey = listItem{
		kind:      listItemModelGroup,
		groupKey:  "gemma-3-4b-it-Q4_K_M",
		modelPath: "/models/gemma.gguf",
	}.key()

	next, _ := m.Update(tea.KeyPressMsg{Text: "n"})
	got := next.(*model)
	if got.editor == nil {
		t.Fatalf("new bookmark should open an editor")
	}
	if got.editor.modelPath != "/models/gemma.gguf" {
		t.Fatalf("unexpected model path: %q", got.editor.modelPath)
	}
	if got.editor.groupKey != "gemma-3-4b-it-Q4_K_M" {
		t.Fatalf("unexpected group key: %q", got.editor.groupKey)
	}
	if got.focus != focusDetailName {
		t.Fatalf("new bookmark should focus the name field, got %v", got.focus)
	}
}

func TestListItemsGroupsBookmarksByModelPath(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.snapshot.Models = []config.DiscoveredModel{
		{
			Path:        "/models/gemma.gguf",
			DisplayName: "gemma-3-4b-it-Q4_K_M",
			GroupKey:    "gemma",
		},
	}
	m.snapshot.Bookmarks = []config.Bookmark{
		{ID: "a", Name: "One", ModelPath: "/models/gemma.gguf", GroupKey: "gemma-A"},
		{ID: "b", Name: "Two", ModelPath: "/models/gemma.gguf", GroupKey: "gemma-B"},
	}

	items := m.listItems()
	if len(items) != 3 {
		t.Fatalf("unexpected list item count: got %d want 3", len(items))
	}
	if items[0].kind != listItemModelGroup {
		t.Fatalf("first item should be a group header")
	}
	if items[0].label != "gemma-3-4b-it-Q4_K_M" {
		t.Fatalf("unexpected group label: %q", items[0].label)
	}
}

func TestListItemsUsesPathFallbackForMissingModel(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.snapshot.Bookmarks = []config.Bookmark{
		{ID: "a", Name: "One", ModelPath: `C:\models\gemma-3-4b-it-Q4_K_M.gguf`, GroupKey: "old-group"},
	}

	items := m.listItems()
	if len(items) != 2 {
		t.Fatalf("unexpected list item count: got %d want 2", len(items))
	}
	if items[0].label != "gemma-3-4b-it-Q4_K_M" {
		t.Fatalf("unexpected fallback group label: %q", items[0].label)
	}
	if !items[0].degraded {
		t.Fatalf("missing model group should be marked degraded")
	}
}

func TestLogViewArrowKeysScrollViewport(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewLogs
	m.logViewHeight = 2
	for i := 0; i < 12; i++ {
		m.logs = append(m.logs, config.LogEntry{
			Seq:    int64(i + 1),
			Stream: "stdout",
			Line:   "line",
		})
	}
	m.scrollLogToBottom()
	if m.logScrollY == 0 {
		t.Fatalf("expected initial scroll position to be at bottom")
	}
	before := m.logScrollY

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	got := next.(*model)
	if got.logScrollY >= before {
		t.Fatalf("up should scroll log viewport upward")
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	got = next.(*model)
	if got.logScrollY != m.logScrollY {
		t.Fatalf("down should scroll log viewport downward")
	}
}

func TestLogViewPageKeysScrollByViewportHeight(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewLogs
	m.logViewHeight = 3
	for i := 0; i < 12; i++ {
		m.logs = append(m.logs, config.LogEntry{
			Seq:    int64(i + 1),
			Stream: "stdout",
			Line:   "line",
		})
	}
	m.scrollLogToBottom()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgUp}))
	got := next.(*model)
	if got.logScrollY != 6 {
		t.Fatalf("pgup should scroll by viewport height, got %d", got.logScrollY)
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	got = next.(*model)
	if got.logScrollY != 9 {
		t.Fatalf("pgdown should scroll by viewport height, got %d", got.logScrollY)
	}
}

func TestLogViewHorizontalScrollUsesLeftRight(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewLogs
	m.logViewWidth = 12
	m.logs = []config.LogEntry{{
		Seq:    1,
		Stream: "stdout",
		Line:   "very long log line for scrolling",
	}}
	m.clampLogScroll()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	got := next.(*model)
	if got.logScrollX <= 0 {
		t.Fatalf("right should increase horizontal log scroll")
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	got = next.(*model)
	if got.logScrollX != 0 {
		t.Fatalf("left should reduce horizontal log scroll back to zero, got %d", got.logScrollX)
	}
}

func TestLogViewTogglePreservesScrollOffsets(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewLogs
	m.logViewHeight = 2
	m.logs = []config.LogEntry{
		{Seq: 1, Stream: "stdout", Line: "one"},
		{Seq: 2, Stream: "stdout", Line: "two"},
		{Seq: 3, Stream: "stdout", Line: "three"},
		{Seq: 4, Stream: "stdout", Line: "four"},
	}
	m.logScrollY = 5
	m.logScrollX = 7
	m.clampLogScroll()
	expectedY := m.logScrollY
	expectedX := m.logScrollX

	next, _ := m.Update(tea.KeyPressMsg{Text: "/"})
	got := next.(*model)
	if got.bottomView != bottomViewBookmarks {
		t.Fatalf("expected bookmark view after toggle")
	}

	next, _ = got.Update(tea.KeyPressMsg{Text: "/"})
	got = next.(*model)
	if got.bottomView != bottomViewLogs {
		t.Fatalf("expected log view after toggle back")
	}
	if got.logScrollY != expectedY || got.logScrollX != expectedX {
		t.Fatalf("expected scroll offsets to be preserved, got y=%d x=%d", got.logScrollY, got.logScrollX)
	}
}

func TestLogViewMouseWheelScrollsOnlyWhenVisible(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewLogs
	m.logViewHeight = 2
	for i := 0; i < 12; i++ {
		m.logs = append(m.logs, config.LogEntry{
			Seq:    int64(i + 1),
			Stream: "stdout",
			Line:   "line",
		})
	}
	m.scrollLogToBottom()
	if m.logScrollY == 0 {
		t.Fatalf("expected initial scroll position to be at bottom")
	}
	before := m.logScrollY

	next, _ := m.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	got := next.(*model)
	if got.logScrollY >= before {
		t.Fatalf("mouse wheel up should scroll log viewport upward")
	}

	got.bottomView = bottomViewBookmarks
	before = got.logScrollY
	next, _ = got.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))
	got = next.(*model)
	if got.logScrollY != before {
		t.Fatalf("mouse wheel should be ignored outside visible log view")
	}
}

func TestNewLogsAutoFollowOnlyAtBottom(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.logViewHeight = 2
	for i := 0; i < 5; i++ {
		m.logs = append(m.logs, config.LogEntry{
			Seq:    int64(i + 1),
			Stream: "stdout",
			Line:   "line",
		})
	}
	m.scrollLogToBottom()
	atBottom := m.logScrollY

	m.followTail = true
	m.followTailEnabled = true
	next, _ := m.Update(logsMsg{entries: []config.LogEntry{{
		Seq:    6,
		Stream: "stdout",
		Line:   "new line",
	}}})
	got := next.(*model)
	if got.logScrollY <= atBottom {
		t.Fatalf("expected auto-follow when already at bottom and tail mode enabled")
	}

	got.followTail = false
	got.followTailEnabled = false
	got.logScrollY = 0
	next, _ = got.Update(logsMsg{entries: []config.LogEntry{{
		Seq:    7,
		Stream: "stdout",
		Line:   "another line",
	}}})
	got = next.(*model)
	if got.logScrollY != 0 {
		t.Fatalf("expected viewport position to stay when scrolled up and tail mode disabled, got %d", got.logScrollY)
	}

	got.followTail = true
	got.followTailEnabled = true
	got.logScrollY = 0
	next, _ = got.Update(logsMsg{entries: []config.LogEntry{{
		Seq:    8,
		Stream: "stdout",
		Line:   "followed line",
	}}})
	got = next.(*model)
	if got.logScrollY != 0 {
		t.Fatalf("expected auto-follow when tail mode is enabled and scroll is at top")
	}
}

func TestPasteIntoNameStripsNewlines(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)
	m.editor.name.col = len([]rune(m.editor.name.Value()))

	next, _ := m.Update(tea.PasteMsg{Content: "multi\nline\nname"})
	got := next.(*model)
	if got.editor.name.Value() != "Gemmamulti line name" {
		t.Fatalf("expected 'Gemmamulti line name', got %q", got.editor.name.Value())
	}
}

func TestPasteIntoArgsPreservesNewlines(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx 4096"}, false)
	m.editor.args.col = len([]rune(m.editor.args.Value()))

	next, _ := m.Update(tea.PasteMsg{Content: "--ctx 4096\n--gpu-layers 32"})
	got := next.(*model)
	if got.editor.args.Value() != "--ctx 4096--ctx 4096\n--gpu-layers 32" {
		t.Fatalf("expected '--ctx 4096--ctx 4096\\n--gpu-layers 32', got %q", got.editor.args.Value())
	}
}

func TestCtrlZUndoesInEditor(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)
	m.editor.name.col = len([]rune(m.editor.name.Value()))

	next, _ := m.Update(tea.PasteMsg{Content: "X"})
	got := next.(*model)
	if got.editor.name.Value() != "GemmaX" {
		t.Fatalf("expected 'GemmaX', got %q", got.editor.name.Value())
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}))
	got = next.(*model)
	if got.editor.name.Value() != "Gemma" {
		t.Fatalf("expected 'Gemma' after Ctrl+Z, got %q", got.editor.name.Value())
	}
}

func TestCtrlZNoOpWithEmptyUndoStack(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}))
	got := next.(*model)
	if got.editor.name.Value() != "Gemma" {
		t.Fatalf("Ctrl+Z on empty stack should not change value, got %q", got.editor.name.Value())
	}
}

func TestPasteIntoArgsNormalizesCRLF(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx 4096"}, false)
	m.editor.args.col = len([]rune(m.editor.args.Value()))

	next, _ := m.Update(tea.PasteMsg{Content: "--ctx 4096\r\n--gpu-layers 32\r\n--temp 0.6"})
	got := next.(*model)
	expected := "--ctx 4096--ctx 4096\n--gpu-layers 32\n--temp 0.6"
	if got.editor.args.Value() != expected {
		t.Fatalf("expected %q, got %q", expected, got.editor.args.Value())
	}
}

func TestPasteIntoNameNormalizesCRLF(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)
	m.editor.name.col = len([]rune(m.editor.name.Value()))

	next, _ := m.Update(tea.PasteMsg{Content: "multi\r\nline\r\nname"})
	got := next.(*model)
	if got.editor.name.Value() != "Gemmamulti line name" {
		t.Fatalf("expected 'Gemmamulti line name', got %q", got.editor.name.Value())
	}
}

func TestPasteIntoArgsNormalizesLoneCR(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx 4096"}, false)
	m.editor.args.col = len([]rune(m.editor.args.Value()))

	next, _ := m.Update(tea.PasteMsg{Content: "--ctx 4096\r--gpu-layers 32\r--temp 0.6"})
	got := next.(*model)
	expected := "--ctx 4096--ctx 4096\n--gpu-layers 32\n--temp 0.6"
	if got.editor.args.Value() != expected {
		t.Fatalf("expected %q, got %q", expected, got.editor.args.Value())
	}
}

func TestPasteIntoArgsStripsANSIColorCodes(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailArgs
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx 4096"}, false)
	m.editor.args.col = len([]rune(m.editor.args.Value()))

	pasted := "\x1b[38;2;255;0;0m--host\x1b[0m \x1b[38;2;0;255;0m0.0.0.0\x1b[0m"
	next, _ := m.Update(tea.PasteMsg{Content: pasted})
	got := next.(*model)
	if got.editor.args.Value() != "--ctx 4096--host 0.0.0.0" {
		t.Fatalf("expected '--ctx 4096--host 0.0.0.0', got %q", got.editor.args.Value())
	}
}

func TestPasteIntoNameStripsANSIColorCodes(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)
	m.editor.name.col = len([]rune(m.editor.name.Value()))

	pasted := "\x1b[38;2;255;0;0mColored\x1b[0m \x1b[38;2;0;0;255mName\x1b[0m"
	next, _ := m.Update(tea.PasteMsg{Content: pasted})
	got := next.(*model)
	if got.editor.name.Value() != "GemmaColored Name" {
		t.Fatalf("expected 'GemmaColored Name', got %q", got.editor.name.Value())
	}
}
