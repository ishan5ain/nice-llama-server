package config

import "time"

const (
	StatusIdle     = "idle"
	StatusLoading  = "loading"
	StatusReady    = "ready"
	StatusStopping = "stopping"
	StatusFailed   = "failed"

	ControllerPingTimeout     = 1200 * time.Millisecond
	ControllerReadyTimeout    = 10 * time.Second
	ControllerPingInterval    = 250 * time.Millisecond
	ControllerMaxPingInterval = 2 * time.Second
)

type Config struct {
	LlamaServerBin string   `json:"llama_server_bin"`
	ModelRoots     []string `json:"model_roots"`
}

func (c Config) Snapshot() Config {
	out := c
	if c.ModelRoots != nil {
		out.ModelRoots = make([]string, len(c.ModelRoots))
		copy(out.ModelRoots, c.ModelRoots)
	}
	return out
}

type Bookmark struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ModelPath string    `json:"model_path"`
	ArgsText  string    `json:"args_text"`
	GroupKey  string    `json:"group_key"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DiscoveredModel struct {
	Path        string `json:"path"`
	DisplayName string `json:"display_name"`
	GroupKey    string `json:"group_key"`
	Root        string `json:"root"`
	MMProjPaths []string `json:"mmproj_paths,omitempty"`
}

type RuntimeState struct {
	Status           string     `json:"status"`
	ActiveBookmarkID string     `json:"active_bookmark_id,omitempty"`
	PID              int        `json:"pid,omitempty"`
	Host             string     `json:"host,omitempty"`
	Port             int        `json:"port,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	ExitCode         *int       `json:"exit_code,omitempty"`
	Error            string     `json:"error,omitempty"`
}

type LogEntry struct {
	Seq    int64     `json:"seq"`
	TS     time.Time `json:"ts"`
	Stream string    `json:"stream"`
	Line   string    `json:"line"`
}

type StoredState struct {
	Config    Config     `json:"config"`
	Bookmarks []Bookmark `json:"bookmarks"`
	UpdatedAt time.Time  `json:"updated_at"`
	Version   int        `json:"version"`
}

type Snapshot struct {
	Config    Config            `json:"config"`
	Bookmarks []Bookmark        `json:"bookmarks"`
	Models    []DiscoveredModel `json:"models"`
	Runtime   RuntimeState      `json:"runtime"`
}

type ControllerInfo struct {
	PID       int       `json:"pid"`
	URL       string    `json:"url"`
	Token     string    `json:"token"`
	StartedAt time.Time `json:"started_at"`
}
