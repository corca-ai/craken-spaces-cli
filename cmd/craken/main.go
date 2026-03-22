package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(argv []string, stdout, stderr io.Writer) int {
	root := flag.NewFlagSet("craken", flag.ContinueOnError)
	root.SetOutput(stderr)
	root.Usage = func() { printUsage(root.Output()) }

	baseURL := root.String("base-url", "", "Craken public control-plane base URL (default: https://agents.borca.ai)")
	sessionFile := root.String("session-file", defaultSessionPath(), "path to the local session file")
	if err := root.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	args := root.Args()
	if len(args) == 0 {
		root.Usage()
		return 2
	}

	cfg := cliConfig{
		BaseURL:     normalizeBaseURL(*baseURL),
		SessionFile: *sessionFile,
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "craken %s\n", version)
		return 0
	case "help":
		printUsage(stdout)
		return 0
	case "auth":
		return cmdAuth(cfg, args[1:], stdout, stderr)
	case "whoami":
		return cmdWhoAmI(cfg, stdout, stderr)
	case "workspace":
		return cmdWorkspace(cfg, args[1:], stdout, stderr)
	case "ssh":
		return cmdSSH(cfg, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func cmdAuth(cfg cliConfig, argv []string, stdout, stderr io.Writer) int {
	if len(argv) == 0 || isHelpWord(argv[0]) {
		printAuthUsage(stdout)
		if len(argv) == 0 {
			return 2
		}
		return 0
	}

	switch argv[0] {
	case "login":
		fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
		fs.SetOutput(stderr)
		email := fs.String("email", "", "user email address")
		key := fs.String("key", "", "one-time auth key")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*email) == "" || strings.TrimSpace(*key) == "" {
			fmt.Fprintln(stderr, "error: --email and --key are required")
			return 2
		}
		baseURL, err := cfg.requireBaseURL()
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 2
		}
		client := apiClient{BaseURL: baseURL}
		var response struct {
			OK           bool   `json:"ok"`
			Error        string `json:"error"`
			Email        string `json:"email"`
			SessionToken string `json:"session_token"`
		}
		if err := client.doJSON("POST", "/api/v1/auth/login", map[string]any{
			"email": *email,
			"key":   *key,
		}, &response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if err := saveSession(cfg.SessionFile, localSession{
			BaseURL:      baseURL,
			Email:        response.Email,
			SessionToken: response.SessionToken,
		}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "authenticated as %s\n", response.Email)
		fmt.Fprintf(stdout, "session saved to %s\n", cfg.SessionFile)
		return 0

	case "logout":
		session, _ := loadSession(cfg.SessionFile)
		if session != nil && session.SessionToken != "" && session.BaseURL != "" {
			client := apiClient{BaseURL: session.BaseURL, SessionToken: session.SessionToken}
			_ = client.doJSON("POST", "/api/v1/auth/logout", nil, nil)
		}
		if err := clearSession(cfg.SessionFile); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "logged out; session removed from %s\n", cfg.SessionFile)
		return 0
	default:
		fmt.Fprintf(stderr, "error: unknown auth subcommand %q\n\n", argv[0])
		printAuthUsage(stderr)
		return 2
	}
}

func cmdWhoAmI(cfg cliConfig, stdout, stderr io.Writer) int {
	client, _, err := cfg.requireAuthenticatedClient()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	var response struct {
		OK   bool `json:"ok"`
		User struct {
			Email string `json:"email"`
		} `json:"user"`
		Error string `json:"error"`
	}
	if err := client.doJSON("GET", "/api/v1/whoami", nil, &response); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, response.User.Email)
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: craken [--base-url URL] [--session-file PATH] <command> [options]

Commands:
  version
  auth login
  auth logout
  whoami
  workspace create
  workspace list
  workspace up
  workspace down
  workspace delete
  workspace issue-member-auth-key
  workspace member-auth-keys
  workspace revoke-member-auth-key
  ssh add-key
  ssh list-keys
  ssh remove-key
  ssh issue-cert
  ssh connect
  ssh client-config

Environment:
  CRAKEN_BASE_URL      Override the default public control-plane base URL (https://agents.borca.ai)
  CRAKEN_SSH_HOST      Override SSH host for Cell entry
  CRAKEN_SSH_PORT      Override SSH port for Cell entry (default: 22)
  CRAKEN_SSH_LOGIN_USER Override SSH login user (default: craken-cell)
  CRAKEN_SSH_BIN       Override ssh binary path for testing
`)
}

func printAuthUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  craken auth login --email EMAIL --key AUTH_KEY")
	fmt.Fprintln(w, "  craken auth logout")
}

func isHelpWord(value string) bool {
	switch strings.TrimSpace(value) {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}
