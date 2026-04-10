package tui

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"nice-llama-server/internal/config"
)

type bookmarkEditor struct {
	originalID      string
	isNew           bool
	focus           int
	name            textBuffer
	model           textBuffer
	args            textBuffer
	completionIndex int
}

func newBookmarkEditor(b config.Bookmark, models []config.DiscoveredModel, isNew bool) *bookmarkEditor {
	return &bookmarkEditor{
		originalID: b.ID,
		isNew:      isNew,
		name:       newTextBuffer(b.Name, false),
		model:      newTextBuffer(displayModelValue(b.ModelPath, models), false),
		args:       newTextBuffer(b.ArgsText, true),
	}
}

func (e *bookmarkEditor) Bookmark(models []config.DiscoveredModel) (config.Bookmark, error) {
	modelPath, err := resolveModelPath(e.model.Value(), models)
	if err != nil {
		return config.Bookmark{}, err
	}
	return config.Bookmark{
		ID:        e.originalID,
		Name:      strings.TrimSpace(e.name.Value()),
		ModelPath: modelPath,
		ArgsText:  e.args.Value(),
	}, nil
}

func (e *bookmarkEditor) NextFocus() {
	e.focus = (e.focus + 1) % 3
}

func (e *bookmarkEditor) PrevFocus() {
	e.focus = (e.focus + 2) % 3
}

func (e *bookmarkEditor) Current() *textBuffer {
	switch e.focus {
	case 0:
		return &e.name
	case 1:
		return &e.model
	default:
		return &e.args
	}
}

func (e *bookmarkEditor) AutocompleteModel(models []config.DiscoveredModel) bool {
	if e.focus != 1 {
		return false
	}
	suggestions := modelSuggestionNames(models, e.model.Value(), 25)
	if len(suggestions) == 0 {
		return false
	}

	current := strings.TrimSpace(e.model.Value())
	index := 0
	for i, suggestion := range suggestions {
		if strings.EqualFold(suggestion, current) {
			index = (i + 1) % len(suggestions)
			break
		}
	}

	e.model.SetValue(suggestions[index])
	e.completionIndex = index
	return true
}

type textBuffer struct {
	lines     [][]rune
	row       int
	col       int
	multiline bool
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
	b.row = len(b.lines) - 1
	b.col = len(b.lines[b.row])
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
	for _, r := range text {
		if r == '\n' {
			if b.multiline {
				b.InsertNewLine()
			}
			continue
		}
		line := b.lines[b.row]
		line = append(line[:b.col], append([]rune{r}, line[b.col:]...)...)
		b.lines[b.row] = line
		b.col++
	}
}

func (b *textBuffer) InsertNewLine() {
	if !b.multiline {
		return
	}
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

func (b *textBuffer) Backspace() {
	if b.col > 0 {
		line := b.lines[b.row]
		line = append(line[:b.col-1], line[b.col:]...)
		b.lines[b.row] = line
		b.col--
		return
	}
	if !b.multiline || b.row == 0 {
		return
	}
	prevLen := len(b.lines[b.row-1])
	b.lines[b.row-1] = append(b.lines[b.row-1], b.lines[b.row]...)
	b.lines = append(b.lines[:b.row], b.lines[b.row+1:]...)
	b.row--
	b.col = prevLen
}

func (b *textBuffer) Delete() {
	line := b.lines[b.row]
	if b.col < len(line) {
		line = append(line[:b.col], line[b.col+1:]...)
		b.lines[b.row] = line
		return
	}
	if !b.multiline || b.row+1 >= len(b.lines) {
		return
	}
	b.lines[b.row] = append(b.lines[b.row], b.lines[b.row+1]...)
	b.lines = append(b.lines[:b.row+1], b.lines[b.row+2:]...)
}

func (b *textBuffer) MoveLeft() {
	if b.col > 0 {
		b.col--
		return
	}
	if b.multiline && b.row > 0 {
		b.row--
		b.col = len(b.lines[b.row])
	}
}

func (b *textBuffer) MoveRight() {
	if b.col < len(b.lines[b.row]) {
		b.col++
		return
	}
	if b.multiline && b.row+1 < len(b.lines) {
		b.row++
		b.col = 0
	}
}

func (b *textBuffer) MoveUp() {
	if !b.multiline || b.row == 0 {
		return
	}
	b.row--
	if b.col > len(b.lines[b.row]) {
		b.col = len(b.lines[b.row])
	}
}

func (b *textBuffer) MoveDown() {
	if !b.multiline || b.row+1 >= len(b.lines) {
		return
	}
	b.row++
	if b.col > len(b.lines[b.row]) {
		b.col = len(b.lines[b.row])
	}
}

func (b *textBuffer) MoveHome() {
	b.col = 0
}

func (b *textBuffer) MoveEnd() {
	b.col = len(b.lines[b.row])
}

func (b *textBuffer) Render(width, height int, focused bool) []string {
	if width < 4 {
		width = 4
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
			lines = append(lines, strings.Repeat(" ", width))
			continue
		}
		text := string(b.lines[idx])
		if focused && idx == b.row {
			text = withCursor(text, b.col)
		}
		lines = append(lines, pad(text, width))
	}
	return lines
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

func displayModelValue(path string, models []config.DiscoveredModel) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	for _, model := range models {
		if model.Path == path {
			return model.DisplayName
		}
	}
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func resolveModelPath(input string, models []config.DiscoveredModel) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", errors.New("model name is required")
	}

	if looksLikePath(value) {
		return value, nil
	}

	matches := exactModelMatches(models, value)
	switch len(matches) {
	case 0:
		return "", errors.New("model name did not match any discovered GGUF")
	case 1:
		return matches[0].Path, nil
	default:
		return "", errors.New("model name matches multiple discovered GGUF files; type a more specific name")
	}
}

func exactModelMatches(models []config.DiscoveredModel, input string) []config.DiscoveredModel {
	value := strings.TrimSpace(strings.ToLower(input))
	if value == "" {
		return nil
	}
	var matches []config.DiscoveredModel
	for _, model := range models {
		base := strings.ToLower(filepath.Base(model.Path))
		display := strings.ToLower(model.DisplayName)
		if value == display || value == base || value == strings.TrimSuffix(base, filepath.Ext(base)) {
			matches = append(matches, model)
		}
	}
	return matches
}

func modelSuggestionNames(models []config.DiscoveredModel, query string, limit int) []string {
	items := matchingModels(models, query, limit)
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item.DisplayName]; ok {
			continue
		}
		seen[item.DisplayName] = struct{}{}
		out = append(out, item.DisplayName)
	}
	slices.SortFunc(out, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func matchingModels(models []config.DiscoveredModel, query string, limit int) []config.DiscoveredModel {
	query = strings.TrimSpace(strings.ToLower(query))
	if limit <= 0 {
		limit = len(models)
	}
	var prefix []config.DiscoveredModel
	var contains []config.DiscoveredModel
	for _, model := range models {
		display := strings.ToLower(model.DisplayName)
		base := strings.ToLower(filepath.Base(model.Path))
		if query == "" || strings.HasPrefix(display, query) || strings.HasPrefix(base, query) {
			prefix = append(prefix, model)
			continue
		}
		if strings.Contains(display, query) || strings.Contains(base, query) {
			contains = append(contains, model)
		}
	}
	out := append(prefix, contains...)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func looksLikePath(value string) bool {
	return strings.Contains(value, `/`) || strings.Contains(value, `\`) || strings.HasSuffix(strings.ToLower(value), ".gguf")
}
