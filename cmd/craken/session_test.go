package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequireBaseURLFallsBackToDefaultProdDomain(t *testing.T) {
	cfg := cliConfig{SessionFile: t.TempDir() + "/session.json"}

	got, err := cfg.requireBaseURL()
	if err != nil {
		t.Fatalf("requireBaseURL returned error: %v", err)
	}
	if got != defaultPublicBaseURL {
		t.Fatalf("requireBaseURL = %q, want %q", got, defaultPublicBaseURL)
	}
}

func TestResolveBaseURLPrefersEnvironmentOverSession(t *testing.T) {
	t.Setenv("CRAKEN_BASE_URL", "https://agents-dev.borca.ai/")

	cfg := cliConfig{}
	session := &localSession{BaseURL: "https://agents.borca.ai"}

	got := cfg.resolveBaseURL(session)
	if got != "https://agents-dev.borca.ai" {
		t.Fatalf("resolveBaseURL = %q, want env override", got)
	}
}

func TestDefaultSessionPathPrefersEnvVar(t *testing.T) {
	t.Setenv("CRAKEN_SESSION_FILE", "/tmp/custom-session.json")

	got := defaultSessionPath()
	if got != "/tmp/custom-session.json" {
		t.Fatalf("defaultSessionPath = %q, want env override", got)
	}
}

func TestResolveBaseURLPrefersExplicitFlagOverEnvironment(t *testing.T) {
	t.Setenv("CRAKEN_BASE_URL", "https://agents-dev.borca.ai")

	cfg := cliConfig{BaseURL: "https://agents.borca.ai"}

	got := cfg.resolveBaseURL(nil)
	if got != "https://agents.borca.ai" {
		t.Fatalf("resolveBaseURL = %q, want explicit flag override", got)
	}
}

func TestClearSession(t *testing.T) {
	t.Run("existing file", func(t *testing.T) {
		path := t.TempDir() + "/session.json"
		if err := saveSession(path, localSession{Email: "a@b.com"}); err != nil {
			t.Fatalf("saveSession: %v", err)
		}
		if err := clearSession(path); err != nil {
			t.Fatalf("clearSession: %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("session file should have been removed")
		}
	})
	t.Run("non-existent file", func(t *testing.T) {
		if err := clearSession(t.TempDir() + "/no-such-file.json"); err != nil {
			t.Fatalf("clearSession on missing file: %v", err)
		}
	})
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"  https://example.com/  ", "https://example.com"},
		{"https://example.com///", "https://example.com"},
		{"https://example.com", "https://example.com"},
		{"  ", ""},
	}
	for _, tc := range tests {
		got := normalizeBaseURL(tc.input)
		if got != tc.want {
			t.Errorf("normalizeBaseURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestLoadSessionNonExistent(t *testing.T) {
	session, err := loadSession(filepath.Join(t.TempDir(), "no-such-file.json"))
	if err != nil {
		t.Fatalf("loadSession on missing file: %v", err)
	}
	if session != nil {
		t.Fatal("expected nil session for missing file")
	}
}

func TestLoadSessionInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadSession(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveAndLoadSessionRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "session.json")
	session := localSession{
		BaseURL:      "https://example.com/",
		Email:        "test@example.com",
		SessionToken: "tok_123",
	}
	if err := saveSession(path, session); err != nil {
		t.Fatalf("saveSession: %v", err)
	}
	loaded, err := loadSession(path)
	if err != nil {
		t.Fatalf("loadSession: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil session")
	}
	if loaded.Email != session.Email || loaded.SessionToken != session.SessionToken {
		t.Fatalf("loaded session mismatch: %+v", loaded)
	}
	// BaseURL should be normalized (trailing slash removed)
	if loaded.BaseURL != "https://example.com" {
		t.Fatalf("BaseURL not normalized: %q", loaded.BaseURL)
	}
}

func TestRequireAuthenticatedClientNoSession(t *testing.T) {
	cfg := cliConfig{SessionFile: filepath.Join(t.TempDir(), "session.json")}
	_, _, err := cfg.requireAuthenticatedClient()
	if err == nil {
		t.Fatal("expected error when no session exists")
	}
}

func TestDefaultSessionPathUsesConfigDir(t *testing.T) {
	t.Setenv("CRAKEN_SESSION_FILE", "")
	t.Setenv("CRAKEN_CONFIG_DIR", "/tmp/test-config")
	got := defaultSessionPath()
	if got != "/tmp/test-config/session.json" {
		t.Fatalf("defaultSessionPath = %q, want config dir path", got)
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Run("env set", func(t *testing.T) {
		t.Setenv("TEST_ENV_OR_DEFAULT", "custom")
		got := envOrDefault("TEST_ENV_OR_DEFAULT", "fallback")
		if got != "custom" {
			t.Fatalf("envOrDefault = %q, want %q", got, "custom")
		}
	})
	t.Run("env unset", func(t *testing.T) {
		got := envOrDefault("TEST_ENV_OR_DEFAULT_UNSET", "fallback")
		if got != "fallback" {
			t.Fatalf("envOrDefault = %q, want %q", got, "fallback")
		}
	})
}
