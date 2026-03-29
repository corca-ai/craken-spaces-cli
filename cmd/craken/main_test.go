package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
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
	originalHomeLookup := lookupUserHomeDir
	lookupUserHomeDir = func() (string, error) {
		return tmpDir, nil
	}
	t.Cleanup(func() {
		lookupUserHomeDir = originalHomeLookup
	})
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

func TestRootLoginAliasAcceptsPositionalEmail(t *testing.T) {
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
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	authKeyFile := writeAuthKeyFile(t, t.TempDir(), "auth_test")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--base-url", server.server.URL, "--session-file", sessionFile, "login", "alice@example.com", "--key-file", authKeyFile}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("login code=%d stderr=%s", code, stderr.String())
	}

	session, err := loadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.Email != "alice@example.com" {
		t.Fatalf("saved session = %#v", session)
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

func TestAuthLoginPromptsForKeyOnInteractiveTerminal(t *testing.T) {
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
				if payload["key"] != "prompted_key" {
					t.Fatalf("unexpected key payload: %+v", payload)
				}
			},
		},
	})

	origIsTerminal := isTerminalFD
	origReadMasked := readMaskedTerminalKeyFD
	t.Cleanup(func() {
		isTerminalFD = origIsTerminal
		readMaskedTerminalKeyFD = origReadMasked
	})

	isTerminalFD = func(fd int) bool { return fd == 99 }
	readMaskedTerminalKeyFD = func(fd int, prompt string, sink io.Writer) ([]byte, error) {
		if fd != 99 {
			t.Fatalf("fd=%d, want 99", fd)
		}
		if prompt != "Auth key: " {
			t.Fatalf("prompt=%q, want Auth key: ", prompt)
		}
		_, _ = sink.Write([]byte("Auth key: ***\n"))
		return []byte("prompted_key\n"), nil
	}

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	var stdout, stderr bytes.Buffer
	code := runWithStdin(
		[]string{"--base-url", server.server.URL, "--session-file", sessionFile, "auth", "login", "--email", "alice@example.com"},
		fakeTerminalInput{Reader: strings.NewReader("ignored"), fd: 99},
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("auth login code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Auth key: ***") {
		t.Fatalf("stderr missing masked prompt: %s", stderr.String())
	}
}

func TestAuthLoginRejectsUnknownKeyFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	sessionFile := filepath.Join(t.TempDir(), "session.json")

	code := run([]string{
		"--session-file", sessionFile,
		"auth", "login",
		"--email", "alice@example.com",
		"--key", "test-key",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when unknown --key is used")
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined: -key") {
		t.Fatalf("stderr missing unknown-flag message: %s", stderr.String())
	}
}

func TestSSHConnectIssuesCertAndRunsLocalSSH(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"listSpaces": {
			Body: map[string]any{
				"ok": true,
				"spaces": []any{
					map[string]any{
						"id":                "sp_123",
						"name":              "alpha",
						"role":              "admin",
						"owner_user_id":     1,
						"created_at":        "2026-01-01T00:00:00Z",
						"cpu_millis":        4000,
						"memory_mib":        8192,
						"disk_mb":           10240,
						"network_egress_mb": 1024,
						"llm_tokens_limit":  100000,
						"llm_tokens_used":   0,
						"actor_cpu_millis":  4000,
						"actor_memory_mib":  8192,
						"actor_disk_mb":     10240,
						"actor_network_mb":  1024,
						"actor_llm_tokens":  100000,
						"guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
						"runtime_state":     "running",
						"runtime_meta":      "",
					},
				},
			},
		},
		"listSSHKeys": {
			Body: map[string]any{
				"ok":   true,
				"keys": []any{},
			},
		},
		"addSSHKey": {
			Body: map[string]any{
				"ok": true,
				"key": map[string]any{
					"id":          1,
					"user_id":     1,
					"user_email":  "alice@example.com",
					"name":        "id_ed25519",
					"public_key":  "ssh-ed25519 AAAATEST alice@example.com",
					"fingerprint": "SHA256:test",
					"created_at":  "2026-01-01T00:00:00Z",
				},
			},
		},
		"sshKnownHosts": {
			Body: map[string]any{
				"ok":               true,
				"host":             "cell.example.com",
				"port":             22,
				"public_key":       "ssh-ed25519 AAAA_FAKE_HOST_KEY",
				"known_hosts_line": "cell.example.com ssh-ed25519 AAAA_FAKE_HOST_KEY",
			},
		},
		"issueSSHCert": {
			Body: map[string]any{
				"ok":          true,
				"fingerprint": "SHA256:test",
				"principal":   "spaces-user",
				"expires_at":  "2026-03-30T00:00:00Z",
				"certificate": "ssh-ed25519-cert-v01@openssh.com AAAATEST cert\n",
			},
			Assert: func(t *testing.T, req *http.Request, body []byte) {
				if got := req.Header.Get("Authorization"); got != "Bearer sess_test" {
					t.Fatalf("Authorization = %q", got)
				}
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				if payload["principal"] != "spaces-user" {
					t.Fatalf("principal = %#v, want spaces-user", payload["principal"])
				}
				if payload["cert_ttl"] != "5m" {
					t.Fatalf("cert_ttl = %#v, want 5m", payload["cert_ttl"])
				}
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
	code := run([]string{"--session-file", sessionFile, "ssh", "connect", "--space", "alpha", "--host", "cell.example.com", "--identity-file", identityFile, "--command", "echo hi"}, &stdout, &stderr)
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
	for _, needle := range []string{"-o", "StrictHostKeyChecking=yes", "CertificateFile=" + sshCertificateFileForIdentity(identityFile), "-i", identityFile, "spaces-user@cell.example.com", "sp_123 -- echo hi"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("ssh args missing %q:\n%s", needle, got)
		}
	}
	if strings.Contains(got, "StrictHostKeyChecking=accept-new") {
		t.Fatalf("ssh args still allow first-use trust:\n%s", got)
	}
	session, err := loadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.DefaultSpace != "sp_123" {
		t.Fatalf("DefaultSpace = %#v, want sp_123", session)
	}
	const knownHostsPrefix = "UserKnownHostsFile="
	index := strings.Index(got, knownHostsPrefix)
	if index < 0 {
		t.Fatalf("ssh args missing managed known_hosts path:\n%s", got)
	}
	rest := got[index+len(knownHostsPrefix):]
	end := strings.IndexByte(rest, '\n')
	if end >= 0 {
		rest = rest[:end]
	}
	knownHostsPath := strings.TrimSpace(rest)
	knownHostsData, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(knownHostsData), "cell.example.com ssh-ed25519 AAAA_FAKE_HOST_KEY") {
		t.Fatalf("known_hosts missing fetched host key:\n%s", string(knownHostsData))
	}
}

func TestRootConnectUsesOnlyVisibleSpaceAndPersistsDefault(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"listSpaces": {
			Body: map[string]any{
				"ok": true,
				"spaces": []any{
					map[string]any{
						"id":                "sp_123",
						"name":              "alpha",
						"role":              "admin",
						"owner_user_id":     1,
						"created_at":        "2026-01-01T00:00:00Z",
						"cpu_millis":        4000,
						"memory_mib":        8192,
						"disk_mb":           10240,
						"network_egress_mb": 1024,
						"llm_tokens_limit":  100000,
						"llm_tokens_used":   0,
						"actor_cpu_millis":  4000,
						"actor_memory_mib":  8192,
						"actor_disk_mb":     10240,
						"actor_network_mb":  1024,
						"actor_llm_tokens":  100000,
						"guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
						"runtime_state":     "running",
						"runtime_meta":      "",
					},
				},
			},
		},
		"listSSHKeys": {
			Body: map[string]any{
				"ok":   true,
				"keys": []any{},
			},
		},
		"addSSHKey": {
			Body: map[string]any{
				"ok": true,
				"key": map[string]any{
					"id":          1,
					"user_id":     1,
					"user_email":  "alice@example.com",
					"name":        "id_ed25519",
					"public_key":  "ssh-ed25519 AAAATEST alice@example.com",
					"fingerprint": "SHA256:test",
					"created_at":  "2026-01-01T00:00:00Z",
				},
			},
		},
		"sshKnownHosts": {
			Body: map[string]any{
				"ok":               true,
				"host":             "cell.example.com",
				"port":             22,
				"public_key":       "ssh-ed25519 AAAA_FAKE_HOST_KEY",
				"known_hosts_line": "cell.example.com ssh-ed25519 AAAA_FAKE_HOST_KEY",
			},
		},
		"issueSSHCert": {
			Body: map[string]any{
				"ok":          true,
				"fingerprint": "SHA256:test",
				"principal":   "spaces-user",
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
	code := run([]string{"--session-file", sessionFile, "connect", "--host", "cell.example.com", "--identity-file", identityFile}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("connect code=%d stderr=%s", code, stderr.String())
	}

	sshArgs, err := os.ReadFile(sshArgsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sshArgs), "sp_123") {
		t.Fatalf("ssh args missing resolved default target:\n%s", string(sshArgs))
	}
	session, err := loadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.DefaultSpace != "sp_123" {
		t.Fatalf("DefaultSpace = %#v, want sp_123", session)
	}
}

func TestSSHConnectGeneratesManagedIdentityWhenMissing(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"listSSHKeys": {
			Body: map[string]any{"ok": true, "keys": []any{}},
		},
		"addSSHKey": {
			Body: map[string]any{
				"ok": true,
				"key": map[string]any{
					"id":          1,
					"user_id":     1,
					"user_email":  "alice@example.com",
					"name":        defaultManagedSSHIdentityName,
					"public_key":  "ssh-ed25519 AAAA_GENERATED spaces@test\n",
					"fingerprint": "SHA256:generated",
					"created_at":  "2026-01-01T00:00:00Z",
				},
			},
		},
		"sshKnownHosts": {
			Body: map[string]any{
				"ok":               true,
				"host":             "cell.example.com",
				"port":             22,
				"public_key":       "ssh-ed25519 AAAA_FAKE_HOST_KEY",
				"known_hosts_line": "cell.example.com ssh-ed25519 AAAA_FAKE_HOST_KEY",
			},
		},
		"issueSSHCert": {
			Body: map[string]any{
				"ok":          true,
				"fingerprint": "SHA256:generated",
				"principal":   "spaces-user",
				"expires_at":  "2026-03-30T00:00:00Z",
				"certificate": "ssh-ed25519-cert-v01@openssh.com AAAA_GENERATED cert\n",
			},
		},
	})

	tmpDir := t.TempDir()
	originalHomeLookup := lookupUserHomeDir
	lookupUserHomeDir = func() (string, error) {
		return tmpDir, nil
	}
	t.Cleanup(func() {
		lookupUserHomeDir = originalHomeLookup
	})

	sessionFile := filepath.Join(tmpDir, "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sshKeygen := filepath.Join(binDir, "ssh-keygen")
	sshKeygenScript := "#!/bin/sh\nset -eu\nout=\nwhile [ \"$#\" -gt 0 ]; do\n  if [ \"$1\" = \"-f\" ]; then\n    out=\"$2\"\n    shift 2\n    continue\n  fi\n  shift\n done\nprintf 'private\\n' >\"$out\"\nprintf 'ssh-ed25519 AAAA_GENERATED spaces@test\\n' >\"${out}.pub\"\n"
	if err := os.WriteFile(sshKeygen, []byte(sshKeygenScript), 0o755); err != nil {
		t.Fatal(err)
	}
	sshArgsFile := filepath.Join(tmpDir, "ssh-args.txt")
	sshBin := filepath.Join(binDir, "ssh")
	if err := os.WriteFile(sshBin, []byte("#!/bin/sh\nprintf '%s\n' \"$@\" >\""+sshArgsFile+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "connect", "--space", "sp_123", "--host", "cell.example.com"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh connect code=%d stderr=%s", code, stderr.String())
	}

	identityFile := filepath.Join(tmpDir, ".ssh", defaultManagedSSHIdentityName)
	if _, err := os.Stat(identityFile); err != nil {
		t.Fatalf("managed identity missing: %v", err)
	}
	if _, err := os.Stat(identityFile + ".pub"); err != nil {
		t.Fatalf("managed public key missing: %v", err)
	}
	if _, err := os.Stat(sshCertificateFileForIdentity(identityFile)); err != nil {
		t.Fatalf("managed cert missing: %v", err)
	}
	sshArgs, err := os.ReadFile(sshArgsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sshArgs), identityFile) {
		t.Fatalf("ssh args missing managed identity:\n%s", string(sshArgs))
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
		"--space", "sp_123",
		"--identity-file", identityFile,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh client-config code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "HostName staging.example.test") {
		t.Fatalf("stdout missing env-resolved host:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: using https://staging.example.test from SPACES_BASE_URL") {
		t.Fatalf("stderr missing origin mismatch warning:\n%s", stderr.String())
	}
}

func TestSSHClientConfigAcceptsExactSpaceName(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"listSpaces": {
			Body: map[string]any{
				"ok": true,
				"spaces": []any{
					map[string]any{
						"id":                "sp_123",
						"name":              "alpha",
						"role":              "admin",
						"owner_user_id":     1,
						"created_at":        "2026-01-01T00:00:00Z",
						"cpu_millis":        4000,
						"memory_mib":        8192,
						"disk_mb":           10240,
						"network_egress_mb": 1024,
						"llm_tokens_limit":  100000,
						"llm_tokens_used":   0,
						"actor_cpu_millis":  4000,
						"actor_memory_mib":  8192,
						"actor_disk_mb":     10240,
						"actor_network_mb":  1024,
						"actor_llm_tokens":  100000,
						"guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
						"runtime_state":     "running",
						"runtime_meta":      "",
					},
				},
			},
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

	identityFile := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(identityFile, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--session-file", sessionFile,
		"ssh", "client-config",
		"--space", "alpha",
		"--identity-file", identityFile,
		"--host", "cell.example.com",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh client-config code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "RemoteCommand sp_123") {
		t.Fatalf("stdout missing resolved space id:\n%s", stdout.String())
	}
}

func TestSSHClientConfigAllowsExplicitBaseURLOverrideForHostResolution(t *testing.T) {
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
		"--base-url", "https://staging.example.test",
		"--session-file", sessionFile,
		"ssh", "client-config",
		"--space", "sp_123",
		"--identity-file", identityFile,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("ssh client-config code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "HostName staging.example.test") {
		t.Fatalf("stdout missing explicit-override host:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: using https://staging.example.test from --base-url") {
		t.Fatalf("stderr missing origin mismatch warning:\n%s", stderr.String())
	}
}

func TestSSHClientConfigRejectsUnsafeUserFromEnvironment(t *testing.T) {
	t.Setenv("SPACES_SSH_LOGIN_USER", "spaces-room\nProxyCommand whoami")

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
		"--space", "sp_123",
		"--identity-file", identityFile,
		"--host", "cell.example.com",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected ssh client-config to reject unsafe user, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "whitespace or control characters") {
		t.Fatalf("stderr missing validation error: %s", stderr.String())
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

func TestSpaceCreateRequiresName(t *testing.T) {
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
	code := run([]string{"--session-file", sessionFile, "space", "create"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --name is missing")
	}
}

func TestSpaceUpRequiresSpaceFlag(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "up"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --space is missing")
	}
	if !strings.Contains(stderr.String(), "--space is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSpaceCreateListUpDownDelete(t *testing.T) {
	spaceBody := map[string]any{
		"id": "sp_1", "name": "test-room", "role": "admin",
		"owner_user_id": 1,
		"runtime_state": "stopped", "runtime_meta": "",
		"cpu_millis": 4000, "memory_mib": 8192, "disk_mb": 10240,
		"network_egress_mb": 1024, "llm_tokens_used": 0, "llm_tokens_limit": 100000,
		"actor_cpu_millis": 4000, "actor_memory_mib": 8192, "actor_disk_mb": 10240,
		"actor_network_mb": 1024, "actor_llm_tokens": 100000, "guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
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
					"owner_user_id": 1,
					"runtime_state": "running", "runtime_meta": "",
					"cpu_millis": 4000, "memory_mib": 8192, "disk_mb": 10240,
					"network_egress_mb": 1024, "llm_tokens_used": 0, "llm_tokens_limit": 100000,
					"actor_cpu_millis": 4000, "actor_memory_mib": 8192, "actor_disk_mb": 10240,
					"actor_network_mb": 1024, "actor_llm_tokens": 100000, "guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
					"created_at": "2026-01-01T00:00:00Z",
				},
			},
		},
		"stopSpace": {
			Body: map[string]any{
				"ok": true,
				"space": map[string]any{
					"id": "sp_1", "name": "test-room", "role": "admin",
					"owner_user_id": 1,
					"runtime_state": "stopped", "runtime_meta": "",
					"cpu_millis": 4000, "memory_mib": 8192, "disk_mb": 10240,
					"network_egress_mb": 1024, "llm_tokens_used": 0, "llm_tokens_limit": 100000,
					"actor_cpu_millis": 4000, "actor_memory_mib": 8192, "actor_disk_mb": 10240,
					"actor_network_mb": 1024, "actor_llm_tokens": 100000, "guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
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
	code := run([]string{"--session-file", sessionFile, "space", "create", "--name", "test-room"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space create code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created space") {
		t.Fatalf("stdout missing 'created space': %s", stdout.String())
	}
	session, err := loadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.DefaultSpace != "sp_1" {
		t.Fatalf("DefaultSpace = %#v, want sp_1", session)
	}

	// List
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "space", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space list code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "test-room") {
		t.Fatalf("stdout missing 'test-room': %s", stdout.String())
	}

	// Up
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "space", "up", "--space", "sp_1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space up code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "is running") {
		t.Fatalf("stdout missing 'is running': %s", stdout.String())
	}

	// Down
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "space", "down", "--space", "sp_1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space down code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "is stopped") {
		t.Fatalf("stdout missing 'is stopped': %s", stdout.String())
	}

	// Delete
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "space", "delete", "--space", "sp_1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space delete code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deleted space") {
		t.Fatalf("stdout missing 'deleted space': %s", stdout.String())
	}
}

func TestRootCreateAndListShortcuts(t *testing.T) {
	spaceBody := map[string]any{
		"id": "sp_1", "name": "shortcut-room", "role": "admin",
		"owner_user_id": 1,
		"runtime_state": "running", "runtime_meta": "",
		"cpu_millis": 4000, "memory_mib": 8192, "disk_mb": 10240,
		"network_egress_mb": 1024, "llm_tokens_used": 0, "llm_tokens_limit": 100000,
		"actor_cpu_millis": 4000, "actor_memory_mib": 8192, "actor_disk_mb": 10240,
		"actor_network_mb": 1024, "actor_llm_tokens": 100000, "guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
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
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "create", "shortcut-room"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("create code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created space sp_1 (shortcut-room)") {
		t.Fatalf("stdout missing created space output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--session-file", sessionFile, "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("list code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "shortcut-room") {
		t.Fatalf("stdout missing shortcut-room: %s", stdout.String())
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
		"listSSHKeys": {
			Body: map[string]any{
				"ok":   true,
				"keys": []any{},
			},
		},
		"addSSHKey": {
			Body: map[string]any{
				"ok": true,
				"key": map[string]any{
					"id":          1,
					"user_id":     1,
					"user_email":  "alice@example.com",
					"name":        "id_ed25519",
					"public_key":  "ssh-ed25519 AAAATEST alice@example.com",
					"fingerprint": "SHA256:test",
					"created_at":  "2026-01-01T00:00:00Z",
				},
			},
		},
		"issueSSHCert": {
			Body: map[string]any{
				"ok":          true,
				"fingerprint": "SHA256:test",
				"principal":   "spaces-user",
				"expires_at":  "2026-03-30T00:00:00Z",
				"certificate": "ssh-ed25519-cert-v01@openssh.com AAAATEST cert\n",
			},
			Assert: func(t *testing.T, req *http.Request, body []byte) {
				if got := req.Header.Get("Authorization"); got != "Bearer sess_test" {
					t.Fatalf("Authorization = %q", got)
				}
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				if payload["principal"] != "spaces-user" {
					t.Fatalf("principal = %#v, want spaces-user", payload["principal"])
				}
				if payload["cert_ttl"] != "5m" {
					t.Fatalf("cert_ttl = %#v, want 5m", payload["cert_ttl"])
				}
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

func TestAuthLogoutRemovesSessionWhenRemoteLogoutFails(t *testing.T) {
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
	if session != nil {
		t.Fatalf("session should have been removed, got %#v", session)
	}
	if !strings.Contains(stderr.String(), "local session removed") {
		t.Fatalf("stderr missing removal warning: %s", stderr.String())
	}
}

func TestAuthLogoutRemovesSessionWhenSavedBaseURLIsInvalid(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{
		BaseURL:      "http://example.com",
		Email:        "alice@example.com",
		SessionToken: "sess_test",
	}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "auth", "logout"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when saved base URL is invalid")
	}
	session, err := loadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if session != nil {
		t.Fatalf("session should have been removed, got %#v", session)
	}
	if !strings.Contains(stderr.String(), "remote logout was skipped") {
		t.Fatalf("stderr missing skipped-remote warning: %s", stderr.String())
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
	if !strings.Contains(stdout.String(), "Shortcut Commands:") || !strings.Contains(stdout.String(), "\nCommands:\n") {
		t.Fatalf("stdout missing section headers:\n%s", stdout.String())
	}
	loginIndex := strings.Index(stdout.String(), "\n  login EMAIL")
	createIndex := strings.Index(stdout.String(), "\n  create SPACE")
	listIndex := strings.Index(stdout.String(), "\n  list")
	connectIndex := strings.Index(stdout.String(), "\n  connect [SPACE]")
	authIndex := strings.Index(stdout.String(), "\n  auth login")
	sshIndex := strings.Index(stdout.String(), "\n  ssh connect")
	shortcutHeaderIndex := strings.Index(stdout.String(), "\nShortcut Commands:\n")
	commandsHeaderIndex := strings.Index(stdout.String(), "\nCommands:\n")
	if loginIndex < 0 || createIndex < 0 || listIndex < 0 || connectIndex < 0 || authIndex < 0 || sshIndex < 0 {
		t.Fatalf("stdout missing expected commands:\n%s", stdout.String())
	}
	if shortcutHeaderIndex >= loginIndex || loginIndex >= createIndex || createIndex >= listIndex || listIndex >= connectIndex || connectIndex >= commandsHeaderIndex || commandsHeaderIndex >= authIndex || authIndex >= sshIndex {
		t.Fatalf("help sections or ordering are wrong:\n%s", stdout.String())
	}
}

func TestHelpCommandDoesNotRequireDefaultSessionPath(t *testing.T) {
	original := lookupUserHomeDir
	lookupUserHomeDir = func() (string, error) {
		return "", errors.New("no home")
	}
	t.Cleanup(func() {
		lookupUserHomeDir = original
	})

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

func TestSSHConnectRequiresExplicitSpaceWhenMultipleSpacesVisible(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"listSpaces": {
			Body: map[string]any{
				"ok": true,
				"spaces": []any{
					map[string]any{
						"id":                "sp_123",
						"name":              "alpha",
						"role":              "admin",
						"owner_user_id":     1,
						"created_at":        "2026-01-01T00:00:00Z",
						"cpu_millis":        4000,
						"memory_mib":        8192,
						"disk_mb":           10240,
						"network_egress_mb": 1024,
						"llm_tokens_limit":  100000,
						"llm_tokens_used":   0,
						"actor_cpu_millis":  4000,
						"actor_memory_mib":  8192,
						"actor_disk_mb":     10240,
						"actor_network_mb":  1024,
						"actor_llm_tokens":  100000,
						"guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
						"runtime_state":     "running",
						"runtime_meta":      "",
					},
					map[string]any{
						"id":                "sp_456",
						"name":              "beta",
						"role":              "admin",
						"owner_user_id":     1,
						"created_at":        "2026-01-01T00:00:00Z",
						"cpu_millis":        4000,
						"memory_mib":        8192,
						"disk_mb":           10240,
						"network_egress_mb": 1024,
						"llm_tokens_limit":  100000,
						"llm_tokens_used":   0,
						"actor_cpu_millis":  4000,
						"actor_memory_mib":  8192,
						"actor_disk_mb":     10240,
						"actor_network_mb":  1024,
						"actor_llm_tokens":  100000,
						"guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
						"runtime_state":     "running",
						"runtime_meta":      "",
					},
				},
			},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "connect"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when multiple Spaces are visible and no default exists")
	}
	if !strings.Contains(stderr.String(), "multiple Spaces are available") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSSHConnectRejectsUnsafeSpaceID(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://127.0.0.1", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "connect", "--space", "sp_123;touch", "--host", "cell.example.com"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code for unsafe space ID")
	}
	if !strings.Contains(stderr.String(), "space ID must contain only") {
		t.Fatalf("stderr missing space validation error: %s", stderr.String())
	}
}

func TestSSHClientConfigRequiresSpace(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "ssh", "client-config"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --space is missing")
	}
	if !strings.Contains(stderr.String(), "--space is required") {
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
		{"space", "create", "-h"},
		{"space", "up", "-h"},
		{"space", "down", "-h"},
		{"space", "delete", "-h"},
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

func TestProtocolFileMatchesCrakenSpacesWhenPresent(t *testing.T) {
	localPath := filepath.Join("..", "..", "protocol", "public-api-v1.openapi.yaml")
	managedPath := filepath.Join("..", "..", "..", "craken-spaces", "protocol", "public-api-v1.openapi.yaml")
	if _, err := os.Stat(managedPath); os.IsNotExist(err) {
		t.Skip("sibling craken-spaces checkout not present")
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
		t.Fatalf("public API contract is out of sync with sibling craken-spaces checkout")
	}
}

func TestAuthRecoverRequestOnly(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authRecover": {
			Body: map[string]any{
				"ok":      true,
				"message": "if that email is registered, a recovery code has been sent",
			},
			Assert: func(t *testing.T, _ *http.Request, body []byte) {
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				if payload["email"] != "alice@example.com" {
					t.Fatalf("unexpected email: %+v", payload)
				}
			},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	var stdout, stderr bytes.Buffer
	// Non-interactive: should request code and print instructions.
	code := runWithStdin([]string{
		"--base-url", server.server.URL,
		"--session-file", sessionFile,
		"auth", "recover", "alice@example.com",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth recover code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "recovery code has been sent") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--code CODE") {
		t.Fatalf("expected hint about --code, got: %s", stdout.String())
	}
}

func TestAuthRecoverWithCode(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authRecoverRedeem": {
			Body: map[string]any{
				"ok":            true,
				"email":         "alice@example.com",
				"session_token": "sess_recovered",
			},
			Assert: func(t *testing.T, _ *http.Request, body []byte) {
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				if payload["email"] != "alice@example.com" {
					t.Fatalf("unexpected email: %+v", payload)
				}
				if payload["code"] != "123456" {
					t.Fatalf("unexpected code: %+v", payload)
				}
			},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	var stdout, stderr bytes.Buffer
	code := runWithStdin([]string{
		"--base-url", server.server.URL,
		"--session-file", sessionFile,
		"auth", "recover", "alice@example.com", "--code", "123456",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("auth recover redeem code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "authenticated as alice@example.com") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}

	// Verify session was saved.
	session, err := loadSession(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.SessionToken != "sess_recovered" {
		t.Fatalf("session not saved correctly: %+v", session)
	}
}

func TestRecoverShortcut(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"authRecover": {
			Body: map[string]any{
				"ok":      true,
				"message": "if that email is registered, a recovery code has been sent",
			},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	var stdout, stderr bytes.Buffer
	code := runWithStdin([]string{
		"--base-url", server.server.URL,
		"--session-file", sessionFile,
		"recover", "alice@example.com",
	}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("recover shortcut code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "recovery code has been sent") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}
