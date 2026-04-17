package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"nice-llama-server/internal/config"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) State(ctx context.Context) (config.Snapshot, error) {
	var out config.Snapshot
	err := c.do(ctx, http.MethodGet, "/v1/state", nil, &out)
	return out, err
}

func (c *Client) Rescan(ctx context.Context, modelRoots []string, llamaServerBin *string) (config.Snapshot, error) {
	var out config.Snapshot
	body := map[string]any{}
	if modelRoots != nil {
		body["model_roots"] = modelRoots
	}
	if llamaServerBin != nil {
		body["llama_server_bin"] = *llamaServerBin
	}
	err := c.do(ctx, http.MethodPost, "/v1/rescan", body, &out)
	return out, err
}

func (c *Client) CreateBookmark(ctx context.Context, b config.Bookmark) (config.Bookmark, error) {
	var out config.Bookmark
	err := c.do(ctx, http.MethodPost, "/v1/bookmarks", b, &out)
	return out, err
}

func (c *Client) UpdateBookmark(ctx context.Context, b config.Bookmark) (config.Bookmark, error) {
	var out config.Bookmark
	err := c.do(ctx, http.MethodPut, "/v1/bookmarks/"+b.ID, b, &out)
	return out, err
}

func (c *Client) DeleteBookmark(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/bookmarks/"+id, nil, nil)
}

func (c *Client) Load(ctx context.Context, bookmarkID string) (config.RuntimeState, error) {
	var out config.RuntimeState
	err := c.do(ctx, http.MethodPost, "/v1/runtime/load", map[string]string{"bookmark_id": bookmarkID}, &out)
	return out, err
}

func (c *Client) Unload(ctx context.Context) (config.RuntimeState, error) {
	var out config.RuntimeState
	err := c.do(ctx, http.MethodPost, "/v1/runtime/unload", map[string]string{}, &out)
	return out, err
}

func (c *Client) Logs(ctx context.Context, after int64) ([]config.LogEntry, error) {
	var out []config.LogEntry
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/logs?after=%d", after), nil, &out)
	return out, err
}

func (c *Client) DoWithRetry(ctx context.Context, method, path string, body any, out any, retries int) error {
	return c.doWithRetry(ctx, method, path, body, out, retries)
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, body any, out any, retries int) error {
	var lastErr error
	delay := 100 * time.Millisecond

	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay = time.Duration(delay.Milliseconds()*2) * time.Millisecond
			if delay > 2*time.Second {
				delay = 2 * time.Second
			}
		}

		lastErr = c.do(ctx, method, path, body, out)
		if lastErr == nil {
			return nil
		}
		if !isTransientError(lastErr) {
			return lastErr
		}
	}
	return lastErr
}

func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	var opErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	if errors.As(err, &opErr) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, payload)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil && payload["error"] != "" {
			return fmt.Errorf("%s", payload["error"])
		}
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
