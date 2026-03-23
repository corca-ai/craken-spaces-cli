package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthLoginAndWhoAmI(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authLogin": {
			Body: map[string]any{
				"ok":            true,
				"email":         "alice@example.com",
				"session_token": "sess_test",
			},
			Assert: func(t *testing.T, _ *http.Request, body []byte) {
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				if payload["email"] != "alice@example.com" || payload["key"] != "auth_test" {
					t.Fatalf("unexpected login payload: %+v", payload)
				}
			},
		},
		"whoAmI": {
			Body: map[string]any{
				"ok": true,
				"user": map[string]any{
					"id":    1,
					"email": "alice@example.com",
					"name":  "Alice",
				},
			},
			Assert: func(t *testing.T, req *http.Request, _ []byte) {
				if got := req.Header.Get("Authorization"); got != "Bearer sess_test" {
					t.Fatalf("Authorization = %q", got)
				}
			},
		},
	})

	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")
	authKeyFile := writeAuthKeyFile(t, tmpDir, "auth_test")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--base-url", server.server.URL, "--session-file", sessionFile, "auth", "login", "--email", "alice@example.com", "--key-file", authKeyFile}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth login code=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	code = run([]string{"--session-file", sessionFile, "whoami"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("whoami code=%d stderr=%s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "alice@example.com" {
		t.Fatalf("whoami stdout=%q", got)
	}
}

func TestAuthLoginUsesEnvironmentBaseURL(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authLogin": {
			Body: map[string]any{
				"ok":            true,
				"email":         "alice@example.com",
				"session_token": "sess_test",
			},
		},
	})
	t.Setenv("SPACES_BASE_URL", server.server.URL)

	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")
	authKeyFile := writeAuthKeyFile(t, tmpDir, "auth_test")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "auth", "login", "--email", "alice@example.com", "--key-file", authKeyFile}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth login code=%d stderr=%s", code, stderr.String())
	}

	session, err := loadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.BaseURL != server.server.URL {
		t.Fatalf("saved session base URL = %#v, want %q", session, server.server.URL)
	}
}

func TestRoomIssueMemberAuthKey(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"issueSpaceMemberAuthKey": {
			Body: map[string]any{
				"ok": true,
				"auth_key": map[string]any{
					"id":                7,
					"space_id":          "sp_123",
					"space_name":        "alpha",
					"issued_by_user_id": 1,
					"issued_by_email":   "alice@example.com",
					"invitee_email":     "bob@example.com",
					"issued_at":         "2026-03-22T12:00:00Z",
					"expires_at":        "2026-03-30T00:00:00Z",
					"redeemed_at":       "",
					"revoked_at":        "",
					"cpu_millis":        1000,
					"memory_mib":        1024,
					"disk_mb":           1024,
					"network_egress_mb": 256,
					"llm_tokens_limit":  10000,
				},
				"key": "wmauth_test",
			},
			Assert: func(t *testing.T, req *http.Request, body []byte) {
				if got := req.Header.Get("Authorization"); got != "Bearer sess_test" {
					t.Fatalf("Authorization = %q", got)
				}
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				if payload["email"] != "bob@example.com" {
					t.Fatalf("unexpected issue-member-auth-key payload: %+v", payload)
				}
			},
		},
	})

	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}
	authKeyFile := filepath.Join(tmpDir, "issued.authkey")

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--session-file", sessionFile,
		"room", "issue-member-auth-key",
		"--room", "sp_123",
		"--email", "bob@example.com",
		"--auth-key-file", authKeyFile,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room issue-member-auth-key code=%d stderr=%s", code, stderr.String())
	}
	if got := stdout.String(); strings.Contains(got, "wmauth_test") {
		t.Fatalf("stdout leaked auth key: %s", got)
	}
	issuedKey, err := os.ReadFile(authKeyFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(issuedKey)) != "wmauth_test" {
		t.Fatalf("auth key file contents=%q", string(issuedKey))
	}
	info, err := os.Stat(authKeyFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("auth key file perms=%#o", info.Mode().Perm())
	}
}

func TestAuthLoginReadsKeyFromStdin(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authLogin": {
			Body: map[string]any{
				"ok":            true,
				"email":         "alice@example.com",
				"session_token": "sess_test",
			},
			Assert: func(t *testing.T, _ *http.Request, body []byte) {
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				if payload["key"] != "stdin_key" {
					t.Fatalf("unexpected key payload: %+v", payload)
				}
			},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	var stdout, stderr bytes.Buffer
	code := runWithStdin(
		[]string{"--base-url", server.server.URL, "--session-file", sessionFile, "auth", "login", "--email", "alice@example.com", "--key-stdin"},
		strings.NewReader("stdin_key\n"),
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("auth login code=%d stderr=%s", code, stderr.String())
	}
}

func TestAuthLoginRejectsInsecureKeyFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	sessionFile := filepath.Join(t.TempDir(), "session.json")

	code := run([]string{
		"--session-file", sessionFile,
		"auth", "login",
		"--email", "alice@example.com",
		"--key", "test-key",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when insecure --key is used")
	}
	if !strings.Contains(stderr.String(), "--key is insecure") {
		t.Fatalf("stderr missing insecure-flag guidance: %s", stderr.String())
	}
}

func TestSSHConnectIssuesCertAndRunsLocalSSH(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"issueSSHCert": {
			Body: map[string]any{
				"ok":          true,
				"fingerprint": "SHA256:test",
				"principal":   "spaces-room",
				"expires_at":  "2026-03-30T00:00:00Z",
				"certificate": "ssh-ed25519-cert-v01@openssh.com AAAATEST cert\n",
			},
		},
	})

	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	identityFile := filepath.Join(tmpDir, "id_ed25519")
	if err := os.WriteFile(identityFile, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identityFile+".pub", []byte("ssh-ed25519 AAAATEST alice@example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	sshArgsFile := filepath.Join(tmpDir, "ssh-args.txt")
	sshBin := filepath.Join(tmpDir, "fake-ssh.sh")
	if err := os.WriteFile(sshBin, []byte("#!/bin/sh\nprintf '%s\n' \"$@\" >\""+sshArgsFile+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SPACES_SSH_BIN", sshBin)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "connect", "--room", "sp_123", "--host", "cell.example.com", "--identity-file", identityFile, "--command", "echo hi"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh connect code=%d stderr=%s", code, stderr.String())
	}

	certData, err := os.ReadFile(sshCertificateFileForIdentity(identityFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(certData), "cert") {
		t.Fatalf("certificate file contents=%q", string(certData))
	}
	sshArgs, err := os.ReadFile(sshArgsFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(sshArgs)
	for _, needle := range []string{"-o", "StrictHostKeyChecking=yes", "CertificateFile=" + sshCertificateFileForIdentity(identityFile), "-i", identityFile, "spaces-room@cell.example.com", "sp_123 -- echo hi"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("ssh args missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "StrictHostKeyChecking=accept-new") {
		t.Fatalf("ssh args still allow first-use trust:\n%s", got)
	}
}

func TestSSHClientConfigUsesEnvironmentBaseURLForHostResolution(t *testing.T) {
	t.Setenv("SPACES_BASE_URL", "https://staging.example.test")

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{
		BaseURL:      "https://spaces.borca.ai",
		Email:        "alice@example.com",
		SessionToken: "sess_test",
	}); err != nil {
		t.Fatal(err)
	}

	identityFile := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(identityFile, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--session-file", sessionFile,
		"ssh", "client-config",
		"--room", "sp_123",
		"--identity-file", identityFile,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh client-config code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "HostName staging.example.test") {
		t.Fatalf("stdout missing env-resolved host:\n%s", stdout.String())
	}
}

func TestAuthLoginRequiresEmailAndKey(t *testing.T) {
	var stdout, stderr bytes.Buffer
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")
	authKeyFile := writeAuthKeyFile(t, tmpDir, "test-key")

	// Missing key source
	code := run([]string{"--session-file", sessionFile, "auth", "login", "--email", "alice@example.com"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when auth key source is missing")
	}

	stdout.Reset()
	stderr.Reset()

	// Missing --email
	code = run([]string{"--session-file", sessionFile, "auth", "login", "--key-file", authKeyFile}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --email is missing")
	}
}

func TestRoomCreateRequiresName(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authLogin": {
			Body: map[string]any{"ok": true, "email": "alice@example.com", "session_token": "sess_test"},
		},
	})
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "create"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --name is missing")
	}
}

func TestRoomUpRequiresRoomFlag(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "up"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --room is missing")
	}
	if !strings.Contains(stderr.String(), "--room is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestRoomCreateListUpDownDelete(t *testing.T) {
	spaceBody := map[string]any{
		"id": "sp_1", "name": "test-room", "role": "admin",
		"owner_user_id":  1,
		"runtime_driver": "mock", "runtime_state": "stopped", "runtime_meta": "",
		"cpu_millis": 4000, "memory_mib": 8192, "disk_mb": 10240,
		"network_egress_mb": 1024, "llm_tokens_used": 0, "llm_tokens_limit": 100000,
		"actor_cpu_millis": 4000, "actor_memory_mib": 8192, "actor_disk_mb": 10240,
		"actor_network_mb": 1024, "actor_llm_tokens": 100000, "byok_bytes_used": 0,
		"created_at": "2026-01-01T00:00:00Z",
	}
	server := newContractFakeServer(t, map[string]fakeOperation{
		"createSpace": {
			Body: map[string]any{"ok": true, "space": spaceBody},
		},
		"listSpaces": {
			Body: map[string]any{
				"ok":     true,
				"spaces": []any{spaceBody},
			},
		},
		"startSpace": {
			Body: map[string]any{
				"ok": true,
				"space": map[string]any{
					"id": "sp_1", "name": "test-room", "role": "admin",
					"owner_user_id":  1,
					"runtime_driver": "mock", "runtime_state": "running", "runtime_meta": "",
					"cpu_millis": 4000, "memory_mib": 8192, "disk_mb": 10240,
					"network_egress_mb": 1024, "llm_tokens_used": 0, "llm_tokens_limit": 100000,
					"actor_cpu_millis": 4000, "actor_memory_mib": 8192, "actor_disk_mb": 10240,
					"actor_network_mb": 1024, "actor_llm_tokens": 100000, "byok_bytes_used": 0,
					"created_at": "2026-01-01T00:00:00Z",
				},
			},
		},
		"stopSpace": {
			Body: map[string]any{
				"ok": true,
				"space": map[string]any{
					"id": "sp_1", "name": "test-room", "role": "admin",
					"owner_user_id":  1,
					"runtime_driver": "mock", "runtime_state": "stopped", "runtime_meta": "",
					"cpu_millis": 4000, "memory_mib": 8192, "disk_mb": 10240,
					"network_egress_mb": 1024, "llm_tokens_used": 0, "llm_tokens_limit": 100000,
					"actor_cpu_millis": 4000, "actor_memory_mib": 8192, "actor_disk_mb": 10240,
					"actor_network_mb": 1024, "actor_llm_tokens": 100000, "byok_bytes_used": 0,
					"created_at": "2026-01-01T00:00:00Z",
				},
			},
		},
		"deleteSpace": {
			Body: map[string]any{"ok": true},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	// Create
	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "create", "--name", "test-room"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room create code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created room") {
		t.Fatalf("stdout missing 'created room': %s", stdout.String())
	}

	// List
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "room", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room list code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "test-room") {
		t.Fatalf("stdout missing 'test-room': %s", stdout.String())
	}

	// Up
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "room", "up", "--room", "sp_1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room up code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "is running") {
		t.Fatalf("stdout missing 'is running': %s", stdout.String())
	}

	// Down
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "room", "down", "--room", "sp_1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room down code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "is stopped") {
		t.Fatalf("stdout missing 'is stopped': %s", stdout.String())
	}

	// Delete
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "room", "delete", "--room", "sp_1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room delete code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deleted room") {
		t.Fatalf("stdout missing 'deleted room': %s", stdout.String())
	}
}

func TestSSHAddListRemoveKeys(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"addSSHKey": {
			Body: map[string]any{
				"ok": true,
				"key": map[string]any{
					"id": 1, "user_id": 1, "user_email": "alice@example.com",
					"name": "my-laptop", "public_key": "ssh-ed25519 AAAATEST alice@example.com",
					"fingerprint": "SHA256:test", "created_at": "2026-01-01T00:00:00Z",
				},
			},
		},
		"listSSHKeys": {
			Body: map[string]any{
				"ok": true,
				"keys": []any{
					map[string]any{
						"id": 1, "user_id": 1, "user_email": "alice@example.com",
						"name": "my-laptop", "public_key": "ssh-ed25519 AAAATEST alice@example.com",
						"fingerprint": "SHA256:test", "created_at": "2026-01-01T00:00:00Z",
					},
				},
			},
		},
		"removeSSHKey": {
			Body: map[string]any{"ok": true},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	// Add key
	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "add-key", "--name", "my-laptop", "--public-key", "ssh-ed25519 AAAATEST alice@example.com"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh add-key code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "registered ssh key") {
		t.Fatalf("stdout missing 'registered ssh key': %s", stdout.String())
	}

	// List keys
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "ssh", "list-keys"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh list-keys code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "my-laptop") {
		t.Fatalf("stdout missing 'my-laptop': %s", stdout.String())
	}

	// Remove key
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "ssh", "remove-key", "--fingerprint", "SHA256:test"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh remove-key code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "removed ssh key") {
		t.Fatalf("stdout missing 'removed ssh key': %s", stdout.String())
	}
}

func TestSSHIssueCert(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"issueSSHCert": {
			Body: map[string]any{
				"ok":          true,
				"fingerprint": "SHA256:test",
				"principal":   "spaces-room",
				"expires_at":  "2026-03-30T00:00:00Z",
				"certificate": "ssh-ed25519-cert-v01@openssh.com AAAATEST cert\n",
			},
		},
	})

	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	identityFile := filepath.Join(tmpDir, "id_ed25519")
	if err := os.WriteFile(identityFile, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identityFile+".pub", []byte("ssh-ed25519 AAAATEST alice@example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "issue-cert", "--identity-file", identityFile}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh issue-cert code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "issued ssh certificate") {
		t.Fatalf("stdout missing 'issued ssh certificate': %s", stdout.String())
	}

	certFile := sshCertificateFileForIdentity(identityFile)
	certData, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("cert file not written: %v", err)
	}
	if !strings.Contains(string(certData), "cert") {
		t.Fatalf("cert file contents=%q", string(certData))
	}
}

func TestAuthLogout(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authLogin": {
			Body: map[string]any{
				"ok":            true,
				"email":         "alice@example.com",
				"session_token": "sess_test",
			},
		},
		"authLogout": {
			Body: map[string]any{"ok": true},
		},
	})

	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.json")
	authKeyFile := writeAuthKeyFile(t, tmpDir, "auth_test")

	// Login first
	var stdout, stderr bytes.Buffer
	code := run([]string{"--base-url", server.server.URL, "--session-file", sessionFile, "auth", "login", "--email", "alice@example.com", "--key-file", authKeyFile}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth login code=%d stderr=%s", code, stderr.String())
	}

	// Logout
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "auth", "logout"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth logout code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "logged out") {
		t.Fatalf("stdout missing 'logged out': %s", stdout.String())
	}
}

func TestAuthLogoutKeepsSessionWhenRemoteLogoutFails(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authLogout": {
			Status: http.StatusBadRequest,
			Body:   map[string]any{"ok": false, "error": "logout failed"},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{
		BaseURL:      server.server.URL,
		Email:        "alice@example.com",
		SessionToken: "sess_test",
	}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "auth", "logout"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when remote logout fails")
	}
	if strings.Contains(stdout.String(), "logged out") {
		t.Fatalf("stdout incorrectly reported success: %s", stdout.String())
	}
	session, err := loadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.SessionToken != "sess_test" {
		t.Fatalf("session should have been preserved, got %#v", session)
	}
	if !strings.Contains(stderr.String(), "local session kept") {
		t.Fatalf("stderr missing preservation warning: %s", stderr.String())
	}
}

func TestVersionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("version code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "spaces ") {
		t.Fatalf("stdout missing version prefix: %s", stdout.String())
	}
}

func TestHelpCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout missing usage text: %s", stdout.String())
	}
}

func TestNoArgsShowsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "bogus"`) {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestAuthUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"auth", "bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unknown auth subcommand "bogus"`) {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestAuthHelpAndNoArgs(t *testing.T) {
	t.Run("no args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"auth"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d", code)
		}
	})
	t.Run("help", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"auth", "help"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	})
	t.Run("-h", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"auth", "-h"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	})
	t.Run("--help", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"auth", "--help"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	})
}

func TestSSHUnknownSubcommand(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unknown ssh subcommand "bogus"`) {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSSHHelpAndNoArgs(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	t.Run("no args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--session-file", sessionFile, "ssh"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d", code)
		}
	})
	t.Run("help", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--session-file", sessionFile, "ssh", "help"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	})
}

func TestSSHRequiresAuth(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "list-keys"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not authenticated") {
		t.Fatalf("stderr missing auth error: %s", stderr.String())
	}
}

func TestSSHRemoveKeyRequiresFingerprint(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "remove-key"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --fingerprint is missing")
	}
	if !strings.Contains(stderr.String(), "--fingerprint is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSSHConnectRequiresRoom(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "connect"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --room is missing")
	}
	if !strings.Contains(stderr.String(), "--room is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSSHClientConfigRequiresRoom(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "client-config"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --room is missing")
	}
	if !strings.Contains(stderr.String(), "--room is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestWhoAmIRequiresAuth(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "whoami"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not authenticated") {
		t.Fatalf("stderr missing auth error: %s", stderr.String())
	}
}

func TestSubcommandHelpWithoutAuth(t *testing.T) {
	// No session file — help should still work for all subcommands.
	sessionFile := filepath.Join(t.TempDir(), "session.json")

	subcmds := [][]string{
		{"room", "create", "-h"},
		{"room", "up", "-h"},
		{"room", "down", "-h"},
		{"room", "delete", "-h"},
		{"room", "issue-member-auth-key", "-h"},
		{"room", "member-auth-keys", "-h"},
		{"room", "revoke-member-auth-key", "-h"},
		{"ssh", "add-key", "-h"},
		{"ssh", "remove-key", "-h"},
		{"ssh", "issue-cert", "-h"},
		{"ssh", "connect", "-h"},
		{"ssh", "client-config", "-h"},
	}
	for _, sub := range subcmds {
		name := strings.Join(sub, " ")
		t.Run(name, func(t *testing.T) {
			args := append([]string{"--session-file", sessionFile}, sub...)
			var stdout, stderr bytes.Buffer
			code := run(args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("%s: code=%d stderr=%s", name, code, stderr.String())
			}
		})
	}
}

func TestContainsHelpFlag(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"create", "-h"}, true},
		{[]string{"create", "--help"}, true},
		{[]string{"create", "-help"}, true},
		{[]string{"create", "--name", "foo"}, false},
		{[]string{"create", "--", "-h"}, false}, // -h after -- is not a help flag
		{nil, false},
	}
	for _, tc := range tests {
		got := containsHelpFlag(tc.args)
		if got != tc.want {
			t.Errorf("containsHelpFlag(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestProtocolFileMatchesManagedAgentsWhenPresent(t *testing.T) {
	localPath := filepath.Join("..", "..", "protocol", "public-api-v1.openapi.yaml")
	managedPath := filepath.Join("..", "..", "..", "craken-managed-agents", "protocol", "public-api-v1.openapi.yaml")
	if _, err := os.Stat(managedPath); os.IsNotExist(err) {
		t.Skip("sibling craken-managed-agents checkout not present")
	}
	localData, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	managedData, err := os.ReadFile(managedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(localData, managedData) {
		t.Fatalf("public API contract is out of sync with sibling managed-agents checkout")
	}
}
