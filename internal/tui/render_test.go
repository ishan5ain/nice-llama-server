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

func TestHeaderHeight(t *testing.T) {
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
	if line := ansi.Strip(m.renderFooter(100)); !strings.Contains(line, "logs") {
		t.Fatalf("bookmark footer should mention logs toggle: %q", line)
	}
	

	m.tabs.SelectID("logs")
	if line := ansi.Strip(m.renderFooter(100)); !strings.Contains(line, "bkmarks") {
		t.Fatalf("log footer should mention bookmarks toggle: %q", line)
	}
}

func TestFocusedBookmarkNameRendersCursor(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.editor = newBookmarkEditor(config.Bookmark{Name: "Gemma"}, false, m.theme)
	m.editorScope.Enter(m.fm); m.editor.name.Focus()

	rendered := ansi.Strip(strings.Join(m.renderDetailLines(50, 10), "\n"))
	if !strings.Contains(rendered, "Gemma") {
		t.Fatalf("expected bookmark name field to show name: %q", rendered)
	}
}




func TestToggleViewDoesNotSetShowingStatusMessage(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.tabs.SelectID("logs")
	if got := m.messageLine(); got != "" {
		t.Fatalf("toggle should not set a showing message, got %q", got)
	}
}

func TestRenderLogViewUsesBottomContainerWidth(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.tabs.SelectID("logs")
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



func TestFormatLogTimestampNilLocation(t *testing.T) {
	t.Parallel()

	ts := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.FixedZone("UTC-07", -7*60*60))
	got := formatLogTimestamp(ts, nil)
	want := ts.Format("15:04:05")
	if got != want {
		t.Fatalf("unexpected timestamp format with nil location: got %q want %q", got, want)
	}
}

func TestFormatLogTimestampFixedLocation(t *testing.T) {
	t.Parallel()

	ts := time.Date(2024, time.January, 2, 1, 2, 3, 0, time.UTC)
	loc := time.FixedZone("UTC+09", 9*60*60)

	got := formatLogTimestamp(ts, loc)
	want := "10:02:03"
	if got != want {
		t.Fatalf("unexpected timestamp format with fixed location: got %q want %q", got, want)
	}
}


func TestBookmarkEditorViewFillsExactBottomRegion(t *testing.T) {
	t.Parallel()

	m := newModel(context.Background(), nil)
	m.tabs.SelectID("bookmarks")
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
	if got := lipgloss.Height(rendered); got != 11 {
		t.Fatalf("unexpected bookmark editor height on narrow layout: got %d want 11", got)
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
