package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"nice-llama-server/internal/config"
	ctrl "nice-llama-server/internal/controller"
	rt "nice-llama-server/internal/runtime"
)

func TestNewServiceRejectsInvalidModelRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("integration helper wrapper is unix-only in CI/dev")
	}

	bin := helperWrapper(t)
	stateDir := t.TempDir()

	_, err := ctrl.NewService(ctrl.Options{
		StateDir:       stateDir,
		LlamaServerBin: bin,
		ModelRoots:     []string{filepath.Join(t.TempDir(), "does-not-exist")},
	})
	if err == nil {
		t.Fatal("expected error for invalid model root, got nil")
	}

	var target *config.ValidationError
	if !errors.As(err, &target) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if target.Err != config.ErrModelRootNotFound {
		t.Fatalf("wrapped error = %v, want %v", target.Err, config.ErrModelRootNotFound)
	}
}

func TestNewServiceRejectsInvalidLlamaServerBin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("integration helper wrapper is unix-only in CI/dev")
	}

	stateDir := t.TempDir()

	_, err := ctrl.NewService(ctrl.Options{
		StateDir:       stateDir,
		LlamaServerBin: filepath.Join(t.TempDir(), "does-not-exist"),
		ModelRoots:     nil,
	})
	if err == nil {
		t.Fatal("expected error for invalid binary, got nil")
	}

	var target *config.ValidationError
	if !errors.As(err, &target) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if target.Err != config.ErrLlamaServerBinNotFound {
		t.Fatalf("wrapped error = %v, want %v", target.Err, config.ErrLlamaServerBinNotFound)
	}
}

func TestRuntimeStopForceKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("force-kill integration is validated on unix-like dev hosts")
	}

	bin := helperWrapper(t)
	port := reservePort(t)

	proc, err := rt.Start(bin, filepath.Join(t.TempDir(), "model.gguf"), fmt.Sprintf("--port %d\n--ignore-sigint", port), rt.Events{})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	readyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := proc.WaitReady(readyCtx); err != nil {
		t.Fatalf("WaitReady returned error: %v", err)
	}

	if _, err := proc.Stop(500 * time.Millisecond); err != nil && !strings.Contains(err.Error(), "killed") {
		t.Fatalf("Stop returned error: %v", err)
	}
}

func TestControllerLifecycleReconnectAndFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("integration helper wrapper is unix-only in CI/dev")
	}

	bin := helperWrapper(t)
	stateDir := t.TempDir()

	svc, err := ctrl.NewService(ctrl.Options{
		StateDir:       stateDir,
		LlamaServerBin: bin,
		ModelRoots:     nil,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	info, err := svc.Start(listener)
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = svc.Shutdown(ctx)
	})

	client1 := ctrl.NewClient(info.URL, info.Token)
	port := reservePort(t)
	bookmark, err := client1.CreateBookmark(context.Background(), config.Bookmark{
		Name:      "Gemma Test",
		ModelPath: filepath.Join(t.TempDir(), "gemma.gguf"),
		ArgsText: strings.Join([]string{
			fmt.Sprintf("--port %d", port),
			"--startup-delay-ms 1200",
			"--shutdown-delay-ms 1200",
		}, "\n"),
	})
	if err != nil {
		t.Fatalf("CreateBookmark returned error: %v", err)
	}

	loadDone := make(chan error, 1)
	go func() {
		_, err := client1.Load(context.Background(), bookmark.ID)
		loadDone <- err
	}()

	time.Sleep(250 * time.Millisecond)
	loadingState, err := client1.State(context.Background())
	if err != nil {
		t.Fatalf("State while loading returned error: %v", err)
	}
	if loadingState.Runtime.Status != config.StatusLoading {
		t.Fatalf("expected loading state, got %q", loadingState.Runtime.Status)
	}

	if err := <-loadDone; err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	readyState, err := client1.State(context.Background())
	if err != nil {
		t.Fatalf("State after load returned error: %v", err)
	}
	if readyState.Runtime.Status != config.StatusReady {
		t.Fatalf("expected ready state, got %q", readyState.Runtime.Status)
	}

	client2 := ctrl.NewClient(info.URL, info.Token)
	reconnectedState, err := client2.State(context.Background())
	if err != nil {
		t.Fatalf("second client state returned error: %v", err)
	}
	if reconnectedState.Runtime.ActiveBookmarkID != bookmark.ID {
		t.Fatalf("expected active bookmark %q, got %q", bookmark.ID, reconnectedState.Runtime.ActiveBookmarkID)
	}

	time.Sleep(250 * time.Millisecond)
	logs, err := client2.Logs(context.Background(), 0)
	if err != nil {
		t.Fatalf("Logs returned error: %v", err)
	}
	if len(logs) == 0 {
		t.Fatalf("expected log entries after load")
	}

	unloadDone := make(chan error, 1)
	go func() {
		_, err := client2.Unload(context.Background())
		unloadDone <- err
	}()

	time.Sleep(250 * time.Millisecond)
	stoppingState, err := client2.State(context.Background())
	if err != nil {
		t.Fatalf("State while stopping returned error: %v", err)
	}
	if stoppingState.Runtime.Status != config.StatusStopping {
		t.Fatalf("expected stopping state, got %q", stoppingState.Runtime.Status)
	}

	if err := <-unloadDone; err != nil {
		t.Fatalf("Unload returned error: %v", err)
	}
	idleState, err := client2.State(context.Background())
	if err != nil {
		t.Fatalf("State after unload returned error: %v", err)
	}
	if idleState.Runtime.Status != config.StatusIdle {
		t.Fatalf("expected idle state, got %q", idleState.Runtime.Status)
	}

	failPort := reservePort(t)
	failing, err := client2.CreateBookmark(context.Background(), config.Bookmark{
		Name:      "Failing Model",
		ModelPath: filepath.Join(t.TempDir(), "broken.gguf"),
		ArgsText: strings.Join([]string{
			fmt.Sprintf("--port %d", failPort),
			"--fail-start",
		}, "\n"),
	})
	if err != nil {
		t.Fatalf("CreateBookmark for failing model returned error: %v", err)
	}

	if _, err := client2.Load(context.Background(), failing.ID); err == nil {
		t.Fatalf("expected load failure")
	}

	failedState, err := client2.State(context.Background())
	if err != nil {
		t.Fatalf("State after failed load returned error: %v", err)
	}
	if failedState.Runtime.Status != config.StatusFailed {
		t.Fatalf("expected failed state, got %q", failedState.Runtime.Status)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("NICE_LLAMA_SERVER_HELPER") != "1" {
		return
	}

	if err := runHelperServer(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func helperWrapper(t *testing.T) string {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	script := filepath.Join(t.TempDir(), "fake-llama-server")
	body := fmt.Sprintf("#!/bin/sh\nNICE_LLAMA_SERVER_HELPER=1 exec %q -test.run=TestHelperProcess -- \"$@\"\n", exe)
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write helper wrapper: %v", err)
	}
	return script
}

func reservePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func runHelperServer() error {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	args := helperArgs(os.Args)
	host := "127.0.0.1"
	port := 8080
	startupDelay := 0 * time.Millisecond
	shutdownDelay := 0 * time.Millisecond
	ignoreSigint := false
	failStart := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m", "--model":
			i++
		case "--host":
			if i+1 < len(args) {
				host = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				value, err := strconv.Atoi(args[i+1])
				if err != nil {
					return err
				}
				port = value
				i++
			}
		case "--startup-delay-ms":
			if i+1 < len(args) {
				value, err := strconv.Atoi(args[i+1])
				if err != nil {
					return err
				}
				startupDelay = time.Duration(value) * time.Millisecond
				i++
			}
		case "--shutdown-delay-ms":
			if i+1 < len(args) {
				value, err := strconv.Atoi(args[i+1])
				if err != nil {
					return err
				}
				shutdownDelay = time.Duration(value) * time.Millisecond
				i++
			}
		case "--ignore-sigint":
			ignoreSigint = true
		case "--fail-start":
			failStart = true
		}
	}

	if failStart {
		return errors.New("forced startup failure")
	}

	time.Sleep(startupDelay)
	fmt.Fprintf(os.Stdout, "helper boot on %s:%d\n", host, port)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:              net.JoinHostPort(host, strconv.Itoa(port)),
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			fmt.Fprintln(os.Stdout, "tick")
		}
	}()

	go func() {
		for sig := range signals {
			if ignoreSigint && sig == syscall.SIGINT {
				continue
			}
			time.Sleep(shutdownDelay)
			_ = server.Shutdown(context.Background())
			return
		}
	}()

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func helperArgs(args []string) []string {
	for i := range args {
		if args[i] == "--" {
			return args[i+1:]
		}
	}
	return nil
}
