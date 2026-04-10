package tui

import (
	"testing"

	"nice-llama-server/internal/config"
)

func TestResolveModelPathByDisplayName(t *testing.T) {
	t.Parallel()

	models := []config.DiscoveredModel{
		{Path: `C:\models\gemma-3-4b-it-Q4_K_M.gguf`, DisplayName: "gemma-3-4b-it-Q4_K_M"},
		{Path: `C:\models\qwen3-8b-q4_k_m.gguf`, DisplayName: "qwen3-8b-q4_k_m"},
	}

	got, err := resolveModelPath("gemma-3-4b-it-Q4_K_M", models)
	if err != nil {
		t.Fatalf("resolveModelPath returned error: %v", err)
	}
	if got != models[0].Path {
		t.Fatalf("unexpected path: got %q want %q", got, models[0].Path)
	}
}

func TestResolveModelPathRejectsUnknownName(t *testing.T) {
	t.Parallel()

	_, err := resolveModelPath("missing-model", []config.DiscoveredModel{
		{Path: `/models/gemma.gguf`, DisplayName: "gemma"},
	})
	if err == nil {
		t.Fatalf("expected error for unknown model")
	}
}

func TestAutocompleteModelUsesDisplayNames(t *testing.T) {
	t.Parallel()

	models := []config.DiscoveredModel{
		{Path: `/models/gemma-3-4b-it-Q4_K_M.gguf`, DisplayName: "gemma-3-4b-it-Q4_K_M"},
		{Path: `/models/gemma-3-12b-it-Q4_K_M.gguf`, DisplayName: "gemma-3-12b-it-Q4_K_M"},
	}
	editor := newBookmarkEditor(config.Bookmark{}, models, true)
	editor.focus = 1
	editor.model.SetValue("gemma")

	if !editor.AutocompleteModel(models) {
		t.Fatalf("expected autocomplete to succeed")
	}
	if editor.model.Value() != "gemma-3-12b-it-Q4_K_M" && editor.model.Value() != "gemma-3-4b-it-Q4_K_M" {
		t.Fatalf("unexpected autocomplete value: %q", editor.model.Value())
	}
}
