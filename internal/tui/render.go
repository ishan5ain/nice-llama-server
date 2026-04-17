package tui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	headerPanelHeight = 5
	footerPanelHeight = 2
)

func (m *model) render() string {
	width := max(m.width, 60)
	height := max(m.height, 16)

	header := m.renderHeader(width)
	footer := m.renderFooter(width)
	contentHeight := max(6, height-lipgloss.Height(header)-lipgloss.Height(footer))
	content := m.renderBottom(width, contentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m *model) renderHeader(width int) string {
	bodyHeight := max(1, headerPanelHeight-m.styles.headerPanel.GetVerticalFrameSize())
	bodyWidth := max(1, width-m.styles.headerPanel.GetHorizontalFrameSize()/2)

	titleLine := lipgloss.JoinHorizontal(
		lipgloss.Left,
		m.styles.headerTitle.Render("Nice Llama Server"),
		"   ",
		m.styles.headerStatus.Render("Runtime "+runtimeSummary(m.snapshot)),
	)
	if m.snapshot.Runtime.Port != 0 {
		titleLine = lipgloss.JoinHorizontal(
			lipgloss.Left,
			titleLine,
			"  ",
			m.styles.headerStats.Render(fmt.Sprintf("@ %s:%d", hostOrDefault(m.snapshot.Runtime.Host), m.snapshot.Runtime.Port)),
		)
	}

	statsLine := m.styles.headerStats.Render(fmt.Sprintf(
		"%d bookmarks   %d models   %d roots",
		len(m.snapshot.Bookmarks),
		len(m.snapshot.Models),
		len(m.snapshot.Config.ModelRoots),
	))
	rows := []string{
		crop(titleLine, bodyWidth),
		crop(statsLine, bodyWidth),
	}
	if status := strings.TrimSpace(m.messageLine()); status != "" {
		rows = append(rows, crop(m.styles.headerMessage.Render(status), bodyWidth))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, clampStyledLines(rows, bodyHeight)...)
	return m.styles.headerPanel.
		Width(bodyWidth).
		Height(bodyHeight).
		Render(content)
}

func (m *model) renderBottom(width, height int) string {
	if m.bottomView == bottomViewLogs {
		return m.renderLogView(width, height)
	}
	return m.renderBookmarkEditorView(width+6, height)
}

func (m *model) renderBookmarkEditorView(width, height int) string {
	gap := 1
	leftWidth, rightWidth := splitBookmarkEditorWidths(width, gap)

	left := m.renderModelListPanel(leftWidth, height)
	right := m.renderDetailPanel(rightWidth, height)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gap), right)
	return fitBox(joined, width, height)
}

func (m *model) renderModelListPanel(width, height int) string {
	items := m.renderListLines(max(1, width-m.styles.panelBase.GetHorizontalFrameSize()), max(1, height-m.styles.panelBase.GetVerticalFrameSize()))
	// items := m.renderListLines(max(1, width), max(1, height-m.styles.panelBase.GetVerticalFrameSize()))
	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	style := m.panelStyleFor(focusModelList)
	return style.Width(max(1, width-style.GetHorizontalFrameSize())).
		Height(max(1, height-style.GetVerticalFrameSize())).
		Render(content)
}

func (m *model) renderDetailPanel(width, height int) string {
	lines := m.renderDetailLines(max(1, width-m.styles.panelBase.GetHorizontalFrameSize()), max(1, height-m.styles.panelBase.GetVerticalFrameSize()))
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	style := m.panelStyleFor(m.focus)
	return style.Width(max(1, width-style.GetHorizontalFrameSize())).
		Height(max(1, height-style.GetVerticalFrameSize())).
		Render(content)
}

func (m *model) renderLogView(width, height int) string {
	lines := m.renderLogLines(max(1, width-m.styles.panelBase.GetHorizontalFrameSize()), max(1, height-m.styles.panelBase.GetVerticalFrameSize()))
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	style := m.styles.logsPanel
	return style.Width(max(1, width-style.GetHorizontalFrameSize())).
		Height(max(1, height-style.GetVerticalFrameSize())).
		Render(content)
}

func (m *model) renderFooter(width int) string {
	bodyHeight := max(1, footerPanelHeight-m.styles.footerPanel.GetVerticalFrameSize())
	bodyWidth := max(1, width-m.styles.footerPanel.GetHorizontalFrameSize())
	content := m.footerLine(bodyWidth)
	return m.styles.footerPanel.Width(bodyWidth).Height(bodyHeight).Render(content)
}

func (m *model) renderListLines(width, height int) []string {
	items := m.listItems()
	if len(items) == 0 {
		return clampStyledLines([]string{
			m.styles.groupLabel.Render("No discovered model groups"),
			m.styles.muted.Render("Press r to rescan model roots."),
		}, height)
	}

	lines := make([]string, 0, len(items)+2)
	for _, item := range items {
		selected := item.key() == m.selectedKey
		switch item.kind {
		case listItemModelGroup:
			line := item.label
			if item.degraded {
				line += " (missing)"
			}
			style := m.styles.groupLabel
			if selected && m.focus == focusModelList {
				style = m.styles.groupLabelSelected
			}
			lines = append(lines, crop(style.Render(line), width))
		case listItemBookmark:
			prefix := "  "
			style := m.styles.bookmarkItem
			if selected {
				if m.focus == focusModelList {
					style = m.styles.bookmarkSelected
				} else {
					style = m.styles.bookmarkActive
				}
				prefix = "▸ "
			}
			line := prefix + item.label
			if item.degraded {
				line += " (missing)"
			}
			lines = append(lines, crop(style.Render(line), width))
		}
	}
	return clampStyledLines(lines, height)
}

func (m *model) renderDetailLines(width, height int) []string {
	// title := m.styles.panelTitle.Render("Bookmark Detail")
	// lines := []string{title}
	lines := []string{}

	if m.focus == focusModelList || m.editor == nil {
		selected := m.selectedBookmark()
		if selected == nil {
			group, ok := m.currentGroupSelection()
			if ok {
				lines = append(lines, m.renderField("Bookmark Name", "", false, width)...)
				lines = append(lines,
					"",
					m.styles.muted.Render("No bookmark selected."),
					m.styles.muted.Render("Press n to create one under "+group.label+"."),
				)
				return clampStyledLines(lines, height)
			}
			lines = append(lines, m.styles.muted.Render("No models discovered. Press r to rescan."))
			return clampStyledLines(lines, height)
		}

		lines = append(lines, m.renderField("Bookmark Name", selected.Name, false, width)...)
		// lines = append(lines, "")
		argsLines := strings.Split(strings.TrimSpace(selected.ArgsText), "\n")
		if len(argsLines) == 1 && argsLines[0] == "" {
			argsLines = []string{"(empty)"}
		}
		lines = append(lines, m.renderFieldBlock("Args", argsLines, false, width, max(4, height-len(lines)-3))...)
		return clampStyledLines(lines, height)
	}

	nameValue := m.editor.name.Value()
	if m.focus == focusDetailName {
		lines = append(lines, m.renderFieldBlock("Bookmark Name", m.editor.name.RenderLines(width, 1, true), true, width, 1)...)
	} else {
		lines = append(lines, m.renderField("Bookmark Name", nameValue, false, width)...)
	}
	// lines = append(lines, "")

	argsHeight := max(4, height-len(lines)-1)
	lines = append(lines, m.renderFieldBlock("Args", m.renderArgsEditorLines(width, argsHeight), m.focus == focusDetailArgs, width, argsHeight)...)
	return clampStyledLines(lines, height)
}

func (m *model) renderArgsEditorLines(width, height int) []string {
	if m.editor == nil {
		return nil
	}
	if m.focus != focusDetailArgs {
		return m.editor.args.RenderLines(width, height, false)
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	buffer := &m.editor.args
	start := buffer.VisibleStart(height)
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		idx := start + i
		if idx >= len(buffer.lines) {
			lines = append(lines, "")
			continue
		}
		text := string(buffer.lines[idx])
		if idx == buffer.row {
			text = withCursor(text, buffer.col)
			if m.editor.completion.active {
				text += m.renderArgCompletionGhost()
			}
		}
		lines = append(lines, text)
	}
	return lines
}

func (m *model) renderArgCompletionGhost() string {
	state := m.editor.completion
	if !state.active || len(state.candidates) == 0 {
		return ""
	}
	window := passiveCompletionWindow(state.candidates, maxVisibleArgCompletions)
	if !state.passive {
		window = completionWindow(state.candidates, state.index, maxVisibleArgCompletions)
	}
	if len(window) == 0 {
		return ""
	}
	labels := make([]string, 0, len(window))
	for _, candidate := range window {
		labels = append(labels, candidate.Text)
	}
	return m.styles.completionGhost.Render("  " + strings.Join(labels, "  "))
}

func (m *model) renderField(label, value string, focused bool, width int) []string {
	if value == "" {
		value = " "
	}
	return append(
		[]string{m.styles.fieldLabel.Render(label)},
		m.renderInputBox([]string{value}, focused, width, 1)...,
	)
}

func (m *model) renderFieldBlock(label string, values []string, focused bool, width, height int) []string {
	lines := []string{m.styles.fieldLabel.Render(label)}
	lines = append(lines, m.renderInputBox(values, focused, width, height)...)
	return lines
}

func (m *model) renderInputBox(values []string, focused bool, width, height int) []string {
	style := m.styles.inputBlur
	if focused {
		style = m.styles.inputFocus
	}

	if len(values) == 0 {
		values = []string{" "}
	}
	content := lipgloss.JoinVertical(lipgloss.Left, clampStyledLines(values, height)...)
	rendered := style.
		Width(max(1, width-style.GetHorizontalFrameSize())).
		Height(max(1, height-style.GetVerticalFrameSize())).
		Render(content)
	return strings.Split(rendered, "\n")
}

func (m *model) renderLogLines(width, height int) []string {
	m.logViewWidth = width
	lines := []string{m.styles.panelTitle.Render("Runtime Logs")}
	contentHeight := max(1, height-1)
	m.logViewHeight = contentHeight

	totalRows := len(m.logs)
	start := m.logScrollY
	if start > totalRows {
		start = totalRows
	}
	end := min(totalRows, start+contentHeight)

	for i := start; i < end; i++ {
		entry := m.logs[i]

		var tsStyle lipgloss.Style
		if entry.Stream == "stderr" {
			tsStyle = m.styles.logTimestampStderr
		} else if entry.Stream == "system" {
			tsStyle = m.styles.logTimestampSystem
		} else {
			tsStyle = m.styles.logTimestampStdout
		}
		ts := tsStyle.Render(entry.TS.Format("15:04:05"))
		tsWidth := lipgloss.Width(ts)

		lineWidth := max(0, width-tsWidth-1)
		line := m.styles.muted.Render(sliceHorizontal(entry.Line, m.logScrollX, lineWidth))

		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Left, ts, " ", line))
	}

	// Fill remaining height
	for len(lines) < height {
		lines = append(lines, "")
	}

	return clampStyledLines(lines, height)
}

func (m *model) footerLine(width int) string {
	if m.confirmDelete {
		return m.styles.footerKey.Render("y") + " delete   " + m.styles.footerKey.Render("n") + " cancel"
	}

	// Determine tail indicator style based on active/paused state
	var tailIndicator string
	if m.followTailEnabled {
		tailIndicator = m.styles.tailIndicator.Render("[Tail]")
	} else {
		tailIndicator = m.styles.tailIndicatorPaused.Render("[tail]")
	}

	switch m.bottomView {
	case bottomViewLogs:
		// Left-aligned key bindings
		leftContent := m.styles.footerKey.Render("/") + " bookmarks  " +
			m.styles.footerKey.Render("Shift+L") + " load  " +
			m.styles.footerKey.Render("Shift+U") + " unload  " +
			m.styles.footerKey.Render("Ctrl+Q") + " quit"

		// Calculate padding to push tail indicator to far right
		leftWidth := lipgloss.Width(leftContent)
		tailWidth := lipgloss.Width(tailIndicator)
		padding := max(0, width-leftWidth-tailWidth-2) // 2 is for manual adjustment to prevent the tail indicator from wrapping onto the next line

		return leftContent + strings.Repeat(" ", padding) + tailIndicator

	default:
		if m.focus == focusModelList {
			return m.styles.footerKey.Render("↑/↓") + " navigate  " +
				m.styles.footerKey.Render("e") + " edit  " +
				m.styles.footerKey.Render("n") + " new  " +
				m.styles.footerKey.Render("c") + " clone  " +
				m.styles.footerKey.Render("d") + " delete  " +
				m.styles.footerKey.Render("r") + " rescan  " +
				m.styles.footerKey.Render("/") + " logs  " +
				m.styles.footerKey.Render("Shift+L/U") + " load/unload  " +
				m.styles.footerKey.Render("Ctrl+Q") + " quit"
		}
		return m.styles.footerKey.Render("↑/↓") + " move focus  " +
			m.styles.footerKey.Render("Enter") + " next/newline  " +
			m.styles.footerKey.Render("Tab/Shift+Tab") + " complete  " +
			m.styles.footerKey.Render("Ctrl+S") + " save  " +
			m.styles.footerKey.Render("Esc") + " discard  " +
			m.styles.footerKey.Render("/") + " logs  " +
			m.styles.footerKey.Render("Shift+L/U") + " load/unload  " +
			m.styles.footerKey.Render("Ctrl+Q") + " quit"
	}
}

func (m *model) messageLine() string {
	if m.errorMessage != "" {
		return "Error · " + m.errorMessage
	}
	if m.flashMessage != "" {
		return m.flashMessage
	}
	return ""
}

func (m *model) panelStyleFor(focus focusArea) lipgloss.Style {
	if m.focus == focus && m.bottomView == bottomViewBookmarks {
		return m.styles.panelFocus
	}
	return m.styles.panelBase
}

func crop(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	style := lipgloss.NewStyle().MaxWidth(width)
	return style.Render(value)
}

func clampStyledLines(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func fitBox(value string, width, height int) string {
	lines := strings.Split(value, "\n")
	for i := range lines {
		lines[i] = padToWidth(crop(lines[i], width), width)
	}
	return strings.Join(clampStyledLines(lines, height), "\n")
}

func padToWidth(value string, width int) string {
	missing := width - lipgloss.Width(value)
	if missing <= 0 {
		return value
	}
	return value + strings.Repeat(" ", missing)
}

func (m *model) renderedLogRows() []string {
	rows := make([]string, 0, len(m.logs))
	for _, entry := range m.logs {
		var tsStyle lipgloss.Style
		if entry.Stream == "stderr" {
			tsStyle = m.styles.logTimestampStderr
		} else if entry.Stream == "system" {
			tsStyle = m.styles.logTimestampSystem
		} else {
			tsStyle = m.styles.logTimestampStdout
		}
		ts := tsStyle.Render(entry.TS.Format("15:04:05"))
		line := m.styles.muted.Render(ansi.Strip(entry.Line))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, ts, " ", line))
	}
	return rows
}

func sliceHorizontal(value string, offset, width int) string {
	plain := []rune(ansi.Strip(value))
	if offset < 0 {
		offset = 0
	}
	if offset > len(plain) {
		offset = len(plain)
	}
	end := min(len(plain), offset+width)
	sliced := string(plain[offset:end])
	return padToWidth(sliced, width)
}

func splitBookmarkEditorWidths(width, gap int) (left int, right int) {
	available := max(1, width-gap)
	if available == 1 {
		return 1, 0
	}

	const (
		minLeft  = 24
		minRight = 20
	)

	preferredLeft := available * 2 / 5
	left = preferredLeft

	if available >= minLeft+minRight {
		if left < minLeft {
			left = minLeft
		}
		maxLeft := available - minRight
		if left > maxLeft {
			left = maxLeft
		}
	} else {
		if left < 1 {
			left = 1
		}
		if left >= available {
			left = available / 2
		}
	}

	right = available - left
	if right < 1 {
		right = 1
		left = available - right
	}
	return left, right
}

func hostOrDefault(host string) string {
	if host == "" {
		return "127.0.0.1"
	}
	return host
}
