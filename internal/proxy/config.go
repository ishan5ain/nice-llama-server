package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type keyFile struct {
	Users []User `json:"users"`
}

type User struct {
	Name          string   `json:"name"`
	APIKey        string   `json:"api_key"`
	RPM           int      `json:"rpm"`
	MaxTokens     int      `json:"max_tokens"`
	AllowedModels []string `json:"allowed_models"`
}

func LoadUsers(path string) ([]User, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	raw, err := os.ReadFile(clean)
	if err != nil {
		return nil, err
	}

	var cfg keyFile
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode api key config: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode api key config: trailing content")
	}

	users := make([]User, len(cfg.Users))
	copy(users, cfg.Users)
	if err := validateUsers(users); err != nil {
		return nil, err
	}
	return users, nil
}

func validateUsers(users []User) error {
	if len(users) == 0 {
		return errors.New("api key config must contain at least one user")
	}

	seenNames := map[string]struct{}{}
	seenKeys := map[string]struct{}{}
	for i := range users {
		users[i].Name = strings.TrimSpace(users[i].Name)
		users[i].APIKey = strings.TrimSpace(users[i].APIKey)
		if users[i].Name == "" {
			return fmt.Errorf("user %d: name is required", i+1)
		}
		if users[i].APIKey == "" {
			return fmt.Errorf("user %d: api_key is required", i+1)
		}
		if _, ok := seenNames[users[i].Name]; ok {
			return fmt.Errorf("user %d: duplicate name %q", i+1, users[i].Name)
		}
		if _, ok := seenKeys[users[i].APIKey]; ok {
			return fmt.Errorf("user %d: duplicate api_key", i+1)
		}
		seenNames[users[i].Name] = struct{}{}
		seenKeys[users[i].APIKey] = struct{}{}
		if users[i].RPM <= 0 {
			return fmt.Errorf("user %s: rpm must be positive", users[i].Name)
		}
		if users[i].MaxTokens <= 0 {
			return fmt.Errorf("user %s: max_tokens must be positive", users[i].Name)
		}

		allowed := normalizeModels(users[i].AllowedModels)
		if len(allowed) == 0 {
			return fmt.Errorf("user %s: allowed_models must not be empty", users[i].Name)
		}
		users[i].AllowedModels = allowed
	}
	return nil
}

func normalizeModels(models []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	slices.Sort(out)
	return out
}
