package tui

import (
	"fmt"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/ishan5ain/tuiweave/frame"
	"github.com/ishan5ain/tuiweave/overlay"
	"github.com/ishan5ain/tuiweave/scrollbar"
	"github.com/ishan5ain/tuiweave/splitpane"
	"github.com/ishan5ain/tuiweave/stack"
	"github.com/ishan5ain/tuiweave/statusbar"
)

const (
	headerPanelHeight = 4
)

func (m *model) render() string {
	width := max(m.width, 60)
	height := max(m.height, 16)

	header := m.renderHeader(width)
	footer := m.renderFooter(width)
	contentHeight := max(6, height-lipgloss.Height(header)-lipgloss.Height(footer))
	content := m.renderBottom(width, contentHeight)

	mainView := lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	if m.showDialog {
		m.deleteDialog.SetSize(40, 8)
		return overlay.Center(mainView, m.deleteDialog.View())
	}
	return mainView
}

func (m *model) renderHeader(width int) string {
	// Build body sections using stack.Vertical.
	// The panel subtracts 2 columns for borders, so body renders at width-2.
	body := stack.Vertical(m.theme, max(0, width-2), stack.Options{},
		func(w int) string {
			parts := []string{
				m.styles.headerStatus.Render("Runtime " + runtimeSummary(m.snapshot)),
			}
			if m.loading {
				parts = append(parts, "   ",
					m.styles.headerStatus.Render("⏳ Working..."),
				)
			}
			if m.snapshot.Runtime.Port != 0 {
				parts = append(parts, "  ",
					m.styles.headerStats.Render(fmt.Sprintf("@ %s:%d", hostOrDefault(m.snapshot.Runtime.Host), m.snapshot.Runtime.Port)),
				)
			}
			return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
		},
		func(w int) string {
			return m.styles.headerStats.Render(fmt.Sprintf(
				"%d bookmarks   %d models   %d roots",
				len(m.snapshot.Bookmarks),
				len(m.snapshot.Models),
				len(m.snapshot.Config.ModelRoots),
			))
		},
		func(w int) string {
			status := strings.TrimSpace(m.messageLine())
			if status == "" {
				return ""
			}
			if m.errorMessage != "" {
				return m.styles.headerError.Render(status)
			}
			return m.styles.headerMessage.Render(status)
		},
	)

	return frame.Panel(m.theme, body, width, frame.PanelOptions{
		Title:   "Nice Llama Server",
		Focused: true,
	})
}

func (m *model) renderBottom(width, height int) string {
	tabBar := m.tabs.View()
	contentHeight := max(1, height-lipgloss.Height(tabBar))
	var content string
	if m.tabs.SelectedID() == "logs" {
		content = m.renderLogView(width, contentHeight)
	} else {
		content = m.renderBookmarkEditorView(width, contentHeight)
	}
	return lipgloss.JoinVertical(lipgloss.Left, tabBar, content)
}

func (m *model) renderBookmarkEditorView(width, height int) string {
	return splitpane.Horizontal(m.theme, width, splitpane.Options{
		Ratio: 40,
		Gap:   1,
	},
		func(w int) string { return m.renderModelListPanel(w, height) },
		func(w int) string { return m.renderDetailPanel(w, height) },
	)
}

func (m *model) renderModelListPanel(width, height int) string {
	innerW := max(1, width-m.styles.panelBase.GetHorizontalFrameSize())
	innerH := max(1, height-m.styles.panelBase.GetVerticalFrameSize())

	m.modelList.SetSize(innerW, innerH)
	m.modelList.SetItems(m.listItemsFlat()...)
	content := m.modelList.View()

	isListFocused := !m.editorScope.Active() && m.fm.Index() == 1
	style := m.panelStyleFor(isListFocused)
	return style.Width(max(1, width-style.GetHorizontalFrameSize())).
		Height(max(1, height-style.GetVerticalFrameSize())).
		Render(content)
}

func (m *model) renderDetailPanel(width, height int) string {
	lines := m.renderDetailLines(max(1, width-m.styles.panelBase.GetHorizontalFrameSize()), max(1, height-m.styles.panelBase.GetVerticalFrameSize()))
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	isDetailFocused := m.editorScope.Active()
	style := m.panelStyleFor(isDetailFocused)
	return style.Width(max(1, width-style.GetHorizontalFrameSize())).
		Height(max(1, height-style.GetVerticalFrameSize())).
		Render(content)
}

func (m *model) renderLogView(width, height int) string {
	innerW := max(1, width-m.styles.logsPanel.GetHorizontalFrameSize())
	innerH := max(1, height-m.styles.logsPanel.GetVerticalFrameSize())

	titleLine := m.styles.panelTitle.Render("Runtime Logs")
	contentHeight := max(1, innerH-1)

	// Format log entries into content string
	var sb strings.Builder
	if len(m.logs) == 0 {
		sb.WriteString(m.styles.muted.Render("Waiting for logs..."))
	}
	for i, entry := range m.logs {
		if i > 0 {
			sb.WriteByte('\n')
		}
		prefix := m.streamPrefix(entry.Stream)
		tsStyle := m.logTimestampStyle(entry.Stream)
		ts := tsStyle.Render(prefix + formatLogTimestamp(entry.TS, time.Local))
		tsWidth := lipgloss.Width(ts)
		lineWidth := max(0, innerW-tsWidth-1)
		line := m.styles.muted.Render(sliceHorizontal(entry.Line, m.logScrollX, lineWidth))
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, ts, " ", line))
	}

	// Reserve 1 column for scrollbar
	contentW := max(1, innerW-1)
	m.logView.SetSize(contentW, contentHeight)
	m.logView.SetContent(sb.String())

	if m.followTailEnabled && m.logView.AtBottom() {
		m.logView.GotoBottom()
	}

	bar := scrollbar.For(m.theme, m.logView)
	viewContent := lipgloss.JoinHorizontal(lipgloss.Top, m.logView.View(), bar)
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, viewContent)
	style := m.styles.logsPanel
	return style.Width(max(1, width-style.GetHorizontalFrameSize())).
		Height(max(1, height-style.GetVerticalFrameSize())).
		Render(content)
}

func (m *model) renderFooter(width int) string {
	m.footer.SetSize(width, 1)

	var leftSegments []statusbar.Segment

	if m.showDialog {
		leftSegments = []statusbar.Segment{
			{Text: "y", Kind: statusbar.KindAccent},
			{Text: " delete  ", Kind: statusbar.KindNormal},
			{Text: "n", Kind: statusbar.KindAccent},
			{Text: " cancel", Kind: statusbar.KindNormal},
		}
	} else if m.editor != nil {
		leftSegments = []statusbar.Segment{
			{Text: "Tab", Kind: statusbar.KindAccent},
			{Text: " complete  ", Kind: statusbar.KindNormal},
			{Text: "Esc", Kind: statusbar.KindAccent},
			{Text: " cancel  ", Kind: statusbar.KindNormal},
			{Text: "Ctrl+S", Kind: statusbar.KindAccent},
			{Text: " save", Kind: statusbar.KindNormal},
		}
	} else if m.tabs.SelectedID() == "logs" {
		leftSegments = []statusbar.Segment{
			{Text: "↑↓/Pg", Kind: statusbar.KindAccent},
			{Text: " scroll  ", Kind: statusbar.KindNormal},
			{Text: "Home/End", Kind: statusbar.KindAccent},
			{Text: " edges  ", Kind: statusbar.KindNormal},
			{Text: "←→", Kind: statusbar.KindAccent},
			{Text: " h-scroll  ", Kind: statusbar.KindNormal},
			{Text: "t/T", Kind: statusbar.KindAccent},
			{Text: " tail  ", Kind: statusbar.KindNormal},
			{Text: "/", Kind: statusbar.KindAccent},
			{Text: " bkmarks  ", Kind: statusbar.KindNormal},
			{Text: "Shift+L/U", Kind: statusbar.KindAccent},
			{Text: " load/unld  ", Kind: statusbar.KindNormal},
			{Text: "Ctrl+Q", Kind: statusbar.KindAccent},
			{Text: " quit", Kind: statusbar.KindNormal},
		}
	} else if !m.editorScope.Active() && m.fm.Index() == 1 {
		leftSegments = []statusbar.Segment{
			{Text: "↑↓", Kind: statusbar.KindAccent},
			{Text: " navigate  ", Kind: statusbar.KindNormal},
			{Text: "e", Kind: statusbar.KindAccent},
			{Text: " edit  ", Kind: statusbar.KindNormal},
			{Text: "n", Kind: statusbar.KindAccent},
			{Text: " new  ", Kind: statusbar.KindNormal},
			{Text: "c", Kind: statusbar.KindAccent},
			{Text: " clone  ", Kind: statusbar.KindNormal},
			{Text: "d", Kind: statusbar.KindAccent},
			{Text: " delete  ", Kind: statusbar.KindNormal},
			{Text: "r", Kind: statusbar.KindAccent},
			{Text: " rescan  ", Kind: statusbar.KindNormal},
			{Text: "/", Kind: statusbar.KindAccent},
			{Text: " logs  ", Kind: statusbar.KindNormal},
			{Text: "Shift+L/U", Kind: statusbar.KindAccent},
			{Text: " load/unload  ", Kind: statusbar.KindNormal},
			{Text: "Ctrl+Q", Kind: statusbar.KindAccent},
			{Text: " quit", Kind: statusbar.KindNormal},
		}
	} else {
		leftSegments = []statusbar.Segment{
			{Text: "↑↓", Kind: statusbar.KindAccent},
			{Text: " move focus  ", Kind: statusbar.KindNormal},
			{Text: "Enter", Kind: statusbar.KindAccent},
			{Text: " next/newline  ", Kind: statusbar.KindNormal},
			{Text: "Tab", Kind: statusbar.KindAccent},
			{Text: " complete  ", Kind: statusbar.KindNormal},
			{Text: "Ctrl+S", Kind: statusbar.KindAccent},
			{Text: " save  ", Kind: statusbar.KindNormal},
			{Text: "Ctrl+Z", Kind: statusbar.KindAccent},
			{Text: " undo  ", Kind: statusbar.KindNormal},
			{Text: "Ctrl+V", Kind: statusbar.KindAccent},
			{Text: " paste  ", Kind: statusbar.KindNormal},
			{Text: "Esc", Kind: statusbar.KindAccent},
			{Text: " discard  ", Kind: statusbar.KindNormal},
			{Text: "/", Kind: statusbar.KindAccent},
			{Text: " logs  ", Kind: statusbar.KindNormal},
			{Text: "Ctrl+Q", Kind: statusbar.KindAccent},
			{Text: " quit", Kind: statusbar.KindNormal},
		}
	}

	m.footer.SetLeft(leftSegments...)

	if m.followTailEnabled {
		m.footer.SetRight(statusbar.Segment{Text: "[Tail]", Kind: statusbar.KindSuccess})
	} else {
		m.footer.SetRight(statusbar.Segment{Text: "[tail]", Kind: statusbar.KindMuted})
	}

	return m.footer.View()
}



func (m *model) renderDetailLines(width, height int) []string {
	// title := m.styles.panelTitle.Render("Bookmark Detail")
	// lines := []string{title}
	lines := []string{}

	if (!m.editorScope.Active() && m.fm.Index() == 1) || m.editor == nil {
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

	if m.editorScope.Active() && m.editorScope.Index() == 0 {
		m.editor.name.SetSize(width, 1)
		lines = append(lines, m.renderFieldBlock("Bookmark Name", []string{m.editor.name.View()}, true, width, 1)...)
	} else {
		lines = append(lines, m.renderField("Bookmark Name", m.editor.name.Value(), false, width)...)
	}
	// lines = append(lines, "")

	argsHeight := max(4, height-len(lines)-1)
	lines = append(lines, m.renderFieldBlock("Args", m.renderArgsEditorLines(width, argsHeight), m.editorScope.Active() && m.editorScope.Index() == 1, width, argsHeight)...)
	return clampStyledLines(lines, height)
}

func (m *model) renderArgsEditorLines(width, height int) []string {
	if m.editor == nil {
		return nil
	}
	// Reserve space for autocomplete popup when completion is active
	acLines := 0
	if m.editor.completion.active && m.ac.FilteredLen() > 0 {
		acLines = min(5, m.ac.FilteredLen())
	}
	textHeight := max(1, height-acLines)
	m.editor.args.SetSize(width, textHeight)
	view := m.editor.args.View()
	if acLines > 0 {
		m.ac.SetSize(width, acLines)
		acView := m.ac.View()
		if acView != "" {
			view = lipgloss.JoinVertical(lipgloss.Left, view, acView)
		}
	}
	return strings.Split(view, "\n")
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



func (m *model) messageLine() string {
	if m.errorMessage != "" {
		return "Error · " + m.errorMessage
	}
	if m.flashMessage != "" {
		return m.flashMessage
	}
	return ""
}

func (m *model) panelStyleFor(focused bool) lipgloss.Style {
	if focused && m.tabs.SelectedID() == "bookmarks" {
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



func formatLogTimestamp(ts time.Time, loc *time.Location) string {
	if loc == nil {
		return ts.Format("15:04:05")
	}
	return ts.In(loc).Format("15:04:05")
}

func sliceHorizontal(value string, offset, width int) string {
	if offset < 0 {
		offset = 0
	}
	sliced := ansi.Cut(value, offset, offset+width)
	return padToWidth(sliced, width)
}

func (m *model) scrollLogHorizontally(delta int) {
	m.logScrollX += delta
	if m.logScrollX < 0 {
		m.logScrollX = 0
	}
}

func (m *model) streamPrefix(stream string) string {
	switch stream {
	case "stderr":
		return "[err] "
	case "system":
		return "[sys] "
	default:
		return "[out] "
	}
}

func (m *model) logTimestampStyle(stream string) lipgloss.Style {
	switch stream {
	case "stderr":
		return lipgloss.NewStyle().Foreground(m.theme.Danger)
	case "system":
		return lipgloss.NewStyle().Foreground(m.theme.Warning)
	default:
		return lipgloss.NewStyle().Foreground(m.theme.Success)
	}
}

func hostOrDefault(host string) string {
	if host == "" {
		return "127.0.0.1"
	}
	return host
}
