package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"nice-llama-server/internal/config"
	"nice-llama-server/internal/controller"
)

const (
	statePollInterval = 1200 * time.Millisecond
	logPollInterval   = 450 * time.Millisecond
	maxVisibleLogs    = 2000
)

type model struct {
	ctx           context.Context
	client        *controller.Client
	width         int
	height        int
	showLogs      bool
	snapshot      config.Snapshot
	selectedID    string
	logs          []config.LogEntry
	lastSeq       int64
	stateReady    bool
	errorMessage  string
	flashMessage  string
	editor        *bookmarkEditor
	confirmDelete bool
}

type stateMsg struct {
	snapshot config.Snapshot
	err      error
}

type logsMsg struct {
	entries []config.LogEntry
	err     error
}

type actionMsg struct {
	snapshot   config.Snapshot
	selectedID string
	note       string
	err        error
}

type pollStateMsg struct{}
type pollLogsMsg struct{}

func newModel(ctx context.Context, client *controller.Client) *model {
	return &model{
		ctx:      ctx,
		client:   client,
		width:    100,
		height:   34,
		showLogs: true,
	}
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		fetchStateCmd(m.ctx, m.client),
		fetchLogsCmd(m.ctx, m.client, 0),
		scheduleStatePoll(),
		scheduleLogPoll(),
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case stateMsg:
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			return m, nil
		}
		m.stateReady = true
		m.snapshot = msg.snapshot
		m.syncSelection()
		return m, nil
	case logsMsg:
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			return m, nil
		}
		if len(msg.entries) > 0 {
			m.logs = append(m.logs, msg.entries...)
			if len(m.logs) > maxVisibleLogs {
				m.logs = append([]config.LogEntry(nil), m.logs[len(m.logs)-maxVisibleLogs:]...)
			}
			m.lastSeq = msg.entries[len(msg.entries)-1].Seq
		}
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			return m, tea.Batch(fetchStateCmd(m.ctx, m.client), fetchLogsCmd(m.ctx, m.client, m.lastSeq))
		}
		m.errorMessage = ""
		m.flashMessage = msg.note
		m.snapshot = msg.snapshot
		m.selectedID = msg.selectedID
		m.syncSelection()
		m.confirmDelete = false
		m.editor = nil
		if m.snapshot.Runtime.Status == config.StatusLoading || m.snapshot.Runtime.Status == config.StatusReady || m.snapshot.Runtime.Status == config.StatusFailed {
			return m, tea.Batch(fetchLogsCmd(m.ctx, m.client, 0))
		}
		return m, nil
	case pollStateMsg:
		return m, tea.Batch(fetchStateCmd(m.ctx, m.client), scheduleStatePoll())
	case pollLogsMsg:
		return m, tea.Batch(fetchLogsCmd(m.ctx, m.client, m.lastSeq), scheduleLogPoll())
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m *model) View() tea.View {
	return tea.NewView(m.render())
}

func (m *model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Keystroke() == "ctrl+c", msg.Text == "q":
		return m, tea.Quit
	}

	if m.confirmDelete {
		switch {
		case strings.EqualFold(msg.Text, "y"):
			selected := m.selectedBookmark()
			if selected == nil {
				m.confirmDelete = false
				return m, nil
			}
			return m, deleteBookmarkCmd(m.ctx, m.client, selected.ID)
		case strings.EqualFold(msg.Text, "n"), msg.Keystroke() == "esc":
			m.confirmDelete = false
			return m, nil
		default:
			return m, nil
		}
	}

	if m.editor != nil {
		return m.handleEditorKey(msg)
	}

	switch {
	case msg.Text == "n":
		m.editor = newBookmarkEditor(config.Bookmark{}, m.snapshot.Models, true)
		m.errorMessage = ""
		m.flashMessage = "creating a new bookmark"
		return m, nil
	case msg.Text == "c":
		if selected := m.selectedBookmark(); selected != nil {
			clone := *selected
			clone.ID = ""
			clone.Name = clone.Name + " Copy"
			m.editor = newBookmarkEditor(clone, m.snapshot.Models, true)
			m.flashMessage = "cloning bookmark"
		}
		return m, nil
	case msg.Text == "e":
		if selected := m.selectedBookmark(); selected != nil {
			copy := *selected
			m.editor = newBookmarkEditor(copy, m.snapshot.Models, false)
			m.flashMessage = "editing bookmark"
		}
		return m, nil
	case msg.Text == "d":
		if m.selectedBookmark() != nil {
			m.confirmDelete = true
		}
		return m, nil
	case msg.Keystroke() == "up", msg.Text == "k":
		m.moveSelection(-1)
		return m, nil
	case msg.Keystroke() == "down", msg.Text == "j":
		m.moveSelection(1)
		return m, nil
	case msg.Text == "r":
		return m, rescanCmd(m.ctx, m.client, nil, nil)
	case msg.Text == "g":
		m.showLogs = !m.showLogs
		if m.showLogs {
			m.flashMessage = "log panel shown"
		} else {
			m.flashMessage = "log panel hidden"
		}
		return m, nil
	case isLoadShortcut(msg):
		if selected := m.selectedBookmark(); selected != nil {
			return m, loadBookmarkCmd(m.ctx, m.client, selected.ID)
		}
		return m, nil
	case isUnloadShortcut(msg):
		return m, unloadCmd(m.ctx, m.client)
	default:
		return m, nil
	}
}

func (m *model) handleEditorKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Keystroke() == "esc":
		m.editor = nil
		m.errorMessage = ""
		return m, nil
	case msg.Keystroke() == "tab":
		if m.editor.focus == 1 && m.editor.AutocompleteModel(m.snapshot.Models) {
			m.errorMessage = ""
		}
		return m, nil
	case msg.Keystroke() == "ctrl+n":
		m.editor.NextFocus()
		return m, nil
	case msg.Keystroke() == "ctrl+p":
		m.editor.PrevFocus()
		return m, nil
	case msg.Keystroke() == "ctrl+s":
		return m.saveEditor(false)
	case msg.Keystroke() == "ctrl+l":
		return m.saveEditor(true)
	default:
		buffer := m.editor.Current()
		if handleBufferKey(buffer, msg.Keystroke()) {
			return m, nil
		}
		if text := printableText(msg); text != "" {
			buffer.InsertText(text)
			m.errorMessage = ""
			return m, nil
		}
		return m, nil
	}
}

func (m *model) saveEditor(andLoad bool) (tea.Model, tea.Cmd) {
	bookmark, err := m.editor.Bookmark(m.snapshot.Models)
	if err != nil {
		m.errorMessage = err.Error()
		return m, nil
	}
	return m, saveBookmarkCmd(m.ctx, m.client, bookmark, m.editor.isNew, andLoad)
}

func handleBufferKey(buffer *textBuffer, key string) bool {
	switch key {
	case "left":
		buffer.MoveLeft()
	case "right":
		buffer.MoveRight()
	case "up":
		buffer.MoveUp()
	case "down":
		buffer.MoveDown()
	case "home":
		buffer.MoveHome()
	case "end":
		buffer.MoveEnd()
	case "backspace":
		buffer.Backspace()
	case "delete":
		buffer.Delete()
	case "enter":
		buffer.InsertNewLine()
	default:
		return false
	}
	return true
}

func printableText(msg tea.KeyPressMsg) string {
	switch msg.Keystroke() {
	case "":
		return ""
	}
	if msg.Text != "" {
		return msg.Text
	}
	if strings.HasPrefix(msg.Keystroke(), "ctrl+") || strings.HasPrefix(msg.Keystroke(), "alt+") {
		return ""
	}
	for _, special := range []string{"up", "down", "left", "right", "tab", "enter", "esc", "backspace", "delete", "home", "end", "shift+tab"} {
		if msg.Keystroke() == special {
			return ""
		}
	}
	return ""
}

func (m *model) syncSelection() {
	if len(m.snapshot.Bookmarks) == 0 {
		m.selectedID = ""
		return
	}
	for _, item := range m.snapshot.Bookmarks {
		if item.ID == m.selectedID {
			return
		}
	}
	m.selectedID = m.snapshot.Bookmarks[0].ID
}

func (m *model) moveSelection(delta int) {
	if len(m.snapshot.Bookmarks) == 0 {
		return
	}
	idx := 0
	for i := range m.snapshot.Bookmarks {
		if m.snapshot.Bookmarks[i].ID == m.selectedID {
			idx = i
			break
		}
	}
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.snapshot.Bookmarks) {
		idx = len(m.snapshot.Bookmarks) - 1
	}
	m.selectedID = m.snapshot.Bookmarks[idx].ID
}

func (m *model) selectedBookmark() *config.Bookmark {
	for i := range m.snapshot.Bookmarks {
		if m.snapshot.Bookmarks[i].ID == m.selectedID {
			return &m.snapshot.Bookmarks[i]
		}
	}
	return nil
}

func fetchStateCmd(ctx context.Context, client *controller.Client) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		snapshot, err := client.State(reqCtx)
		return stateMsg{snapshot: snapshot, err: err}
	}
}

func fetchLogsCmd(ctx context.Context, client *controller.Client, after int64) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		entries, err := client.Logs(reqCtx, after)
		return logsMsg{entries: entries, err: err}
	}
}

func scheduleStatePoll() tea.Cmd {
	return tea.Tick(statePollInterval, func(time.Time) tea.Msg {
		return pollStateMsg{}
	})
}

func scheduleLogPoll() tea.Cmd {
	return tea.Tick(logPollInterval, func(time.Time) tea.Msg {
		return pollLogsMsg{}
	})
}

func saveBookmarkCmd(ctx context.Context, client *controller.Client, b config.Bookmark, isNew bool, andLoad bool) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()

		var (
			saved config.Bookmark
			err   error
		)
		if isNew || b.ID == "" {
			saved, err = client.CreateBookmark(reqCtx, b)
		} else {
			saved, err = client.UpdateBookmark(reqCtx, b)
		}
		if err == nil && andLoad {
			_, err = client.Load(reqCtx, saved.ID)
		}
		snapshot, stateErr := client.State(reqCtx)
		if err == nil {
			err = stateErr
		}
		note := "bookmark saved"
		if andLoad && err == nil {
			note = "bookmark saved and loading started"
		}
		return actionMsg{snapshot: snapshot, selectedID: saved.ID, note: note, err: err}
	}
}

func deleteBookmarkCmd(ctx context.Context, client *controller.Client, id string) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		err := client.DeleteBookmark(reqCtx, id)
		snapshot, stateErr := client.State(reqCtx)
		if err == nil {
			err = stateErr
		}
		return actionMsg{snapshot: snapshot, note: "bookmark deleted", err: err}
	}
}

func loadBookmarkCmd(ctx context.Context, client *controller.Client, id string) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		_, err := client.Load(reqCtx, id)
		snapshot, stateErr := client.State(reqCtx)
		if err == nil {
			err = stateErr
		}
		return actionMsg{snapshot: snapshot, selectedID: id, note: "model loaded", err: err}
	}
}

func unloadCmd(ctx context.Context, client *controller.Client) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		_, err := client.Unload(reqCtx)
		snapshot, stateErr := client.State(reqCtx)
		if err == nil {
			err = stateErr
		}
		return actionMsg{snapshot: snapshot, note: "model unloaded", err: err}
	}
}

func rescanCmd(ctx context.Context, client *controller.Client, roots []string, bin *string) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		snapshot, err := client.Rescan(reqCtx, roots, bin)
		return actionMsg{snapshot: snapshot, note: "model directories rescanned", err: err}
	}
}

func statusLabel(state config.RuntimeState) string {
	if state.Status == "" {
		return config.StatusIdle
	}
	return state.Status
}

func activeBookmarkName(snapshot config.Snapshot) string {
	if snapshot.Runtime.ActiveBookmarkID == "" {
		return ""
	}
	for _, b := range snapshot.Bookmarks {
		if b.ID == snapshot.Runtime.ActiveBookmarkID {
			return b.Name
		}
	}
	return snapshot.Runtime.ActiveBookmarkID
}

func runtimeSummary(snapshot config.Snapshot) string {
	label := statusLabel(snapshot.Runtime)
	name := activeBookmarkName(snapshot)
	if name == "" {
		return fmt.Sprintf("runtime: %s", label)
	}
	return fmt.Sprintf("runtime: %s (%s)", label, name)
}

func isLoadShortcut(msg tea.KeyPressMsg) bool {
	return msg.Text == "L" || msg.Keystroke() == "shift+l"
}

func isUnloadShortcut(msg tea.KeyPressMsg) bool {
	return msg.Text == "U" || msg.Keystroke() == "shift+u"
}
