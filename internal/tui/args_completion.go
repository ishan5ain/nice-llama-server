package tui

import (
	"sort"
	"strings"
	"unicode"
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
	if ctx.token != "" && ctx.prefix != "" && !strings.HasPrefix(ctx.prefix, "-") {
		m.editor.completion = argCompletionState{}
		return false
	}

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
		col := 0
		for col < len(line) {
			for col < len(line) && unicode.IsSpace(line[col]) {
				col++
			}
			if col >= len(line) {
				break
			}
			start := col
			var quote rune
			escaped := false
			for col < len(line) {
				r := line[col]
				switch {
				case escaped:
					escaped = false
				case r == '\\' && quote != '\'':
					escaped = true
				case quote != 0:
					if r == quote {
						quote = 0
					}
				case r == '\'' || r == '"':
					quote = r
				case unicode.IsSpace(r):
					goto tokenDone
				}
				col++
			}
		tokenDone:
			end := col
			text := string(line[start:end])
			if strings.HasPrefix(text, "-") {
				tokens = append(tokens, bufferToken{
					row:   row,
					start: start,
					end:   end,
					text:  text,
				})
			}
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
