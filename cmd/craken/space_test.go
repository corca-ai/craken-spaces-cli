package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

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
						"actor_llm_tokens": 1, "guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
						"runtime_state": "running", "runtime_meta": "",
					},
					map[string]any{
						"id": "sp_2", "name": "alpha", "role": "admin",
						"owner_user_id": 1, "created_at": "2026-01-01T00:00:00Z",
						"cpu_millis": 1, "memory_mib": 1, "disk_mb": 1, "network_egress_mb": 1,
						"llm_tokens_limit": 1, "llm_tokens_used": 0, "actor_cpu_millis": 1,
						"actor_memory_mib": 1, "actor_disk_mb": 1, "actor_network_mb": 1,
						"actor_llm_tokens": 1, "guardian_bytes_used": 0, "guardian_requests_per_hour": 0,
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
