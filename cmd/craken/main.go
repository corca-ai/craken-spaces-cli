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
	return runWithStdin(argv, os.Stdin, stdout, stderr)
}

func runWithStdin(argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	defaultSessionFile, defaultSessionErr := defaultSessionPath()

	root := flag.NewFlagSet("spaces", flag.ContinueOnError)
	root.SetOutput(stderr)
	root.Usage = func() { printUsage(root.Output()) }

	baseURL := root.String("base-url", "", "Spaces public control-plane base URL (default: https://spaces.borca.ai; http only for localhost/loopback)")
	sessionFile := root.String("session-file", defaultSessionFile, "path to the local session file")
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
		fmt.Fprintf(stdout, "spaces %s\n", version)
		return 0
	case "help":
		printUsage(stdout)
		return 0
	}
	if strings.TrimSpace(cfg.SessionFile) == "" && defaultSessionErr != nil {
		fmt.Fprintf(stderr, "error: %v\n", defaultSessionErr)
		return 1
	}

	switch args[0] {
	case "auth":
		return cmdAuth(cfg, args[1:], stdin, stdout, stderr)
	case "login":
		return cmdLogin(cfg, args[1:], stdin, stdout, stderr)
	case "create":
		return cmdCreate(cfg, args[1:], stdout, stderr)
	case "list":
		return cmdList(cfg, stdout, stderr)
	case "whoami":
		return cmdWhoAmI(cfg, stdout, stderr)
	case "space":
		return cmdSpace(cfg, args[1:], stdout, stderr)
	case "ssh":
		return cmdSSH(cfg, args[1:], stdin, stdout, stderr)
	case "connect":
		return cmdConnect(cfg, args[1:], stdin, stdout, stderr)
	case "recover":
		return cmdAuthRecover(cfg, args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func cmdAuth(cfg cliConfig, argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(argv) == 0 || isHelpWord(argv[0]) {
		printAuthUsage(stdout)
		if len(argv) == 0 {
			return 2
		}
		return 0
	}

	switch argv[0] {
	case "login":
		return cmdAuthLogin(cfg, argv[1:], stdin, stdout, stderr)

	case "logout":
		return cmdAuthLogout(cfg, stdout, stderr)

	case "recover":
		return cmdAuthRecover(cfg, argv[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown auth subcommand %q\n\n", argv[0])
		printAuthUsage(stderr)
		return 2
	}
}

func cmdAuthLogin(cfg cliConfig, argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(argv) > 0 && isHelpWord(argv[0]) {
		printLoginUsage(stdout)
		return 0
	}
	request, code, done := parseLoginRequest("auth login", argv, stderr)
	if done {
		return code
	}
	return runLoginRequest(cfg, request, stdin, stdout, stderr)
}

func cmdLogin(cfg cliConfig, argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(argv) > 0 && isHelpWord(argv[0]) {
		printLoginUsage(stdout)
		return 0
	}
	request, code, done := parseLoginRequest("login", argv, stderr)
	if done {
		return code
	}
	return runLoginRequest(cfg, request, stdin, stdout, stderr)
}

func cmdCreate(cfg cliConfig, argv []string, stdout, stderr io.Writer) int {
	return cmdCreateCommand(cfg, "create", argv, stdout, stderr)
}

func cmdList(cfg cliConfig, stdout, stderr io.Writer) int {
	return cmdSpace(cfg, []string{"list"}, stdout, stderr)
}

type loginRequest struct {
	Email    string
	KeyFile  string
	KeyStdin bool
}

func parseLoginRequest(commandName string, argv []string, stderr io.Writer) (loginRequest, int, bool) {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage:\n  spaces %s [EMAIL] [--key-file PATH | --key-stdin]\n\nFlags:\n", commandName)
		fs.PrintDefaults()
	}
	email := fs.String("email", "", "user email address")
	keyFile := fs.String("key-file", "", "path to a file containing the one-time auth key")
	keyStdin := fs.Bool("key-stdin", false, "read the one-time auth key from stdin")
	if len(argv) > 0 && !strings.HasPrefix(argv[0], "-") && !isHelpWord(argv[0]) {
		argv = append([]string{"--email", argv[0]}, argv[1:]...)
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return loginRequest{}, 0, true
		}
		return loginRequest{}, 2, true
	}
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(extra, " "))
		fs.Usage()
		return loginRequest{}, 2, true
	}
	request := loginRequest{
		Email:    *email,
		KeyFile:  *keyFile,
		KeyStdin: *keyStdin,
	}
	return request, 0, false
}

func runLoginRequest(cfg cliConfig, request loginRequest, stdin io.Reader, stdout, stderr io.Writer) int {
	if strings.TrimSpace(request.Email) == "" {
		fmt.Fprintln(stderr, "error: --email is required")
		return 2
	}
	if strings.TrimSpace(request.KeyFile) != "" && request.KeyStdin {
		fmt.Fprintln(stderr, "error: use only one of --key-file or --key-stdin")
		return 2
	}
	origTerminalStatusSink := terminalStatusSink
	terminalStatusSink = stderr
	defer func() { terminalStatusSink = origTerminalStatusSink }()
	authKey, err := resolveAuthKey(request.KeyFile, request.KeyStdin, stdin)
	if err != nil {
		return printCLIError(stderr, err)
	}
	baseURL, err := cfg.requireBaseURL()
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", sanitizeTerminalText(err.Error()))
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
		"email": request.Email,
		"key":   authKey,
	}, &response); err != nil {
		return printCLIError(stderr, err)
	}
	if err := saveSession(cfg.SessionFile, localSession{
		BaseURL:      baseURL,
		Email:        response.Email,
		SessionToken: response.SessionToken,
	}); err != nil {
		return printCLIError(stderr, err)
	}
	fmt.Fprintf(stdout, "authenticated as %s\n", sanitizeTerminalText(response.Email))
	fmt.Fprintf(stdout, "session saved to %s\n", sanitizeTerminalText(cfg.SessionFile))
	return 0
}

func cmdAuthLogout(cfg cliConfig, stdout, stderr io.Writer) int {
	session, err := loadSession(cfg.SessionFile)
	if err != nil {
		return printCLIError(stderr, err)
	}
	sessionPath := sanitizeTerminalText(cfg.SessionFile)
	if session == nil || session.SessionToken == "" || session.BaseURL == "" {
		if err := clearSession(cfg.SessionFile); err != nil {
			return printCLIError(stderr, err)
		}
		fmt.Fprintf(stdout, "logged out; session removed from %s\n", sessionPath)
		return 0
	}
	sessionToken := session.SessionToken
	sessionBaseURL := session.BaseURL
	if err := clearSession(cfg.SessionFile); err != nil {
		return printCLIError(stderr, err)
	}
	baseURL, err := validateBaseURL(sessionBaseURL)
	if err != nil {
		fmt.Fprintf(stderr, "warning: local session removed from %s, but remote logout was skipped: %s\n", sessionPath, sanitizeTerminalText(err.Error()))
		return 1
	}
	client := apiClient{BaseURL: baseURL, SessionToken: sessionToken}
	if err := client.doJSON("POST", "/api/v1/auth/logout", nil, nil); err != nil {
		fmt.Fprintf(stderr, "warning: local session removed from %s, but remote logout failed: %s\n", sessionPath, sanitizeTerminalText(err.Error()))
		return 1
	}
	fmt.Fprintf(stdout, "logged out; session removed from %s\n", sessionPath)
	return 0
}

func cmdAuthRecover(cfg cliConfig, argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(argv) > 0 && isHelpWord(argv[0]) {
		printRecoverUsage(stdout)
		return 0
	}

	fs := flag.NewFlagSet("auth recover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printRecoverUsage(fs.Output()) }
	email := fs.String("email", "", "user email address")
	code := fs.String("code", "", "6-digit recovery code from email")
	codeStdin := fs.Bool("code-stdin", false, "read recovery code from stdin")

	// Accept email as positional argument.
	if len(argv) > 0 && !strings.HasPrefix(argv[0], "-") && !isHelpWord(argv[0]) {
		argv = append([]string{"--email", argv[0]}, argv[1:]...)
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(extra, " "))
		fs.Usage()
		return 2
	}
	if strings.TrimSpace(*email) == "" {
		fmt.Fprintln(stderr, "error: email is required")
		fs.Usage()
		return 2
	}
	if strings.TrimSpace(*code) != "" && *codeStdin {
		fmt.Fprintln(stderr, "error: use only one of --code or --code-stdin")
		return 2
	}

	baseURL, err := cfg.requireBaseURL()
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", sanitizeTerminalText(err.Error()))
		return 2
	}
	client := apiClient{BaseURL: baseURL}

	recoveryCode, rc := obtainRecoveryCode(client, *email, strings.TrimSpace(*code), *codeStdin, stdin, stdout, stderr)
	if rc >= 0 {
		return rc
	}

	// Redeem the code.
	var response struct {
		OK           bool   `json:"ok"`
		Error        string `json:"error"`
		Email        string `json:"email"`
		SessionToken string `json:"session_token"`
	}
	if err := client.doJSON("POST", "/api/v1/auth/recover/redeem", map[string]any{
		"email": *email,
		"code":  recoveryCode,
	}, &response); err != nil {
		return printCLIError(stderr, err)
	}
	if err := saveSession(cfg.SessionFile, localSession{
		BaseURL:      baseURL,
		Email:        response.Email,
		SessionToken: response.SessionToken,
	}); err != nil {
		return printCLIError(stderr, err)
	}
	fmt.Fprintf(stdout, "authenticated as %s\n", sanitizeTerminalText(response.Email))
	fmt.Fprintf(stdout, "session saved to %s\n", sanitizeTerminalText(cfg.SessionFile))
	return 0
}

// obtainRecoveryCode resolves the recovery code from flags, stdin, or an
// interactive prompt. It returns the code and a negative rc when the caller
// should continue, or a non-negative rc the caller should return immediately.
func obtainRecoveryCode(client apiClient, email, flagCode string, codeStdin bool, stdin io.Reader, stdout, stderr io.Writer) (code string, rc int) {
	if flagCode != "" {
		return flagCode, -1
	}
	if codeStdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", printCLIError(stderr, err)
		}
		code := strings.TrimSpace(string(data))
		if code == "" {
			fmt.Fprintln(stderr, "error: recovery code is required")
			return "", 2
		}
		return code, -1
	}

	// No code supplied via flag or stdin — request one from the server.
	if err := requestRecoveryCode(client, email); err != nil {
		return "", printCLIError(stderr, err)
	}
	fmt.Fprintln(stdout, "If that email is registered, a recovery code has been sent. Check your inbox.")

	// Interactive terminal: prompt for the code inline.
	if file, ok := stdin.(stdinWithFD); ok && isTerminalFD(int(file.Fd())) {
		origTerminalStatusSink := terminalStatusSink
		terminalStatusSink = stderr
		defer func() { terminalStatusSink = origTerminalStatusSink }()
		payload, promptErr := readMaskedTerminalKeyFD(int(file.Fd()), "Recovery code: ", terminalStatusSink)
		if promptErr != nil {
			return "", printCLIError(stderr, promptErr)
		}
		code := strings.TrimSpace(string(payload))
		if code == "" {
			fmt.Fprintln(stderr, "error: recovery code is required")
			return "", 2
		}
		return code, -1
	}

	// Non-interactive: tell the user how to finish.
	fmt.Fprintf(stdout, "Then run: spaces auth recover %s --code CODE\n", sanitizeTerminalText(email))
	return "", 0
}

func requestRecoveryCode(client apiClient, email string) error {
	var resp struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	return client.doJSON("POST", "/api/v1/auth/recover", map[string]any{
		"email": email,
	}, &resp)
}

func printRecoverUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  spaces auth recover EMAIL                Request a recovery code")
	fmt.Fprintln(w, "  spaces auth recover EMAIL --code CODE    Redeem a recovery code")
	fmt.Fprintln(w, "  spaces recover EMAIL                     Shortcut")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "On an interactive terminal, the code is prompted after requesting.")
	fmt.Fprintln(w, "For non-interactive use, pass --code or --code-stdin.")
}

func cmdWhoAmI(cfg cliConfig, stdout, stderr io.Writer) int {
	client, session, err := cfg.requireAuthenticatedClient()
	if err != nil {
		return printCLIError(stderr, err)
	}
	warnAuthenticatedBaseURLOverride(stderr, cfg, session)
	var response struct {
		OK   bool `json:"ok"`
		User struct {
			Email string `json:"email"`
		} `json:"user"`
		Error string `json:"error"`
	}
	if err := client.doJSON("GET", "/api/v1/whoami", nil, &response); err != nil {
		return printCLIError(stderr, err)
	}
	fmt.Fprintln(stdout, sanitizeTerminalText(response.User.Email))
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: spaces [--base-url URL] [--session-file PATH] <command> [options]

Shortcut Commands:
  login EMAIL                    Log in with email and auth key
  recover EMAIL                  Recover a lost session via email
  create SPACE                   Create a new Space
  list                           List Spaces you have access to
  connect [SPACE]                Connect to a Space; uses the default Space when omitted

Commands:
  auth login                     Log in with email and auth key
  auth logout                    End session and remove local credentials
  auth recover EMAIL             Recover a lost session via email code
  whoami                         Show the currently authenticated user
  space create                   Create a new Space
  space list                     List Spaces you have access to
  space up                       Start a Space
  space down                     Stop a Space
  space delete                   Permanently delete a Space
  ssh add-key                    Register an SSH public key
  ssh list-keys                  List registered SSH keys
  ssh remove-key                 Unregister an SSH key
  ssh issue-cert                 Issue a short-lived SSH certificate
  ssh connect                    Connect to a Space via SSH
  ssh client-config              Generate an OpenSSH config block
  version                        Print version
  help                           Show this help

Environment:
  SPACES_BASE_URL             Override default control-plane URL (default: https://spaces.borca.ai; http only for localhost/loopback)
  SPACES_SESSION_FILE         Override local session file path
  SPACES_CONFIG_DIR           Override the config directory used for the default session path
  SPACES_SSH_HOST             Override SSH host for Space entry
  SPACES_SSH_PORT             Override SSH port (default: 22)
  SPACES_SSH_LOGIN_USER       Override SSH login user (default: spaces-user)
  SPACES_SSH_KNOWN_HOSTS_FILE Override known_hosts file used for SSH host verification
  SPACES_SSH_BIN              Override ssh binary path
`)
}

func printAuthUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  spaces auth login [EMAIL] [--key-file PATH | --key-stdin]")
	fmt.Fprintln(w, "  spaces auth logout")
	fmt.Fprintln(w, "  spaces auth recover EMAIL [--code CODE | --code-stdin]")
}

func printLoginUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  spaces login EMAIL [--key-file PATH | --key-stdin]")
	fmt.Fprintln(w, "  spaces auth login [EMAIL] [--key-file PATH | --key-stdin]")
}

func isHelpWord(value string) bool {
	switch strings.TrimSpace(value) {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}

// containsHelpFlag returns true if any element of args is a help flag.
// This lets dispatchers show subcommand help without requiring auth first.
func containsHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "-h" || arg == "--help" || arg == "-help" {
			return true
		}
	}
	return false
}
