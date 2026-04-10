package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"nice-llama-server/internal/config"
	"nice-llama-server/internal/controller"
	"nice-llama-server/internal/tui"
)

type cliOptions struct {
	mode           string
	stateDir       string
	controllerURL  string
	llamaServerBin string
	modelRoots     []string
}

func Run(ctx context.Context, argv []string) error {
	opts, err := parseArgs(argv)
	if err != nil {
		return err
	}

	switch opts.mode {
	case "controller":
		return runControllerMode(ctx, opts)
	default:
		return runTUIMode(ctx, opts)
	}
}

func parseArgs(argv []string) (cliOptions, error) {
	opts := cliOptions{}
	args := argv
	if len(args) > 0 && args[0] == "controller" {
		opts.mode = "controller"
		args = args[1:]
	}

	fs := flag.NewFlagSet("nice-llama-server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&opts.stateDir, "state-dir", "", "override the state directory")
	fs.StringVar(&opts.llamaServerBin, "llama-server-bin", "", "path to llama-server executable")
	if opts.mode != "controller" {
		fs.StringVar(&opts.controllerURL, "controller-url", "", "connect to a running controller directly")
	}
	if err := fs.Parse(args); err != nil {
		return cliOptions{}, err
	}

	opts.modelRoots = fs.Args()
	return opts, nil
}

func runControllerMode(ctx context.Context, opts cliOptions) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	return controller.Run(ctx, controller.Options{
		StateDir:       opts.stateDir,
		LlamaServerBin: opts.llamaServerBin,
		ModelRoots:     opts.modelRoots,
	})
}

func runTUIMode(ctx context.Context, opts cliOptions) error {
	store, err := config.NewStore(opts.stateDir)
	if err != nil {
		return err
	}

	var info config.ControllerInfo
	if opts.controllerURL != "" {
		info.URL = opts.controllerURL
		if saved, err := store.LoadControllerInfo(); err == nil {
			if saved.URL == info.URL {
				info.Token = saved.Token
			}
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

func ensureController(ctx context.Context, store *config.Store, opts cliOptions) (config.ControllerInfo, error) {
	if info, err := store.LoadControllerInfo(); err == nil {
		client := controller.NewClient(info.URL, info.Token)
		healthCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
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

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		info, err := store.LoadControllerInfo()
		if err == nil {
			client := controller.NewClient(info.URL, info.Token)
			healthCtx, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
			pingErr := client.Health(healthCtx)
			cancel()
			if pingErr == nil {
				return info, nil
			}
		}
		select {
		case <-ctx.Done():
			return config.ControllerInfo{}, ctx.Err()
		case <-time.After(250 * time.Millisecond):
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
