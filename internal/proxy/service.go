package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const maxRequestBodyBytes int64 = 1 << 20

var (
	tailscaleIPv4Prefix = netipMustParsePrefix("100.64.0.0/10")
	tailscaleIPv6Prefix = netipMustParsePrefix("fd7a:115c:a1e0::/48")
)

type Options struct {
	ListenAddr    string
	UpstreamURL   string
	APIKeysPath   string
	UsageLogPath  string
	TailscaleOnly bool
}

type Service struct {
	upstream      *url.URL
	client        *http.Client
	usersByKey    map[string]User
	usersByName   map[string]User
	usageLogger   *usageLogger
	limiter       *rateLimiter
	tailscaleOnly bool
	server        *http.Server
}

func Serve(ctx context.Context, opts Options) error {
	svc, err := NewService(opts)
	if err != nil {
		return err
	}
	defer svc.Close()

	ln, err := net.Listen("tcp", opts.ListenAddr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		err := svc.server.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = svc.server.Shutdown(shutdownCtx)
		return <-errCh
	}
}

func NewService(opts Options) (*Service, error) {
	upstream, err := normalizeUpstreamURL(opts.UpstreamURL)
	if err != nil {
		return nil, err
	}
	users, err := LoadUsers(opts.APIKeysPath)
	if err != nil {
		return nil, err
	}
	logger, err := newUsageLogger(opts.UsageLogPath)
	if err != nil {
		return nil, err
	}

	svc := &Service{
		upstream: upstream,
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
		usersByKey:    make(map[string]User, len(users)),
		usersByName:   make(map[string]User, len(users)),
		usageLogger:   logger,
		limiter:       newRateLimiter(),
		tailscaleOnly: opts.TailscaleOnly,
	}
	for _, user := range users {
		svc.usersByKey[user.APIKey] = user
		svc.usersByName[user.Name] = user
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", svc.withBaseGuards(svc.handleHealth))
	mux.HandleFunc("/v1/models", svc.withBaseGuards(svc.withAuth(svc.handleModels)))
	mux.HandleFunc("/v1/chat/completions", svc.withBaseGuards(svc.withAuth(svc.handleChatCompletions)))
	svc.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return svc, nil
}

func (s *Service) Close() error {
	if s.usageLogger != nil {
		return s.usageLogger.Close()
	}
	return nil
}

func (s *Service) withBaseGuards(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.tailscaleOnly {
			ip, err := remoteIP(r.RemoteAddr)
			if err != nil || !isTailscaleIP(ip) {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}
		next(w, r)
	}
}

type contextKey string

const userContextKey contextKey = "proxy-user"

func (s *Service) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.authenticate(r.Header.Get("Authorization"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	}
}

func (s *Service) authenticate(header string) (User, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return User{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	user, ok := s.usersByKey[token]
	return user, ok
}

func userFromContext(ctx context.Context) User {
	user, _ := ctx.Value(userContextKey).(User)
	return user
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.upstreamURL("/health"), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "upstream health check failed")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusServiceUnavailable, "upstream health check failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Service) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := userFromContext(r.Context())
	resp, body, err := s.forwardBuffered(r.Context(), http.MethodGet, "/v1/models", nil, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
		return
	}

	filtered, err := filterModelsResponse(body, user)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, filtered)
}

func (s *Service) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	user := userFromContext(r.Context())
	record := UsageRecord{
		TS:        start.UTC(),
		RequestID: randomID(),
		User:      user.Name,
		RemoteIP:  remoteIPString(r.RemoteAddr),
		Method:    r.Method,
		Path:      r.URL.Path,
	}
	defer func() {
		record.LatencyMS = time.Since(start).Milliseconds()
		_ = s.usageLogger.Log(record)
	}()

	if r.Method != http.MethodPost {
		record.StatusCode = http.StatusMethodNotAllowed
		record.Error = "method not allowed"
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, body, err := parseChatCompletionRequest(r)
	if err != nil {
		record.StatusCode = statusCodeForRequestError(err)
		record.Error = err.Error()
		writeError(w, record.StatusCode, err.Error())
		return
	}
	record.Model = payload.Model
	record.Stream = payload.Stream
	record.RequestedMaxTokens = payload.RequestedMaxTokens

	if !modelAllowed(user, payload.Model) {
		record.StatusCode = http.StatusForbidden
		record.Error = "model not allowed"
		writeError(w, http.StatusForbidden, "model not allowed")
		return
	}
	if err := validateTokenBudget(user, payload); err != nil {
		record.StatusCode = http.StatusBadRequest
		record.Error = err.Error()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.limiter.Allow(user.Name, user.RPM) {
		record.StatusCode = http.StatusTooManyRequests
		record.Error = "rate limit exceeded"
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.upstreamURL("/v1/chat/completions"), bytes.NewReader(body))
	if err != nil {
		record.StatusCode = http.StatusBadGateway
		record.Error = err.Error()
		writeError(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	copyForwardHeaders(req.Header, r.Header)

	resp, err := s.client.Do(req)
	if err != nil {
		record.StatusCode = http.StatusBadGateway
		record.Error = err.Error()
		writeError(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer resp.Body.Close()

	if payload.Stream {
		s.proxyStreamResponse(w, resp, &record)
		return
	}
	s.proxyBufferedResponse(w, resp, &record)
}

func (s *Service) proxyBufferedResponse(w http.ResponseWriter, resp *http.Response, record *UsageRecord) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		record.StatusCode = http.StatusBadGateway
		record.Error = err.Error()
		writeError(w, http.StatusBadGateway, "failed to read upstream response")
		return
	}

	record.StatusCode = resp.StatusCode
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		extractUsage(body, record)
		return
	}
	record.Error = strings.TrimSpace(string(body))
}

func (s *Service) proxyStreamResponse(w http.ResponseWriter, resp *http.Response, record *UsageRecord) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		record.StatusCode = http.StatusInternalServerError
		record.Error = "streaming unsupported"
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	record.StatusCode = resp.StatusCode
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			record.StreamBytes += int64(len(line))
			if bytes.HasPrefix(line, []byte("data:")) {
				record.StreamEvents++
				if bytes.Contains(line, []byte("[DONE]")) {
					record.StreamDone = true
				}
			}
			_, writeErr := w.Write(line)
			if writeErr != nil {
				record.Error = writeErr.Error()
				return
			}
			flusher.Flush()
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			record.Error = err.Error()
			return
		}
	}
}

func (s *Service) forwardBuffered(ctx context.Context, method, reqPath string, body []byte, headers http.Header) (*http.Response, []byte, error) {
	var payload io.Reader
	if body != nil {
		payload = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.upstreamURL(reqPath), payload)
	if err != nil {
		return nil, nil, err
	}
	copyHeaders(req.Header, headers)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	return resp, raw, nil
}

type chatCompletionRequest struct {
	Model               string
	Stream              bool
	MaxTokens           *int
	MaxCompletionTokens *int
	RequestedMaxTokens  *int
}

func parseChatCompletionRequest(r *http.Request) (chatCompletionRequest, []byte, error) {
	if r.Body == nil {
		return chatCompletionRequest{}, nil, errors.New("request body is required")
	}
	defer r.Body.Close()

	limited := io.LimitReader(r.Body, maxRequestBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return chatCompletionRequest{}, nil, err
	}
	if int64(len(body)) > maxRequestBodyBytes {
		return chatCompletionRequest{}, nil, errRequestTooLarge
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	var payload map[string]any
	if err := dec.Decode(&payload); err != nil {
		return chatCompletionRequest{}, nil, errors.New("request body must be a JSON object")
	}

	model, _ := payload["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		return chatCompletionRequest{}, nil, errors.New("model is required")
	}

	stream := false
	if raw, ok := payload["stream"]; ok {
		value, ok := raw.(bool)
		if !ok {
			return chatCompletionRequest{}, nil, errors.New("stream must be a boolean")
		}
		stream = value
	}

	maxTokens, err := optionalInt(payload["max_tokens"])
	if err != nil {
		return chatCompletionRequest{}, nil, fmt.Errorf("max_tokens: %w", err)
	}
	maxCompletionTokens, err := optionalInt(payload["max_completion_tokens"])
	if err != nil {
		return chatCompletionRequest{}, nil, fmt.Errorf("max_completion_tokens: %w", err)
	}

	requested := maxOfPointers(maxTokens, maxCompletionTokens)
	return chatCompletionRequest{
		Model:               model,
		Stream:              stream,
		MaxTokens:           maxTokens,
		MaxCompletionTokens: maxCompletionTokens,
		RequestedMaxTokens:  requested,
	}, body, nil
}

var errRequestTooLarge = errors.New("request body too large")

func statusCodeForRequestError(err error) int {
	if errors.Is(err, errRequestTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func validateTokenBudget(user User, req chatCompletionRequest) error {
	if req.MaxTokens == nil && req.MaxCompletionTokens == nil {
		return errors.New("max_tokens or max_completion_tokens is required")
	}
	if req.MaxTokens != nil {
		if *req.MaxTokens <= 0 {
			return errors.New("max_tokens must be positive")
		}
		if *req.MaxTokens > user.MaxTokens {
			return fmt.Errorf("max_tokens exceeds limit of %d", user.MaxTokens)
		}
	}
	if req.MaxCompletionTokens != nil {
		if *req.MaxCompletionTokens <= 0 {
			return errors.New("max_completion_tokens must be positive")
		}
		if *req.MaxCompletionTokens > user.MaxTokens {
			return fmt.Errorf("max_completion_tokens exceeds limit of %d", user.MaxTokens)
		}
	}
	return nil
}

func modelAllowed(user User, model string) bool {
	for _, allowed := range user.AllowedModels {
		if allowed == "*" || allowed == model {
			return true
		}
	}
	return false
}

func filterModelsResponse(body []byte, user User) (map[string]any, error) {
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, errors.New("malformed upstream /v1/models response")
	}

	rawData, ok := payload["data"].([]any)
	if !ok {
		return nil, errors.New("malformed upstream /v1/models response")
	}

	filtered := make([]any, 0, len(rawData))
	for _, item := range rawData {
		model, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("malformed upstream /v1/models response")
		}
		id, _ := model["id"].(string)
		if modelAllowed(user, id) {
			filtered = append(filtered, model)
		}
	}
	payload["data"] = filtered
	return payload, nil
}

func extractUsage(body []byte, record *UsageRecord) {
	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return
	}
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return
	}
	record.PromptTokens = parseUsageInt(usage["prompt_tokens"])
	record.CompletionTokens = parseUsageInt(usage["completion_tokens"])
	record.TotalTokens = parseUsageInt(usage["total_tokens"])
}

func parseUsageInt(value any) *int {
	num, err := optionalInt(value)
	if err != nil {
		return nil
	}
	return num
}

func optionalInt(value any) (*int, error) {
	if value == nil {
		return nil, nil
	}
	switch n := value.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return nil, errors.New("must be an integer")
		}
		value := int(i)
		return &value, nil
	case float64:
		value := int(n)
		if float64(value) != n {
			return nil, errors.New("must be an integer")
		}
		return &value, nil
	case int:
		value := n
		return &value, nil
	default:
		return nil, errors.New("must be an integer")
	}
}

func copyForwardHeaders(dst, src http.Header) {
	forward := []string{"Accept", "Content-Type", "User-Agent"}
	for _, key := range forward {
		for _, value := range src.Values(key) {
			dst.Add(key, value)
		}
	}
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func normalizeUpstreamURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("upstream URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("upstream URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("upstream URL host is required")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("upstream URL must not include query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed, nil
}

func (s *Service) upstreamURL(reqPath string) string {
	upstream := *s.upstream
	basePath := strings.TrimRight(upstream.Path, "/")
	upstream.Path = path.Clean(strings.TrimRight(basePath+"/"+strings.TrimLeft(reqPath, "/"), "/"))
	if !strings.HasPrefix(upstream.Path, "/") {
		upstream.Path = "/" + upstream.Path
	}
	return upstream.String()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func remoteIP(addr string) (net.IP, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, errors.New("invalid remote ip")
	}
	return ip, nil
}

func remoteIPString(addr string) string {
	ip, err := remoteIP(addr)
	if err != nil {
		return ""
	}
	return ip.String()
}

func isTailscaleIP(ip net.IP) bool {
	addr, ok := netipAddrFromIP(ip)
	if !ok {
		return false
	}
	return tailscaleIPv4Prefix.Contains(addr) || tailscaleIPv6Prefix.Contains(addr)
}

func netipMustParsePrefix(raw string) netip.Prefix {
	prefix, err := netip.ParsePrefix(raw)
	if err != nil {
		panic(err)
	}
	return prefix
}

func netipAddrFromIP(ip net.IP) (netip.Addr, bool) {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func maxOfPointers(values ...*int) *int {
	var max *int
	for _, value := range values {
		if value == nil {
			continue
		}
		if max == nil || *value > *max {
			v := *value
			max = &v
		}
	}
	return max
}

func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}
