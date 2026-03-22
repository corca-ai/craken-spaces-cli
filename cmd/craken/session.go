package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type localSession struct {
	BaseURL      string `json:"base_url"`
	Email        string `json:"email"`
	SessionToken string `json:"session_token"`
}

type cliConfig struct {
	BaseURL     string
	SessionFile string
}

const defaultPublicBaseURL = "https://spaces.borca.ai"

func defaultSessionPath() string {
	if path := os.Getenv("SPACES_SESSION_FILE"); strings.TrimSpace(path) != "" {
		return path
	}
	if base := os.Getenv("SPACES_CONFIG_DIR"); base != "" {
		return filepath.Join(base, "session.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".craken-session.json"
	}
	return filepath.Join(home, ".config", "craken", "session.json")
}

func loadSession(path string) (*localSession, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var session localSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	session.BaseURL = normalizeBaseURL(session.BaseURL)
	return &session, nil
}

func saveSession(path string, session localSession) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}

func clearSession(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func normalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func configuredBaseURLOverride() string {
	return normalizeBaseURL(os.Getenv("SPACES_BASE_URL"))
}

func (cfg cliConfig) resolveBaseURL(session *localSession) string {
	if cfg.BaseURL != "" {
		return cfg.BaseURL
	}
	if envURL := configuredBaseURLOverride(); envURL != "" {
		return envURL
	}
	if session != nil && session.BaseURL != "" {
		return session.BaseURL
	}
	return defaultPublicBaseURL
}

func (cfg cliConfig) requireBaseURL() (string, error) {
	session, err := loadSession(cfg.SessionFile)
	if err != nil {
		return "", err
	}
	return cfg.resolveBaseURL(session), nil
}

func (cfg cliConfig) requireAuthenticatedClient() (apiClient, *localSession, error) { //nolint:unparam // session used by future commands
	session, err := loadSession(cfg.SessionFile)
	if err != nil {
		return apiClient{}, nil, err
	}
	if session == nil || session.SessionToken == "" {
		return apiClient{}, nil, errors.New("not authenticated; run 'spaces auth login'")
	}
	baseURL := cfg.resolveBaseURL(session)
	return apiClient{BaseURL: baseURL, SessionToken: session.SessionToken}, session, nil
}
