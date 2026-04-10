package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const stateVersion = 1

type Store struct {
	dir  string
	file string
	lock string
}

func ResolveStateDir(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		return filepath.Join(base, "nice-llama-server"), nil
	}

	return filepath.Join(base, "nice-llama-server"), nil
}

func NewStore(stateDir string) (*Store, error) {
	dir, err := ResolveStateDir(stateDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{
		dir:  dir,
		file: filepath.Join(dir, "state.json"),
		lock: filepath.Join(dir, "controller.json"),
	}, nil
}

func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) ControllerPath() string {
	return s.lock
}

func (s *Store) Load() (StoredState, error) {
	raw, err := os.ReadFile(s.file)
	if errors.Is(err, os.ErrNotExist) {
		return defaultStoredState(), nil
	}
	if err != nil {
		return StoredState{}, err
	}

	var state StoredState
	if err := json.Unmarshal(raw, &state); err != nil {
		return StoredState{}, err
	}
	if state.Version == 0 {
		state.Version = stateVersion
	}
	if state.Config.LlamaServerBin == "" {
		state.Config.LlamaServerBin = "llama-server"
	}
	state.Config.ModelRoots = normalizeRoots(state.Config.ModelRoots)
	s.normalizeBookmarks(&state)
	return state, nil
}

func (s *Store) Save(state StoredState) error {
	state.Version = stateVersion
	state.UpdatedAt = time.Now().UTC()
	if state.Config.LlamaServerBin == "" {
		state.Config.LlamaServerBin = "llama-server"
	}
	state.Config.ModelRoots = normalizeRoots(state.Config.ModelRoots)
	s.normalizeBookmarks(&state)

	tmp := s.file + ".tmp"
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.file)
}

func (s *Store) LoadControllerInfo() (ControllerInfo, error) {
	raw, err := os.ReadFile(s.lock)
	if err != nil {
		return ControllerInfo{}, err
	}
	var info ControllerInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return ControllerInfo{}, err
	}
	return info, nil
}

func (s *Store) SaveControllerInfo(info ControllerInfo) error {
	tmp := s.lock + ".tmp"
	body, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.lock)
}

func (s *Store) RemoveControllerInfo() error {
	if err := os.Remove(s.lock); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func defaultStoredState() StoredState {
	return StoredState{
		Version: stateVersion,
		Config: Config{
			LlamaServerBin: "llama-server",
			ModelRoots:     nil,
		},
		Bookmarks: nil,
		UpdatedAt: time.Now().UTC(),
	}
}

func normalizeRoots(roots []string) []string {
	if len(roots) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		abs, err := filepath.Abs(root)
		if err == nil {
			root = abs
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		out = append(out, root)
	}
	slices.Sort(out)
	return out
}

func (s *Store) normalizeBookmarks(state *StoredState) {
	for i := range state.Bookmarks {
		if state.Bookmarks[i].ID == "" {
			continue
		}
		if state.Bookmarks[i].CreatedAt.IsZero() {
			state.Bookmarks[i].CreatedAt = time.Now().UTC()
		}
		if state.Bookmarks[i].UpdatedAt.IsZero() {
			state.Bookmarks[i].UpdatedAt = state.Bookmarks[i].CreatedAt
		}
		if state.Bookmarks[i].GroupKey == "" {
			state.Bookmarks[i].GroupKey = DeriveGroupKey(state.Bookmarks[i].ModelPath)
		}
	}
}

func DeriveGroupKey(modelPath string) string {
	name := filepath.Base(strings.TrimSpace(modelPath))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	for {
		if len(name) < 12 {
			break
		}
		idx := strings.LastIndex(name, "-")
		if idx < 0 {
			break
		}
		suffix := name[idx+1:]
		if len(suffix) != 5 || !isDigits(suffix) {
			break
		}
		prefix := name[:idx]
		ofIdx := strings.LastIndex(prefix, "-of-")
		if ofIdx < 0 {
			break
		}
		total := prefix[ofIdx+4:]
		if len(total) != 5 || !isDigits(total) {
			break
		}
		name = prefix[:ofIdx]
		break
	}
	if name == "" {
		return "Ungrouped"
	}
	return name
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
