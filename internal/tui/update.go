package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ishan5ain/tuiweave"
	"github.com/ishan5ain/tuiweave/autocomplete"
	"github.com/ishan5ain/tuiweave/dialog"
	"github.com/ishan5ain/tuiweave/list"
	"github.com/ishan5ain/tuiweave/statusbar"
	"github.com/ishan5ain/tuiweave/tabs"
)

func (m *model) rebuildTheme(theme tuiweave.Theme) {
	m.theme = theme
	// Derive a chrome theme with transparent surfaces for structural
	// components (header panel, tabs, statusbar, autocomplete).
	chrome := theme
	chrome.SurfaceRaised = lipgloss.NoColor{}
	chrome.SurfaceSunken = lipgloss.NoColor{}
	m.chromeTheme = chrome

	m.styles = newStyles(theme)

	// Recreate each component with the new theme, preserving state.

	// tabs
	{
		savedTabs := m.tabs.Tabs()
		savedSel := m.tabs.Selected()
		savedFocused := m.tabs.Focused()
		m.tabs = tabs.New(chrome)
		m.tabs.SetTabs(savedTabs...)
		m.tabs.Select(savedSel)
		if savedFocused {
			m.tabs.Focus()
		}
	}

	// list
	{
		savedItems := m.modelList.Items()
		savedSel := m.modelList.Selected()
		savedFilter := m.modelList.Filter()
		savedFocused := m.modelList.Focused()
		m.modelList = list.New(theme)
		m.modelList.SetItems(savedItems...)
		m.modelList.SetFilter(savedFilter)
		m.modelList.Select(savedSel)
		if savedFocused {
			m.modelList.Focus()
		}
	}

	// autocomplete
	{
		savedItems := m.ac.Items()
		savedSel := m.ac.Selected()
		savedQuery := m.ac.Query()
		savedFocused := m.ac.Focused()
		m.ac = autocomplete.New(chrome)
		m.ac.SetItems(savedItems...)
		m.ac.SetQuery(savedQuery)
		m.ac.Select(savedSel)
		if savedFocused {
			m.ac.Focus()
		}
	}

	// statusbar — segments repopulated on next renderFooter() call
	m.footer = statusbar.New(chrome)

	// dialog
	{
		savedTitle := m.deleteDialog.Title
		savedBody := m.deleteDialog.Body
		savedConfirm := m.deleteDialog.ConfirmLabel
		savedCancel := m.deleteDialog.CancelLabel
		m.deleteDialog = dialog.New(theme)
		m.deleteDialog.Title = savedTitle
		m.deleteDialog.Body = savedBody
		m.deleteDialog.ConfirmLabel = savedConfirm
		m.deleteDialog.CancelLabel = savedCancel
	}

	// Editor components (textarea, textinput) are created fresh per edit
	// in newBookmarkEditor — no state to preserve here.
}

func (m *model) nextPresetID() string {
	if len(m.presetIDs) == 0 {
		return m.currentPresetID
	}
	for i, id := range m.presetIDs {
		if id == m.currentPresetID {
			next := (i + 1) % len(m.presetIDs)
			return m.presetIDs[next]
		}
	}
	return m.presetIDs[0]
}

// resolveTheme resolves a preset ID to a Theme. Custom app presets are
// checked first, then gotui built-in presets.
func (m *model) resolveTheme(id string) (tuiweave.Theme, bool) {
	switch id {
	case "glow-warm":
		return glowWarm(), true
	default:
		return tuiweave.ThemeForPreset(id)
	}
}

// presetName returns the display name for a preset ID.
func (m *model) presetName(id string) string {
	switch id {
	case "glow-warm":
		return "Glow Warm"
	default:
		for _, p := range tuiweave.Presets() {
			if p.ID == id {
				return p.Name
			}
		}
		return id
	}
}

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Esc dismisses error messages
	if msg.Keystroke() == "esc" && m.errorMessage != "" && !m.showDialog {
		m.errorMessage = ""
		return m, nil
	}

	// Dialog absorbs ALL input first
	if m.showDialog {
		var cmd tea.Cmd
		m.deleteDialog, cmd = m.deleteDialog.Update(msg)
		return m, cmd
	}

	switch {
	case msg.Keystroke() == "ctrl+q":
		return m, tea.Quit
	case msg.Keystroke() == "ctrl+t":
		if m.editorScope.Active() || m.showDialog {
			return m, nil
		}
		nextID := m.nextPresetID()
		if theme, ok := m.resolveTheme(nextID); ok {
			m.rebuildTheme(theme)
			m.currentPresetID = nextID
			m.flashMessage = "Theme: " + m.presetName(nextID)
		}
		return m, nil
	case msg.Keystroke() == "tab":
		if m.ac.Focused() {
			// Navigate down in autocomplete
			idx := m.ac.Selected()
			if idx >= 0 && idx < m.ac.FilteredLen()-1 {
				m.ac.Select(idx + 1)
			}
			return m, nil
		}
		if m.editorScope.Active() {
			if m.editorScope.Index() == 1 {
				// On args field: trigger arg completion
				if m.handleArgCompletionTab(argCompletionForward) {
					m.errorMessage = ""
					return m, nil
				}
			}
			m.editorScope.Next()
			m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
		} else {
			m.fm.Next()
			m.applyFocus()
		}
		return m, nil
	case msg.Keystroke() == "shift+tab":
		if m.ac.Focused() {
			// Navigate up in autocomplete
			idx := m.ac.Selected()
			if idx > 0 {
				m.ac.Select(idx - 1)
			}
			return m, nil
		}
		if m.editorScope.Active() {
			if m.editorScope.Index() == 1 {
				// On args field: trigger backward arg completion
				if m.handleArgCompletionTab(argCompletionBackward) {
					m.errorMessage = ""
					return m, nil
				}
			}
			m.editorScope.Prev()
			m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
		} else {
			m.fm.Prev()
			m.applyFocus()
		}
		return m, nil
	}

	if m.tabs.SelectedID() == "logs" {
		return m.handleLogKey(msg)
	}

	if m.editorScope.Active() {
		return m.handleDetailKey(msg)
	}
	if m.fm.Index() == 1 {
		return m.handleListKey(msg)
	}
	return m, nil
}

func (m *model) handleLogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Keystroke() == "up":
		m.followTailEnabled = false
		m.logView.ScrollBy(-1)
		return m, nil
	case msg.Keystroke() == "down":
		m.followTailEnabled = false
		m.logView.ScrollBy(1)
		return m, nil
	case msg.Keystroke() == "pgup":
		m.followTailEnabled = false
		m.logView.ScrollBy(-m.logView.VisibleLines())
		return m, nil
	case msg.Keystroke() == "pgdown":
		m.followTailEnabled = false
		m.logView.ScrollBy(m.logView.VisibleLines())
		return m, nil
	case msg.Keystroke() == "home":
		m.followTailEnabled = false
		m.logView.GotoTop()
		return m, nil
	case msg.Keystroke() == "end":
		m.followTailEnabled = true
		m.logView.GotoBottom()
		return m, nil
	case msg.Keystroke() == "left":
		m.followTailEnabled = false
		m.scrollLogHorizontally(-4)
		return m, nil
	case msg.Keystroke() == "right":
		m.followTailEnabled = false
		m.scrollLogHorizontally(4)
		return m, nil
	case msg.Keystroke() == "t":
		m.followTailEnabled = true
		m.logView.GotoBottom()
		return m, nil
	case msg.Keystroke() == "T":
		m.followTailEnabled = false
		return m, nil
	case msg.Text == "/":
		if m.tabs.SelectedID() == "bookmarks" {
			m.tabs.SelectID("logs")
		} else {
			m.tabs.SelectID("bookmarks")
		}
		return m, nil
	case isLoadShortcut(msg):
		if selected := m.selectedBookmark(); selected != nil {
			m.loading = true
			return m, loadBookmarkCmd(m.ctx, m.client, selected.ID)
		}
		m.errorMessage = "select a bookmark to load"
		return m, nil
	case isUnloadShortcut(msg):
		m.loading = true
		return m, unloadCmd(m.ctx, m.client)
	default:
		return m, nil
	}
}

func (m *model) handleMouseWheel(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	if m.tabs.SelectedID() != "logs" {
		return m, nil
	}
	m.followTailEnabled = false
	var cmd tea.Cmd
	m.logView, cmd = m.logView.Update(msg)
	return m, cmd
}

func (m *model) handleListKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Keystroke() == "up", msg.Keystroke() == "down":
		var cmd tea.Cmd
		m.modelList, cmd = m.modelList.Update(msg)
		m.syncSelectionFromList()
		return m, cmd
	case msg.Text == "e":
		if err := m.beginEditSelected(); err != nil {
			m.errorMessage = err.Error()
		}
		return m, nil
	case msg.Text == "n":
		editor, err := m.newBookmarkForCurrentGroup()
		if err != nil {
			m.errorMessage = err.Error()
			return m, nil
		}
		m.editor = editor
		m.pendingDiscard = false
		m.applyFocus()
		m.flashMessage = "creating bookmark"
		m.errorMessage = ""
		return m, nil
	case msg.Text == "c":
		editor, err := m.cloneSelectedBookmark()
		if err != nil {
			m.errorMessage = err.Error()
			return m, nil
		}
		m.editor = editor
		m.pendingDiscard = false
		m.applyFocus()
		m.flashMessage = "cloning bookmark"
		m.errorMessage = ""
		return m, nil
	case msg.Text == "d":
		if selected := m.selectedBookmark(); selected != nil {
			m.deleteDialog.Title = "Delete Bookmark"
			m.deleteDialog.Body = fmt.Sprintf("Delete %q?", selected.Name)
			m.deleteDialog.ConfirmLabel = "Delete"
			m.deleteDialog.CancelLabel = "Cancel"
			m.pendingDeleteID = selected.ID
			m.showDialog = true
		} else {
			m.errorMessage = "select a bookmark to delete"
		}
		return m, nil
	case msg.Text == "r":
		m.loading = true
		return m, rescanCmd(m.ctx, m.client, nil, nil)
	case msg.Text == "/":
		if m.tabs.SelectedID() == "bookmarks" {
			m.tabs.SelectID("logs")
		} else {
			m.tabs.SelectID("bookmarks")
		}
		return m, nil
	case isLoadShortcut(msg):
		if selected := m.selectedBookmark(); selected != nil {
			m.loading = true
			return m, loadBookmarkCmd(m.ctx, m.client, selected.ID)
		}
		m.errorMessage = "select a bookmark to load"
		return m, nil
	case isUnloadShortcut(msg):
		m.loading = true
		return m, unloadCmd(m.ctx, m.client)
	default:
		return m, nil
	}
}

func (m *model) handleDetailKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Text == "/" && m.editor == nil:
		if m.tabs.SelectedID() == "bookmarks" {
			m.tabs.SelectID("logs")
		} else {
			m.tabs.SelectID("bookmarks")
		}
		return m, nil
	case m.editor == nil:
		m.editorScope.Exit(&m.fm)
		return m, nil
	}

	switch {
	case msg.Keystroke() == "esc":
		if m.ac.Focused() {
			m.ac.Blur()
			return m, nil
		}
		if m.editor.Dirty() && !m.pendingDiscard {
			m.pendingDiscard = true
			m.errorMessage = "unsaved changes — press Esc again to discard"
			return m, nil
		}
		m.pendingDiscard = false
		m.editor = nil
		m.editorScope.Exit(&m.fm)
		m.ac.SetItems()
		m.applyFocus()
		m.errorMessage = ""
		m.flashMessage = "discarded changes"
		return m, nil
	case msg.Keystroke() == "ctrl+s":
		return m.saveEditor()
	case msg.Keystroke() == "ctrl+z":
		if m.editorScope.Active() && m.editorScope.Index() == 1 {
			m.editor.args.Undo()
			m.editor.completion = argCompletionState{}
			m.errorMessage = ""
		}
		return m, nil
	case msg.Keystroke() == "up":
		m.editor.completion = argCompletionState{}
		if m.editorScope.Index() == 0 {
			return m, nil
		}
		// In args field: switch to name only if cursor is on first line;
		// otherwise let the textarea handle vertical cursor movement.
		row, _ := m.editor.args.Cursor()
		if row == 0 {
			m.editorScope.Prev()
			m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
			return m, nil
		}
		var upCmd tea.Cmd
		m.editor.args, upCmd = m.editor.args.Update(msg)
		m.errorMessage = ""
		return m, upCmd
	case msg.Keystroke() == "down":
		m.editor.completion = argCompletionState{}
		if m.editorScope.Index() == 0 {
			m.editorScope.Next()
			m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
			return m, nil
		}
		// In args field: forward to textarea for vertical cursor movement.
		// The textarea clamps at the last line.
		var downCmd tea.Cmd
		m.editor.args, downCmd = m.editor.args.Update(msg)
		m.errorMessage = ""
		return m, downCmd
	case msg.Keystroke() == "enter":
		if m.ac.Focused() {
			var cmd tea.Cmd
			m.ac, cmd = m.ac.Update(msg)
			return m, cmd
		}
		m.editor.completion = argCompletionState{}
		if m.editorScope.Index() == 0 {
			m.editorScope.Next()
			m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
		} else {
			m.editor.args.InsertString("\n")
		}
		m.errorMessage = ""
		return m, nil
	case msg.Keystroke() == "ctrl+v":
		clipboardContent, err := readClipboard()
		if err != nil {
			m.errorMessage = "failed to read clipboard"
			return m, nil
		}
		m.pasteIntoBuffer(clipboardContent)
		return m, nil
	default:
		if m.ac.Focused() {
			var cmd tea.Cmd
			m.ac, cmd = m.ac.Update(msg)
			return m, cmd
		}
		if m.editorScope.Index() == 0 {
			var cmd tea.Cmd
			m.editor.name, cmd = m.editor.name.Update(msg)
			m.editor.completion = argCompletionState{}
			m.errorMessage = ""
			return m, cmd
		}
		var cmd tea.Cmd
		m.editor.args, cmd = m.editor.args.Update(msg)
		m.refreshPassiveArgCompletion()
		m.errorMessage = ""
		return m, cmd
	}
}

func (m *model) saveEditor() (tea.Model, tea.Cmd) {
	if m.editor == nil {
		return m, nil
	}
	bookmark := m.editor.Bookmark()
	if strings.TrimSpace(bookmark.Name) == "" {
		m.errorMessage = "bookmark name cannot be empty"
		return m, nil
	}
	return m, saveBookmarkCmd(m.ctx, m.client, bookmark, m.editor.isNew, false)
}

func (m *model) pasteIntoBuffer(text string) {
	if text == "" || m.editor == nil {
		return
	}
	text = sanitizeClipboard(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if m.editorScope.Active() && m.editorScope.Index() == 0 {
		text = strings.ReplaceAll(text, "\n", " ")
		text = ansi.Strip(text)
		m.editor.name.SetValue(m.editor.name.Value() + text)
		m.editor.completion = argCompletionState{}
		m.errorMessage = ""
		return
	}
	stripped := ansi.Strip(text)
	m.editor.args.InsertString(stripped)
	m.editor.completion = argCompletionState{}
	m.errorMessage = ""
}

func (m *model) applyFocus() {
	if m.tabs.SelectedID() == "logs" {
		m.fm.Apply(&m.tabs, &m.logView)
	} else if m.editor != nil {
		m.editorScope.Enter(m.fm)
		m.editorScope.ApplyBackground(m.fm, &m.tabs, &m.modelList)
		m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
	} else {
		m.fm.Apply(&m.tabs, &m.modelList)
	}
}

func printableText(msg tea.KeyPressMsg) string {
	return msg.Text
}

func readClipboard() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "linux":
		cmd = exec.Command("xclip", "-o", "-selection", "clipboard")
	default:
		return "", fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// sanitizeClipboard strips null bytes and control characters (except
// newline, carriage return, tab, and ESC for ANSI sequences) from
// clipboard content.
func sanitizeClipboard(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		if r == '\n' || r == '\r' || r == '\t' || r == 0x1b {
			b.WriteRune(r)
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
