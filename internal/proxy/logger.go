package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type UsageRecord struct {
	TS                 time.Time `json:"ts"`
	RequestID          string    `json:"request_id"`
	User               string    `json:"user"`
	RemoteIP           string    `json:"remote_ip"`
	Method             string    `json:"method"`
	Path               string    `json:"path"`
	Model              string    `json:"model,omitempty"`
	Stream             bool      `json:"stream"`
	RequestedMaxTokens *int      `json:"requested_max_tokens,omitempty"`
	StatusCode         int       `json:"status_code"`
	LatencyMS          int64     `json:"latency_ms"`
	PromptTokens       *int      `json:"prompt_tokens,omitempty"`
	CompletionTokens   *int      `json:"completion_tokens,omitempty"`
	TotalTokens        *int      `json:"total_tokens,omitempty"`
	StreamEvents       int       `json:"stream_events,omitempty"`
	StreamBytes        int64     `json:"stream_bytes,omitempty"`
	StreamDone         bool      `json:"stream_done,omitempty"`
	Error              string    `json:"error,omitempty"`
}

type usageLogger struct {
	mu   sync.Mutex
	file *os.File
}

func newUsageLogger(path string) (*usageLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &usageLogger{file: f}, nil
}

func (l *usageLogger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

func (l *usageLogger) Log(record UsageRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = l.file.Write(body)
	return err
}
