package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"nice-llama-server/internal/config"
)

const (
	maxVisibleArgCompletions = 4
	maxUndoDepth             = 50
)

type textSnapshot struct {
	lines [][]rune
	row   int
	col   int
}

type bookmarkEditor struct {
	originalID  string
	isNew       bool
	modelPath   string
	groupKey    string
	initialName string
	initialArgs string
	name        textBuffer
	args        textBuffer
	completion  argCompletionState
}

func newBookmarkEditor(b config.Bookmark, isNew bool) *bookmarkEditor {
	return &bookmarkEditor{
		originalID:  b.ID,
		isNew:       isNew,
		modelPath:   b.ModelPath,
		groupKey:    b.GroupKey,
		initialName: strings.TrimSpace(b.Name),
		initialArgs: strings.TrimSpace(b.ArgsText),
		name:        newTextBuffer(b.Name, false),
		args:        newTextBuffer(b.ArgsText, true),
	}
}

func (e *bookmarkEditor) Bookmark() config.Bookmark {
	return config.Bookmark{
		ID:        e.originalID,
		Name:      strings.TrimSpace(e.name.Value()),
		ModelPath: e.modelPath,
		GroupKey:  e.groupKey,
		ArgsText:  strings.TrimSpace(e.args.Value()),
	}
}

func (e *bookmarkEditor) Dirty() bool {
	return strings.TrimSpace(e.name.Value()) != e.initialName ||
		strings.TrimSpace(e.args.Value()) != e.initialArgs
}

type argCompletionState struct {
	active     bool
	passive    bool
	row        int
	start      int
	end        int
	prefix     string
	index      int
	candidates []argCompletionCandidate
}

type argCompletionCandidate struct {
	Text        string
	optionIndex int
}

type tokenContext struct {
	row    int
	start  int
	end    int
	prefix string
	token  string
}

type textBuffer struct {
	lines     [][]rune
	row       int
	col       int
	multiline bool
	undoStack []textSnapshot
	redoStack []textSnapshot
}

func newTextBuffer(value string, multiline bool) textBuffer {
	buf := textBuffer{multiline: multiline}
	buf.SetValue(value)
	return buf
}

func (b *textBuffer) SetValue(value string) {
	if value == "" {
		b.lines = [][]rune{{}}
		b.row, b.col = 0, 0
		b.undoStack = nil
		b.redoStack = nil
		return
	}
	parts := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	b.lines = make([][]rune, 0, len(parts))
	for _, part := range parts {
		b.lines = append(b.lines, []rune(part))
	}
	if len(b.lines) == 0 {
		b.lines = [][]rune{{}}
	}
	b.row = min(len(b.lines)-1, b.row)
	b.col = min(len(b.lines[b.row]), b.col)
	b.undoStack = nil
	b.redoStack = nil
}

func (b *textBuffer) snapshot() textSnapshot {
	snap := textSnapshot{
		lines: make([][]rune, len(b.lines)),
		row:   b.row,
		col:   b.col,
	}
	for i, line := range b.lines {
		snap.lines[i] = make([]rune, len(line))
		copy(snap.lines[i], line)
	}
	return snap
}

func (b *textBuffer) pushUndo() {
	snap := b.snapshot()
	b.undoStack = append(b.undoStack, snap)
	if len(b.undoStack) > maxUndoDepth {
		b.undoStack = b.undoStack[len(b.undoStack)-maxUndoDepth:]
	}
}

func (b *textBuffer) Undo() bool {
	if len(b.undoStack) == 0 {
		return false
	}
	b.redoStack = append(b.redoStack, b.snapshot())
	if len(b.redoStack) > maxUndoDepth {
		b.redoStack = b.redoStack[len(b.redoStack)-maxUndoDepth:]
	}
	prev := b.undoStack[len(b.undoStack)-1]
	b.undoStack = b.undoStack[:len(b.undoStack)-1]
	b.lines = prev.lines
	b.row = prev.row
	b.col = prev.col
	return true
}

func (b *textBuffer) Value() string {
	parts := make([]string, len(b.lines))
	for i := range b.lines {
		parts[i] = string(b.lines[i])
	}
	return strings.Join(parts, "\n")
}

func (b *textBuffer) InsertText(text string) {
	if text == "" {
		return
	}
	b.pushUndo()
	text = ansi.Strip(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	for _, r := range text {
		if r == '\n' {
			if b.multiline {
				b.InsertNewLine()
			} else {
				b.insertRune(' ')
			}
			continue
		}
		if !unicode.IsPrint(r) && r != '\t' {
			continue
		}
		b.insertRune(r)
	}
}

func (b *textBuffer) insertRune(r rune) {
	line := b.lines[b.row]
	line = append(line[:b.col], append([]rune{r}, line[b.col:]...)...)
	b.lines[b.row] = line
	b.col++
}

func (b *textBuffer) InsertNewLine() {
	if !b.multiline {
		return
	}
	b.pushUndo()
	line := b.lines[b.row]
	left := append([]rune{}, line[:b.col]...)
	right := append([]rune{}, line[b.col:]...)
	b.lines[b.row] = left
	next := make([][]rune, 0, len(b.lines)+1)
	next = append(next, b.lines[:b.row+1]...)
	next = append(next, right)
	next = append(next, b.lines[b.row+1:]...)
	b.lines = next
	b.row++
	b.col = 0
}

func (b *textBuffer) ReplaceRange(row, start, end int, text string) bool {
	if row < 0 || row >= len(b.lines) {
		return false
	}
	b.pushUndo()
	line := b.lines[row]
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if start > len(line) {
		start = len(line)
	}
	if end > len(line) {
		end = len(line)
	}
	replacement := []rune(text)
	next := make([]rune, 0, len(line)-end+start+len(replacement))
	next = append(next, line[:start]...)
	next = append(next, replacement...)
	next = append(next, line[end:]...)
	b.lines[row] = next
	b.row = row
	b.col = start + len(replacement)
	return true
}

func (b *textBuffer) Backspace() {
	if b.col > 0 {
		b.pushUndo()
		line := b.lines[b.row]
		line = append(line[:b.col-1], line[b.col:]...)
		b.lines[b.row] = line
		b.col--
		return
	}
	if !b.multiline || b.row == 0 {
		return
	}
	b.pushUndo()
	prevLen := len(b.lines[b.row-1])
	b.lines[b.row-1] = append(b.lines[b.row-1], b.lines[b.row]...)
	b.lines = append(b.lines[:b.row], b.lines[b.row+1:]...)
	b.row--
	b.col = prevLen
}

func (b *textBuffer) Delete() {
	line := b.lines[b.row]
	if b.col < len(line) {
		b.pushUndo()
		line = append(line[:b.col], line[b.col+1:]...)
		b.lines[b.row] = line
		return
	}
	if !b.multiline || b.row+1 >= len(b.lines) {
		return
	}
	b.pushUndo()
	b.lines[b.row] = append(b.lines[b.row], b.lines[b.row+1]...)
	b.lines = append(b.lines[:b.row+1], b.lines[b.row+2:]...)
}

func (b *textBuffer) MoveLeft() bool {
	if b.col > 0 {
		b.col--
		return true
	}
	if b.multiline && b.row > 0 {
		b.row--
		b.col = len(b.lines[b.row])
		return true
	}
	return false
}

func (b *textBuffer) MoveRight() bool {
	if b.col < len(b.lines[b.row]) {
		b.col++
		return true
	}
	if b.multiline && b.row+1 < len(b.lines) {
		b.row++
		b.col = 0
		return true
	}
	return false
}

func (b *textBuffer) MoveUp() bool {
	if !b.multiline || b.row == 0 {
		return false
	}
	b.row--
	if b.col > len(b.lines[b.row]) {
		b.col = len(b.lines[b.row])
	}
	return true
}

func (b *textBuffer) MoveDown() bool {
	if !b.multiline || b.row+1 >= len(b.lines) {
		return false
	}
	b.row++
	if b.col > len(b.lines[b.row]) {
		b.col = len(b.lines[b.row])
	}
	return true
}

func (b *textBuffer) MoveHome() bool {
	if b.col == 0 {
		return false
	}
	b.col = 0
	return true
}

func (b *textBuffer) MoveEnd() bool {
	if b.col == len(b.lines[b.row]) {
		return false
	}
	b.col = len(b.lines[b.row])
	return true
}

func (b *textBuffer) RenderLines(width, height int, focused bool) []string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	start := 0
	if b.multiline && b.row >= height {
		start = b.row - height + 1
	}
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		idx := start + i
		if idx >= len(b.lines) {
			lines = append(lines, "")
			continue
		}
		text := string(b.lines[idx])
		if focused && idx == b.row {
			text = withCursor(text, b.col)
		}
		lines = append(lines, text)
	}
	return lines
}

func (b *textBuffer) VisibleStart(height int) int {
	if height < 1 {
		height = 1
	}
	if b.multiline && b.row >= height {
		return b.row - height + 1
	}
	return 0
}

func (b *textBuffer) TokenAtCursor() tokenContext {
	row := b.row
	if row < 0 || row >= len(b.lines) {
		return tokenContext{row: row}
	}
	line := b.lines[row]
	col := b.col
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}

	start := col
	for start > 0 && !unicode.IsSpace(line[start-1]) {
		start--
	}
	end := col
	for end < len(line) && !unicode.IsSpace(line[end]) {
		end++
	}

	return tokenContext{
		row:    row,
		start:  start,
		end:    end,
		prefix: string(line[start:col]),
		token:  string(line[start:end]),
	}
}

func withCursor(text string, col int) string {
	runes := []rune(text)
	if col < 0 {
		col = 0
	}
	if col > len(runes) {
		col = len(runes)
	}
	cursor := '█'
	if col == len(runes) {
		return string(append(runes, cursor))
	}
	out := make([]rune, 0, len(runes)+1)
	out = append(out, runes[:col]...)
	out = append(out, cursor)
	out = append(out, runes[col:]...)
	return string(out)
}
