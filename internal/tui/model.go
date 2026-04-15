package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

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
	ctx           context.Context
	client        *controller.Client
	styles        styles
	width         int
	height        int
	bottomView    bottomView
	focus         focusArea
	snapshot      config.Snapshot
	selectedKey   string
	logs          []config.LogEntry
	lastSeq       int64
	logScrollY    int
	logScrollX    int
	logViewWidth  int
	logViewHeight int
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
		ctx:        ctx,
		client:     client,
		styles:     newStyles(),
		width:      100,
		height:     34,
		bottomView: bottomViewBookmarks,
		focus:      focusModelList,
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
			if wasAtBottom {
				m.scrollLogToBottom()
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
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)
	default:
		return m, nil
	}
}

func (m *model) View() tea.View {
	return tea.NewView(m.render())
}

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
		m.scrollLogBy(-1)
		return m, nil
	case msg.Keystroke() == "down":
		m.scrollLogBy(1)
		return m, nil
	case msg.Keystroke() == "pgup":
		m.scrollLogBy(-m.logPageSize())
		return m, nil
	case msg.Keystroke() == "pgdown":
		m.scrollLogBy(m.logPageSize())
		return m, nil
	case msg.Keystroke() == "left":
		m.scrollLogHorizontally(-4)
		return m, nil
	case msg.Keystroke() == "right":
		m.scrollLogHorizontally(4)
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
		return m.handleEditorUp()
	case msg.Keystroke() == "down":
		return m.handleEditorDown()
	case msg.Keystroke() == "enter":
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
			m.errorMessage = ""
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

func (m *model) listItems() []listItem {
	bookmarksByGroup := map[string][]config.Bookmark{}
	for _, bookmark := range m.snapshot.Bookmarks {
		bookmarksByGroup[bookmark.ModelPath] = append(bookmarksByGroup[bookmark.ModelPath], bookmark)
	}

	items := make([]listItem, 0, len(m.snapshot.Models)+len(m.snapshot.Bookmarks))
	seenGroups := map[string]struct{}{}
	for _, model := range m.snapshot.Models {
		if _, ok := seenGroups[model.Path]; ok {
			continue
		}
		seenGroups[model.Path] = struct{}{}
		items = append(items, listItem{
			kind:      listItemModelGroup,
			groupKey:  model.GroupKey,
			modelPath: model.Path,
			label:     model.DisplayName,
		})
		for _, bookmark := range bookmarksByGroup[model.Path] {
			items = append(items, listItem{
				kind:       listItemBookmark,
				groupKey:   bookmark.GroupKey,
				modelPath:  bookmark.ModelPath,
				label:      bookmark.Name,
				bookmarkID: bookmark.ID,
			})
		}
		delete(bookmarksByGroup, model.Path)
	}

	leftoverGroups := make([]string, 0, len(bookmarksByGroup))
	for modelPath := range bookmarksByGroup {
		leftoverGroups = append(leftoverGroups, modelPath)
	}
	slices.Sort(leftoverGroups)
	for _, modelPath := range leftoverGroups {
		groupLabel := displayNameFromPath(modelPath)
		groupKey := config.DeriveGroupKey(modelPath)
		if len(bookmarksByGroup[modelPath]) > 0 && bookmarksByGroup[modelPath][0].GroupKey != "" {
			groupKey = bookmarksByGroup[modelPath][0].GroupKey
		}
		items = append(items, listItem{
			kind:      listItemModelGroup,
			groupKey:  groupKey,
			modelPath: modelPath,
			label:     groupLabel,
			degraded:  true,
		})
		for _, bookmark := range bookmarksByGroup[modelPath] {
			items = append(items, listItem{
				kind:       listItemBookmark,
				groupKey:   bookmark.GroupKey,
				modelPath:  bookmark.ModelPath,
				label:      bookmark.Name,
				bookmarkID: bookmark.ID,
				degraded:   true,
			})
		}
	}

	return items
}

func (m *model) syncSelection() {
	items := m.listItems()
	if len(items) == 0 {
		m.selectedKey = ""
		return
	}

	if m.selectedKey != "" {
		for _, item := range items {
			if item.key() == m.selectedKey {
				return
			}
		}
	}

	for _, item := range items {
		if item.kind == listItemBookmark {
			m.selectedKey = item.key()
			return
		}
	}
	m.selectedKey = items[0].key()
}

func (m *model) moveSelection(delta int) {
	items := m.listItems()
	if len(items) == 0 {
		m.selectedKey = ""
		return
	}

	index := 0
	for i, item := range items {
		if item.key() == m.selectedKey {
			index = i
			break
		}
	}
	index += delta
	if index < 0 {
		index = 0
	}
	if index >= len(items) {
		index = len(items) - 1
	}
	m.selectedKey = items[index].key()
}

func (m *model) selectedItem() (listItem, bool) {
	for _, item := range m.listItems() {
		if item.key() == m.selectedKey {
			return item, true
		}
	}
	return listItem{}, false
}

func (m *model) selectedBookmark() *config.Bookmark {
	item, ok := m.selectedItem()
	if !ok || item.kind != listItemBookmark {
		return nil
	}
	for i := range m.snapshot.Bookmarks {
		if m.snapshot.Bookmarks[i].ID == item.bookmarkID {
			return &m.snapshot.Bookmarks[i]
		}
	}
	return nil
}

func (m *model) currentGroupSelection() (listItem, bool) {
	item, ok := m.selectedItem()
	if ok {
		if item.kind == listItemBookmark {
			return listItem{
				kind:      listItemModelGroup,
				groupKey:  item.groupKey,
				modelPath: item.modelPath,
				label:     m.groupLabelForPath(item.modelPath),
				degraded:  m.isMissingModelPath(item.modelPath),
			}, true
		}
		return item, true
	}

	items := m.listItems()
	for _, candidate := range items {
		if candidate.kind == listItemModelGroup {
			return candidate, true
		}
	}
	return listItem{}, false
}

func (m *model) beginEditSelected() error {
	selected := m.selectedBookmark()
	if selected == nil {
		return fmt.Errorf("select a bookmark to edit")
	}
	m.editor = newBookmarkEditor(*selected, false)
	m.focus = focusDetailName
	m.errorMessage = ""
	return nil
}

func (m *model) newBookmarkForCurrentGroup() (*bookmarkEditor, error) {
	group, ok := m.currentGroupSelection()
	if !ok {
		return nil, fmt.Errorf("no discovered models are available")
	}
	base := config.Bookmark{
		ModelPath: group.modelPath,
		GroupKey:  group.groupKey,
	}
	return newBookmarkEditor(base, true), nil
}

func (m *model) cloneSelectedBookmark() (*bookmarkEditor, error) {
	selected := m.selectedBookmark()
	if selected == nil {
		return nil, fmt.Errorf("select a bookmark to clone")
	}
	clone := *selected
	clone.ID = ""
	clone.Name = clone.Name + " Copy"
	return newBookmarkEditor(clone, true), nil
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
		return actionMsg{
			snapshot:    snapshot,
			selectedKey: listItem{kind: listItemBookmark, bookmarkID: saved.ID}.key(),
			note:        note,
			err:         err,
			clearEditor: err == nil,
			focus:       focusModelList,
		}
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
		return actionMsg{
			snapshot:    snapshot,
			note:        "bookmark deleted",
			err:         err,
			clearEditor: err == nil,
			focus:       focusModelList,
		}
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
		return actionMsg{
			snapshot:    snapshot,
			selectedKey: listItem{kind: listItemBookmark, bookmarkID: id}.key(),
			note:        "model loaded",
			err:         err,
		}
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
		return actionMsg{
			snapshot: snapshot,
			note:     "model unloaded",
			err:      err,
		}
	}
}

func rescanCmd(ctx context.Context, client *controller.Client, roots []string, bin *string) tea.Cmd {
	return func() tea.Msg {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		snapshot, err := client.Rescan(reqCtx, roots, bin)
		return actionMsg{
			snapshot: snapshot,
			note:     "model directories rescanned",
			err:      err,
		}
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
	label := strings.ToUpper(statusLabel(snapshot.Runtime))
	name := activeBookmarkName(snapshot)
	if name == "" {
		return label
	}
	return fmt.Sprintf("%s · %s", label, name)
}

func isLoadShortcut(msg tea.KeyPressMsg) bool {
	return msg.Text == "L" || msg.Keystroke() == "shift+l"
}

func isUnloadShortcut(msg tea.KeyPressMsg) bool {
	return msg.Text == "U" || msg.Keystroke() == "shift+u"
}

func (m *model) groupLabelForPath(modelPath string) string {
	for _, model := range m.snapshot.Models {
		if model.Path == modelPath {
			return model.DisplayName
		}
	}
	return displayNameFromPath(modelPath)
}

func (m *model) isMissingModelPath(modelPath string) bool {
	for _, model := range m.snapshot.Models {
		if model.Path == modelPath {
			return false
		}
	}
	return true
}

func displayNameFromPath(modelPath string) string {
	name := modelPath
	if idx := strings.LastIndexAny(name, `/\`); idx >= 0 && idx+1 < len(name) {
		name = name[idx+1:]
	}
	if idx := strings.LastIndex(name, "."); idx > 0 {
		name = name[:idx]
	}
	if strings.TrimSpace(name) == "" {
		return "Unknown model"
	}
	return name
}

func (m *model) logPageSize() int {
	if m.logViewHeight <= 1 {
		return 1
	}
	return m.logViewHeight
}

func (m *model) scrollLogBy(delta int) {
	m.logScrollY += delta
	m.clampLogScroll()
}

func (m *model) scrollLogHorizontally(delta int) {
	m.logScrollX += delta
	m.clampLogScroll()
}

func (m *model) scrollLogToBottom() {
	rows := m.renderedLogRows()
	maxScroll := max(0, len(rows)-m.logPageSize())
	m.logScrollY = maxScroll
	m.clampLogScroll()
}

func (m *model) logAtBottom() bool {
	rows := m.renderedLogRows()
	maxScroll := max(0, len(rows)-m.logPageSize())
	return m.logScrollY >= maxScroll
}

func (m *model) clampLogScroll() {
	rows := m.renderedLogRows()
	page := m.logPageSize()
	maxY := max(0, len(rows)-page)
	if m.logScrollY < 0 {
		m.logScrollY = 0
	}
	if m.logScrollY > maxY {
		m.logScrollY = maxY
	}

	maxWidth := 0
	for _, row := range rows {
		if width := lipgloss.Width(ansi.Strip(row)); width > maxWidth {
			maxWidth = width
		}
	}
	maxX := max(0, maxWidth-m.logViewWidth)
	if m.logScrollX < 0 {
		m.logScrollX = 0
	}
	if m.logScrollX > maxX {
		m.logScrollX = maxX
	}
}
