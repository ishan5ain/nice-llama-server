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
	seenModels := map[string]struct{}{}
	seenMMProj := map[string]struct{}{}
	mmprojByDir := map[string][]string{}

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

			stem := strings.TrimSuffix(name, filepath.Ext(name))
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}

			if isMMProjFile(name) {
				if _, ok := seenMMProj[abs]; ok {
					return nil
				}
				seenMMProj[abs] = struct{}{}
				dir := filepath.Dir(abs)
				mmprojByDir[dir] = append(mmprojByDir[dir], abs)
				return nil
			}

			if isShard(stem) && !strings.Contains(stem, "-00001-of-") {
				return nil
			}

			if _, ok := seenModels[abs]; ok {
				return nil
			}
			seenModels[abs] = struct{}{}

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

	for dir, paths := range mmprojByDir {
		slices.SortFunc(paths, compareFoldedStrings)
		mmprojByDir[dir] = paths
	}
	for i := range models {
		dir := filepath.Dir(models[i].Path)
		if paths := mmprojByDir[dir]; len(paths) > 0 {
			models[i].MMProjPaths = append([]string(nil), paths...)
		}
	}

	slices.SortFunc(models, func(a, b config.DiscoveredModel) int {
		if a.GroupKey != b.GroupKey {
			return compareFoldedStrings(a.GroupKey, b.GroupKey)
		}
		if a.DisplayName != b.DisplayName {
			return compareFoldedStrings(a.DisplayName, b.DisplayName)
		}
		return compareFoldedStrings(a.Path, b.Path)
	})
	return models, nil
}

func isMMProjFile(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "mmproj")
}

func compareFoldedStrings(a, b string) int {
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
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
