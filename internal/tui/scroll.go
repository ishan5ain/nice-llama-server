package tui

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *model) logPageSize() int {
	if m.logViewHeight <= 1 {
		return 1
	}
	return m.logViewHeight
}

func (m *model) scrollLogBy(delta int) {
	m.logScrollY += delta
	m.clampLogScroll()
}

func (m *model) scrollLogHorizontally(delta int) {
	m.logScrollX += delta
	m.clampLogScroll()
}

func (m *model) scrollLogToBottom() {
	rows := m.renderedLogRows()
	maxScroll := max(0, len(rows)-m.logPageSize())
	m.logScrollY = maxScroll
	m.clampLogScroll()
}

func (m *model) logAtBottom() bool {
	rows := m.renderedLogRows()
	maxScroll := max(0, len(rows)-m.logPageSize())
	return m.logScrollY >= maxScroll
}

func (m *model) clampLogScroll() {
	rows := m.renderedLogRows()
	page := m.logPageSize()
	maxY := max(0, len(rows)-page)
	if m.logScrollY < 0 {
		m.logScrollY = 0
	}
	if m.logScrollY > maxY {
		m.logScrollY = maxY
	}

	maxWidth := 0
	for _, row := range rows {
		if width := lipgloss.Width(ansi.Strip(row)); width > maxWidth {
			maxWidth = width
		}
	}
	maxX := max(0, maxWidth-m.logViewWidth)
	if m.logScrollX < 0 {
		m.logScrollX = 0
	}
	if m.logScrollX > maxX {
		m.logScrollX = maxX
	}
}
