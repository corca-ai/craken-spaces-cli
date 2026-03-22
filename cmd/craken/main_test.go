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

	sessionFile := filepath.Join(t.TempDir(), "session.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--base-url", server.server.URL, "--session-file", sessionFile, "auth", "login", "--email", "alice@example.com", "--key", "auth_test"}, &stdout, &stderr)
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
	t.Setenv("CRAKEN_BASE_URL", server.server.URL)

	sessionFile := filepath.Join(t.TempDir(), "session.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "auth", "login", "--email", "alice@example.com", "--key", "auth_test"}, &stdout, &stderr)
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

func TestWorkspaceIssueMemberAuthKey(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"issueWorkspaceMemberAuthKey": {
			Body: map[string]any{
				"ok": true,
				"auth_key": map[string]any{
					"id":                7,
					"workspace_id":      "ws_123",
					"workspace_name":    "alpha",
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

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "workspace", "issue-member-auth-key", "--workspace", "ws_123", "--email", "bob@example.com"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("workspace issue-member-auth-key code=%d stderr=%s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "wmauth_test") {
		t.Fatalf("stdout missing auth key: %s", got)
	}
}

func TestSSHConnectIssuesCertAndRunsLocalSSH(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"issueSSHCert": {
			Body: map[string]any{
				"ok":          true,
				"fingerprint": "SHA256:test",
				"principal":   "craken-cell",
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
	t.Setenv("CRAKEN_SSH_BIN", sshBin)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "connect", "--workspace", "ws_123", "--host", "cell.example.com", "--identity-file", identityFile, "--command", "echo hi"}, &stdout, &stderr)
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
	for _, needle := range []string{"-o", "CertificateFile=" + sshCertificateFileForIdentity(identityFile), "-i", identityFile, "craken-cell@cell.example.com", "ws_123 -- echo hi"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("ssh args missing %q:\n%s", needle, got)
		}
	}
}

func TestSSHClientConfigUsesEnvironmentBaseURLForHostResolution(t *testing.T) {
	t.Setenv("CRAKEN_BASE_URL", "https://agents-dev.borca.ai")

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{
		BaseURL:      "https://agents.borca.ai",
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
		"--workspace", "ws_123",
		"--identity-file", identityFile,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh client-config code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "HostName agents-dev.borca.ai") {
		t.Fatalf("stdout missing env-resolved host:\n%s", stdout.String())
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
