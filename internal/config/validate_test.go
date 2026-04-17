package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateModelRoots(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		roots   []string
		setup   func(t *testing.T) []string
		wantErr bool
	}{
		{
			name:    "nil roots",
			roots:   nil,
			wantErr: false,
		},
		{
			name:    "empty roots",
			roots:   []string{},
			wantErr: false,
		},
		{
			name:    "single valid root",
			roots:   []string{t.TempDir()},
			wantErr: false,
		},
		{
			name:    "multiple valid roots",
			roots:   []string{t.TempDir(), t.TempDir()},
			wantErr: false,
		},
		{
			name:    "non-existent root",
			roots:   []string{filepath.Join(t.TempDir(), "does-not-exist")},
			wantErr: true,
		},
		{
			name:    "file instead of directory",
			roots:   []string{t.Name()},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateModelRoots(tt.roots)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateLlamaServerBin(t *testing.T) {
	t.Parallel()

	path, err := os.Executable()
	if err != nil {
		t.Skip("cannot determine executable path for test")
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid binary",
			path:    path,
			wantErr: false,
		},
		{
			name:    "non-existent binary",
			path:    filepath.Join(t.TempDir(), "does-not-exist"),
			wantErr: true,
		},
		{
			name:    "directory instead of file",
			path:    t.TempDir(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateLlamaServerBin(tt.path)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateModelPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "model.gguf")
	if err := os.WriteFile(validFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid model file",
			path:    validFile,
			wantErr: false,
		},
		{
			name:    "non-existent model",
			path:    filepath.Join(tmpDir, "missing.gguf"),
			wantErr: true,
		},
		{
			name:    "directory instead of file",
			path:    tmpDir,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateModelPath(tt.path)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	t.Parallel()

	err := &ValidationError{
		Path: "/some/path",
		Err:  ErrModelRootNotFound,
	}

	if err.Error() != ErrModelRootNotFound.Error() {
		t.Fatalf("Error() = %q, want %q", err.Error(), ErrModelRootNotFound.Error())
	}

	unwrapped := err.Unwrap()
	if unwrapped != ErrModelRootNotFound {
		t.Fatalf("Unwrap() = %v, want %v", unwrapped, ErrModelRootNotFound)
	}

	var target *ValidationError
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed to extract *ValidationError")
	}
	if target.Path != "/some/path" {
		t.Fatalf("extracted path = %q, want %q", target.Path, "/some/path")
	}
	if target.Err != ErrModelRootNotFound {
		t.Fatalf("extracted Err = %v, want %v", target.Err, ErrModelRootNotFound)
	}
}
