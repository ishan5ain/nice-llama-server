package main

import (
	"errors"
	"testing"

	"nice-llama-server/internal/bookmark"
	"nice-llama-server/internal/config"
)

func TestParseArgs(t *testing.T) {
	t.Parallel()

	args, err := bookmark.ParseArgs(`
# comment
--ctx-size 4096
--host "0.0.0.0"
--chat-template-file "/tmp/chat template.jinja"
`)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}

	want := []string{"--ctx-size", "4096", "--host", "0.0.0.0", "--chat-template-file", "/tmp/chat template.jinja"}
	if len(args) != len(want) {
		t.Fatalf("unexpected arg count: got %d want %d (%v)", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg %d mismatch: got %q want %q", i, args[i], want[i])
		}
	}
}

func TestParseArgsRejectsModelFlag(t *testing.T) {
	t.Parallel()

	_, err := bookmark.ParseArgs("--model /tmp/model.gguf")
	if !errors.Is(err, bookmark.ErrModelFlag) {
		t.Fatalf("expected ErrModelFlag, got %v", err)
	}
}

func TestNormalizeBookmark(t *testing.T) {
	t.Parallel()

	normalized, err := bookmark.NormalizeBookmark(config.Bookmark{
		Name:      " Gemma 3 4B ",
		ModelPath: "/models/gemma-3-4b-it-Q4_K_M.gguf",
		ArgsText:  "--ctx-size 4096   \n--port 8081\n",
	})
	if err != nil {
		t.Fatalf("NormalizeBookmark returned error: %v", err)
	}
	if normalized.ID == "" {
		t.Fatalf("expected generated ID")
	}
	if normalized.GroupKey != "gemma-3-4b-it-Q4_K_M" {
		t.Fatalf("unexpected group key: %q", normalized.GroupKey)
	}
	if normalized.ArgsText != "--ctx-size 4096\n--port 8081" {
		t.Fatalf("args text was not normalized: %q", normalized.ArgsText)
	}
}
