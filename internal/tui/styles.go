package tui

import (
	"github.com/ishan5ain/tuiweave"
	lipgloss "charm.land/lipgloss/v2"
)

type styles struct {
	headerStats   lipgloss.Style
	headerStatus  lipgloss.Style
	headerMessage lipgloss.Style
	headerError   lipgloss.Style
	panelBase     lipgloss.Style
	panelFocus    lipgloss.Style
	logsPanel     lipgloss.Style
	panelTitle    lipgloss.Style
	fieldLabel    lipgloss.Style
	inputBlur     lipgloss.Style
	inputFocus    lipgloss.Style
	muted         lipgloss.Style
}

func newStyles(theme tuiweave.Theme) styles {
	return styles{
		headerStats: lipgloss.NewStyle().
			Foreground(theme.TextMuted),
		headerStatus: lipgloss.NewStyle().
			Foreground(theme.Accent),
		headerMessage: lipgloss.NewStyle().
			Foreground(theme.Success),
		headerError: lipgloss.NewStyle().
			Foreground(theme.Danger),
		panelBase: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		panelFocus: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.BorderFocused).
			Bold(true).
			Padding(0, 1),
		logsPanel: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Warning).
			Padding(0, 0),
		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Text),
		fieldLabel: lipgloss.NewStyle().
			Foreground(theme.TextMuted).
			Bold(true),
		inputBlur: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		inputFocus: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(theme.Success).
			Padding(0, 1),
		muted: lipgloss.NewStyle().
			Foreground(theme.TextFaint),
	}
}
