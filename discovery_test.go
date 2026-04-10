package main

import (
	"os"
	"path/filepath"
	"testing"

	"nice-llama-server/internal/discovery"
)

func TestDiscoveryScan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := []string{
		"llama-3.2-3b-Q4_K_M.gguf",
		"mmproj-F16.gguf",
		filepath.Join("kimi-k2", "Kimi-K2-Thinking-UD-IQ1_S-00001-of-00006.gguf"),
		filepath.Join("kimi-k2", "Kimi-K2-Thinking-UD-IQ1_S-00002-of-00006.gguf"),
		filepath.Join("multimodal", "vision-model.gguf"),
		filepath.Join("multimodal", "mmproj-model.gguf"),
	}
	for _, file := range files {
		path := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	models, err := discovery.Scan([]string{root})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("unexpected model count: got %d", len(models))
	}

	seen := map[string]bool{}
	for _, model := range models {
		seen[filepath.Base(model.Path)] = true
	}
	if !seen["llama-3.2-3b-Q4_K_M.gguf"] {
		t.Fatalf("expected single GGUF model to be discovered")
	}
	if !seen["Kimi-K2-Thinking-UD-IQ1_S-00001-of-00006.gguf"] {
		t.Fatalf("expected first shard to be discovered")
	}
	if seen["Kimi-K2-Thinking-UD-IQ1_S-00002-of-00006.gguf"] {
		t.Fatalf("did not expect non-entry shard to be discovered")
	}
	if seen["mmproj-F16.gguf"] || seen["mmproj-model.gguf"] {
		t.Fatalf("did not expect mmproj files to be discovered")
	}
}
