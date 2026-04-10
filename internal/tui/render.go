package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"nice-llama-server/internal/config"
)

func (m *model) render() string {
	header := box("Nice Llama Server", []string{
		runtimeSummary(m.snapshot),
		fmt.Sprintf("bookmarks: %d  models: %d  roots: %d", len(m.snapshot.Bookmarks), len(m.snapshot.Models), len(m.snapshot.Config.ModelRoots)),
		m.messageLine(),
	}, m.width, 5)

	mainWidth := m.width
	mainHeight := m.height - len(header)
	if mainHeight < 6 {
		mainHeight = 6
	}

	content := m.renderContent(mainWidth, mainHeight)
	return strings.Join(append(header, content...), "\n")
}

func (m *model) renderContent(width, height int) []string {
	showLogs := m.showLogs && (m.snapshot.Runtime.Status != config.StatusIdle || len(m.logs) > 0)
	if !showLogs {
		return m.renderManager(width, height)
	}
	if width >= height*2 {
		leftWidth := width / 2
		rightWidth := width - leftWidth
		left := m.renderManager(leftWidth, height)
		right := box("Logs", m.renderLogs(rightWidth-2, height-2), rightWidth, height)
		return sideBySide(left, right, leftWidth, rightWidth)
	}
	topHeight := height / 2
	if topHeight < 6 {
		topHeight = 6
	}
	bottomHeight := height - topHeight
	return append(
		m.renderManager(width, topHeight),
		box("Logs", m.renderLogs(width-2, bottomHeight-2), width, bottomHeight)...,
	)
}

func (m *model) renderManager(width, height int) []string {
	leftWidth := max(30, width/3)
	if leftWidth > width-20 {
		leftWidth = width / 2
	}
	rightWidth := width - leftWidth

	left := box("Bookmarks", m.renderBookmarks(leftWidth-2, height-2), leftWidth, height)
	rightTitle := "Details"
	var rightBody []string
	if m.editor != nil {
		rightTitle = "Editor"
		rightBody = m.renderEditor(rightWidth-2, height-2)
	} else {
		rightBody = m.renderDetails(rightWidth-2, height-2)
	}
	right := box(rightTitle, rightBody, rightWidth, height)
	return sideBySide(left, right, leftWidth, rightWidth)
}

func (m *model) renderBookmarks(width, height int) []string {
	if len(m.snapshot.Bookmarks) == 0 {
		return []string{
			pad("No bookmarks yet.", width),
			pad("Press n to create one.", width),
		}
	}

	lines := make([]string, 0, len(m.snapshot.Bookmarks)+4)
	group := ""
	for _, item := range m.snapshot.Bookmarks {
		if item.GroupKey != group {
			group = item.GroupKey
			lines = append(lines, pad("["+group+"]", width))
		}
		marker := "  "
		if item.ID == m.selectedID {
			marker = "> "
		}
		lines = append(lines, pad(marker+item.Name, width))
	}
	return clampLines(lines, height, width)
}

func (m *model) renderDetails(width, height int) []string {
	selected := m.selectedBookmark()
	if selected == nil {
		return clampLines([]string{
			"No bookmark selected.",
			"",
			"Keys:",
			"n new bookmark",
			"c clone bookmark",
			"e edit bookmark",
			"d delete bookmark",
			"L load selected bookmark",
			"U unload active runtime",
			"g toggle log panel",
			"r rescan model roots",
		}, height, width)
	}

	lines := []string{
		"Name: " + selected.Name,
		"Model: " + displayModelValue(selected.ModelPath, m.snapshot.Models),
		"Path:  " + selected.ModelPath,
		"Group: " + selected.GroupKey,
		"",
		"Args:",
	}
	args := strings.Split(strings.TrimSpace(selected.ArgsText), "\n")
	if len(args) == 1 && args[0] == "" {
		lines = append(lines, "  (none)")
	} else {
		for _, line := range args {
			lines = append(lines, "  "+line)
		}
	}
	lines = append(lines,
		"",
		"Discovered Models:",
	)
	for _, model := range firstModels(m.snapshot.Models, 6) {
		lines = append(lines, "  "+filepath.Base(model.Path))
	}
	if len(m.snapshot.Models) == 0 {
		lines = append(lines, "  (none)")
	}
	lines = append(lines,
		"",
		"Keys:",
		"n new  c clone  e edit  d delete",
		"L load  U unload  g logs  r rescan  q quit",
	)
	if m.confirmDelete {
		lines = append(lines, "", "Delete selected bookmark? Press y to confirm.")
	}
	return clampLines(lines, height, width)
}

func (m *model) renderEditor(width, height int) []string {
	if m.editor == nil {
		return nil
	}
	lines := []string{
		"Ctrl+N / Ctrl+P changes fields.",
		"Tab autocompletes the model filename.",
		"Ctrl+S saves. Ctrl+L saves and loads.",
		"Esc cancels editing.",
		"",
		"Name:",
	}
	lines = append(lines, m.editor.name.Render(width, 1, m.editor.focus == 0)...)
	lines = append(lines, "", "Model Name:")
	lines = append(lines, m.editor.model.Render(width, 1, m.editor.focus == 1)...)
	lines = append(lines, "", "Suggested Models:")
	suggestions := modelSuggestionLines(m.snapshot.Models, m.editor.model.Value(), 5)
	for _, line := range suggestions {
		lines = append(lines, "  "+line)
	}
	if len(suggestions) == 0 {
		lines = append(lines, "  (no matches)")
	}
	lines = append(lines, "", "Args:")
	argsHeight := height - len(lines) - 1
	if argsHeight < 4 {
		argsHeight = 4
	}
	lines = append(lines, m.editor.args.Render(width, argsHeight, m.editor.focus == 2)...)
	return clampLines(lines, height, width)
}

func (m *model) renderLogs(width, height int) []string {
	if len(m.logs) == 0 {
		return clampLines([]string{
			"Waiting for logs...",
		}, height, width)
	}
	lines := make([]string, 0, len(m.logs))
	for _, entry := range m.logs {
		lines = append(lines, fmt.Sprintf("%s [%s] %s", entry.TS.Format(time.Kitchen), entry.Stream, entry.Line))
	}
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	return clampLines(lines, height, width)
}

func (m *model) messageLine() string {
	if m.errorMessage != "" {
		return "error: " + m.errorMessage
	}
	if m.flashMessage != "" {
		return m.flashMessage
	}
	return "keys: n new  c clone  e edit  d delete  L load  U unload  g logs  r rescan  q quit"
}

func modelSuggestionLines(models []config.DiscoveredModel, query string, limit int) []string {
	items := matchingModels(models, query, limit)
	out := make([]string, 0, len(items))
	for _, model := range items {
		root := filepath.Base(model.Root)
		if root == "." || root == string(filepath.Separator) || root == "" {
			out = append(out, model.DisplayName)
			continue
		}
		out = append(out, model.DisplayName+"  ["+root+"]")
	}
	return out
}

func firstModels(models []config.DiscoveredModel, limit int) []config.DiscoveredModel {
	if len(models) <= limit {
		return models
	}
	return models[:limit]
}

func box(title string, body []string, width, height int) []string {
	if width < 4 {
		width = 4
	}
	if height < 3 {
		height = 3
	}
	bodyWidth := width - 2
	if bodyWidth < 1 {
		bodyWidth = 1
	}
	lines := make([]string, 0, height)
	lines = append(lines, "┌"+pad(" "+title+" ", bodyWidth, '─')+"┐")
	innerHeight := height - 2
	for i := 0; i < innerHeight; i++ {
		line := ""
		if i < len(body) {
			line = truncate(body[i], bodyWidth)
		}
		lines = append(lines, "│"+pad(line, bodyWidth)+"│")
	}
	lines = append(lines, "└"+strings.Repeat("─", bodyWidth)+"┘")
	return lines
}

func sideBySide(left, right []string, leftWidth, rightWidth int) []string {
	height := max(len(left), len(right))
	out := make([]string, 0, height)
	for i := 0; i < height; i++ {
		l := strings.Repeat(" ", leftWidth)
		r := strings.Repeat(" ", rightWidth)
		if i < len(left) {
			l = pad(left[i], leftWidth)
		}
		if i < len(right) {
			r = pad(right[i], rightWidth)
		}
		out = append(out, l+r)
	}
	return out
}

func clampLines(lines []string, height, width int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	out := make([]string, 0, height)
	for _, line := range lines {
		out = append(out, pad(truncate(line, width), width))
	}
	for len(out) < height {
		out = append(out, strings.Repeat(" ", max(width, 0)))
	}
	return out
}

func pad(value string, width int, filler ...rune) string {
	runes := []rune(value)
	if len(runes) > width {
		return string(runes[:width])
	}
	fill := ' '
	if len(filler) > 0 {
		fill = filler[0]
	}
	return value + strings.Repeat(string(fill), width-len(runes))
}

func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
