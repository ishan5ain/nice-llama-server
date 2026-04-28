package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoadUsersValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "valid.json"), `{
  "users": [
    {
      "name": "alice",
      "api_key": "secret",
      "rpm": 30,
      "max_tokens": 512,
      "allowed_models": ["  model-a  ", "model-a", "model-b"]
    }
  ]
}`)
	users, err := LoadUsers(filepath.Join(dir, "valid.json"))
	if err != nil {
		t.Fatalf("LoadUsers returned error: %v", err)
	}
	if len(users) != 1 || len(users[0].AllowedModels) != 2 {
		t.Fatalf("unexpected users: %#v", users)
	}

	tests := map[string]string{
		"empty": `{"users":[]}`,
		"duplicate_name": `{"users":[
			{"name":"a","api_key":"1","rpm":1,"max_tokens":1,"allowed_models":["m"]},
			{"name":"a","api_key":"2","rpm":1,"max_tokens":1,"allowed_models":["m"]}
		]}`,
		"duplicate_key": `{"users":[
			{"name":"a","api_key":"1","rpm":1,"max_tokens":1,"allowed_models":["m"]},
			{"name":"b","api_key":"1","rpm":1,"max_tokens":1,"allowed_models":["m"]}
		]}`,
		"bad_rpm":        `{"users":[{"name":"a","api_key":"1","rpm":0,"max_tokens":1,"allowed_models":["m"]}]}`,
		"bad_max_tokens": `{"users":[{"name":"a","api_key":"1","rpm":1,"max_tokens":0,"allowed_models":["m"]}]}`,
		"empty_models":   `{"users":[{"name":"a","api_key":"1","rpm":1,"max_tokens":1,"allowed_models":[" ",""]}]}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".json")
			writeFile(t, path, body)
			if _, err := LoadUsers(path); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestNewServiceRejectsBadUpstream(t *testing.T) {
	t.Parallel()

	_, err := NewService(Options{
		ListenAddr:   "127.0.0.1:0",
		UpstreamURL:  "://bad",
		APIKeysPath:  writeAPIKeys(t, []string{"allowed"}),
		UsageLogPath: filepath.Join(t.TempDir(), "usage.jsonl"),
	})
	if err == nil {
		t.Fatalf("expected upstream validation error")
	}
}

func TestHandleModelsFiltersAllowedModels(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"allowed"},{"id":"blocked"}]}`))
	}))
	defer upstream.Close()

	svc := newTestService(t, upstream.URL, false)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if strings.Contains(resp.Body.String(), "blocked") {
		t.Fatalf("blocked model leaked: %s", resp.Body.String())
	}
	assertTrailingNewline(t, resp.Body.Bytes())
}

func TestHandleModelsWildcardAllowsAllModels(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"allowed"},{"id":"blocked"}]}`))
	}))
	defer upstream.Close()

	svc := newTestServiceWithModels(t, upstream.URL, false, []string{"*"})
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "allowed") || !strings.Contains(resp.Body.String(), "blocked") {
		t.Fatalf("wildcard did not preserve all models: %s", resp.Body.String())
	}
	assertTrailingNewline(t, resp.Body.Bytes())
}

func TestHandleModelsMalformedUpstream(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":"bad"}`))
	}))
	defer upstream.Close()

	svc := newTestService(t, upstream.URL, false)
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
}

func TestProxyRejectsUnauthorizedAndNonTailscale(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	svc := newTestService(t, upstream.URL, true)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "192.168.1.10:1234"
	resp := httptest.NewRecorder()
	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	assertTrailingNewline(t, resp.Body.Bytes())

	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.RemoteAddr = "[fd7a:115c:a1e0::1]:1234"
	resp = httptest.NewRecorder()
	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	assertTrailingNewline(t, resp.Body.Bytes())
}

func TestHandleHealthAddsTrailingNewline(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	svc := newTestService(t, upstream.URL, false)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()

	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if got := resp.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content type: %q", got)
	}
	assertTrailingNewline(t, resp.Body.Bytes())
}

func TestChatCompletionsBufferedAndUsageLogging(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("client auth leaked upstream: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30}}`))
	}))
	defer upstream.Close()

	svc, logPath := newTestServiceWithLogPath(t, upstream.URL, false)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"allowed","max_tokens":12}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}

	records := readUsageRecords(t, logPath)
	if len(records) != 1 {
		t.Fatalf("unexpected log count: %d", len(records))
	}
	if records[0].PromptTokens == nil || *records[0].PromptTokens != 10 {
		t.Fatalf("missing prompt tokens: %#v", records[0])
	}
	if records[0].RequestedMaxTokens == nil || *records[0].RequestedMaxTokens != 12 {
		t.Fatalf("missing requested max tokens: %#v", records[0])
	}
}

func TestChatCompletionsStreamingPassthrough(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"delta\":\"a\"}\n\n"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	svc, logPath := newTestServiceWithLogPath(t, upstream.URL, false)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"allowed","stream":true,"max_tokens":5}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "[DONE]") {
		t.Fatalf("missing done marker: %q", resp.Body.String())
	}

	records := readUsageRecords(t, logPath)
	if len(records) != 1 || !records[0].StreamDone || records[0].StreamEvents < 2 {
		t.Fatalf("unexpected stream log record: %#v", records)
	}
}

func TestChatCompletionsValidationAndRateLimit(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer upstream.Close()

	svc := newTestService(t, upstream.URL, false)
	svc.limiter = newRateLimiter()
	frozen := time.Now()
	svc.limiter.now = func() time.Time { return frozen }

	tests := []struct {
		name   string
		body   string
		status int
	}{
		{"missing_model", `{"max_tokens":1}`, http.StatusBadRequest},
		{"missing_budget", `{"model":"allowed"}`, http.StatusBadRequest},
		{"disallowed_model", `{"model":"blocked","max_tokens":1}`, http.StatusForbidden},
		{"too_large", `{"model":"allowed","max_tokens":1,"messages":"` + strings.Repeat("x", int(maxRequestBodyBytes)) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer secret")
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			svc.server.Handler.ServeHTTP(resp, req)
			if resp.Code != tt.status {
				t.Fatalf("unexpected status: got %d want %d body=%s", resp.Code, tt.status, resp.Body.String())
			}
		})
	}

	okReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"allowed","max_tokens":1}`))
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	resp := httptest.NewRecorder()
	svc.server.Handler.ServeHTTP(resp, okReq())
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	resp = httptest.NewRecorder()
	svc.server.Handler.ServeHTTP(resp, okReq())
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: %d", resp.Code)
	}
	if calls.Load() != 1 {
		t.Fatalf("unexpected upstream calls: %d", calls.Load())
	}
}

func TestChatCompletionsWildcardAllowsAnyModel(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer upstream.Close()

	svc := newTestServiceWithModels(t, upstream.URL, false, []string{"*"})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"some-new-model","max_tokens":1}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	svc.server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", resp.Code, resp.Body.String())
	}
}

func TestChatCompletionsPassthroughAndCancellation(t *testing.T) {
	t.Parallel()

	t.Run("upstream_error_passthrough", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "upstream bad request", http.StatusBadRequest)
		}))
		defer upstream.Close()

		svc := newTestService(t, upstream.URL, false)
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"allowed","max_tokens":1}`))
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		svc.server.Handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
	})

	t.Run("request_cancellation", func(t *testing.T) {
		cancelled := make(chan struct{}, 1)
		svc := newTestService(t, "http://127.0.0.1:8080", false)
		svc.client = &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				<-req.Context().Done()
				cancelled <- struct{}{}
				return nil, req.Context().Err()
			}),
		}
		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"allowed","max_tokens":1}`)).WithContext(ctx)
		req.Header.Set("Authorization", "Bearer secret")
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()

		done := make(chan struct{})
		go func() {
			svc.server.Handler.ServeHTTP(resp, req)
			close(done)
		}()
		time.Sleep(10 * time.Millisecond)
		cancel()

		select {
		case <-cancelled:
		case <-time.After(2 * time.Second):
			t.Fatalf("upstream request was not cancelled")
		}
		<-done
		if resp.Code != http.StatusBadGateway {
			t.Fatalf("unexpected status: %d", resp.Code)
		}
	})
}

func TestUsageLoggerConcurrentWrites(t *testing.T) {
	t.Parallel()

	logPath := filepath.Join(t.TempDir(), "usage.jsonl")
	logger, err := newUsageLogger(logPath)
	if err != nil {
		t.Fatalf("newUsageLogger returned error: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := logger.Log(UsageRecord{
				TS:         time.Now().UTC(),
				RequestID:  strconv.Itoa(i),
				User:       "alice",
				Method:     http.MethodPost,
				Path:       "/v1/chat/completions",
				StatusCode: http.StatusOK,
			}); err != nil {
				t.Errorf("Log returned error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	records := readUsageRecords(t, logPath)
	if len(records) != 25 {
		t.Fatalf("unexpected log count: %d", len(records))
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestService(t *testing.T, upstreamURL string, tailscaleOnly bool) *Service {
	t.Helper()
	svc, _ := newTestServiceWithLogPath(t, upstreamURL, tailscaleOnly)
	return svc
}

func newTestServiceWithLogPath(t *testing.T, upstreamURL string, tailscaleOnly bool) (*Service, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "usage.jsonl")
	svc, err := NewService(Options{
		ListenAddr:    "127.0.0.1:0",
		UpstreamURL:   upstreamURL,
		APIKeysPath:   writeAPIKeys(t, []string{"allowed"}),
		UsageLogPath:  logPath,
		TailscaleOnly: tailscaleOnly,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = svc.Close()
	})
	return svc, logPath
}

func newTestServiceWithModels(t *testing.T, upstreamURL string, tailscaleOnly bool, models []string) *Service {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "usage.jsonl")
	svc, err := NewService(Options{
		ListenAddr:    "127.0.0.1:0",
		UpstreamURL:   upstreamURL,
		APIKeysPath:   writeAPIKeys(t, models),
		UsageLogPath:  logPath,
		TailscaleOnly: tailscaleOnly,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = svc.Close()
	})
	return svc
}

func writeAPIKeys(t *testing.T, models []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "api-keys.json")
	rawModels, err := json.Marshal(models)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	writeFile(t, path, `{
  "users": [
    {
      "name": "alice",
      "api_key": "secret",
      "rpm": 1,
      "max_tokens": 32,
      "allowed_models": `+string(rawModels)+`
    }
  ]
}`)
	return path
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func readUsageRecords(t *testing.T, path string) []UsageRecord {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer f.Close()

	var records []UsageRecord
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var record UsageRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("invalid log line: %v", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Scanner returned error: %v", err)
	}
	return records
}

func assertTrailingNewline(t *testing.T, body []byte) {
	t.Helper()
	if len(body) == 0 {
		t.Fatalf("response body is empty")
	}
	if body[len(body)-1] != '\n' {
		t.Fatalf("response missing trailing newline: %q", string(body))
	}
}
