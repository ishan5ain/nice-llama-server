package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"nice-llama-server/internal/bookmark"
	"nice-llama-server/internal/config"
	"nice-llama-server/internal/discovery"
	"nice-llama-server/internal/logbuf"
	"nice-llama-server/internal/runtime"
)

const (
	loadReadyTimeout = 2 * time.Minute
	stopTimeout      = 10 * time.Second
)

type Options struct {
	StateDir       string
	LlamaServerBin string
	ModelRoots     []string
}

type Service struct {
	store        *config.Store
	state        config.StoredState
	models       []config.DiscoveredModel
	runtimeState config.RuntimeState
	logs         *logbuf.Buffer
	token        string
	server       *http.Server
	active       *runtime.Process
	expectedExit bool
	mu           sync.RWMutex
}

func NewService(opts Options) (*Service, error) {
	store, err := config.NewStore(opts.StateDir)
	if err != nil {
		return nil, err
	}

	state, err := store.Load()
	if err != nil {
		return nil, err
	}

	if opts.LlamaServerBin != "" {
		state.Config.LlamaServerBin = opts.LlamaServerBin
	}
	if opts.ModelRoots != nil {
		state.Config.ModelRoots = opts.ModelRoots
	}
	if err := store.Save(state); err != nil {
		return nil, err
	}

	models, err := discovery.Scan(state.Config.ModelRoots)
	if err != nil {
		return nil, err
	}

	return &Service{
		store:  store,
		state:  state,
		models: models,
		runtimeState: config.RuntimeState{
			Status: config.StatusIdle,
		},
		logs:  logbuf.New(2000),
		token: randomToken(),
	}, nil
}

func Run(ctx context.Context, opts Options) error {
	svc, err := NewService(opts)
	if err != nil {
		return err
	}

	ln, err := ListenLoopback()
	if err != nil {
		return err
	}
	if _, err := svc.Start(ln); err != nil {
		return err
	}

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return svc.Shutdown(shutdownCtx)
}

func ListenLoopback() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

func (s *Service) Start(listener net.Listener) (config.ControllerInfo, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/state", s.withAuth(s.handleState))
	mux.HandleFunc("/v1/rescan", s.withAuth(s.handleRescan))
	mux.HandleFunc("/v1/bookmarks", s.withAuth(s.handleBookmarks))
	mux.HandleFunc("/v1/bookmarks/", s.withAuth(s.handleBookmarkByID))
	mux.HandleFunc("/v1/runtime/load", s.withAuth(s.handleLoad))
	mux.HandleFunc("/v1/runtime/unload", s.withAuth(s.handleUnload))
	mux.HandleFunc("/v1/logs", s.withAuth(s.handleLogs))
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	info := config.ControllerInfo{
		PID:       os.Getpid(),
		URL:       "http://" + listener.Addr().String(),
		Token:     s.token,
		StartedAt: time.Now().UTC(),
	}
	if err := s.store.SaveControllerInfo(info); err != nil {
		return config.ControllerInfo{}, err
	}

	go func() {
		_ = s.server.Serve(listener)
	}()
	return info, nil
}

func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	active := s.active
	s.expectedExit = true
	s.runtimeState.Status = config.StatusStopping
	s.mu.Unlock()

	if active != nil {
		_, _ = active.Stop(stopTimeout)
	}

	if s.server != nil {
		_ = s.server.Shutdown(ctx)
	}
	return s.store.RemoveControllerInfo()
}

func (s *Service) Snapshot() config.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return config.Snapshot{
		Config:    s.state.Config,
		Bookmarks: cloneBookmarks(s.state.Bookmarks),
		Models:    cloneModels(s.models),
		Runtime:   s.runtimeState,
	}
}

func (s *Service) LogsSince(seq int64) []config.LogEntry {
	return s.logs.Since(seq)
}

func (s *Service) Rescan(modelRoots []string, llamaServerBin *string) (config.Snapshot, error) {
	s.mu.Lock()
	if modelRoots != nil {
		s.state.Config.ModelRoots = slices.Clone(modelRoots)
	}
	if llamaServerBin != nil && strings.TrimSpace(*llamaServerBin) != "" {
		s.state.Config.LlamaServerBin = strings.TrimSpace(*llamaServerBin)
	}

	models, err := discovery.Scan(s.state.Config.ModelRoots)
	if err != nil {
		s.mu.Unlock()
		return config.Snapshot{}, err
	}

	s.models = models
	if err := s.store.Save(s.state); err != nil {
		s.mu.Unlock()
		return config.Snapshot{}, err
	}
	s.mu.Unlock()
	return s.Snapshot(), nil
}

func (s *Service) CreateBookmark(b config.Bookmark) (config.Bookmark, error) {
	normalized, err := bookmark.NormalizeBookmark(b)
	if err != nil {
		return config.Bookmark{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.Bookmarks = append(s.state.Bookmarks, normalized)
	sortBookmarks(s.state.Bookmarks)
	if err := s.store.Save(s.state); err != nil {
		return config.Bookmark{}, err
	}
	return normalized, nil
}

func (s *Service) UpdateBookmark(id string, b config.Bookmark) (config.Bookmark, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := indexBookmark(s.state.Bookmarks, id)
	if idx < 0 {
		return config.Bookmark{}, errors.New("bookmark not found")
	}

	b.ID = id
	b.CreatedAt = s.state.Bookmarks[idx].CreatedAt
	normalized, err := bookmark.NormalizeBookmark(b)
	if err != nil {
		return config.Bookmark{}, err
	}

	s.state.Bookmarks[idx] = normalized
	sortBookmarks(s.state.Bookmarks)
	if err := s.store.Save(s.state); err != nil {
		return config.Bookmark{}, err
	}
	return normalized, nil
}

func (s *Service) DeleteBookmark(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runtimeState.ActiveBookmarkID == id && s.runtimeState.Status != config.StatusIdle && s.runtimeState.Status != config.StatusFailed {
		return errors.New("cannot delete active bookmark")
	}

	idx := indexBookmark(s.state.Bookmarks, id)
	if idx < 0 {
		return errors.New("bookmark not found")
	}
	s.state.Bookmarks = append(s.state.Bookmarks[:idx], s.state.Bookmarks[idx+1:]...)
	return s.store.Save(s.state)
}

func (s *Service) LoadBookmark(ctx context.Context, bookmarkID string) (config.RuntimeState, error) {
	s.mu.Lock()
	if s.active != nil || s.runtimeState.Status == config.StatusLoading || s.runtimeState.Status == config.StatusStopping {
		s.mu.Unlock()
		return config.RuntimeState{}, errors.New("a model is already active")
	}

	idx := indexBookmark(s.state.Bookmarks, bookmarkID)
	if idx < 0 {
		s.mu.Unlock()
		return config.RuntimeState{}, errors.New("bookmark not found")
	}
	b := s.state.Bookmarks[idx]
	bin := s.state.Config.LlamaServerBin
	s.runtimeState = config.RuntimeState{
		Status:           config.StatusLoading,
		ActiveBookmarkID: bookmarkID,
	}
	s.expectedExit = false
	s.logs.Reset()
	s.mu.Unlock()

	s.logs.Add("system", fmt.Sprintf("launching %s", b.Name))
	proc, err := runtime.Start(bin, b.ModelPath, b.ArgsText, runtime.Events{
		OnLog: func(stream, line string) {
			s.logs.Add(stream, line)
		},
		OnExit: func(exitCode int, waitErr error) {
			s.handleExit(bookmarkID, exitCode, waitErr)
		},
	})
	if err != nil {
		s.mu.Lock()
		code := 1
		s.runtimeState = config.RuntimeState{
			Status:           config.StatusFailed,
			ActiveBookmarkID: bookmarkID,
			ExitCode:         &code,
			Error:            err.Error(),
		}
		s.mu.Unlock()
		return s.runtimeState, err
	}

	s.mu.Lock()
	s.active = proc
	startedAt := time.Now().UTC()
	s.runtimeState.PID = proc.PID()
	s.runtimeState.Host = proc.Host()
	s.runtimeState.Port = proc.Port()
	s.runtimeState.StartedAt = &startedAt
	s.mu.Unlock()

	waitCtx, cancel := context.WithTimeout(ctx, loadReadyTimeout)
	defer cancel()
	if err := proc.WaitReady(waitCtx); err != nil {
		_, _ = proc.Stop(2 * time.Second)
		s.mu.Lock()
		code := 1
		if s.runtimeState.ExitCode != nil {
			code = *s.runtimeState.ExitCode
		}
		s.runtimeState = config.RuntimeState{
			Status:           config.StatusFailed,
			ActiveBookmarkID: bookmarkID,
			ExitCode:         &code,
			Error:            err.Error(),
		}
		s.active = nil
		s.mu.Unlock()
		return s.runtimeState, err
	}

	s.mu.Lock()
	s.runtimeState.Status = config.StatusReady
	state := s.runtimeState
	s.mu.Unlock()
	return state, nil
}

func (s *Service) Unload(ctx context.Context) (config.RuntimeState, error) {
	s.mu.Lock()
	if s.active == nil {
		state := s.runtimeState
		s.mu.Unlock()
		return state, errors.New("no active model")
	}
	proc := s.active
	s.expectedExit = true
	s.runtimeState.Status = config.StatusStopping
	s.mu.Unlock()

	done := make(chan struct{})
	var (
		exitCode int
		err      error
	)
	go func() {
		defer close(done)
		exitCode, err = proc.Stop(stopTimeout)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return s.Snapshot().Runtime, ctx.Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = nil
	s.expectedExit = false
	if err != nil {
		s.logs.Add("system", fmt.Sprintf("unload completed with warning: %v", err))
	}
	if exitCode != 0 {
		s.logs.Add("system", fmt.Sprintf("process exited with code %d", exitCode))
	}
	s.runtimeState = config.RuntimeState{
		Status: config.StatusIdle,
	}
	return s.runtimeState, nil
}

func (s *Service) handleExit(bookmarkID string, exitCode int, waitErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active == nil || s.runtimeState.ActiveBookmarkID != bookmarkID {
		return
	}

	if s.expectedExit {
		return
	}

	s.active = nil

	var name string
	for _, b := range s.state.Bookmarks {
		if b.ID == bookmarkID {
			name = b.Name
			break
		}
	}

	code := exitCode
	message := ""
	if waitErr != nil {
		message = waitErr.Error()
	}
	s.runtimeState = config.RuntimeState{
		Status:           config.StatusFailed,
		ActiveBookmarkID: bookmarkID,
		ExitCode:         &code,
		Error:            message,
	}
	s.logs.Add("system", fmt.Sprintf("model exited: %s (code %d%s)", name, code, func() string {
		if message != "" {
			return ": " + message
		}
		return ""
	}()))
}

func sortBookmarks(items []config.Bookmark) {
	slices.SortFunc(items, func(a, b config.Bookmark) int {
		if a.GroupKey != b.GroupKey {
			return strings.Compare(strings.ToLower(a.GroupKey), strings.ToLower(b.GroupKey))
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
}

func indexBookmark(items []config.Bookmark, id string) int {
	for i := range items {
		if items[i].ID == id {
			return i
		}
	}
	return -1
}

func cloneBookmarks(items []config.Bookmark) []config.Bookmark {
	out := make([]config.Bookmark, len(items))
	copy(out, items)
	return out
}

func cloneModels(items []config.DiscoveredModel) []config.DiscoveredModel {
	out := make([]config.DiscoveredModel, len(items))
	copy(out, items)
	return out
}

func randomToken() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "local"
	}
	return hex.EncodeToString(b[:])
}
