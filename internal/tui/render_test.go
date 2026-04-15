package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"nice-llama-server/internal/config"
)

func TestHeaderRespectsFiveLineCap(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.width = 100
	m.snapshot = config.Snapshot{
		Bookmarks: []config.Bookmark{{ID: "1", Name: "Gemma"}},
		Models: []config.DiscoveredModel{{
			Path:        "/models/gemma.gguf",
			DisplayName: "gemma-3-4b-it-Q4_K_M",
			GroupKey:    "gemma-3-4b-it-Q4_K_M",
		}},
		Config: config.Config{ModelRoots: []string{"/models"}},
		Runtime: config.RuntimeState{
			Status: config.StatusReady,
			Host:   "127.0.0.1",
			Port:   8080,
		},
	}

	header := ansi.Strip(m.renderHeader(100))
	if lines := strings.Count(header, "\n") + 1; lines != headerPanelHeight {
		t.Fatalf("unexpected header height: got %d want %d\n%s", lines, headerPanelHeight, header)
	}
	if !strings.Contains(header, "Nice Llama Server") {
		t.Fatalf("expected header title to remain visible: %q", header)
	}
}

func TestFooterChangesByContext(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	if line := ansi.Strip(m.footerLine(100)); !strings.Contains(line, "logs") {
		t.Fatalf("bookmark footer should mention logs toggle: %q", line)
	}
	if line := ansi.Strip(m.footerLine(100)); strings.Contains(line, "Enter save") {
		t.Fatalf("footer should no longer advertise enter as save: %q", line)
	}

	m.bottomView = bottomViewLogs
	if line := ansi.Strip(m.footerLine(100)); !strings.Contains(line, "bookmarks") {
		t.Fatalf("log footer should mention bookmarks toggle: %q", line)
	}
}

func TestFocusedBookmarkNameRendersCursor(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.focus = focusDetailName
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false)

	rendered := ansi.Strip(strings.Join(m.renderDetailLines(50, 10), "\n"))
	if !strings.Contains(rendered, "█") {
		t.Fatalf("expected visible cursor in focused bookmark name field: %q", rendered)
	}
}

func TestToggleViewDoesNotSetShowingStatusMessage(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.toggleBottomView()
	if got := m.messageLine(); got != "" {
		t.Fatalf("toggle should not set a showing message, got %q", got)
	}
}

func TestRenderLogViewUsesBottomContainerWidth(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewLogs
	m.width = 90
	m.height = 24
	m.logs = []config.LogEntry{{
		Seq:    1,
		TS:     time.Unix(0, 0),
		Stream: "stdout",
		Line:   "server started",
	}}

	rendered := ansi.Strip(m.renderBottom(90, 12))
	if !strings.Contains(rendered, "Runtime Logs") {
		t.Fatalf("expected log title in log view: %q", rendered)
	}
	if strings.Contains(rendered, "Bookmark Detail") {
		t.Fatalf("log view should not render bookmark detail panel")
	}
}

func TestLogViewHorizontalSliceShowsScrolledPortion(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewLogs
	m.logScrollX = 5
	m.logs = []config.LogEntry{{
		Seq:    1,
		TS:     time.Unix(0, 0),
		Stream: "stdout",
		Line:   "abcdefghijklmno",
	}}

	lines := m.renderLogLines(12, 4)
	joined := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "abcdefgh") {
		t.Fatalf("expected log content to be present, got %q", joined)
	}
	if strings.Contains(joined, "abcdefghijklmno") {
		t.Fatalf("expected long line to be horizontally sliced, got %q", joined)
	}
}

func TestLogViewVerticalWindowUsesScrollOffset(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewLogs
	m.logScrollY = 1
	for i := 0; i < 4; i++ {
		m.logs = append(m.logs, config.LogEntry{
			Seq:    int64(i + 1),
			TS:     time.Unix(int64(i), 0),
			Stream: "stdout",
			Line:   string(rune('A' + i)),
		})
	}

	lines := m.renderLogLines(30, 3)
	joined := ansi.Strip(strings.Join(lines, "\n"))
	if strings.Contains(joined, "A") {
		t.Fatalf("expected first log line to be scrolled out, got %q", joined)
	}
	if !strings.Contains(joined, "B") {
		t.Fatalf("expected scrolled window to start later, got %q", joined)
	}
}

func TestBookmarkEditorViewFillsExactBottomRegion(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.bottomView = bottomViewBookmarks
	m.snapshot.Models = []config.DiscoveredModel{
		{
			Path:        "/models/gemma.gguf",
			DisplayName: "gemma-3-4b-it-Q4_K_M",
			GroupKey:    "gemma",
		},
	}
	m.snapshot.Bookmarks = []config.Bookmark{
		{
			ID:        "1",
			Name:      "Gemma",
			ModelPath: "/models/gemma.gguf",
			GroupKey:  "gemma",
		},
	}
	m.selectedKey = listItem{kind: listItemBookmark, bookmarkID: "1"}.key()

	rendered := m.renderBookmarkEditorView(90, 14)
	if got := lipgloss.Width(rendered); got != 90 {
		t.Fatalf("unexpected bookmark editor width: got %d want 90", got)
	}
	if got := lipgloss.Height(rendered); got != 14 {
		t.Fatalf("unexpected bookmark editor height: got %d want 14", got)
	}
}

func TestBookmarkEditorViewFillsOnNarrowWidths(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	rendered := m.renderBookmarkEditorView(60, 10)
	if got := lipgloss.Width(rendered); got != 60 {
		t.Fatalf("unexpected bookmark editor width on narrow layout: got %d want 60", got)
	}
	if got := lipgloss.Height(rendered); got != 10 {
		t.Fatalf("unexpected bookmark editor height on narrow layout: got %d want 10", got)
	}
}

func TestHeaderOmitsEmptyStatusRowContent(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.width = 100
	header := ansi.Strip(m.renderHeader(100))
	if strings.Contains(header, "Error ·") {
		t.Fatalf("did not expect error content in empty header: %q", header)
	}
	if !strings.Contains(header, "Nice Llama Server") {
		t.Fatalf("expected title in header: %q", header)
	}
}
