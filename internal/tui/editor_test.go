package tui

import (
	"testing"

	"nice-llama-server/internal/config"
)

func TestBookmarkEditorBookmarkTrimsFields(t *testing.T) {
	t.Parallel()

	editor := newBookmarkEditor(config.Bookmark{
		ID:        "bookmark-1",
		ModelPath: "/models/gemma.gguf",
		GroupKey:  "gemma",
	}, false)
	editor.name.SetValue("  Gemma Fast  ")
	editor.args.SetValue("  --ctx-size 8192  \n")

	bookmark := editor.Bookmark()
	if bookmark.Name != "Gemma Fast" {
		t.Fatalf("unexpected bookmark name: %q", bookmark.Name)
	}
	if bookmark.ArgsText != "--ctx-size 8192" {
		t.Fatalf("unexpected args text: %q", bookmark.ArgsText)
	}
	if bookmark.ModelPath != "/models/gemma.gguf" {
		t.Fatalf("unexpected model path: %q", bookmark.ModelPath)
	}
	if bookmark.GroupKey != "gemma" {
		t.Fatalf("unexpected group key: %q", bookmark.GroupKey)
	}
}

func TestBookmarkEditorDirtyTracksNameAndArgs(t *testing.T) {
	t.Parallel()

	editor := newBookmarkEditor(config.Bookmark{
		Name:     "Gemma",
		ArgsText: "--ctx-size 4096",
	}, false)
	if editor.Dirty() {
		t.Fatalf("new editor should start clean")
	}

	editor.args.InsertText(" --temp 0.7")
	if !editor.Dirty() {
		t.Fatalf("editor should be dirty after args change")
	}
}

func TestTextBufferRenderLinesShowsCursorOnFocusedRow(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("first\nsecond", true)
	buffer.row = 1
	buffer.col = 3

	lines := buffer.RenderLines(20, 3, true)
	if got := lines[1]; got != "sec█ond" {
		t.Fatalf("unexpected focused line: %q", got)
	}
}
