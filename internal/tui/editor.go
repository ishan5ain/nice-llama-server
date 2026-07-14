package tui

import (
	"strings"
	"unicode"

	"github.com/ishan5ain/tuiweave"
	"github.com/ishan5ain/tuiweave/textarea"
	"github.com/ishan5ain/tuiweave/textinput"

	"nice-llama-server/internal/config"
)

const (
	maxVisibleArgCompletions = 4
)

type bookmarkEditor struct {
	originalID  string
	isNew       bool
	modelPath   string
	groupKey    string
	initialName string
	initialArgs string
	name        textinput.Model
	args        textarea.Model
	completion  argCompletionState
}

func newBookmarkEditor(b config.Bookmark, isNew bool, theme tuiweave.Theme) *bookmarkEditor {
	e := &bookmarkEditor{
		originalID:  b.ID,
		isNew:       isNew,
		modelPath:   b.ModelPath,
		groupKey:    b.GroupKey,
		initialName: strings.TrimSpace(b.Name),
		initialArgs: strings.TrimSpace(b.ArgsText),
		name:        textinput.New(theme),
		args:        textarea.New(theme),
	}
	e.name.SetValue(b.Name)
	e.name.Prompt = ""
	e.args.SetValue(b.ArgsText)
	e.args.Prompt = ""
	return e
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

type lineToken struct {
	start int
	end   int
	text  string
}

func scanLineTokens(line []rune) []lineToken {
	tokens := make([]lineToken, 0)
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
		tokens = append(tokens, lineToken{
			start: start,
			end:   col,
			text:  string(line[start:col]),
		})
	}
	return tokens
}
