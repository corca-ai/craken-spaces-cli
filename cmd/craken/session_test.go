package main

import "testing"

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

func TestResolveBaseURLPrefersExplicitFlagOverEnvironment(t *testing.T) {
	t.Setenv("CRAKEN_BASE_URL", "https://agents-dev.borca.ai")

	cfg := cliConfig{BaseURL: "https://agents.borca.ai"}

	got := cfg.resolveBaseURL(nil)
	if got != "https://agents.borca.ai" {
		t.Fatalf("resolveBaseURL = %q, want explicit flag override", got)
	}
}
