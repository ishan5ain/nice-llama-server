package tui

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"nice-llama-server/internal/controller"
)

type Options struct {
	Client *controller.Client
}

func Run(ctx context.Context, opts Options) error {
	if opts.Client == nil {
		return errors.New("controller client is required")
	}
	model := newModel(ctx, opts.Client)
	program := tea.NewProgram(model)
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}
	return nil
}
