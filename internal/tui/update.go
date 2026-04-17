package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Keystroke() == "ctrl+q":
		return m, tea.Quit
	case msg.Text == "/":
		m.toggleBottomView()
		return m, nil
	}

	if m.confirmDelete {
		switch {
		case strings.EqualFold(msg.Text, "y"):
			if selected := m.selectedBookmark(); selected != nil {
				return m, deleteBookmarkCmd(m.ctx, m.client, selected.ID)
			}
			m.confirmDelete = false
			return m, nil
		case strings.EqualFold(msg.Text, "n"), msg.Keystroke() == "esc":
			m.confirmDelete = false
			return m, nil
		default:
			return m, nil
		}
	}

	if m.bottomView == bottomViewLogs {
		return m.handleLogKey(msg)
	}

	if m.focus == focusModelList {
		return m.handleListKey(msg)
	}
	return m.handleDetailKey(msg)
}

func (m *model) handleLogKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Keystroke() == "up":
		m.followTail = false
		m.followTailEnabled = false
		m.scrollLogBy(-1)
		return m, nil
	case msg.Keystroke() == "down":
		m.followTail = false
		m.followTailEnabled = false
		m.scrollLogBy(1)
		return m, nil
	case msg.Keystroke() == "pgup":
		m.followTail = false
		m.followTailEnabled = false
		m.scrollLogBy(-m.logPageSize())
		return m, nil
	case msg.Keystroke() == "pgdown":
		m.followTail = false
		m.followTailEnabled = false
		m.scrollLogBy(m.logPageSize())
		return m, nil
	case msg.Keystroke() == "left":
		m.followTail = false
		m.followTailEnabled = false
		m.scrollLogHorizontally(-4)
		return m, nil
	case msg.Keystroke() == "right":
		m.followTail = false
		m.followTailEnabled = false
		m.scrollLogHorizontally(4)
		return m, nil
	case msg.Keystroke() == "t":
		m.followTail = true
		m.followTailEnabled = true
		m.scrollLogToBottom()
		return m, nil
	case msg.Keystroke() == "T":
		m.followTail = false
		m.followTailEnabled = false
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
	if m.bottomView != bottomViewLogs {
		return m, nil
	}
	m.followTail = false
	m.followTailEnabled = false
	switch msg.Mouse().Button {
	case tea.MouseWheelUp:
		m.scrollLogBy(-3)
	case tea.MouseWheelDown:
		m.scrollLogBy(3)
	}
	return m, nil
}

func (m *model) handleListKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Keystroke() == "up":
		m.moveSelection(-1)
		return m, nil
	case msg.Keystroke() == "down":
		m.moveSelection(1)
		return m, nil
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
		m.focus = focusDetailName
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
		m.focus = focusDetailName
		m.flashMessage = "cloning bookmark"
		m.errorMessage = ""
		return m, nil
	case msg.Text == "d":
		if m.selectedBookmark() != nil {
			m.confirmDelete = true
		} else {
			m.errorMessage = "select a bookmark to delete"
		}
		return m, nil
	case msg.Text == "r":
		return m, rescanCmd(m.ctx, m.client, nil, nil)
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
	if m.editor == nil {
		m.focus = focusModelList
		return m, nil
	}

	switch {
	case msg.Keystroke() == "esc":
		m.editor = nil
		m.focus = focusModelList
		m.errorMessage = ""
		m.flashMessage = "discarded changes"
		return m, nil
	case msg.Keystroke() == "ctrl+s":
		return m.saveEditor()
	case msg.Keystroke() == "tab":
		if m.handleArgCompletionTab(argCompletionForward) {
			m.errorMessage = ""
		}
		return m, nil
	case msg.Keystroke() == "shift+tab":
		if m.handleArgCompletionTab(argCompletionBackward) {
			m.errorMessage = ""
		}
		return m, nil
	case isLoadShortcut(msg):
		if m.editor.isNew {
			m.errorMessage = "save the new bookmark before loading it"
			return m, nil
		}
		if m.editor.Dirty() {
			m.errorMessage = "save or discard changes before loading"
			return m, nil
		}
		return m, loadBookmarkCmd(m.ctx, m.client, m.editor.originalID)
	case isUnloadShortcut(msg):
		return m, unloadCmd(m.ctx, m.client)
	case msg.Keystroke() == "up":
		m.editor.completion = argCompletionState{}
		return m.handleEditorUp()
	case msg.Keystroke() == "down":
		m.editor.completion = argCompletionState{}
		return m.handleEditorDown()
	case msg.Keystroke() == "enter":
		m.editor.completion = argCompletionState{}
		switch m.focus {
		case focusDetailName:
			m.focus = focusDetailArgs
		case focusDetailArgs:
			m.editor.args.InsertNewLine()
		}
		m.errorMessage = ""
		return m, nil
	default:
		buffer := m.currentEditorBuffer()
		if buffer == nil {
			return m, nil
		}
		if handleBufferKey(buffer, msg.Keystroke()) {
			if msg.Keystroke() == "backspace" || msg.Keystroke() == "delete" {
				m.refreshPassiveArgCompletion()
			} else {
				m.editor.completion = argCompletionState{}
			}
			m.errorMessage = ""
			return m, nil
		}
		if text := printableText(msg); text != "" {
			buffer.InsertText(text)
			m.refreshPassiveArgCompletion()
			m.errorMessage = ""
			return m, nil
		}
		return m, nil
	}
}

func (m *model) handleEditorUp() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusDetailName:
		return m, nil
	case focusDetailArgs:
		if !m.editor.args.MoveUp() {
			m.focus = focusDetailName
		}
	}
	return m, nil
}

func (m *model) handleEditorDown() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusDetailName:
		m.focus = focusDetailArgs
	case focusDetailArgs:
		_ = m.editor.args.MoveDown()
	}
	return m, nil
}

func (m *model) saveEditor() (tea.Model, tea.Cmd) {
	if m.editor == nil {
		return m, nil
	}
	bookmark := m.editor.Bookmark()
	return m, saveBookmarkCmd(m.ctx, m.client, bookmark, m.editor.isNew, false)
}

func (m *model) currentEditorBuffer() *textBuffer {
	if m.editor == nil {
		return nil
	}
	switch m.focus {
	case focusDetailName:
		return &m.editor.name
	case focusDetailArgs:
		return &m.editor.args
	default:
		return nil
	}
}

func (m *model) toggleBottomView() {
	if m.bottomView == bottomViewBookmarks {
		m.bottomView = bottomViewLogs
		m.clampLogScroll()
		return
	}
	m.bottomView = bottomViewBookmarks
}

func handleBufferKey(buffer *textBuffer, key string) bool {
	switch key {
	case "left":
		return buffer.MoveLeft()
	case "right":
		return buffer.MoveRight()
	case "home":
		return buffer.MoveHome()
	case "end":
		return buffer.MoveEnd()
	case "backspace":
		buffer.Backspace()
	case "delete":
		buffer.Delete()
	default:
		return false
	}
	return true
}

func printableText(msg tea.KeyPressMsg) string {
	return msg.Text
}
