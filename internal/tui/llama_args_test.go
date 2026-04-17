package tui

import "testing"

func TestParseLlamaArgCatalogExtractsAliasesAndSkipsModel(t *testing.T) {
	t.Parallel()

	catalog := parseLlamaArgCatalog(
		"| Argument | Explanation |\n" +
			"| -------- | ----------- |\n" +
			"| `-c, --ctx-size N` | size of context |\n" +
			"| `--poll <0\\|1>` | escaped pipe value |\n" +
			"| `-m, --model FNAME` | model path |\n",
	)

	if len(catalog) != 2 {
		t.Fatalf("unexpected option count: got %d want 2", len(catalog))
	}
	if got := catalog[0].Aliases; len(got) != 2 || got[0] != "-c" || got[1] != "--ctx-size" {
		t.Fatalf("unexpected first aliases: %#v", got)
	}
	if got := catalog[1].Aliases; len(got) != 1 || got[0] != "--poll" {
		t.Fatalf("unexpected escaped-pipe aliases: %#v", got)
	}
}

func TestParseLlamaArgCatalogSupportsTrackedOneColumnCatalog(t *testing.T) {
	t.Parallel()

	catalog := parseLlamaArgCatalog(
		"| Argument |\n" +
			"| -------- |\n" +
			"| `-c, --ctx-size N` |\n" +
			"| `-m, --model FNAME` |\n",
	)

	if len(catalog) != 1 {
		t.Fatalf("unexpected option count: got %d want 1", len(catalog))
	}
	if got := catalog[0].Aliases; len(got) != 2 || got[0] != "-c" || got[1] != "--ctx-size" {
		t.Fatalf("unexpected aliases: %#v", got)
	}
}

func TestEmbeddedLlamaArgCatalogLoadsWithoutDevlocal(t *testing.T) {
	t.Parallel()

	catalog := loadLlamaArgCatalog()
	if len(catalog) == 0 {
		t.Fatalf("expected embedded llama arg catalog to load")
	}
	if !catalogContainsAlias(catalog, "--ctx-size") {
		t.Fatalf("embedded catalog should include --ctx-size")
	}
	if catalogContainsAlias(catalog, "--model") {
		t.Fatalf("embedded catalog should not include controller-owned --model")
	}
}

func catalogContainsAlias(catalog []llamaArgOption, alias string) bool {
	for _, option := range catalog {
		for _, candidate := range option.Aliases {
			if candidate == alias {
				return true
			}
		}
	}
	return false
}
