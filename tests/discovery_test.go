package main

import (
	"os"
	"path/filepath"
	"testing"

	"nice-llama-server/internal/config"
	"nice-llama-server/internal/discovery"
)

func TestDiscoveryScan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := []string{
		"llama-3.2-3b-Q4_K_M.gguf",
		"mmproj-F16.gguf",
		"MMPROJ-vision.gguf",
		filepath.Join("kimi-k2", "Kimi-K2-Thinking-UD-IQ1_S-00001-of-00006.gguf"),
		filepath.Join("kimi-k2", "Kimi-K2-Thinking-UD-IQ1_S-00002-of-00006.gguf"),
		filepath.Join("multimodal", "vision-model.gguf"),
		filepath.Join("multimodal", "mmproj-model.gguf"),
		filepath.Join("multimodal", "mmproj-extra.gguf"),
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

	var rootModel config.DiscoveredModel
	var multimodalModel config.DiscoveredModel
	for _, model := range models {
		switch filepath.Base(model.Path) {
		case "llama-3.2-3b-Q4_K_M.gguf":
			rootModel = model
		case "vision-model.gguf":
			multimodalModel = model
		}
	}

	if got, want := len(rootModel.MMProjPaths), 2; got != want {
		t.Fatalf("expected %d root mmproj files, got %d", want, got)
	}
	if filepath.Base(rootModel.MMProjPaths[0]) != "mmproj-F16.gguf" || filepath.Base(rootModel.MMProjPaths[1]) != "MMPROJ-vision.gguf" {
		t.Fatalf("unexpected root mmproj paths: %#v", rootModel.MMProjPaths)
	}
	if got, want := len(multimodalModel.MMProjPaths), 2; got != want {
		t.Fatalf("expected %d multimodal mmproj files, got %d", want, got)
	}
	if filepath.Base(multimodalModel.MMProjPaths[0]) != "mmproj-extra.gguf" || filepath.Base(multimodalModel.MMProjPaths[1]) != "mmproj-model.gguf" {
		t.Fatalf("unexpected multimodal mmproj paths: %#v", multimodalModel.MMProjPaths)
	}
}
