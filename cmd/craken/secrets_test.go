package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

type fakeTerminalInput struct {
	*strings.Reader
	fd uintptr
}

func (f fakeTerminalInput) Fd() uintptr {
	return f.fd
}

func TestReadAuthKeyFromTerminalDisablesEcho(t *testing.T) {
	origIsTerminal := isTerminalFD
	origReadMasked := readMaskedTerminalKeyFD
	origStatusSink := terminalStatusSink
	t.Cleanup(func() {
		isTerminalFD = origIsTerminal
		readMaskedTerminalKeyFD = origReadMasked
		terminalStatusSink = origStatusSink
	})

	var status bytes.Buffer
	isTerminalFD = func(fd int) bool {
		return fd == 99
	}
	readMaskedTerminalKeyFD = func(fd int, prompt string, sink io.Writer) ([]byte, error) {
		if fd != 99 {
			t.Fatalf("readMaskedTerminalKeyFD fd=%d, want 99", fd)
		}
		if prompt != "Auth key: " {
			t.Fatalf("prompt=%q, want Auth key: ", prompt)
		}
		if sink != &status {
			t.Fatalf("sink=%v, want status buffer", sink)
		}
		_, _ = io.WriteString(sink, "Auth key: ***\n")
		return []byte("tty_secret\n"), nil
	}
	terminalStatusSink = &status

	key, err := readAuthKeyFromStdin(fakeTerminalInput{
		Reader: strings.NewReader("should-not-be-read"),
		fd:     99,
	})
	if err != nil {
		t.Fatalf("readAuthKeyFromStdin returned error: %v", err)
	}
	if key != "tty_secret" {
		t.Fatalf("key=%q, want tty_secret", key)
	}
	if status.String() != "Auth key: ***\n" {
		t.Fatalf("status output=%q, want masked prompt", status.String())
	}
}

func TestReadAuthKeyFromPipeReadsPlainStdin(t *testing.T) {
	key, err := readAuthKeyFromStdin(strings.NewReader("pipe_secret\n"))
	if err != nil {
		t.Fatalf("readAuthKeyFromStdin returned error: %v", err)
	}
	if key != "pipe_secret" {
		t.Fatalf("key=%q, want pipe_secret", key)
	}
}

func TestResolveAuthKeyPromptsOnInteractiveTerminalByDefault(t *testing.T) {
	origIsTerminal := isTerminalFD
	origReadMasked := readMaskedTerminalKeyFD
	t.Cleanup(func() {
		isTerminalFD = origIsTerminal
		readMaskedTerminalKeyFD = origReadMasked
	})

	isTerminalFD = func(fd int) bool { return fd == 99 }
	readMaskedTerminalKeyFD = func(fd int, prompt string, _ io.Writer) ([]byte, error) {
		if fd != 99 {
			t.Fatalf("fd=%d, want 99", fd)
		}
		if prompt != "Auth key: " {
			t.Fatalf("prompt=%q, want Auth key: ", prompt)
		}
		return []byte("prompted_secret"), nil
	}

	key, err := resolveAuthKey("", false, fakeTerminalInput{
		Reader: strings.NewReader("ignored"),
		fd:     99,
	})
	if err != nil {
		t.Fatalf("resolveAuthKey returned error: %v", err)
	}
	if key != "prompted_secret" {
		t.Fatalf("key=%q, want prompted_secret", key)
	}
}
