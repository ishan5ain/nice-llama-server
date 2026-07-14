package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Dialog absorbs ALL input first
	if m.showDialog {
		var cmd tea.Cmd
		m.deleteDialog, cmd = m.deleteDialog.Update(msg)
		return m, cmd
	}

	switch {
	case msg.Keystroke() == "ctrl+q":
		return m, tea.Quit
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
				}
				return m, nil
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
				}
				return m, nil
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
			return m, loadBookmarkCmd(m.ctx, m.client, selected.ID)
		}
		m.errorMessage = "select a bookmark to load"
		return m, nil
	case isUnloadShortcut(msg):
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
			return m, loadBookmarkCmd(m.ctx, m.client, selected.ID)
		}
		m.errorMessage = "select a bookmark to load"
		return m, nil
	case isUnloadShortcut(msg):
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
		m.editor = nil
		m.editorScope.Exit(&m.fm)
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
		m.editorScope.Prev()
		m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
		return m, nil
	case msg.Keystroke() == "down":
		m.editor.completion = argCompletionState{}
		if m.editorScope.Index() == 0 {
			m.editorScope.Next()
			m.editorScope.Apply(&m.editor.name, &m.editor.args, &m.ac)
			return m, nil
		}
		return m, nil
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
