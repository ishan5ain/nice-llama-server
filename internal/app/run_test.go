package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nice-llama-server/internal/config"
)

func TestParseArgsTUIControllerFlags(t *testing.T) {
	t.Parallel()

	opts, err := parseArgs([]string{
		"--state-dir", "/tmp/nls",
		"--controller-url", "http://127.0.0.1:51234",
		"--controller-token", "secret-token",
		"/models",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if opts.controllerURL != "http://127.0.0.1:51234" {
		t.Fatalf("unexpected controller URL: %q", opts.controllerURL)
	}
	if opts.controllerToken != "secret-token" {
		t.Fatalf("unexpected controller token: %q", opts.controllerToken)
	}
	if opts.printControllerInfo {
		t.Fatalf("printControllerInfo should be false in TUI mode")
	}
}

func TestParseArgsControllerPrintInfo(t *testing.T) {
	t.Parallel()

	opts, err := parseArgs([]string{
		"controller",
		"--print-controller-info",
		"--llama-server-bin", "/bin/llama-server",
		"/models",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if opts.mode != "controller" {
		t.Fatalf("unexpected mode: %q", opts.mode)
	}
	if !opts.printControllerInfo {
		t.Fatalf("expected printControllerInfo to be true")
	}
	if opts.controllerToken != "" {
		t.Fatalf("controller token should be empty in controller mode")
	}
}

func TestParseArgsProxyMode(t *testing.T) {
	t.Parallel()

	opts, err := parseArgs([]string{
		"proxy",
		"--listen", "100.64.0.1:8088",
		"--upstream", "http://127.0.0.1:8080/api",
		"--api-keys", "/tmp/proxy.keys.json",
		"--usage-log", "/tmp/usage.jsonl",
		"--tailscale-only",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	if opts.mode != "proxy" {
		t.Fatalf("unexpected mode: %q", opts.mode)
	}
	if opts.proxyListen != "100.64.0.1:8088" || opts.proxyUpstream != "http://127.0.0.1:8080/api" {
		t.Fatalf("unexpected proxy args: %#v", opts)
	}
	if !opts.proxyTailscaleOnly {
		t.Fatalf("expected tailscale-only to be set")
	}
}

func TestParseArgsProxyModeRequiresFlags(t *testing.T) {
	t.Parallel()

	_, err := parseArgs([]string{"proxy", "--listen", "127.0.0.1:8088"})
	if err == nil {
		t.Fatalf("expected missing flag error")
	}
	if !strings.Contains(err.Error(), "--upstream") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseArgsProxyModeRejectsModelRoots(t *testing.T) {
	t.Parallel()

	_, err := parseArgs([]string{
		"proxy",
		"--listen", "127.0.0.1:8088",
		"--upstream", "http://127.0.0.1:8080",
		"--api-keys", "/tmp/proxy.keys.json",
		"--usage-log", "/tmp/usage.jsonl",
		"/models",
	})
	if err == nil {
		t.Fatalf("expected proxy model root rejection")
	}
}

func TestResolveControllerInfoUsesExplicitToken(t *testing.T) {
	t.Parallel()

	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}

	info, err := resolveControllerInfo(store, cliOptions{
		controllerURL:   "http://127.0.0.1:51234",
		controllerToken: "secret-token",
	})
	if err != nil {
		t.Fatalf("resolveControllerInfo returned error: %v", err)
	}
	if info.Token != "secret-token" {
		t.Fatalf("unexpected token: %q", info.Token)
	}
}

func TestResolveControllerInfoUsesMatchingLocalControllerFile(t *testing.T) {
	t.Parallel()

	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	err = store.SaveControllerInfo(config.ControllerInfo{
		URL:       "http://127.0.0.1:51234",
		Token:     "stored-token",
		PID:       1234,
		StartedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("SaveControllerInfo returned error: %v", err)
	}

	info, err := resolveControllerInfo(store, cliOptions{
		controllerURL: "http://127.0.0.1:51234",
	})
	if err != nil {
		t.Fatalf("resolveControllerInfo returned error: %v", err)
	}
	if info.Token != "stored-token" {
		t.Fatalf("unexpected token: %q", info.Token)
	}
}

func TestResolveControllerInfoErrorsWithoutUsableToken(t *testing.T) {
	t.Parallel()

	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}

	_, err = resolveControllerInfo(store, cliOptions{
		controllerURL: "http://127.0.0.1:51234",
	})
	if err == nil {
		t.Fatalf("expected missing token error")
	}
	if !strings.Contains(err.Error(), "--controller-token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintControllerInfoFormat(t *testing.T) {
	t.Parallel()

	info := config.ControllerInfo{
		URL:   "http://127.0.0.1:51234",
		Token: "secret-token",
	}
	got := controllerInfoLine(info)
	want := "url=http://127.0.0.1:51234 token=secret-token\n"
	if got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestResolveControllerInfoIgnoresMismatchedStoredURL(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	store, err := config.NewStore(tempDir)
	if err != nil {
		t.Fatalf("NewStore returned error: %v", err)
	}
	err = os.WriteFile(filepath.Join(tempDir, "controller.json"), []byte("{\n  \"url\": \"http://127.0.0.1:9999\",\n  \"token\": \"stored-token\"\n}\n"), 0o644)
	if err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err = resolveControllerInfo(store, cliOptions{
		controllerURL: "http://127.0.0.1:51234",
	})
	if err == nil {
		t.Fatalf("expected error for mismatched stored URL")
	}
}
