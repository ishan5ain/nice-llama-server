package tui

import lipgloss "charm.land/lipgloss/v2"

type styles struct {
	headerPanel          lipgloss.Style
	headerTitle          lipgloss.Style
	headerStats          lipgloss.Style
	headerStatus         lipgloss.Style
	headerMessage        lipgloss.Style
	panelBase            lipgloss.Style
	panelFocus           lipgloss.Style
	logsPanel            lipgloss.Style
	panelTitle           lipgloss.Style
	groupLabel           lipgloss.Style
	groupLabelSelected   lipgloss.Style
	bookmarkItem         lipgloss.Style
	bookmarkSelected     lipgloss.Style
	bookmarkActive       lipgloss.Style
	fieldLabel           lipgloss.Style
	inputBlur            lipgloss.Style
	inputFocus           lipgloss.Style
	completionGhost      lipgloss.Style
	footerPanel          lipgloss.Style
	footerKey            lipgloss.Style
	muted                lipgloss.Style
	logTimestamp         lipgloss.Style
	logTimestampStdout   lipgloss.Style
	logTimestampStderr   lipgloss.Style
	logTimestampSystem   lipgloss.Style
	logTimestampStreamed lipgloss.Style
	tailIndicator        lipgloss.Style
	tailIndicatorPaused  lipgloss.Style
}

func newStyles() styles {
	return styles{
		headerPanel: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#4D7CFE")).
			Padding(0, 1),
		headerTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F8FAFC")),
		headerStats: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CBD5E1")),
		headerStatus: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#93C5FD")),
		headerMessage: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FCD34D")),
		panelBase: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#475569")).
			Padding(0, 1),
		panelFocus: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#38BDF8")).
			Padding(0, 1),
		logsPanel: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F59E0B")).
			Padding(0, 0),
		panelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E2E8F0")),
		groupLabel: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7DD3FC")),
		groupLabelSelected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0F172A")).
			Background(lipgloss.Color("#7DD3FC")).
			Padding(0, 1),
		bookmarkItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CBD5E1")),
		bookmarkSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0F172A")).
			Background(lipgloss.Color("#F8FAFC")).
			Bold(true),
		bookmarkActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#334155")),
		fieldLabel: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			Bold(true),
		inputBlur: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#475569")).
			Padding(0, 1),
		inputFocus: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#22C55E")).
			Padding(0, 1),
		completionGhost: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B")),
		footerPanel: lipgloss.NewStyle().
			// BorderStyle(lipgloss.Border{Top: "─"}).
			// BorderTop(true).
			// BorderForeground(lipgloss.Color("#334155")).
			Padding(0, 1),
		footerKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F8FAFC")).
			Background(lipgloss.Color("#334155")).
			Padding(0, 1).
			Bold(true),
		muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B")),
		logTimestamp: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")),
		logTimestampStdout: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#86EFAC")),
		logTimestampStderr: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FCA5A5")),
		logTimestampSystem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FCD34D")),
		logTimestampStreamed: lipgloss.NewStyle().
			Bold(true),
		tailIndicator: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#4ade80")),
		tailIndicatorPaused: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B")),
	}
}
