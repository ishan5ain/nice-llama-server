package tui

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed llama_server_args.md
var llamaServerArgsCatalogMarkdown string

type llamaArgOption struct {
	Aliases []string
}

var (
	llamaArgCatalogOnce sync.Once
	llamaArgCatalog     []llamaArgOption
)

func loadLlamaArgCatalog() []llamaArgOption {
	llamaArgCatalogOnce.Do(func() {
		llamaArgCatalog = parseLlamaArgCatalog(llamaServerArgsCatalogMarkdown)
	})
	return llamaArgCatalog
}

func parseLlamaArgCatalog(markdown string) []llamaArgOption {
	lines := strings.Split(markdown, "\n")
	options := make([]llamaArgOption, 0)
	seen := map[string]struct{}{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.Contains(line, "`") {
			continue
		}
		cells := markdownTableCells(line)
		if len(cells) == 0 || strings.EqualFold(strings.TrimSpace(cells[0]), "Argument") {
			continue
		}
		aliases := parseArgAliases(cells[0])
		if len(aliases) == 0 || containsModelAlias(aliases) {
			continue
		}
		key := strings.Join(aliases, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		options = append(options, llamaArgOption{Aliases: aliases})
	}
	return options
}

func markdownTableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	var cells []string
	var cell strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			cell.WriteRune(r)
			escaped = false
		case r == '\\':
			cell.WriteRune(r)
			escaped = true
		case r == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteRune(r)
		}
	}
	cells = append(cells, strings.TrimSpace(cell.String()))
	return cells
}

func parseArgAliases(cell string) []string {
	cell = strings.ReplaceAll(cell, "`", "")
	parts := strings.Split(cell, ",")
	aliases := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "-") {
			continue
		}
		alias := fields[0]
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		aliases = append(aliases, alias)
	}
	return aliases
}

func containsModelAlias(aliases []string) bool {
	for _, alias := range aliases {
		if alias == "-m" || alias == "--model" {
			return true
		}
	}
	return false
}
