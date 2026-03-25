package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpaceMemberAuthKeysList(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"listSpaceMemberAuthKeys": {
			Body: map[string]any{
				"ok": true,
				"auth_keys": []any{
					map[string]any{
						"id":                1,
						"space_id":          "sp_1",
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
					map[string]any{
						"id":                2,
						"space_id":          "sp_1",
						"space_name":        "alpha",
						"issued_by_user_id": 1,
						"issued_by_email":   "alice@example.com",
						"invitee_email":     "charlie@example.com",
						"issued_at":         "2026-03-22T13:00:00Z",
						"expires_at":        "2026-03-30T00:00:00Z",
						"redeemed_at":       "2026-03-23T10:00:00Z",
						"revoked_at":        "",
						"cpu_millis":        2000,
						"memory_mib":        2048,
						"disk_mb":           2048,
						"network_egress_mb": 512,
						"llm_tokens_limit":  20000,
					},
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
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "member-auth-keys", "--space", "sp_1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space member-auth-keys code=%d stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "bob@example.com") {
		t.Fatalf("stdout missing bob@example.com: %s", output)
	}
	if !strings.Contains(output, "charlie@example.com") {
		t.Fatalf("stdout missing charlie@example.com: %s", output)
	}
	// Check status derivation: bob is active, charlie is redeemed
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (header + 2 rows), got %d", len(lines))
	}
	if !strings.Contains(lines[1], "active") {
		t.Fatalf("expected bob row to contain 'active': %s", lines[1])
	}
	if !strings.Contains(lines[2], "redeemed") {
		t.Fatalf("expected charlie row to contain 'redeemed': %s", lines[2])
	}
}

func TestSpaceRevokeMemberAuthKey(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"revokeSpaceMemberAuthKey": {
			Body: map[string]any{"ok": true},
			Assert: func(t *testing.T, req *http.Request, _ []byte) {
				if got := req.Header.Get("Authorization"); got != "Bearer sess_test" {
					t.Fatalf("Authorization = %q", got)
				}
				// Verify the URL contains the auth key ID
				if !strings.Contains(req.URL.Path, "/member-auth-keys/42") {
					t.Fatalf("URL path missing auth key ID: %s", req.URL.Path)
				}
			},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "revoke-member-auth-key", "--space", "sp_1", "--id", "42"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space revoke-member-auth-key code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "revoked space member auth key 42") {
		t.Fatalf("stdout missing revocation message: %s", stdout.String())
	}
}

func TestSpaceRevokeMemberAuthKeyRequiresFlags(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{"missing both", []string{"--session-file", sessionFile, "space", "revoke-member-auth-key"}},
		{"missing id", []string{"--session-file", sessionFile, "space", "revoke-member-auth-key", "--space", "sp_1"}},
		{"missing space", []string{"--session-file", sessionFile, "space", "revoke-member-auth-key", "--id", "1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			if code == 0 {
				t.Fatal("expected non-zero exit code")
			}
		})
	}
}

func TestSpaceMemberAuthKeysRequiresSpace(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "member-auth-keys"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --space is missing")
	}
	if !strings.Contains(stderr.String(), "--space is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSpaceDeleteRequiresSpace(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "delete"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --space is missing")
	}
	if !strings.Contains(stderr.String(), "--space is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSpaceDownRequiresSpace(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "down"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --space is missing")
	}
	if !strings.Contains(stderr.String(), "--space is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSpaceIssueMemberAuthKeyRequiresFlags(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{"missing both", []string{"--session-file", sessionFile, "space", "issue-member-auth-key"}},
		{"missing email", []string{"--session-file", sessionFile, "space", "issue-member-auth-key", "--space", "sp_1", "--auth-key-file", filepath.Join(t.TempDir(), "issued.key")}},
		{"missing space", []string{"--session-file", sessionFile, "space", "issue-member-auth-key", "--email", "bob@example.com", "--auth-key-file", filepath.Join(t.TempDir(), "issued.key")}},
		{"missing auth key file", []string{"--session-file", sessionFile, "space", "issue-member-auth-key", "--space", "sp_1", "--email", "bob@example.com"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			if code == 0 {
				t.Fatal("expected non-zero exit code")
			}
		})
	}
}

func TestSpaceUnknownSubcommand(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unknown space subcommand "bogus"`) {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestSpaceHelpAndNoArgs(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	t.Run("no args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--session-file", sessionFile, "space"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d", code)
		}
	})
	t.Run("help", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--session-file", sessionFile, "space", "help"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	})
}

func TestSpaceRequiresAuth(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	// No session saved — should fail with auth error

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "list"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not authenticated") {
		t.Fatalf("stderr missing auth error: %s", stderr.String())
	}
}

func TestSpaceCreatePayload(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"createSpace": {
			Body: map[string]any{
				"ok": true,
				"space": map[string]any{
					"id": "sp_1", "name": "custom-room", "role": "admin",
					"owner_user_id":  1,
					"runtime_state": "running", "runtime_meta": "",
					"cpu_millis": 2000, "memory_mib": 4096, "disk_mb": 5120,
					"network_egress_mb": 512, "llm_tokens_used": 0, "llm_tokens_limit": 50000,
					"actor_cpu_millis": 2000, "actor_memory_mib": 4096, "actor_disk_mb": 5120,
					"actor_network_mb": 512, "actor_llm_tokens": 50000, "byok_bytes_used": 0, "byok_requests_per_hour": 0,
					"created_at": "2026-01-01T00:00:00Z",
				},
			},
			Assert: func(t *testing.T, _ *http.Request, body []byte) {
				var payload map[string]any
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("json.Unmarshal failed: %v", err)
				}
				if payload["name"] != "custom-room" {
					t.Fatalf("expected name=custom-room, got %v", payload["name"])
				}
				if _, ok := payload["runtime_driver"]; ok {
					t.Fatal("expected runtime_driver to be absent from request payload")
				}
			},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--session-file", sessionFile,
		"space", "create",
		"--name", "custom-room",
		"--cpu-millis", "2000",
		"--memory-mib", "4096",
		"--disk-mb", "5120",
		"--network-egress-mb", "512",
		"--llm-tokens-limit", "50000",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space create code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created space sp_1 (custom-room)") {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

func TestSpaceUpEscapesSpaceIDInPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/spaces":
			_, _ = w.Write([]byte(`{"ok":true,"spaces":[{"id":"sp_1/../../evil","name":"danger-zone","role":"admin","runtime_state":"stopped","cpu_millis":1,"memory_mib":1,"disk_mb":1,"network_egress_mb":1,"llm_tokens_used":0,"llm_tokens_limit":1,"created_at":"2026-01-01T00:00:00Z"}]}`))
		case r.Method == http.MethodPost:
			gotPath = r.URL.EscapedPath()
			_, _ = w.Write([]byte(`{"ok":true,"space":{"id":"sp_1/../../evil","runtime_state":"running"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "up", "--space", "danger-zone"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("space up code=%d stderr=%s", code, stderr.String())
	}
	if gotPath != "/api/v1/spaces/sp_1%2F..%2F..%2Fevil/up" {
		t.Fatalf("escaped path = %q", gotPath)
	}
}

func TestSpaceDeleteRejectsAmbiguousExactName(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"listSpaces": {
			Body: map[string]any{
				"ok": true,
				"spaces": []any{
					map[string]any{
						"id": "sp_1", "name": "alpha", "role": "admin",
						"owner_user_id": 1, "created_at": "2026-01-01T00:00:00Z",
						"cpu_millis": 1, "memory_mib": 1, "disk_mb": 1, "network_egress_mb": 1,
						"llm_tokens_limit": 1, "llm_tokens_used": 0, "actor_cpu_millis": 1,
						"actor_memory_mib": 1, "actor_disk_mb": 1, "actor_network_mb": 1,
						"actor_llm_tokens": 1, "byok_bytes_used": 0, "byok_requests_per_hour": 0,
						"runtime_state": "running", "runtime_meta": "",
					},
					map[string]any{
						"id": "sp_2", "name": "alpha", "role": "admin",
						"owner_user_id": 1, "created_at": "2026-01-01T00:00:00Z",
						"cpu_millis": 1, "memory_mib": 1, "disk_mb": 1, "network_egress_mb": 1,
						"llm_tokens_limit": 1, "llm_tokens_used": 0, "actor_cpu_millis": 1,
						"actor_memory_mib": 1, "actor_disk_mb": 1, "actor_network_mb": 1,
						"actor_llm_tokens": 1, "byok_bytes_used": 0, "byok_requests_per_hour": 0,
						"runtime_state": "running", "runtime_meta": "",
					},
				},
			},
		},
	})

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "space", "delete", "--space", "alpha"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code for ambiguous space name")
	}
	if !strings.Contains(stderr.String(), "is ambiguous") {
		t.Fatalf("stderr missing ambiguity error: %s", stderr.String())
	}
}
