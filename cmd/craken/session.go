package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type localSession struct {
	BaseURL      string `json:"base_url"`
	Email        string `json:"email"`
	SessionToken string `json:"session_token"`
	DefaultSpace string `json:"default_space,omitempty"`
}

type cliConfig struct {
	BaseURL     string
	SessionFile string
}

const defaultPublicBaseURL = "https://spaces.borca.ai"

var lookupUserHomeDir = os.UserHomeDir

func defaultSessionPath() (string, error) {
	if path := os.Getenv("SPACES_SESSION_FILE"); strings.TrimSpace(path) != "" {
		return path, nil
	}
	if base := os.Getenv("SPACES_CONFIG_DIR"); base != "" {
		return filepath.Join(base, "session.json"), nil
	}
	home, err := lookupUserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("could not resolve a default session file path; set SPACES_SESSION_FILE, SPACES_CONFIG_DIR, or --session-file")
	}
	return filepath.Join(home, ".config", "spaces", "session.json"), nil
}

func loadSession(path string) (*localSession, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("session file path is required")
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := validateSecretParentDir(path); err != nil {
		return nil, err
	}
	if err := validateSecretFile(path, info); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
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
	payload, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateFile(path, payload)
}

func clearSession(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func setSessionDefaultSpace(path string, session *localSession, spaceID string) error {
	spaceID = strings.TrimSpace(spaceID)
	if session == nil || spaceID == "" {
		return nil
	}
	if strings.TrimSpace(session.DefaultSpace) == spaceID {
		return nil
	}
	updated := *session
	updated.DefaultSpace = spaceID
	if err := saveSession(path, updated); err != nil {
		return err
	}
	session.DefaultSpace = spaceID
	return nil
}

func clearSessionDefaultSpace(path string, session *localSession) error {
	if session == nil || strings.TrimSpace(session.DefaultSpace) == "" {
		return nil
	}
	updated := *session
	updated.DefaultSpace = ""
	if err := saveSession(path, updated); err != nil {
		return err
	}
	session.DefaultSpace = ""
	return nil
}

func warnSessionUpdate(stderr io.Writer, action string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(stderr, "warning: %s: %s\n", action, sanitizeTerminalText(err.Error()))
}

func warnAuthenticatedBaseURLOverride(stderr io.Writer, cfg cliConfig, session *localSession) {
	if stderr == nil {
		return
	}
	if warning := cfg.authenticatedBaseURLOverrideWarning(session); warning != "" {
		fmt.Fprintf(stderr, "warning: %s\n", sanitizeTerminalText(warning))
	}
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

func (cfg cliConfig) authenticatedBaseURLOverride() (value, source string) {
	if cfg.BaseURL != "" {
		return cfg.BaseURL, "--base-url"
	}
	if envURL := configuredBaseURLOverride(); envURL != "" {
		return envURL, "SPACES_BASE_URL"
	}
	return "", ""
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

func (cfg cliConfig) resolveAuthenticatedBaseURL(session *localSession) string {
	if overrideURL, _ := cfg.authenticatedBaseURLOverride(); overrideURL != "" {
		return overrideURL
	}
	if session != nil && session.BaseURL != "" {
		return session.BaseURL
	}
	return defaultPublicBaseURL
}

func (cfg cliConfig) authenticatedBaseURLOverrideWarning(session *localSession) string {
	if session == nil {
		return ""
	}
	savedBaseURL := normalizeBaseURL(session.BaseURL)
	if savedBaseURL == "" {
		return ""
	}
	overrideURL, source := cfg.authenticatedBaseURLOverride()
	overrideURL = normalizeBaseURL(overrideURL)
	if overrideURL == "" || overrideURL == savedBaseURL {
		return ""
	}
	return fmt.Sprintf("using %s from %s, but the saved session was issued by %s; the target deployment may reject this session token", overrideURL, source, savedBaseURL)
}

func validateBaseURL(value string) (string, error) {
	value = normalizeBaseURL(value)
	if value == "" {
		return "", errors.New("base URL is required")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("base URL must include a scheme and host")
	}
	if parsed.User != nil {
		return "", errors.New("base URL must not include user info")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("base URL must not include a query or fragment")
	}
	switch parsed.Scheme {
	case "https":
		return normalizeBaseURL(parsed.String()), nil
	case "http":
		if isLoopbackHost(parsed.Hostname()) {
			return normalizeBaseURL(parsed.String()), nil
		}
		return "", errors.New("base URL must use https unless it targets localhost or a loopback address")
	default:
		return "", errors.New("base URL must use https unless it targets localhost or a loopback address")
	}
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (cfg cliConfig) requireBaseURL() (string, error) {
	session, err := loadSession(cfg.SessionFile)
	if err != nil {
		return "", err
	}
	return validateBaseURL(cfg.resolveBaseURL(session))
}

func (cfg cliConfig) requireAuthenticatedClient() (apiClient, *localSession, error) { //nolint:unparam // session used by future commands
	session, err := loadSession(cfg.SessionFile)
	if err != nil {
		return apiClient{}, nil, err
	}
	if session == nil || session.SessionToken == "" {
		return apiClient{}, nil, errors.New("not authenticated; run 'spaces auth login'")
	}
	baseURL, err := validateBaseURL(cfg.resolveAuthenticatedBaseURL(session))
	if err != nil {
		return apiClient{}, nil, err
	}
	return apiClient{BaseURL: baseURL, SessionToken: session.SessionToken}, session, nil
}
