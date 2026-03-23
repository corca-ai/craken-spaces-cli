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

func TestRoomMemberAuthKeysList(t *testing.T) {
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
	code := run([]string{"--session-file", sessionFile, "room", "member-auth-keys", "--room", "sp_1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room member-auth-keys code=%d stderr=%s", code, stderr.String())
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

func TestRoomRevokeMemberAuthKey(t *testing.T) {
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
	code := run([]string{"--session-file", sessionFile, "room", "revoke-member-auth-key", "--room", "sp_1", "--id", "42"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room revoke-member-auth-key code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "revoked room member auth key 42") {
		t.Fatalf("stdout missing revocation message: %s", stdout.String())
	}
}

func TestRoomRevokeMemberAuthKeyRequiresFlags(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{"missing both", []string{"--session-file", sessionFile, "room", "revoke-member-auth-key"}},
		{"missing id", []string{"--session-file", sessionFile, "room", "revoke-member-auth-key", "--room", "sp_1"}},
		{"missing room", []string{"--session-file", sessionFile, "room", "revoke-member-auth-key", "--id", "1"}},
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

func TestRoomMemberAuthKeysRequiresRoom(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "member-auth-keys"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --room is missing")
	}
	if !strings.Contains(stderr.String(), "--room is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestRoomDeleteRequiresRoom(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "delete"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --room is missing")
	}
	if !strings.Contains(stderr.String(), "--room is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestRoomDownRequiresRoom(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "down"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code when --room is missing")
	}
	if !strings.Contains(stderr.String(), "--room is required") {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestRoomIssueMemberAuthKeyRequiresFlags(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{"missing both", []string{"--session-file", sessionFile, "room", "issue-member-auth-key"}},
		{"missing email", []string{"--session-file", sessionFile, "room", "issue-member-auth-key", "--room", "sp_1", "--auth-key-file", filepath.Join(t.TempDir(), "issued.key")}},
		{"missing room", []string{"--session-file", sessionFile, "room", "issue-member-auth-key", "--email", "bob@example.com", "--auth-key-file", filepath.Join(t.TempDir(), "issued.key")}},
		{"missing auth key file", []string{"--session-file", sessionFile, "room", "issue-member-auth-key", "--room", "sp_1", "--email", "bob@example.com"}},
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

func TestRoomUnknownSubcommand(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unknown room subcommand "bogus"`) {
		t.Fatalf("stderr missing expected message: %s", stderr.String())
	}
}

func TestRoomHelpAndNoArgs(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: "http://localhost", Email: "a@b.com", SessionToken: "sess"}); err != nil {
		t.Fatal(err)
	}

	t.Run("no args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--session-file", sessionFile, "room"}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected exit code 2, got %d", code)
		}
	})
	t.Run("help", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--session-file", sessionFile, "room", "help"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	})
}

func TestRoomRequiresAuth(t *testing.T) {
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	// No session saved — should fail with auth error

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "list"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not authenticated") {
		t.Fatalf("stderr missing auth error: %s", stderr.String())
	}
}

func TestRoomCreatePayload(t *testing.T) {
	server := newContractFakeServer(t, map[string]fakeOperation{
		"createSpace": {
			Body: map[string]any{
				"ok": true,
				"space": map[string]any{
					"id": "sp_1", "name": "custom-room", "role": "admin",
					"owner_user_id":  1,
					"runtime_driver": "docker", "runtime_state": "stopped", "runtime_meta": "",
					"cpu_millis": 2000, "memory_mib": 4096, "disk_mb": 5120,
					"network_egress_mb": 512, "llm_tokens_used": 0, "llm_tokens_limit": 50000,
					"actor_cpu_millis": 2000, "actor_memory_mib": 4096, "actor_disk_mb": 5120,
					"actor_network_mb": 512, "actor_llm_tokens": 50000, "byok_bytes_used": 0,
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
				if payload["runtime_driver"] != "docker" {
					t.Fatalf("expected runtime_driver=docker, got %v", payload["runtime_driver"])
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
		"room", "create",
		"--name", "custom-room",
		"--runtime-driver", "docker",
		"--cpu-millis", "2000",
		"--memory-mib", "4096",
		"--disk-mb", "5120",
		"--network-egress-mb", "512",
		"--llm-tokens-limit", "50000",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room create code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "created room sp_1 (custom-room)") {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

func TestRoomUpEscapesRoomIDInPath(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"space":{"id":"sp_1","runtime_state":"running"}}`))
	}))
	defer server.Close()

	sessionFile := filepath.Join(t.TempDir(), "session.json")
	if err := saveSession(sessionFile, localSession{BaseURL: server.URL, Email: "alice@example.com", SessionToken: "sess_test"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--session-file", sessionFile, "room", "up", "--room", "sp_1/../../evil"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("room up code=%d stderr=%s", code, stderr.String())
	}
	if gotPath != "/api/v1/spaces/sp_1%2F..%2F..%2Fevil/up" {
		t.Fatalf("escaped path = %q", gotPath)
	}
}
