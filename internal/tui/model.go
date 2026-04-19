package tui

import (
	"context"
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

type bottomView int

const (
	bottomViewBookmarks bottomView = iota
	bottomViewLogs
)

type focusArea int

const (
	focusModelList focusArea = iota
	focusDetailName
	focusDetailArgs
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
	styles            styles
	width             int
	height            int
	bottomView        bottomView
	focus             focusArea
	snapshot          config.Snapshot
	selectedKey       string
	logs              []config.LogEntry
	lastSeq           int64
	logScrollY        int
	logScrollX        int
	logViewWidth      int
	logViewHeight     int
	stateReady        bool
	errorMessage      string
	flashMessage      string
	editor            *bookmarkEditor
	confirmDelete     bool
	followTail        bool
	followTailEnabled bool
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
	snapshot    config.Snapshot
	selectedKey string
	note        string
	err         error
	clearEditor bool
	focus       focusArea
}

type pollStateMsg struct{}
type pollLogsMsg struct{}

func newModel(ctx context.Context, client *controller.Client) *model {
	return &model{
		ctx:               ctx,
		client:            client,
		styles:            newStyles(),
		width:             100,
		height:            34,
		bottomView:        bottomViewBookmarks,
		focus:             focusModelList,
		followTail:        true,
		followTailEnabled: true,
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
		m.clampLogScroll()
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
			wasAtBottom := m.logAtBottom()
			m.logs = append(m.logs, msg.entries...)
			if len(m.logs) > maxVisibleLogs {
				m.logs = append([]config.LogEntry(nil), m.logs[len(m.logs)-maxVisibleLogs:]...)
			}
			m.lastSeq = msg.entries[len(msg.entries)-1].Seq
			if m.followTail && m.followTailEnabled {
				if wasAtBottom {
					m.scrollLogToBottom()
				} else {
					m.clampLogScroll()
				}
			} else {
				m.clampLogScroll()
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
		m.snapshot = msg.snapshot
		if msg.selectedKey != "" {
			m.selectedKey = msg.selectedKey
		}
		if msg.clearEditor {
			m.editor = nil
			m.focus = msg.focus
			m.confirmDelete = false
		}
		m.syncSelection()
		if m.snapshot.Runtime.Status == config.StatusLoading || m.snapshot.Runtime.Status == config.StatusReady || m.snapshot.Runtime.Status == config.StatusFailed {
			return m, fetchLogsCmd(m.ctx, m.client, 0)
		}
		return m, nil
	case pollStateMsg:
		return m, tea.Batch(fetchStateCmd(m.ctx, m.client), scheduleStatePoll())
	case pollLogsMsg:
		return m, tea.Batch(fetchLogsCmd(m.ctx, m.client, m.lastSeq), scheduleLogPoll())
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
