package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"nice-llama-server/internal/config"
	"nice-llama-server/internal/controller"
)

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
