package main

import (
	"os"
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
