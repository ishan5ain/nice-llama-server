package tui

import (
	"strings"
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

func TestTextBufferUndoAfterInsertText(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("hello", false)
	buffer.col = 5
	buffer.InsertText(" world")
	if buffer.Value() != "hello world" {
		t.Fatalf("expected 'hello world', got %q", buffer.Value())
	}
	if !buffer.Undo() {
		t.Fatalf("undo should succeed")
	}
	if buffer.Value() != "hello" {
		t.Fatalf("expected 'hello' after undo, got %q", buffer.Value())
	}
}

func TestTextBufferUndoAfterBackspace(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("hello", false)
	buffer.col = 5
	buffer.Backspace()
	if buffer.Value() != "hell" {
		t.Fatalf("expected 'hell', got %q", buffer.Value())
	}
	if !buffer.Undo() {
		t.Fatalf("undo should succeed")
	}
	if buffer.Value() != "hello" {
		t.Fatalf("expected 'hello' after undo, got %q", buffer.Value())
	}
}

func TestTextBufferUndoAfterDelete(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("hello", false)
	buffer.col = 0
	buffer.Delete()
	if buffer.Value() != "ello" {
		t.Fatalf("expected 'ello', got %q", buffer.Value())
	}
	if !buffer.Undo() {
		t.Fatalf("undo should succeed")
	}
	if buffer.Value() != "hello" {
		t.Fatalf("expected 'hello' after undo, got %q", buffer.Value())
	}
}

func TestTextBufferUndoEmptyStackReturnsFalse(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("hello", false)
	if buffer.Undo() {
		t.Fatalf("undo on empty stack should return false")
	}
}

func TestTextBufferUndoAfterSetValueClearsStack(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("hello", false)
	buffer.InsertText(" world")
	buffer.SetValue("new value")
	if buffer.Undo() {
		t.Fatalf("undo after SetValue should return false")
	}
	if buffer.Value() != "new value" {
		t.Fatalf("expected 'new value', got %q", buffer.Value())
	}
}

func TestTextBufferUndoCappedAtMaxDepth(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("start", false)
	for i := 0; i < maxUndoDepth+5; i++ {
		buffer.col = len([]rune(buffer.Value()))
		buffer.InsertText("x")
	}
	for i := 0; i < maxUndoDepth+5; i++ {
		buffer.Undo()
	}
	expected := "start" + strings.Repeat("x", 5)
	if buffer.Value() != expected {
		t.Fatalf("expected %q after undo overflow, got %q", expected, buffer.Value())
	}
}

func TestTextBufferMultipleUndosWithRedo(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("a", false)
	buffer.col = 1
	buffer.InsertText("b")
	buffer.col = 2
	buffer.InsertText("c")
	if buffer.Value() != "abc" {
		t.Fatalf("expected 'abc', got %q", buffer.Value())
	}
	buffer.Undo()
	if buffer.Value() != "ab" {
		t.Fatalf("expected 'ab' after first undo, got %q", buffer.Value())
	}
	buffer.Undo()
	if buffer.Value() != "a" {
		t.Fatalf("expected 'a' after second undo, got %q", buffer.Value())
	}
}

func TestTextBufferUndoPreservesCursorPosition(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("hello", false)
	buffer.col = 3
	buffer.InsertText("XX")
	if buffer.col != 5 {
		t.Fatalf("expected col=5 after insert, got %d", buffer.col)
	}
	buffer.Undo()
	if buffer.col != 3 {
		t.Fatalf("expected col=3 after undo, got %d", buffer.col)
	}
}

func TestTextBufferUndoAfterNewLine(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("hello\nworld", true)
	buffer.row = 0
	buffer.col = 5
	buffer.InsertNewLine()
	if buffer.Value() != "hello\n\nworld" {
		t.Fatalf("expected 'hello\\n\\nworld', got %q", buffer.Value())
	}
	if !buffer.Undo() {
		t.Fatalf("undo should succeed")
	}
	if buffer.Value() != "hello\nworld" {
		t.Fatalf("expected 'hello\\nworld' after undo, got %q", buffer.Value())
	}
}

func TestTextBufferReplaceRangeWithUndo(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("hello world", true)
	buffer.ReplaceRange(0, 6, 11, "moon")
	if buffer.Value() != "hello moon" {
		t.Fatalf("expected 'hello moon', got %q", buffer.Value())
	}
	if !buffer.Undo() {
		t.Fatalf("undo should succeed")
	}
	if buffer.Value() != "hello world" {
		t.Fatalf("expected 'hello world' after undo, got %q", buffer.Value())
	}
}

func TestInsertTextStripsANSIEscapeSequences(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("", true)
	buffer.InsertText("\x1b[38;2;255;0;0mHello\x1b[0m\x1b[38;2;0;255;0m World\x1b[0m")
	if buffer.Value() != "Hello World" {
		t.Fatalf("expected 'Hello World', got %q", buffer.Value())
	}
}

func TestInsertTextStripsNonPrintableCharacters(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("", true)
	buffer.InsertText("hello\x00world\x01test\x02end")
	if buffer.Value() != "helloworldtestend" {
		t.Fatalf("expected 'helloworldtestend', got %q", buffer.Value())
	}
}

func TestInsertTextNormalizesCRLF(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("", true)
	buffer.InsertText("line1\r\nline2\r\nline3")
	if buffer.Value() != "line1\nline2\nline3" {
		t.Fatalf("expected 'line1\\nline2\\nline3', got %q", buffer.Value())
	}
}

func TestInsertTextNormalizesLoneCR(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("", true)
	buffer.InsertText("line1\rline2\rline3")
	if buffer.Value() != "line1\nline2\nline3" {
		t.Fatalf("expected 'line1\\nline2\\nline3', got %q", buffer.Value())
	}
}

func TestInsertTextSingleLineStripsNewlines(t *testing.T) {
	t.Parallel()

	buffer := newTextBuffer("", false)
	buffer.InsertText("line1\nline2\r\nline3")
	if buffer.Value() != "line1 line2 line3" {
		t.Fatalf("expected 'line1 line2 line3', got %q", buffer.Value())
	}
}
