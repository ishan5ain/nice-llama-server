package tui

import (
	"github.com/ishan5ain/tuiweave"
	lipgloss "charm.land/lipgloss/v2"
)

// glowWarm returns a warm-toned dark theme inspired by Glow's color palette.
// The accent is a warm orange (#ff9e64) with warm grays and earthy intent
// colors throughout.
func glowWarm() tuiweave.Theme {
	return tuiweave.Theme{
		Surface:       lipgloss.Color("#1e1e1e"),
		SurfaceRaised: lipgloss.Color("#2c2c2c"),
		SurfaceSunken: lipgloss.Color("#141414"),

		Text:         lipgloss.Color("#dadada"),
		TextMuted:    lipgloss.Color("#a0a0a0"),
		TextFaint:    lipgloss.Color("#6a6a6a"),
		TextInverted: lipgloss.Color("#1e1e1e"),

		Accent:      lipgloss.Color("#ff9e64"),
		AccentMuted: lipgloss.Color("#7a5030"),
		Success:     lipgloss.Color("#9ece6a"),
		Warning:     lipgloss.Color("#e0af68"),
		Danger:      lipgloss.Color("#f7768e"),
		Info:        lipgloss.Color("#7dcfff"),

		Border:        lipgloss.Color("#4a4a4a"),
		BorderFocused: lipgloss.Color("#ff9e64"),
		BorderMuted:   lipgloss.Color("#353535"),

		SelectionBg: lipgloss.Color("#5a4030"),
		SelectionFg: lipgloss.Color("#dadada"),
	}
}
