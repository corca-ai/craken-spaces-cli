package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePublicKeyInput(t *testing.T) {
	t.Run("inline value provided", func(t *testing.T) {
		got, err := resolvePublicKeyInput("ssh-ed25519 AAAA", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "ssh-ed25519 AAAA" {
			t.Fatalf("got %q, want inline value", got)
		}
	})
	t.Run("file path provided", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "key.pub")
		if err := os.WriteFile(path, []byte("ssh-ed25519 FROM_FILE"), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := resolvePublicKeyInput("", path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "ssh-ed25519 FROM_FILE" {
			t.Fatalf("got %q, want file content", got)
		}
	})
	t.Run("both provided", func(t *testing.T) {
		_, err := resolvePublicKeyInput("inline", "/some/file")
		if err == nil {
			t.Fatal("expected error when both provided")
		}
	})
	t.Run("neither provided", func(t *testing.T) {
		_, err := resolvePublicKeyInput("", "")
		if err == nil {
			t.Fatal("expected error when neither provided")
		}
	})
}

func TestResolveSSHHost(t *testing.T) {
	t.Run("explicit host", func(t *testing.T) {
		got, err := resolveSSHHost("myhost.example.com", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "myhost.example.com" {
			t.Fatalf("got %q, want explicit host", got)
		}
	})
	t.Run("env var", func(t *testing.T) {
		t.Setenv("CRAKEN_SSH_HOST", "env-host.example.com")
		got, err := resolveSSHHost("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "env-host.example.com" {
			t.Fatalf("got %q, want env host", got)
		}
	})
	t.Run("base URL", func(t *testing.T) {
		got, err := resolveSSHHost("", "https://agents.borca.ai")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "agents.borca.ai" {
			t.Fatalf("got %q, want hostname from base URL", got)
		}
	})
	t.Run("empty everything", func(t *testing.T) {
		_, err := resolveSSHHost("", "")
		if err == nil {
			t.Fatal("expected error when everything is empty")
		}
	})
}

func TestParseIntEnv(t *testing.T) {
	t.Run("valid int", func(t *testing.T) {
		t.Setenv("TEST_PARSE_INT", "8080")
		got := parseIntEnv("TEST_PARSE_INT", 22)
		if got != 8080 {
			t.Fatalf("got %d, want 8080", got)
		}
	})
	t.Run("not set", func(t *testing.T) {
		got := parseIntEnv("TEST_PARSE_INT_UNSET", 22)
		if got != 22 {
			t.Fatalf("got %d, want fallback 22", got)
		}
	})
	t.Run("invalid value", func(t *testing.T) {
		t.Setenv("TEST_PARSE_INT_BAD", "notanumber")
		got := parseIntEnv("TEST_PARSE_INT_BAD", 42)
		if got != 42 {
			t.Fatalf("got %d, want fallback 42", got)
		}
	})
}

func TestSshCertificateFileForIdentity(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"/home/user/.ssh/id_ed25519", "/home/user/.ssh/id_ed25519-cert.pub"},
		{"/home/user/.ssh/id_ed25519.pub", "/home/user/.ssh/id_ed25519-cert.pub"},
		{"  /tmp/key.pub  ", "/tmp/key-cert.pub"},
	}
	for _, tc := range tests {
		got := sshCertificateFileForIdentity(tc.input)
		if got != tc.want {
			t.Errorf("sshCertificateFileForIdentity(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPrintTable(t *testing.T) {
	var buf bytes.Buffer
	header := []string{"id", "name", "status"}
	rows := [][]string{
		{"1", "alpha", "active"},
		{"123", "b", "inactive"},
	}
	printTable(&buf, header, rows)
	output := buf.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), output)
	}
	// Header should contain all column names.
	for _, col := range header {
		if !strings.Contains(lines[0], col) {
			t.Errorf("header missing column %q: %q", col, lines[0])
		}
	}
	// Columns should be aligned: "id" column is 3 wide (for "123"), so "1" should be padded.
	if !strings.HasPrefix(lines[1], "1  ") {
		t.Errorf("expected padded first column in row 1: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "123") {
		t.Errorf("expected '123' at start of row 2: %q", lines[2])
	}
}
