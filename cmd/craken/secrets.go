package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func resolveAuthKey(filePath string, readFromStdin bool, stdin io.Reader) (string, error) {
	switch {
	case strings.TrimSpace(filePath) != "":
		payload, err := os.ReadFile(filepath.Clean(filePath))
		if err != nil {
			return "", err
		}
		key := strings.TrimSpace(string(payload))
		if key == "" {
			return "", errors.New("auth key file is empty")
		}
		return key, nil
	case readFromStdin:
		if stdin == nil {
			return "", errors.New("stdin is not available")
		}
		payload, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		key := strings.TrimSpace(string(payload))
		if key == "" {
			return "", errors.New("auth key is empty")
		}
		return key, nil
	default:
		return "", errors.New("one of --key-file or --key-stdin is required")
	}
}

func writeSecretFile(path, value string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("secret file path is required")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("secret value is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(value+"\n"), 0o600)
}
