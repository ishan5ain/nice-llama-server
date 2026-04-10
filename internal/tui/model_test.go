package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestLoadShortcutRequiresUppercase(t *testing.T) {
	t.Parallel()

	if isLoadShortcut(tea.KeyPressMsg{Text: "l"}) {
		t.Fatalf("lowercase l should not trigger load")
	}
	if !isLoadShortcut(tea.KeyPressMsg{Text: "L"}) {
		t.Fatalf("uppercase L should trigger load")
	}
}

func TestUnloadShortcutRequiresUppercase(t *testing.T) {
	t.Parallel()

	if isUnloadShortcut(tea.KeyPressMsg{Text: "u"}) {
		t.Fatalf("lowercase u should not trigger unload")
	}
	if !isUnloadShortcut(tea.KeyPressMsg{Text: "U"}) {
		t.Fatalf("uppercase U should trigger unload")
	}
}
