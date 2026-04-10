package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"nice-llama-server/internal/bookmark"
)

type Events struct {
	OnLog  func(stream, line string)
	OnExit func(exitCode int, err error)
}

type Process struct {
	cmd           *exec.Cmd
	done          chan waitResult
	events        Events
	host          string
	port          int
	healthTarget  string
	unixSocket    string
	stopOnce      sync.Once
	waitOnce      sync.Once
	forcedStopErr error
}

type waitResult struct {
	exitCode int
	err      error
}

func BuildArgs(modelPath, argsText string) ([]string, error) {
	args, err := bookmark.ParseArgs(argsText)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(args)+2)
	out = append(out, "-m", modelPath)
	out = append(out, args...)
	return out, nil
}

func Start(binary, modelPath, argsText string, events Events) (*Process, error) {
	argv, err := BuildArgs(modelPath, argsText)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(binary, argv...)
	prepareChildProcess(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	host, port, socket := parseHostPort(argv)
	healthTarget := host
	if socket == "" {
		if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
			healthTarget = "127.0.0.1"
		}
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	p := &Process{
		cmd:          cmd,
		done:         make(chan waitResult, 1),
		events:       events,
		host:         host,
		port:         port,
		healthTarget: healthTarget,
		unixSocket:   socket,
	}

	go p.scanPipe("stdout", stdout)
	go p.scanPipe("stderr", stderr)
	go p.wait()
	return p, nil
}

func (p *Process) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *Process) Host() string {
	return p.host
}

func (p *Process) Port() int {
	return p.port
}

func (p *Process) WaitReady(ctx context.Context) error {
	httpClient := &http.Client{
		Timeout: 500 * time.Millisecond,
	}

	reqURL := p.healthURL()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return err
		}
		if p.unixSocket != "" {
			httpClient.Transport = &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", p.unixSocket)
				},
			}
		}

		resp, err := httpClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}

		select {
		case result := <-p.done:
			p.done <- result
			if result.err != nil {
				return fmt.Errorf("process exited before ready: %w", result.err)
			}
			return errors.New("process exited before ready")
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *Process) Stop(timeout time.Duration) (int, error) {
	var signalErr error
	p.stopOnce.Do(func() {
		signalErr = gracefulStop(p.PID())
	})
	if signalErr != nil && !errors.Is(signalErr, syscall.ESRCH) {
		p.forcedStopErr = signalErr
	}

	select {
	case result := <-p.done:
		p.done <- result
		return result.exitCode, result.err
	case <-time.After(timeout):
	}

	if err := forceKill(p.PID()); err != nil && !errors.Is(err, syscall.ESRCH) {
		return 0, err
	}

	result := <-p.done
	p.done <- result
	if p.forcedStopErr != nil {
		return result.exitCode, p.forcedStopErr
	}
	return result.exitCode, result.err
}

func (p *Process) wait() {
	err := p.cmd.Wait()
	exitCode := 0
	if p.cmd.ProcessState != nil {
		exitCode = p.cmd.ProcessState.ExitCode()
	}
	result := waitResult{exitCode: exitCode, err: err}
	p.done <- result
	if p.events.OnExit != nil {
		p.events.OnExit(exitCode, err)
	}
}

func (p *Process) scanPipe(stream string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if p.events.OnLog != nil {
			p.events.OnLog(stream, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil && p.events.OnLog != nil {
		p.events.OnLog("stderr", fmt.Sprintf("log scanner error: %v", err))
	}
}

func (p *Process) healthURL() string {
	if p.unixSocket != "" {
		return "http://unix/health"
	}
	host := p.healthTarget
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%d/health", host, p.port)
}

func parseHostPort(args []string) (string, int, string) {
	host := "127.0.0.1"
	port := 8080

	for i := 0; i < len(args); i++ {
		token := args[i]
		switch {
		case token == "--host" && i+1 < len(args):
			host = args[i+1]
			i++
		case strings.HasPrefix(token, "--host="):
			host = strings.TrimPrefix(token, "--host=")
		case token == "--port" && i+1 < len(args):
			if value, err := strconv.Atoi(args[i+1]); err == nil {
				port = value
			}
			i++
		case strings.HasPrefix(token, "--port="):
			if value, err := strconv.Atoi(strings.TrimPrefix(token, "--port=")); err == nil {
				port = value
			}
		}
	}

	if strings.HasSuffix(host, ".sock") {
		socket := host
		if !filepathLike(socket) {
			if value, err := url.PathUnescape(socket); err == nil {
				socket = value
			}
		}
		return host, 0, socket
	}

	return host, port, ""
}

func filepathLike(value string) bool {
	return strings.HasPrefix(value, "/") || strings.Contains(value, `\`)
}
