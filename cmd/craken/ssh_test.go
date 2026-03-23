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
		t.Setenv("SPACES_SSH_HOST", "env-host.example.com")
		got, err := resolveSSHHost("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "env-host.example.com" {
			t.Fatalf("got %q, want env host", got)
		}
	})
	t.Run("base URL", func(t *testing.T) {
		got, err := resolveSSHHost("", "https://spaces.borca.ai")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "spaces.borca.ai" {
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

func TestResolveKnownHostsFile(t *testing.T) {
	t.Run("explicit path", func(t *testing.T) {
		got := resolveKnownHostsFile("/tmp/known_hosts")
		if got != "/tmp/known_hosts" {
			t.Fatalf("got %q, want explicit path", got)
		}
	})
	t.Run("env var", func(t *testing.T) {
		t.Setenv("SPACES_SSH_KNOWN_HOSTS_FILE", "/tmp/env-known_hosts")
		got := resolveKnownHostsFile("")
		if got != "/tmp/env-known_hosts" {
			t.Fatalf("got %q, want env override", got)
		}
	})
	t.Run("default empty", func(t *testing.T) {
		got := resolveKnownHostsFile("")
		if got != "" {
			t.Fatalf("got %q, want empty default", got)
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

func writeKeyPair(t *testing.T, dir, name string) (privKey, pubKey string) {
	t.Helper()
	privKey = filepath.Join(dir, name)
	pubKey = privKey + ".pub"
	if err := os.WriteFile(privKey, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubKey, []byte("public"), 0o644); err != nil {
		t.Fatal(err)
	}
	return privKey, pubKey
}

func TestResolveSSHIdentityFileExplicit(t *testing.T) {
	privKey, pubKey := writeKeyPair(t, t.TempDir(), "id_test")
	gotPriv, gotPub, err := resolveSSHIdentityFile(privKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPriv != privKey || gotPub != pubKey {
		t.Fatalf("got (%q, %q), want (%q, %q)", gotPriv, gotPub, privKey, pubKey)
	}
}

func TestResolveSSHIdentityFileStripsPubSuffix(t *testing.T) {
	privKey, pubKey := writeKeyPair(t, t.TempDir(), "id_test")
	gotPriv, gotPub, err := resolveSSHIdentityFile(pubKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPriv != privKey || gotPub != pubKey {
		t.Fatalf("got (%q, %q), want (%q, %q)", gotPriv, gotPub, privKey, pubKey)
	}
}

func TestResolveSSHIdentityFileExplicitMissing(t *testing.T) {
	_, _, err := resolveSSHIdentityFile(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestResolveSSHIdentityFilePublicKeyMissing(t *testing.T) {
	dir := t.TempDir()
	privKey := filepath.Join(dir, "id_test")
	if err := os.WriteFile(privKey, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := resolveSSHIdentityFile(privKey)
	if err == nil {
		t.Fatal("expected error when public key is missing")
	}
}

func TestResolveSSHIdentityFileFallbackToHome(t *testing.T) {
	dir := t.TempDir()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	privKey, _ := writeKeyPair(t, sshDir, "id_ed25519")
	t.Setenv("HOME", dir)
	gotPriv, gotPub, err := resolveSSHIdentityFile("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPriv != privKey || gotPub != privKey+".pub" {
		t.Fatalf("got (%q, %q)", gotPriv, gotPub)
	}
}

func TestResolveSSHIdentityFileNoKeysFound(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	_, _, err := resolveSSHIdentityFile("")
	if err == nil {
		t.Fatal("expected error when no SSH keys found")
	}
	if !strings.Contains(err.Error(), "no SSH identity file was found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveSSHBinary(t *testing.T) {
	t.Run("env override", func(t *testing.T) {
		t.Setenv("SPACES_SSH_BIN", "/usr/local/bin/custom-ssh")
		got, err := resolveSSHBinary()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/usr/local/bin/custom-ssh" {
			t.Fatalf("got %q, want env override", got)
		}
	})
	t.Run("default from path", func(t *testing.T) {
		got, err := resolveSSHBinary()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == "" {
			t.Fatal("expected non-empty path")
		}
	})
}

func TestBuildSSHConnectArgsUsesStrictHostKeyChecking(t *testing.T) {
	args := buildSSHConnectArgs(sshConnectOptions{
		Port:           2222,
		KnownHostsFile: "/tmp/known_hosts",
		CertFile:       "/tmp/id_ed25519-cert.pub",
		IdentityFile:   "/tmp/id_ed25519",
		User:           "spaces-room",
		Host:           "cell.example.com",
		Target:         "sp_123",
	})

	joined := strings.Join(args, "\n")
	if !strings.Contains(joined, "StrictHostKeyChecking=yes") {
		t.Fatalf("args missing strict host key checking:\n%s", joined)
	}
	if strings.Contains(joined, "StrictHostKeyChecking=accept-new") {
		t.Fatalf("args still allow first-use trust:\n%s", joined)
	}
	if !strings.Contains(joined, "UserKnownHostsFile=/tmp/known_hosts") {
		t.Fatalf("args missing known hosts override:\n%s", joined)
	}
}

func TestRenderSSHClientConfigUsesStrictHostKeyChecking(t *testing.T) {
	config, err := renderSSHClientConfig(sshClientConfig{
		Alias:           "spaces-sp_123",
		Host:            "cell.example.com",
		User:            "spaces-room",
		Port:            22,
		IdentityFile:    "/tmp/id_ed25519",
		CertificateFile: "/tmp/id_ed25519-cert.pub",
		RoomID:          "sp_123",
		KnownHostsFile:  "/tmp/known_hosts",
	})
	if err != nil {
		t.Fatalf("renderSSHClientConfig returned error: %v", err)
	}

	if !strings.Contains(config, "  StrictHostKeyChecking yes\n") {
		t.Fatalf("config missing strict host key checking:\n%s", config)
	}
	if !strings.Contains(config, "  UserKnownHostsFile /tmp/known_hosts\n") {
		t.Fatalf("config missing known hosts override:\n%s", config)
	}
}

func TestRenderSSHClientConfigRejectsWhitespaceAndControlCharacters(t *testing.T) {
	_, err := renderSSHClientConfig(sshClientConfig{
		Alias:           "spaces-sp_123",
		Host:            "cell.example.com",
		User:            "spaces-room\nProxyCommand whoami",
		Port:            22,
		IdentityFile:    "/tmp/id_ed25519",
		CertificateFile: "/tmp/id_ed25519-cert.pub",
		RoomID:          "sp_123",
		KnownHostsFile:  "/tmp/known_hosts",
	})
	if err == nil || !strings.Contains(err.Error(), "whitespace or control characters") {
		t.Fatalf("renderSSHClientConfig error = %v, want whitespace/control validation", err)
	}
}

func TestValidateSSHRoomID(t *testing.T) {
	if _, err := validateSSHRoomID("sp_123"); err != nil {
		t.Fatalf("validateSSHRoomID rejected safe value: %v", err)
	}
	if _, err := validateSSHRoomID("sp_123;touch"); err == nil {
		t.Fatal("expected validateSSHRoomID to reject shell metacharacters")
	}
}

func TestRenderSSHClientConfigRejectsUnsafeRoomID(t *testing.T) {
	_, err := renderSSHClientConfig(sshClientConfig{
		Alias:           "spaces-sp_123",
		Host:            "cell.example.com",
		User:            "spaces-room",
		Port:            22,
		IdentityFile:    "/tmp/id_ed25519",
		CertificateFile: "/tmp/id_ed25519-cert.pub",
		RoomID:          "sp_123;touch",
		KnownHostsFile:  "/tmp/known_hosts",
	})
	if err == nil || !strings.Contains(err.Error(), "room ID must contain only") {
		t.Fatalf("renderSSHClientConfig error = %v, want room ID validation", err)
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
