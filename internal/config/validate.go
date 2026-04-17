package config

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	ErrModelRootNotFound      = errors.New("model root directory does not exist")
	ErrLlamaServerBinNotFound = errors.New("llama-server executable not found")
	ErrModelPathNotFound      = errors.New("model file does not exist")
)

func ValidateModelRoots(roots []string) error {
	for _, root := range roots {
		root = filepath.Clean(root)
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				return &ValidationError{Path: root, Err: ErrModelRootNotFound}
			}
			return &ValidationError{Path: root, Err: err}
		}
		if !info.IsDir() {
			return &ValidationError{Path: root, Err: ErrModelRootNotFound}
		}
	}
	return nil
}

func ValidateLlamaServerBin(path string) error {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ValidationError{Path: path, Err: ErrLlamaServerBinNotFound}
		}
		return &ValidationError{Path: path, Err: err}
	}
	if info.IsDir() {
		return &ValidationError{Path: path, Err: ErrLlamaServerBinNotFound}
	}
	return nil
}

func ValidateModelPath(path string) error {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ValidationError{Path: path, Err: ErrModelPathNotFound}
		}
		return &ValidationError{Path: path, Err: err}
	}
	if info.IsDir() {
		return &ValidationError{Path: path, Err: ErrModelPathNotFound}
	}
	return nil
}

type ValidationError struct {
	Path string
	Err  error
}

func (e *ValidationError) Error() string {
	return e.Err.Error()
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}
