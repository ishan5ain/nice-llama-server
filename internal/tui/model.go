package tui

import (
	"context"
	"net/http"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/ishan5ain/tuiweave"
	"github.com/ishan5ain/tuiweave/autocomplete"
	"github.com/ishan5ain/tuiweave/dialog"
	"github.com/ishan5ain/tuiweave/focus"
	"github.com/ishan5ain/tuiweave/list"
	"github.com/ishan5ain/tuiweave/statusbar"
	"github.com/ishan5ain/tuiweave/tabs"
	"github.com/ishan5ain/tuiweave/textarea"
	"github.com/ishan5ain/tuiweave/viewport"

	"nice-llama-server/internal/config"
	"nice-llama-server/internal/controller"
)

const (
	statePollInterval = 1200 * time.Millisecond
	logPollInterval   = 450 * time.Millisecond
	maxVisibleLogs    = 2000
)

type listItemKind int

const (
	listItemModelGroup listItemKind = iota
	listItemBookmark
)

type listItem struct {
	kind       listItemKind
	groupKey   string
	modelPath  string
	label      string
	bookmarkID string
	degraded   bool
}

func (i listItem) key() string {
	if i.kind == listItemBookmark {
		return "bookmark:" + i.bookmarkID
	}
	return "group:" + i.modelPath
}

type model struct {
	ctx               context.Context
	client            *controller.Client
	theme             tuiweave.Theme
	styles            styles
	width             int
	height            int
	fm              focus.Manager
	editorScope     focus.Scope
	snapshot        config.Snapshot
	selectedKey       string
	modelList         list.Model
	logs              []config.LogEntry
	lastSeq           int64
	logScrollX        int
	stateReady        bool
	stateVersion      int
	errorMessage      string
	flashMessage      string
	editor            *bookmarkEditor
	followTailEnabled bool
	footer            statusbar.Model
	tabs              tabs.Model
	logView           viewport.Model
	deleteDialog      dialog.Model
	showDialog        bool
	pendingDeleteID   string
	pendingDiscard    bool
	ac                autocomplete.Model
}

type stateMsg struct {
	snapshot config.Snapshot
	err      error
	version  int
}

type logsMsg struct {
	entries []config.LogEntry
	err     error
}

type actionMsg struct {
	snapshot    config.Snapshot
	selectedKey string
	note        string
	err         error
	clearEditor bool
}

type pollStateMsg struct{}
type pollLogsMsg struct{}

func newModel(ctx context.Context, client *controller.Client) *model {
	m := &model{
		ctx:               ctx,
		client:            client,
		theme:             tuiweave.Dark(),
		styles:            newStyles(tuiweave.Dark()),
		width:             100,
		height:            34,
		fm:                focus.NewManager(2),
		editorScope:        focus.NewScope(3),
		followTailEnabled: true,
		footer:            statusbar.New(tuiweave.Dark()),
		logView:           viewport.New(tuiweave.Dark()),
		deleteDialog:      dialog.New(tuiweave.Dark()),
	}
	m.tabs = tabs.New(tuiweave.Dark())
	m.tabs.SetTabs(tabs.Tab{ID: "bookmarks", Label: "Bookmarks"}, tabs.Tab{ID: "logs", Label: "Logs"})
	m.modelList = list.New(tuiweave.Dark())
	m.ac = autocomplete.New(tuiweave.Dark())
	m.fm.Next() // Start with list focused
	m.fm.Apply(&m.tabs, &m.modelList)
	return m
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
		m.footer.SetSize(msg.Width, 1)
		m.tabs.SetSize(msg.Width, 1)
		m.modelList.SetSize(msg.Width/2, msg.Height-10)
		return m, nil
	case stateMsg:
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			return m, nil
		}
		if msg.version < m.stateVersion {
			return m, nil
		}
		m.errorMessage = ""
		m.flashMessage = ""
		m.stateReady = true
		m.snapshot = msg.snapshot
		m.syncSelection()
		return m, nil
	case logsMsg:
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			return m, nil
		}
		m.errorMessage = ""
		if len(msg.entries) > 0 {
			wasAtBottom := m.logView.AtBottom()
			for _, entry := range msg.entries {
				if entry.Seq <= m.lastSeq {
					continue
				}
				m.logs = append(m.logs, entry)
			}
			if len(m.logs) > maxVisibleLogs {
				m.logs = append([]config.LogEntry(nil), m.logs[len(m.logs)-maxVisibleLogs:]...)
			}
			m.lastSeq = msg.entries[len(msg.entries)-1].Seq
			if m.followTailEnabled && wasAtBottom {
				m.logView.GotoBottom()
			}
		}
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			return m, tea.Batch(fetchStateCmd(m.ctx, m.client), fetchLogsCmd(m.ctx, m.client, m.lastSeq))
		}
		m.errorMessage = ""
		m.flashMessage = msg.note
		m.stateVersion++
		m.snapshot = msg.snapshot
		if msg.selectedKey != "" {
			m.selectedKey = msg.selectedKey
		}
		if msg.clearEditor {
			m.editor = nil
			m.editorScope.Exit(&m.fm)
			m.applyFocus()
			m.showDialog = false
		}
		m.syncSelection()
		if m.snapshot.Runtime.Status == config.StatusLoading || m.snapshot.Runtime.Status == config.StatusReady || m.snapshot.Runtime.Status == config.StatusFailed {
			return m, fetchLogsCmd(m.ctx, m.client, 0)
		}
		return m, nil
	case pollStateMsg:
		ver := m.stateVersion
		return m, tea.Batch(func() tea.Msg {
			reqCtx, cancel := context.WithTimeout(m.ctx, 5*time.Second)
			defer cancel()
			var snapshot config.Snapshot
			err := m.client.DoWithRetry(reqCtx, http.MethodGet, "/v1/state", nil, &snapshot, 2)
			return stateMsg{snapshot: snapshot, err: err, version: ver}
		}, scheduleStatePoll())
	case pollLogsMsg:
		return m, tea.Batch(fetchLogsCmd(m.ctx, m.client, m.lastSeq), scheduleLogPoll())
	case dialog.ResultMsg:
		if m.showDialog {
			m.showDialog = false
			if msg.OK && m.pendingDeleteID != "" {
				id := m.pendingDeleteID
				m.pendingDeleteID = ""
				return m, deleteBookmarkCmd(m.ctx, m.client, id)
			}
			m.pendingDeleteID = ""
		}
		return m, nil
	case autocomplete.SelectedMsg:
		if m.ac.Focused() && m.editor != nil && m.editorScope.Active() {
			m.editor.args.ReplaceRange(
				textarea.Position{Row: m.editor.completion.row, Column: m.editor.completion.start},
				textarea.Position{Row: m.editor.completion.row, Column: m.editor.completion.end},
				msg.Value,
			)
			m.editor.completion = argCompletionState{}
			m.ac.Blur()
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.PasteMsg:
		m.pasteIntoBuffer(msg.Content)
		return m, nil
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	default:
		return m, nil
	}
}

func (m *model) View() tea.View {
	return tea.NewView(m.render())
}
