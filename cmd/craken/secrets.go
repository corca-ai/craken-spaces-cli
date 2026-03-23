package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

var (
	isTerminalFD                      = term.IsTerminal
	readMaskedTerminalKeyFD           = defaultReadMaskedTerminalKeyFD
	terminalStatusSink      io.Writer = os.Stderr
)

type stdinWithFD interface {
	io.Reader
	Fd() uintptr
}

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
		return readAuthKeyFromStdin(stdin)
	default:
		if stdin == nil {
			return "", errors.New("stdin is not available")
		}
		if file, ok := stdin.(stdinWithFD); ok && isTerminalFD(int(file.Fd())) {
			return readAuthKeyFromStdin(stdin)
		}
		return "", errors.New("one of --key-file or --key-stdin is required when stdin is not interactive")
	}
}

func readAuthKeyFromStdin(stdin io.Reader) (string, error) {
	if file, ok := stdin.(stdinWithFD); ok && isTerminalFD(int(file.Fd())) {
		payload, err := readMaskedTerminalKeyFD(int(file.Fd()), "Auth key: ", terminalStatusSink)
		if err != nil {
			return "", err
		}
		return normalizeAuthKey(payload)
	}
	payload, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	return normalizeAuthKey(payload)
}

func defaultReadMaskedTerminalKeyFD(fd int, prompt string, sink io.Writer) ([]byte, error) {
	if sink == nil {
		sink = io.Discard
	}
	if _, err := fmt.Fprint(sink, prompt); err != nil {
		return nil, err
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	defer func() { _ = term.Restore(fd, state) }()

	input := os.NewFile(uintptr(fd), "stdin")
	var payload bytes.Buffer
	buffer := make([]byte, 1)
	for {
		n, err := input.Read(buffer)
		if err != nil {
			_, _ = fmt.Fprintln(sink)
			return nil, err
		}
		if n == 0 {
			continue
		}
		switch buffer[0] {
		case '\r', '\n':
			_, _ = fmt.Fprintln(sink)
			return payload.Bytes(), nil
		case 0x03:
			_, _ = fmt.Fprintln(sink)
			return nil, errors.New("auth key entry interrupted")
		case 0x08, 0x7f:
			if payload.Len() == 0 {
				continue
			}
			payload.Truncate(payload.Len() - 1)
			_, _ = io.WriteString(sink, "\b \b")
		default:
			if buffer[0] < 0x20 {
				continue
			}
			payload.WriteByte(buffer[0])
			_, _ = io.WriteString(sink, "*")
		}
	}
}

func normalizeAuthKey(payload []byte) (string, error) {
	key := strings.TrimSpace(string(payload))
	if key == "" {
		return "", errors.New("auth key is empty")
	}
	return key, nil
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

func validateSecretParentDir(path string) error {
	dir := filepath.Dir(strings.TrimSpace(path))
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("parent directory %s must not be group- or world-writable", dir)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("parent directory %s must be owned by the current user", dir)
	}
	return nil
}

func validateSecretFile(path string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s must not be a symlink", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s must not be accessible by group or others", path)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("%s must be owned by the current user", path)
	}
	return nil
}

func writePrivateFile(path string, data []byte) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("file path is required")
	}
	dir := filepath.Dir(path)
	if err := validateSecretParentDir(path); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := validateSecretParentDir(path); err != nil {
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
