package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"nice-llama-server/internal/config"
	"nice-llama-server/internal/controller"
	"nice-llama-server/internal/proxy"
	"nice-llama-server/internal/tui"
)

type cliOptions struct {
	mode                string
	stateDir            string
	controllerURL       string
	controllerToken     string
	printControllerInfo bool
	llamaServerBin      string
	modelRoots          []string
	proxyListen         string
	proxyUpstream       string
	proxyAPIKeysPath    string
	proxyUsageLogPath   string
	proxyTailscaleOnly  bool
}

func Run(ctx context.Context, argv []string) error {
	opts, err := parseArgs(argv)
	if err != nil {
		return err
	}

	switch opts.mode {
	case "controller":
		return runControllerMode(ctx, opts)
	case "proxy":
		return runProxyMode(ctx, opts)
	default:
		return runTUIMode(ctx, opts)
	}
}

func parseArgs(argv []string) (cliOptions, error) {
	opts := cliOptions{}
	args := argv
	if len(args) > 0 {
		switch args[0] {
		case "controller", "proxy":
			opts.mode = args[0]
			args = args[1:]
		}
	}

	fs := flag.NewFlagSet("nice-llama-server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	switch opts.mode {
	case "controller":
		parseControllerFlags(fs, &opts)
	case "proxy":
		parseProxyFlags(fs, &opts)
	default:
		parseTUIFlags(fs, &opts)
	}
	if err := fs.Parse(args); err != nil {
		return cliOptions{}, err
	}
	opts.modelRoots = fs.Args()
	if err := validateCLIOptions(opts); err != nil {
		return cliOptions{}, err
	}
	return opts, nil
}

func parseTUIFlags(fs *flag.FlagSet, opts *cliOptions) {
	fs.StringVar(&opts.stateDir, "state-dir", "", "override the state directory")
	fs.StringVar(&opts.llamaServerBin, "llama-server-bin", "", "path to llama-server executable")
	fs.StringVar(&opts.controllerURL, "controller-url", "", "connect to a running controller directly")
	fs.StringVar(&opts.controllerToken, "controller-token", "", "authenticate to the controller directly")
}

func parseControllerFlags(fs *flag.FlagSet, opts *cliOptions) {
	fs.StringVar(&opts.stateDir, "state-dir", "", "override the state directory")
	fs.StringVar(&opts.llamaServerBin, "llama-server-bin", "", "path to llama-server executable")
	fs.BoolVar(&opts.printControllerInfo, "print-controller-info", false, "print controller URL and token after startup")
}

func parseProxyFlags(fs *flag.FlagSet, opts *cliOptions) {
	fs.StringVar(&opts.proxyListen, "listen", "", "listen address for the proxy")
	fs.StringVar(&opts.proxyUpstream, "upstream", "", "base URL for the upstream llama-server")
	fs.StringVar(&opts.proxyAPIKeysPath, "api-keys", "", "path to the proxy API key JSON file")
	fs.StringVar(&opts.proxyUsageLogPath, "usage-log", "", "path to the usage log JSONL file")
	fs.BoolVar(&opts.proxyTailscaleOnly, "tailscale-only", false, "restrict access to Tailscale client IPs")
}

func validateCLIOptions(opts cliOptions) error {
	if opts.mode != "proxy" {
		return nil
	}
	if len(opts.modelRoots) > 0 {
		return errors.New("proxy mode does not accept model roots")
	}
	if strings.TrimSpace(opts.proxyListen) == "" {
		return errors.New("--listen is required in proxy mode")
	}
	if strings.TrimSpace(opts.proxyUpstream) == "" {
		return errors.New("--upstream is required in proxy mode")
	}
	if strings.TrimSpace(opts.proxyAPIKeysPath) == "" {
		return errors.New("--api-keys is required in proxy mode")
	}
	if strings.TrimSpace(opts.proxyUsageLogPath) == "" {
		return errors.New("--usage-log is required in proxy mode")
	}
	return nil
}

func runControllerMode(ctx context.Context, opts cliOptions) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	info, err := controller.Serve(ctx, controller.Options{
		StateDir:       opts.stateDir,
		LlamaServerBin: opts.llamaServerBin,
		ModelRoots:     opts.modelRoots,
	})
	if err != nil {
		return err
	}
	if opts.printControllerInfo {
		fmt.Fprint(os.Stdout, controllerInfoLine(info))
	}
	return nil
}

func runProxyMode(ctx context.Context, opts cliOptions) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return proxy.Serve(ctx, proxy.Options{
		ListenAddr:    opts.proxyListen,
		UpstreamURL:   opts.proxyUpstream,
		APIKeysPath:   opts.proxyAPIKeysPath,
		UsageLogPath:  opts.proxyUsageLogPath,
		TailscaleOnly: opts.proxyTailscaleOnly,
	})
}

func runTUIMode(ctx context.Context, opts cliOptions) error {
	store, err := config.NewStore(opts.stateDir)
	if err != nil {
		return err
	}

	var info config.ControllerInfo
	if opts.controllerURL != "" {
		info, err = resolveControllerInfo(store, opts)
		if err != nil {
			return err
		}
	} else {
		info, err = ensureController(ctx, store, opts)
		if err != nil {
			return err
		}
	}

	client := controller.NewClient(info.URL, info.Token)
	if len(opts.modelRoots) > 0 || opts.llamaServerBin != "" {
		var roots []string
		if len(opts.modelRoots) > 0 {
			roots = opts.modelRoots
		}
		var bin *string
		if opts.llamaServerBin != "" {
			bin = &opts.llamaServerBin
		}
		if _, err := client.Rescan(ctx, roots, bin); err != nil {
			return err
		}
	}

	return tui.Run(ctx, tui.Options{
		Client: client,
	})
}

func resolveControllerInfo(store *config.Store, opts cliOptions) (config.ControllerInfo, error) {
	if opts.controllerURL == "" {
		return config.ControllerInfo{}, errors.New("controller URL is required")
	}

	info := config.ControllerInfo{
		URL:   opts.controllerURL,
		Token: opts.controllerToken,
	}
	if info.Token != "" {
		return info, nil
	}

	if saved, err := store.LoadControllerInfo(); err == nil && saved.URL == info.URL && saved.Token != "" {
		info.Token = saved.Token
		return info, nil
	}

	return config.ControllerInfo{}, fmt.Errorf("controller token required for %s; provide --controller-token or use a matching local controller.json", opts.controllerURL)
}

func ensureController(ctx context.Context, store *config.Store, opts cliOptions) (config.ControllerInfo, error) {
	if info, err := store.LoadControllerInfo(); err == nil {
		client := controller.NewClient(info.URL, info.Token)
		healthCtx, cancel := context.WithTimeout(ctx, config.ControllerPingTimeout)
		err = client.Health(healthCtx)
		cancel()
		if err == nil {
			return info, nil
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return config.ControllerInfo{}, err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return config.ControllerInfo{}, err
	}

	if err := launchDetachedController(exe, opts); err != nil {
		return config.ControllerInfo{}, err
	}

	deadline := time.Now().Add(config.ControllerReadyTimeout)
	delay := config.ControllerPingInterval
	for time.Now().Before(deadline) {
		info, err := store.LoadControllerInfo()
		if err == nil {
			client := controller.NewClient(info.URL, info.Token)
			healthCtx, cancel := context.WithTimeout(ctx, config.ControllerPingTimeout)
			pingErr := client.Health(healthCtx)
			cancel()
			if pingErr == nil {
				return info, nil
			}
		}
		select {
		case <-ctx.Done():
			return config.ControllerInfo{}, ctx.Err()
		case <-time.After(delay):
			delay = delay * 2
			if delay > config.ControllerMaxPingInterval {
				delay = config.ControllerMaxPingInterval
			}
		}
	}

	return config.ControllerInfo{}, errors.New("controller did not become ready in time")
}

func buildControllerArgs(opts cliOptions) []string {
	args := []string{"controller"}
	if opts.stateDir != "" {
		args = append(args, "--state-dir", opts.stateDir)
	}
	if opts.llamaServerBin != "" {
		args = append(args, "--llama-server-bin", opts.llamaServerBin)
	}
	args = append(args, opts.modelRoots...)
	return args
}

func wrapErr(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

func controllerInfoLine(info config.ControllerInfo) string {
	return fmt.Sprintf("url=%s token=%s\n", info.URL, info.Token)
}
