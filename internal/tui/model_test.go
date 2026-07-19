package tui

import (
	"context"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/ishan5ain/tuiweave"
	"github.com/ishan5ain/tuiweave/textarea"

	"nice-llama-server/internal/config"
)

// cursorEnd moves the textarea cursor to the end of the current line.
func cursorEnd(ta *textarea.Model) {
	row, _ := ta.Cursor()
	lines := strings.Split(ta.Value(), "\n")
	if row >= 0 && row < len(lines) {
		ta.SetCursor(row, len([]rune(lines[row])))
	}
}

// cursorDown moves the textarea cursor down one line.
func cursorDown(ta *textarea.Model) {
	row, col := ta.Cursor()
	ta.SetCursor(row+1, col)
}

func TestSlashTogglesBetweenBookmarkAndLogViews(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	if m.tabs.SelectedID() != "bookmarks" {
		t.Fatalf("unexpected default bottom view: %v", m.tabs.SelectedID())
	}

	next, _ := m.Update(tea.KeyPressMsg{Text: "/"})
	got := next.(*model)
	if got.tabs.SelectedID() != "logs" {
		t.Fatalf("expected log view after slash, got %v", got.tabs.SelectedID())
	}

	next, _ = got.Update(tea.KeyPressMsg{Text: "/"})
	got = next.(*model)
	if got.tabs.SelectedID() != "bookmarks" {
		t.Fatalf("expected bookmark view after second slash, got %v", got.tabs.SelectedID())
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
	m.editor = newBookmarkEditor(config.Bookmark{
		ID:        "bookmark-1",
		Name:      "Gemma",
		ModelPath: "/models/gemma.gguf",
		GroupKey:  "gemma",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editor.name.Focus()

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 's', Mod: tea.ModCtrl}))
	got := next.(*model)
	if cmd == nil {
		t.Fatalf("ctrl+s should trigger save command")
	}
	if !got.editorScope.Active() || got.editorScope.Index() != 0 {
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
	})
	got = next.(*model)
	if got.editor != nil {
		t.Fatalf("save result should clear the editor")
	}
	if got.editorScope.Active() || got.fm.Index() != 1 {
		t.Fatalf("save result should return focus to the list, got %v", "focus state")
	}
}

func TestEnterInNameMovesFocusToArgs(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editor.name.Focus()

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("enter in name field should not trigger a command")
	}
	if !got.editorScope.Active() || got.editorScope.Index() != 1 {
		t.Fatalf("enter in name field should move focus to args, got %v", "focus state")
	}
}

func TestEnterInArgsInsertsNewLine(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx-size 8192"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorEnd(&m.editor.args)

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("enter in args should not trigger a command")
	}
	if got.editor.args.Value() != "--ctx-size 8192\n" {
		t.Fatalf("enter in args should insert a newline, got %q", got.editor.args.Value())
	}
	if !got.editorScope.Active() || got.editorScope.Index() != 1 {
		t.Fatalf("focus should stay in args, got %v", "focus state")
	}
}

func TestTabCompletesArgsFromLlamaServerCatalog(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorEnd(&m.editor.args)

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("tab completion should not trigger a command")
	}
	if !got.ac.Focused() || got.ac.FilteredLen() == 0 {
		t.Fatalf("expected autocomplete to be populated after tab")
	}
	first := got.ac.SelectedItem().Value
	if first == "" {
		t.Fatalf("expected a selected item in autocomplete")
	}
	if !strings.HasPrefix(first, "--ctx") {
		t.Fatalf("expected completion to start with --ctx, got %q", first)
	}
	if !got.editor.completion.active {
		t.Fatalf("completion state should remain active for cycling")
	}
}

func TestTabCyclesArgsCompletions(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorEnd(&m.editor.args)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if !got.ac.Focused() || got.ac.FilteredLen() == 0 {
		t.Fatalf("expected autocomplete to be populated after tab")
	}
	first := got.ac.SelectedItem().Value

	// Second tab navigates within autocomplete
	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got = next.(*model)
	second := got.ac.SelectedItem().Value

	if first == second {
		t.Fatalf("second tab should cycle to a different completion, still got %q", second)
	}
	if !strings.HasPrefix(second, "--ctx") {
		t.Fatalf("cycled completion should keep the original prefix, got %q", second)
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	got = next.(*model)
	if got.ac.SelectedItem().Value != first {
		t.Fatalf("shift+tab should cycle back to the previous completion, got %q want %q", got.ac.SelectedItem().Value, first)
	}
}

func TestTabCompletionExcludesAlreadyUsedArgs(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx-size 8192\n--ctx"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorDown(&m.editor.args)
	cursorEnd(&m.editor.args)

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
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "-mm ",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
		MMProjPaths: []string{"/models/mmproj-a.gguf", "/models/mmproj-b.gguf"},
	}}
	cursorEnd(&m.editor.args)

	next, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if cmd != nil {
		t.Fatalf("tab completion should not trigger a command")
	}
	if !got.ac.Focused() || got.ac.FilteredLen() == 0 {
		t.Fatalf("expected autocomplete to be populated after tab")
	}
	first := got.ac.SelectedItem().Value
	if first == "" {
		t.Fatalf("expected a selected item in autocomplete")
	}
	if !strings.Contains(first, "mmproj") {
		t.Fatalf("expected mmproj completion, got %q", first)
	}
	if !got.editor.completion.active {
		t.Fatalf("completion state should remain active for cycling")
	}
}

func TestTabCompletesMMProjValueAfterLongFlag(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "--mmproj ",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
		MMProjPaths: []string{"/models/mmproj-a.gguf"},
	}}
	cursorEnd(&m.editor.args)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if !got.ac.Focused() || got.ac.FilteredLen() == 0 {
		t.Fatalf("expected autocomplete to be populated after tab")
	}
	first := got.ac.SelectedItem().Value
	if first == "" || !strings.Contains(first, "mmproj") {
		t.Fatalf("expected mmproj completion, got %q", first)
	}
}

func TestTabCyclesMMProjValueCompletions(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "-mm ",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
		MMProjPaths: []string{"/models/mmproj-a.gguf", "/models/mmproj-b.gguf"},
	}}
	cursorEnd(&m.editor.args)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if !got.ac.Focused() || got.ac.FilteredLen() == 0 {
		t.Fatalf("expected autocomplete to be populated after tab")
	}
	first := got.ac.SelectedItem().Value

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got = next.(*model)
	second := got.ac.SelectedItem().Value
	if first == second {
		t.Fatalf("expected second tab to cycle mmproj candidates, still got %q", second)
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab, Mod: tea.ModShift}))
	got = next.(*model)
	if got.ac.SelectedItem().Value != first {
		t.Fatalf("expected shift+tab to cycle back, got %q want %q", got.ac.SelectedItem().Value, first)
	}
}

func TestMMProjCompletionFiltersByTypedPrefix(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "-mm mmproj-m",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
		MMProjPaths: []string{"/models/mmproj-extra.gguf", "/models/mmproj-model.gguf"},
	}}
	cursorEnd(&m.editor.args)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	got := next.(*model)
	if !got.ac.Focused() || got.ac.FilteredLen() == 0 {
		t.Fatalf("expected autocomplete to be populated after tab")
	}
	first := got.ac.SelectedItem().Value
	if !strings.Contains(first, "mmproj-m") {
		t.Fatalf("expected prefix-filtered mmproj completion, got %q", first)
	}
}

func expectedMMProjArgValue(flag, path string) string {
	return flag + " " + formatMMProjCompletionPathForOS(path, runtime.GOOS)
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
		m.editor = newBookmarkEditor(config.Bookmark{
			ModelPath: "/models/vision.gguf",
			ArgsText:  args,
		}, false, tuiweave.Dark())
		m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
		m.snapshot.Models = []config.DiscoveredModel{{
			Path:        "/models/vision.gguf",
			DisplayName: "vision",
			MMProjPaths: []string{"/models/mmproj-a.gguf"},
		}}
		cursorEnd(&m.editor.args)

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
	m.editor = newBookmarkEditor(config.Bookmark{
		ModelPath: "/models/vision.gguf",
		ArgsText:  "-mm ",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.snapshot.Models = []config.DiscoveredModel{{
		Path:        "/models/vision.gguf",
		DisplayName: "vision",
	}}
	cursorEnd(&m.editor.args)

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
	m.editor = newBookmarkEditor(config.Bookmark{}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()

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
	m.editor = newBookmarkEditor(config.Bookmark{}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()

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
	m.editor = newBookmarkEditor(config.Bookmark{ID: "current", ArgsText: "--"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.snapshot.Bookmarks = []config.Bookmark{
		{ID: "other-1", ArgsText: "--temp 0.7"},
		{ID: "other-2", ArgsText: "--temp 0.8\n--ctx-size 4096"},
		{ID: "other-3", ArgsText: "--temp 0.9"},
	}
	cursorEnd(&m.editor.args)
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
	m.editor = newBookmarkEditor(config.Bookmark{ID: "current", ArgsText: "--"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.snapshot.Bookmarks = []config.Bookmark{
		{ID: "current", ArgsText: "--temp 0.7"},
		{ID: "other", ArgsText: "--ctx-size 8192"},
	}
	cursorEnd(&m.editor.args)
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
	m.editor = newBookmarkEditor(config.Bookmark{}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()

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
	m.editor = newBookmarkEditor(config.Bookmark{}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()

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
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editor.name.Focus()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	got := next.(*model)
	if !got.editorScope.Active() || got.editorScope.Index() != 0 {
		t.Fatalf("up in name field should keep focus in name, got %v", "focus state")
	}
	if got.editor == nil {
		t.Fatalf("editor should remain active")
	}
}

func TestUpInArgsMovesCursorUpWithinField(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{
		ArgsText: "--ctx-size 8192\n--temp 0.7",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.editor.args.SetSize(50, 10)
	// Move cursor to line 1
	m.editor.args.SetCursor(1, 0)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	got := next.(*model)
	row, col := got.editor.args.Cursor()
	if row != 0 || col != 0 {
		t.Fatalf("up in args should move cursor to line 0, got row=%d col=%d", row, col)
	}
	if !got.editorScope.Active() || got.editorScope.Index() != 1 {
		t.Fatalf("focus should stay in args field")
	}
}

func TestUpInArgsSwitchesToNameAtFirstLine(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{
		ArgsText: "--ctx-size 8192\n--temp 0.7",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	// Cursor already on line 0
	m.editor.args.SetCursor(0, 0)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	got := next.(*model)
	if !got.editorScope.Active() || got.editorScope.Index() != 0 {
		t.Fatalf("up on first line of args should switch focus to name, got index %d", got.editorScope.Index())
	}
}

func TestDownInArgsMovesCursorDownWithinField(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{
		ArgsText: "--ctx-size 8192\n--temp 0.7",
	}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	m.editor.args.SetSize(50, 10)
	// Cursor on line 0
	m.editor.args.SetCursor(0, 0)

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	got := next.(*model)
	row, col := got.editor.args.Cursor()
	if row != 1 || col != 0 {
		t.Fatalf("down in args should move cursor to line 1, got row=%d col=%d", row, col)
	}
	if !got.editorScope.Active() || got.editorScope.Index() != 1 {
		t.Fatalf("focus should stay in args field")
	}
}

func TestEscDiscardsEditorAndReturnsFocusToList(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	got := next.(*model)
	if got.editor != nil {
		t.Fatalf("esc should discard the editor")
	}
	if got.editorScope.Active() || got.fm.Index() != 1 {
		t.Fatalf("esc should return focus to model list, got %v", "focus state")
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
	if !got.editorScope.Active() || got.editorScope.Index() != 0 {
		t.Fatalf("new bookmark should focus the name field, got %v", "focus state")
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







func TestPasteIntoNameStripsNewlines(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editor.name.Focus()

	next, _ := m.Update(tea.PasteMsg{Content: "multi\nline\nname"})
	got := next.(*model)
	if !strings.Contains(got.editor.name.Value(), "multi line name") {
		t.Fatalf("expected paste to strip newlines, got %q", got.editor.name.Value())
	}
}

func TestPasteIntoArgsPreservesNewlines(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx 4096"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorEnd(&m.editor.args)

	next, _ := m.Update(tea.PasteMsg{Content: "--ctx 4096\n--gpu-layers 32"})
	got := next.(*model)
	if got.editor.args.Value() != "--ctx 4096--ctx 4096\n--gpu-layers 32" {
		t.Fatalf("expected '--ctx 4096--ctx 4096\\n--gpu-layers 32', got %q", got.editor.args.Value())
	}
}

func TestCtrlZUndoesInEditor(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorEnd(&m.editor.args)

	next, _ := m.Update(tea.PasteMsg{Content: "-size"})
	got := next.(*model)
	if got.editor.args.Value() != "--ctx-size" {
		t.Fatalf("expected '--ctx-size', got %q", got.editor.args.Value())
	}

	next, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}))
	got = next.(*model)
	if got.editor.args.Value() != "--ctx" {
		t.Fatalf("expected '--ctx' after Ctrl+Z, got %q", got.editor.args.Value())
	}
}

func TestCtrlZNoOpWithEmptyUndoStack(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editor.name.Focus()

	next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'z', Mod: tea.ModCtrl}))
	got := next.(*model)
	if got.editor.name.Value() != "Gemma" {
		t.Fatalf("Ctrl+Z on empty stack should not change value, got %q", got.editor.name.Value())
	}
}

func TestPasteIntoArgsNormalizesCRLF(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx 4096"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorEnd(&m.editor.args)

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
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editor.name.Focus()

	next, _ := m.Update(tea.PasteMsg{Content: "multi\r\nline\r\nname"})
	got := next.(*model)
	if got.editor.name.Value() != "Gemmamulti line name" {
		t.Fatalf("expected 'Gemmamulti line name', got %q", got.editor.name.Value())
	}
}

func TestPasteIntoArgsNormalizesLoneCR(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx 4096"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorEnd(&m.editor.args)

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
	m.editor = newBookmarkEditor(config.Bookmark{ArgsText: "--ctx 4096"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editorScope.Next(); m.editor.args.Focus()
	cursorEnd(&m.editor.args)

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
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false, tuiweave.Dark())
	m.editorScope.Enter(m.fm); m.editor.name.Focus()

	pasted := "\x1b[38;2;255;0;0mColored\x1b[0m \x1b[38;2;0;0;255mName\x1b[0m"
	next, _ := m.Update(tea.PasteMsg{Content: pasted})
	got := next.(*model)
	if got.editor.name.Value() != "GemmaColored Name" {
		t.Fatalf("expected 'GemmaColored Name', got %q", got.editor.name.Value())
	}
}
