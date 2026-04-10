package bookmark

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"nice-llama-server/internal/config"
)

var ErrModelFlag = errors.New("args must not include -m or --model")

func ParseArgs(text string) ([]string, error) {
	lines := strings.Split(text, "\n")
	var out []string
	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tokens, err := splitLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", idx+1, err)
		}
		if len(tokens) == 0 {
			continue
		}
		if !strings.HasPrefix(tokens[0], "-") {
			return nil, fmt.Errorf("line %d: each line must start with a flag", idx+1)
		}
		for _, token := range tokens {
			if token == "-m" || token == "--model" {
				return nil, fmt.Errorf("line %d: %w", idx+1, ErrModelFlag)
			}
		}
		out = append(out, tokens...)
	}
	return out, nil
}

func NormalizeBookmark(input config.Bookmark) (config.Bookmark, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ModelPath = strings.TrimSpace(input.ModelPath)
	input.ArgsText = normalizeArgsText(input.ArgsText)
	if input.Name == "" {
		return config.Bookmark{}, errors.New("bookmark name is required")
	}
	if input.ModelPath == "" {
		return config.Bookmark{}, errors.New("model path is required")
	}
	if _, err := ParseArgs(input.ArgsText); err != nil {
		return config.Bookmark{}, err
	}
	now := time.Now().UTC()
	if input.ID == "" {
		input.ID = NewID()
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.UpdatedAt = now
	input.GroupKey = config.DeriveGroupKey(input.ModelPath)
	return input, nil
}

func normalizeArgsText(text string) string {
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func splitLine(line string) ([]string, error) {
	var (
		out         []string
		buf         strings.Builder
		inSingle    bool
		inDouble    bool
		escaped     bool
		justClosedQ bool
	)

	flush := func(force bool) {
		if buf.Len() == 0 && !force {
			return
		}
		out = append(out, buf.String())
		buf.Reset()
	}

	for _, r := range line {
		switch {
		case escaped:
			buf.WriteRune(r)
			escaped = false
		case r == '\\' && !inSingle:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
			justClosedQ = true
		case r == '"' && !inSingle:
			inDouble = !inDouble
			justClosedQ = true
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush(justClosedQ)
			justClosedQ = false
		default:
			buf.WriteRune(r)
			justClosedQ = false
		}
	}
	if escaped {
		return nil, errors.New("dangling escape")
	}
	if inSingle || inDouble {
		return nil, errors.New("unterminated quote")
	}
	flush(justClosedQ)
	return out, nil
}
