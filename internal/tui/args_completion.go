package tui

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	argCompletionForward  = 1
	argCompletionBackward = -1
)

func (m *model) handleArgCompletionTab(direction int) bool {
	if m.editor == nil || m.focus != focusDetailArgs {
		return false
	}
	if m.continueArgCompletionCycle(direction) {
		return true
	}

	ctx := m.editor.args.TokenAtCursor()
	candidates := m.argCompletionCandidates(ctx)
	if len(candidates) == 0 {
		m.editor.completion = argCompletionState{}
		return false
	}

	m.editor.completion = argCompletionState{
		active:     true,
		row:        ctx.row,
		start:      ctx.start,
		end:        ctx.end,
		prefix:     ctx.prefix,
		candidates: candidates,
	}
	index := 0
	if direction == argCompletionBackward {
		index = len(candidates) - 1
	}
	m.applyArgCompletionCandidate(index)
	return true
}

func (m *model) continueArgCompletionCycle(direction int) bool {
	state := m.editor.completion
	if !state.active || len(state.candidates) == 0 {
		return false
	}
	if m.editor.args.row != state.row || m.editor.args.col != state.end {
		m.editor.completion = argCompletionState{}
		return false
	}
	nextIndex := 0
	if state.passive {
		if direction == argCompletionBackward {
			nextIndex = len(state.candidates) - 1
		}
		m.applyArgCompletionCandidate(nextIndex)
		return true
	}
	nextIndex = (state.index + direction + len(state.candidates)) % len(state.candidates)
	m.applyArgCompletionCandidate(nextIndex)
	return true
}

func (m *model) applyArgCompletionCandidate(index int) {
	state := &m.editor.completion
	if index < 0 || index >= len(state.candidates) {
		return
	}
	text := state.candidates[index].Text
	if !m.editor.args.ReplaceRange(state.row, state.start, state.end, text) {
		state.active = false
		return
	}
	state.index = index
	state.end = state.start + len([]rune(text))
	state.passive = false
}

func (m *model) argCompletionCandidates(ctx tokenContext) []argCompletionCandidate {
	if mmctx, ok := m.mmprojValueCompletionContext(); ok {
		return m.mmprojArgCompletionCandidates(mmctx, runtime.GOOS)
	}
	if ctx.token != "" && ctx.prefix != "" && !strings.HasPrefix(ctx.prefix, "-") {
		return nil
	}

	catalog := loadLlamaArgCatalog()
	if len(catalog) == 0 {
		return nil
	}

	used := usedLlamaArgOptions(m.editor.args, ctx)
	passive := isPassiveArgCompletionPrefix(ctx.prefix)
	popularity := map[int]int{}
	if passive {
		popularity = m.argOptionPopularity(catalog)
	}
	var exact []argCompletionCandidate
	candidates := make([]argCompletionCandidate, 0)
	seenText := map[string]struct{}{}
	for optionIndex, option := range catalog {
		if _, ok := used[optionIndex]; ok {
			continue
		}
		for _, alias := range option.Aliases {
			if ctx.prefix != "" && !strings.HasPrefix(alias, ctx.prefix) {
				continue
			}
			if ctx.prefix == "--" && !strings.HasPrefix(alias, "--") {
				continue
			}
			if ctx.prefix == "" && strings.HasPrefix(alias, "-") && !strings.HasPrefix(alias, "--") {
				continue
			}
			if _, ok := seenText[alias]; ok {
				continue
			}
			seenText[alias] = struct{}{}
			candidate := argCompletionCandidate{
				Text:        alias,
				optionIndex: optionIndex,
			}
			if ctx.prefix != "" && alias == ctx.prefix {
				exact = append(exact, candidate)
				continue
			}
			candidates = append(candidates, candidate)
		}
	}
	candidates = append(exact, candidates...)
	if passive {
		sort.SliceStable(candidates, func(i, j int) bool {
			left := popularity[candidates[i].optionIndex]
			right := popularity[candidates[j].optionIndex]
			return left > right
		})
	}
	return candidates
}

func (m *model) mmprojValueCompletionContext() (tokenContext, bool) {
	if m.editor == nil {
		return tokenContext{}, false
	}

	buffer := &m.editor.args
	row := buffer.row
	if row < 0 || row >= len(buffer.lines) {
		return tokenContext{}, false
	}

	line := buffer.lines[row]
	col := buffer.col
	if col < 0 {
		col = 0
	}
	if col > len(line) {
		col = len(line)
	}

	tokens := scanLineTokens(line)
	currentIndex := -1
	previousIndex := -1
	for i, token := range tokens {
		if token.end <= col {
			previousIndex = i
		}
		if col >= token.start && col <= token.end {
			currentIndex = i
			break
		}
	}

	if currentIndex >= 0 {
		if currentIndex == 0 || !isMMProjValueFlag(tokens[currentIndex-1].text) {
			return tokenContext{}, false
		}
		return tokenContext{
			row:    row,
			start:  tokens[currentIndex].start,
			end:    tokens[currentIndex].end,
			prefix: string(line[tokens[currentIndex].start:col]),
			token:  tokens[currentIndex].text,
		}, true
	}

	if previousIndex < 0 || !isMMProjValueFlag(tokens[previousIndex].text) {
		return tokenContext{}, false
	}

	return tokenContext{
		row:   row,
		start: col,
		end:   col,
	}, true
}

func isMMProjValueFlag(token string) bool {
	return token == "-mm" || token == "--mmproj"
}

func (m *model) mmprojArgCompletionCandidates(ctx tokenContext, goos string) []argCompletionCandidate {
	model := m.discoveredModelByPath(m.editor.modelPath)
	if model == nil || len(model.MMProjPaths) == 0 {
		return nil
	}

	var exact []argCompletionCandidate
	candidates := make([]argCompletionCandidate, 0, len(model.MMProjPaths))
	for _, path := range model.MMProjPaths {
		if !matchesMMProjPrefix(ctx.prefix, path, goos) {
			continue
		}
		candidate := argCompletionCandidate{
			Text: formatMMProjCompletionPathForOS(path, goos),
		}
		if ctx.prefix != "" && candidate.Text == ctx.prefix {
			exact = append(exact, candidate)
			continue
		}
		candidates = append(candidates, candidate)
	}
	return append(exact, candidates...)
}

func matchesMMProjPrefix(prefix, path, goos string) bool {
	if prefix == "" {
		return true
	}

	formatted := formatMMProjCompletionPathForOS(path, goos)
	if hasPathPrefix(formatted, prefix, goos) {
		return true
	}

	prefix = strings.TrimLeft(prefix, `'"`)
	if prefix == "" {
		return true
	}

	basename := mmprojPathBase(path, goos)
	return hasPathPrefix(path, prefix, goos) ||
		hasPathPrefix(basename, prefix, goos)
}

func hasPathPrefix(value, prefix, goos string) bool {
	if goos == "windows" {
		value = strings.ToLower(value)
		prefix = strings.ToLower(prefix)
	}
	return strings.HasPrefix(value, prefix)
}

func formatMMProjCompletionPathForOS(path, goos string) string {
	if goos == "windows" {
		return "'" + path + "'"
	}
	return path
}

func mmprojPathBase(path, goos string) string {
	if goos == "windows" {
		path = strings.ReplaceAll(path, `\`, "/")
		return filepath.Base(path)
	}
	return filepath.Base(path)
}

func (m *model) refreshPassiveArgCompletion() {
	if m.editor == nil || m.focus != focusDetailArgs {
		return
	}
	ctx := m.editor.args.TokenAtCursor()
	if ctx.token != ctx.prefix || !isPassiveArgCompletionPrefix(ctx.prefix) {
		m.editor.completion = argCompletionState{}
		return
	}
	candidates := m.argCompletionCandidates(ctx)
	if len(candidates) == 0 {
		m.editor.completion = argCompletionState{}
		return
	}
	m.editor.completion = argCompletionState{
		active:     true,
		passive:    true,
		row:        ctx.row,
		start:      ctx.start,
		end:        ctx.end,
		prefix:     ctx.prefix,
		candidates: candidates,
	}
}

func isPassiveArgCompletionPrefix(prefix string) bool {
	return prefix == "-" || prefix == "--"
}

func (m *model) argOptionPopularity(catalog []llamaArgOption) map[int]int {
	aliasToOption := aliasOptionIndex(catalog)
	popularity := map[int]int{}
	for _, bookmark := range m.snapshot.Bookmarks {
		if m.editor != nil && m.editor.originalID != "" && bookmark.ID == m.editor.originalID {
			continue
		}
		buffer := newTextBuffer(bookmark.ArgsText, true)
		seenInBookmark := map[int]struct{}{}
		for _, token := range scanBufferTokens(buffer) {
			optionIndex, ok := aliasToOption[token.text]
			if !ok {
				continue
			}
			seenInBookmark[optionIndex] = struct{}{}
		}
		for optionIndex := range seenInBookmark {
			popularity[optionIndex]++
		}
	}
	return popularity
}

func usedLlamaArgOptions(buffer textBuffer, skip tokenContext) map[int]struct{} {
	catalog := loadLlamaArgCatalog()
	aliasToOption := aliasOptionIndex(catalog)

	used := map[int]struct{}{}
	for _, token := range scanBufferTokens(buffer) {
		if token.row == skip.row && token.start == skip.start && token.end == skip.end {
			continue
		}
		optionIndex, ok := aliasToOption[token.text]
		if !ok {
			continue
		}
		used[optionIndex] = struct{}{}
	}
	return used
}

func aliasOptionIndex(catalog []llamaArgOption) map[string]int {
	aliasToOption := make(map[string]int)
	for optionIndex, option := range catalog {
		for _, alias := range option.Aliases {
			aliasToOption[alias] = optionIndex
		}
	}
	return aliasToOption
}

type bufferToken struct {
	row   int
	start int
	end   int
	text  string
}

func scanBufferTokens(buffer textBuffer) []bufferToken {
	var tokens []bufferToken
	for row, line := range buffer.lines {
		for _, token := range scanLineTokens(line) {
			if !strings.HasPrefix(token.text, "-") {
				continue
			}
			tokens = append(tokens, bufferToken{
				row:   row,
				start: token.start,
				end:   token.end,
				text:  token.text,
			})
		}
	}
	return tokens
}

func completionWindow(candidates []argCompletionCandidate, index, limit int) []argCompletionCandidate {
	if len(candidates) == 0 || limit <= 0 {
		return nil
	}
	if len(candidates) == 1 {
		return nil
	}
	if limit > len(candidates)-1 {
		limit = len(candidates) - 1
	}
	out := make([]argCompletionCandidate, 0, limit)
	for offset := 1; offset <= limit; offset++ {
		out = append(out, candidates[(index+offset)%len(candidates)])
	}
	return out
}

func passiveCompletionWindow(candidates []argCompletionCandidate, limit int) []argCompletionCandidate {
	if len(candidates) == 0 || limit <= 0 {
		return nil
	}
	if limit > len(candidates) {
		limit = len(candidates)
	}
	return append([]argCompletionCandidate(nil), candidates[:limit]...)
}
