package discovery

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"nice-llama-server/internal/config"
)

func Scan(roots []string) ([]config.DiscoveredModel, error) {
	if len(roots) == 0 {
		return nil, nil
	}

	var models []config.DiscoveredModel
	seen := map[string]struct{}{}

	for _, root := range roots {
		root := root
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}

			name := d.Name()
			if !strings.EqualFold(filepath.Ext(name), ".gguf") {
				return nil
			}
			if strings.HasPrefix(strings.ToLower(name), "mmproj") {
				return nil
			}

			stem := strings.TrimSuffix(name, filepath.Ext(name))
			if isShard(stem) && !strings.Contains(stem, "-00001-of-") {
				return nil
			}

			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}
			if _, ok := seen[abs]; ok {
				return nil
			}
			seen[abs] = struct{}{}

			models = append(models, config.DiscoveredModel{
				Path:        abs,
				DisplayName: stem,
				GroupKey:    config.DeriveGroupKey(abs),
				Root:        root,
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	slices.SortFunc(models, func(a, b config.DiscoveredModel) int {
		if a.GroupKey != b.GroupKey {
			return strings.Compare(strings.ToLower(a.GroupKey), strings.ToLower(b.GroupKey))
		}
		if a.DisplayName != b.DisplayName {
			return strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
		}
		return strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path))
	})
	return models, nil
}

func isShard(stem string) bool {
	parts := strings.Split(stem, "-")
	n := len(parts)
	if n < 4 {
		return false
	}
	if parts[n-2] != "of" {
		return false
	}
	return len(parts[n-3]) == 5 && len(parts[n-1]) == 5 && digits(parts[n-3]) && digits(parts[n-1])
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
