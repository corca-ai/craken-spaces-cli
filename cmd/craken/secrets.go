package main

import (
	"errors"
	"fmt"
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
	return writePrivateFile(path, []byte(value+"\n"))
}

func writePrivateFile(path string, data []byte) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("file path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	cleanup := true
	defer func() {
		_ = tmpFile.Close()
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmpFile.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmpFile.Write(data); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.IsDir() {
			return fmt.Errorf("%s is a directory", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
