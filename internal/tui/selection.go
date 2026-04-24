package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"nice-llama-server/internal/config"
)

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

func (m *model) discoveredModelByPath(modelPath string) *config.DiscoveredModel {
	for i := range m.snapshot.Models {
		if m.snapshot.Models[i].Path == modelPath {
			return &m.snapshot.Models[i]
		}
	}
	return nil
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
