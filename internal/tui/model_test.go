package tui

import (
	"context"
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

	next, _ := m.Update(logsMsg{entries: []config.LogEntry{{
		Seq:    6,
		Stream: "stdout",
		Line:   "new line",
	}}})
	got := next.(*model)
	if got.logScrollY <= atBottom {
		t.Fatalf("expected auto-follow when already at bottom")
	}

	got.logScrollY = 0
	next, _ = got.Update(logsMsg{entries: []config.LogEntry{{
		Seq:    7,
		Stream: "stdout",
		Line:   "another line",
	}}})
	got = next.(*model)
	if got.logScrollY != 0 {
		t.Fatalf("expected viewport position to stay when scrolled up, got %d", got.logScrollY)
	}
}
