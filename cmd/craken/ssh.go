package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type sshKeyRecord struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"created_at"`
}

type sshConnectOptions struct {
	Port           int
	KnownHostsFile string
	CertFile       string
	IdentityFile   string
	User           string
	Host           string
	Target         string
}

type sshClientConfig struct {
	Alias           string
	Host            string
	User            string
	Port            int
	IdentityFile    string
	CertificateFile string
	RoomID          string
	KnownHostsFile  string
}

func cmdSSH(cfg cliConfig, argv []string, stdin io.Reader, stdout, stderr io.Writer) int { //nolint:gocognit // CLI command dispatcher
	if len(argv) == 0 || isHelpWord(argv[0]) {
		printSSHUsage(stdout)
		if len(argv) == 0 {
			return 2
		}
		return 0
	}
	var client apiClient
	if !containsHelpFlag(argv) {
		c, _, err := cfg.requireAuthenticatedClient()
		if err != nil {
			return printCLIError(stderr, err)
		}
		client = c
	}

	switch argv[0] {
	case "add-key":
		fs := flag.NewFlagSet("ssh add-key", flag.ContinueOnError)
		fs.SetOutput(stderr)
		name := fs.String("name", "", "friendly name for this SSH public key")
		publicKey := fs.String("public-key", "", "SSH public key material")
		publicKeyFile := fs.String("public-key-file", "", "path to the SSH public key file")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		keyMaterial, err := resolvePublicKeyInput(*publicKey, *publicKeyFile)
		if err != nil {
			return printCLIError(stderr, err)
		}
		var response struct {
			OK    bool         `json:"ok"`
			Error string       `json:"error"`
			Key   sshKeyRecord `json:"key"`
		}
		if err := client.doJSON("POST", "/api/v1/ssh/keys", map[string]any{"name": *name, "public_key": keyMaterial}, &response); err != nil {
			return printCLIError(stderr, err)
		}
		fmt.Fprintf(stdout, "registered ssh key %s\n", sanitizeTerminalText(response.Key.Fingerprint))
		return 0

	case "list-keys":
		var response struct {
			OK    bool           `json:"ok"`
			Error string         `json:"error"`
			Keys  []sshKeyRecord `json:"keys"`
		}
		if err := client.doJSON("GET", "/api/v1/ssh/keys", nil, &response); err != nil {
			return printCLIError(stderr, err)
		}
		rows := make([][]string, 0, len(response.Keys))
		for _, key := range response.Keys {
			rows = append(rows, []string{strconv.FormatInt(key.ID, 10), key.Name, key.Fingerprint, key.CreatedAt})
		}
		printTable(stdout, []string{"id", "name", "fingerprint", "created_at"}, rows)
		return 0

	case "remove-key":
		fs := flag.NewFlagSet("ssh remove-key", flag.ContinueOnError)
		fs.SetOutput(stderr)
		fingerprint := fs.String("fingerprint", "", "SSH key fingerprint")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*fingerprint) == "" {
			fmt.Fprintln(stderr, "error: --fingerprint is required")
			return 2
		}
		if err := client.doJSON("DELETE", "/api/v1/ssh/keys/"+url.PathEscape(*fingerprint), nil, nil); err != nil {
			return printCLIError(stderr, err)
		}
		fmt.Fprintf(stdout, "removed ssh key %s\n", sanitizeTerminalText(*fingerprint))
		return 0

	case "issue-cert":
		return cmdSSHIssueCert(client, argv[1:], stdout, stderr)

	case "client-config":
		fs := flag.NewFlagSet("ssh client-config", flag.ContinueOnError)
		fs.SetOutput(stderr)
		roomID := fs.String("room", "", "room ID to target")
		host := fs.String("host", "", "SSH host name")
		user := fs.String("user", envOrDefault("SPACES_SSH_LOGIN_USER", "spaces-room"), "SSH login user")
		port := fs.Int("port", parseIntEnv("SPACES_SSH_PORT", 22), "SSH port")
		identityFile := fs.String("identity-file", "", "SSH private key path")
		knownHostsFile := fs.String("known-hosts-file", resolveKnownHostsFile(""), "known_hosts file used for strict host verification")
		alias := fs.String("alias", "", "SSH host alias")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*roomID) == "" {
			fmt.Fprintln(stderr, "error: --room is required")
			return 2
		}
		validRoomID, err := validateSSHRoomID(*roomID)
		if err != nil {
			return printCLIError(stderr, err)
		}
		resolvedHost, err := resolveSSHHost(*host, client.BaseURL)
		if err != nil {
			return printCLIError(stderr, err)
		}
		if strings.TrimSpace(*alias) == "" {
			*alias = "spaces-" + validRoomID
		}
		if strings.TrimSpace(*identityFile) == "" {
			*identityFile, _, err = resolveSSHIdentityFile("")
			if err != nil {
				return printCLIError(stderr, err)
			}
		}
		config, err := renderSSHClientConfig(sshClientConfig{
			Alias:           *alias,
			Host:            resolvedHost,
			User:            *user,
			Port:            *port,
			IdentityFile:    *identityFile,
			CertificateFile: sshCertificateFileForIdentity(*identityFile),
			RoomID:          validRoomID,
			KnownHostsFile:  resolveKnownHostsFile(*knownHostsFile),
		})
		if err != nil {
			return printCLIError(stderr, err)
		}
		fmt.Fprint(stdout, config)
		return 0

	case "connect":
		fs := flag.NewFlagSet("ssh connect", flag.ContinueOnError)
		fs.SetOutput(stderr)
		roomID := fs.String("room", "", "room ID to target")
		host := fs.String("host", "", "SSH host name")
		user := fs.String("user", envOrDefault("SPACES_SSH_LOGIN_USER", "spaces-room"), "SSH login user")
		port := fs.Int("port", parseIntEnv("SPACES_SSH_PORT", 22), "SSH port")
		identityFile := fs.String("identity-file", "", "SSH private key path")
		knownHostsFile := fs.String("known-hosts-file", resolveKnownHostsFile(""), "known_hosts file used for strict host verification")
		certTTL := fs.String("cert-ttl", "5m", "certificate lifetime")
		remoteCommand := fs.String("command", "", "optional command to run inside the Room")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*roomID) == "" {
			fmt.Fprintln(stderr, "error: --room is required")
			return 2
		}
		validRoomID, err := validateSSHRoomID(*roomID)
		if err != nil {
			return printCLIError(stderr, err)
		}
		resolvedHost, err := resolveSSHHost(*host, client.BaseURL)
		if err != nil {
			return printCLIError(stderr, err)
		}
		issued, err := issueSSHCert(client, *identityFile, *user, *certTTL)
		if err != nil {
			return printCLIError(stderr, err)
		}
		sshPath, err := resolveSSHBinary()
		if err != nil {
			return printCLIError(stderr, err)
		}
		target := validRoomID
		if strings.TrimSpace(*remoteCommand) != "" {
			target = target + " -- " + *remoteCommand
		}
		args := buildSSHConnectArgs(sshConnectOptions{
			Port:           *port,
			KnownHostsFile: resolveKnownHostsFile(*knownHostsFile),
			CertFile:       issued.CertFile,
			IdentityFile:   issued.IdentityFile,
			User:           *user,
			Host:           resolvedHost,
			Target:         target,
		})
		cmd := exec.Command(sshPath, args...)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return printCLIError(stderr, err)
		}
		return 0

	default:
		fmt.Fprintf(stderr, "error: unknown ssh subcommand %q\n\n", argv[0])
		printSSHUsage(stderr)
		return 2
	}
}

type issuedSSHCert struct {
	IdentityFile string
	CertFile     string
	Fingerprint  string
	Principal    string
	ExpiresAt    string
}

func cmdSSHIssueCert(client apiClient, argv []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ssh issue-cert", flag.ContinueOnError)
	fs.SetOutput(stderr)
	identityFile := fs.String("identity-file", "", "SSH private key path")
	principal := fs.String("principal", envOrDefault("SPACES_SSH_LOGIN_USER", "spaces-room"), "certificate principal/login user")
	certTTL := fs.String("cert-ttl", "5m", "certificate lifetime")
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	issued, err := issueSSHCert(client, *identityFile, *principal, *certTTL)
	if err != nil {
		return printCLIError(stderr, err)
	}
	fmt.Fprintf(stdout, "issued ssh certificate %s\n", sanitizeTerminalText(issued.CertFile))
	fmt.Fprintf(stdout, "identity_file=%s\n", sanitizeTerminalText(issued.IdentityFile))
	fmt.Fprintf(stdout, "key_fingerprint=%s\n", sanitizeTerminalText(issued.Fingerprint))
	fmt.Fprintf(stdout, "principal=%s\n", sanitizeTerminalText(issued.Principal))
	fmt.Fprintf(stdout, "expires_at=%s\n", sanitizeTerminalText(issued.ExpiresAt))
	return 0
}

func issueSSHCert(client apiClient, identityFile, principal, certTTL string) (issuedSSHCert, error) {
	identityFile, publicKeyFile, err := resolveSSHIdentityFile(identityFile)
	if err != nil {
		return issuedSSHCert{}, err
	}
	publicKeyData, err := os.ReadFile(filepath.Clean(publicKeyFile))
	if err != nil {
		return issuedSSHCert{}, err
	}
	var response struct {
		OK          bool   `json:"ok"`
		Error       string `json:"error"`
		Fingerprint string `json:"fingerprint"`
		Principal   string `json:"principal"`
		ExpiresAt   string `json:"expires_at"`
		Certificate string `json:"certificate"`
	}
	if err := client.doJSON("POST", "/api/v1/ssh/issue-cert", map[string]any{
		"public_key": string(publicKeyData),
		"principal":  principal,
		"cert_ttl":   certTTL,
	}, &response); err != nil {
		return issuedSSHCert{}, err
	}
	certFile := sshCertificateFileForIdentity(identityFile)
	if err := writePrivateFile(certFile, []byte(response.Certificate)); err != nil {
		return issuedSSHCert{}, err
	}
	return issuedSSHCert{
		IdentityFile: identityFile,
		CertFile:     certFile,
		Fingerprint:  response.Fingerprint,
		Principal:    response.Principal,
		ExpiresAt:    response.ExpiresAt,
	}, nil
}

func resolveSSHIdentityFile(identityFile string) (privateKey, publicKey string, _ error) {
	candidates := make([]string, 0, 4)
	if strings.TrimSpace(identityFile) != "" {
		candidates = append(candidates, strings.TrimSpace(identityFile))
	} else {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return "", "", errors.New("no SSH identity file specified and the home directory could not be resolved")
		}
		for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
			candidates = append(candidates, filepath.Join(home, ".ssh", name))
		}
	}
	for _, candidate := range candidates {
		privateKey := strings.TrimSuffix(candidate, ".pub")
		publicKey := privateKey + ".pub"
		if _, err := os.Stat(privateKey); err != nil {
			if os.IsNotExist(err) && strings.TrimSpace(identityFile) == "" {
				continue
			}
			return "", "", err
		}
		if _, err := os.Stat(publicKey); err != nil {
			if os.IsNotExist(err) && strings.TrimSpace(identityFile) == "" {
				continue
			}
			return "", "", err
		}
		return privateKey, publicKey, nil
	}
	return "", "", errors.New("no SSH identity file was found; pass --identity-file or create ~/.ssh/id_ed25519")
}

func sshCertificateFileForIdentity(identityFile string) string {
	identityFile = strings.TrimSuffix(strings.TrimSpace(identityFile), ".pub")
	return identityFile + "-cert.pub"
}

func buildSSHConnectArgs(options sshConnectOptions) []string {
	args := []string{
		"-t",
		"-p", strconv.Itoa(options.Port),
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=yes",
	}
	if strings.TrimSpace(options.KnownHostsFile) != "" {
		args = append(args, "-o", "UserKnownHostsFile="+options.KnownHostsFile)
	}
	args = append(args,
		"-o", "CertificateFile="+options.CertFile,
		"-i", options.IdentityFile,
		fmt.Sprintf("%s@%s", options.User, options.Host),
		options.Target,
	)
	return args
}

func renderSSHClientConfig(config sshClientConfig) (string, error) {
	if err := validateSSHClientConfig(config); err != nil {
		return "", err
	}
	var output strings.Builder
	fmt.Fprintf(&output, "Host %s\n", config.Alias)
	fmt.Fprintf(&output, "  HostName %s\n", config.Host)
	fmt.Fprintf(&output, "  User %s\n", config.User)
	fmt.Fprintf(&output, "  Port %d\n", config.Port)
	fmt.Fprintf(&output, "  RequestTTY yes\n")
	fmt.Fprintf(&output, "  IdentitiesOnly yes\n")
	fmt.Fprintf(&output, "  StrictHostKeyChecking yes\n")
	if strings.TrimSpace(config.KnownHostsFile) != "" {
		fmt.Fprintf(&output, "  UserKnownHostsFile %s\n", config.KnownHostsFile)
	}
	fmt.Fprintf(&output, "  IdentityFile %s\n", config.IdentityFile)
	fmt.Fprintf(&output, "  CertificateFile %s\n", config.CertificateFile)
	fmt.Fprintf(&output, "  RemoteCommand %s\n", config.RoomID)
	fmt.Fprintf(&output, "  ServerAliveInterval 30\n")
	fmt.Fprintf(&output, "  ServerAliveCountMax 3\n")
	return output.String(), nil
}

func validateSSHClientConfig(config sshClientConfig) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "host alias", value: config.Alias},
		{name: "host name", value: config.Host},
		{name: "user", value: config.User},
		{name: "identity file", value: config.IdentityFile},
		{name: "certificate file", value: config.CertificateFile},
	} {
		if err := validateSSHConfigValue(field.name, field.value); err != nil {
			return err
		}
	}
	if _, err := validateSSHRoomID(config.RoomID); err != nil {
		return err
	}
	if strings.TrimSpace(config.KnownHostsFile) != "" {
		if err := validateSSHConfigValue("known hosts file", config.KnownHostsFile); err != nil {
			return err
		}
	}
	return nil
}

func validateSSHRoomID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("room ID is required")
	}
	const allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._:-"
	for _, r := range value {
		if !strings.ContainsRune(allowed, r) {
			return "", errors.New("room ID must contain only letters, numbers, '.', '_', ':', or '-'")
		}
	}
	return value, nil
}

func validateSSHConfigValue(label, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", label)
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("%s must not contain whitespace or control characters", label)
		}
	}
	return nil
}

func resolvePublicKeyInput(inlineValue, filePath string) (string, error) {
	if strings.TrimSpace(inlineValue) != "" && strings.TrimSpace(filePath) != "" {
		return "", errors.New("use only one of --public-key or --public-key-file")
	}
	if strings.TrimSpace(inlineValue) != "" {
		return inlineValue, nil
	}
	if strings.TrimSpace(filePath) == "" {
		return "", errors.New("one of --public-key or --public-key-file is required")
	}
	payload, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func resolveSSHHost(explicitHost, baseURL string) (string, error) {
	if strings.TrimSpace(explicitHost) != "" {
		return strings.TrimSpace(explicitHost), nil
	}
	if envHost := strings.TrimSpace(os.Getenv("SPACES_SSH_HOST")); envHost != "" {
		return envHost, nil
	}
	if strings.TrimSpace(baseURL) == "" {
		return "", errors.New("SSH host is required; pass --host or set SPACES_SSH_HOST")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Hostname() == "" {
		return "", errors.New("base URL does not include a host")
	}
	return parsed.Hostname(), nil
}

func resolveKnownHostsFile(explicitPath string) string {
	if strings.TrimSpace(explicitPath) != "" {
		return strings.TrimSpace(explicitPath)
	}
	return strings.TrimSpace(os.Getenv("SPACES_SSH_KNOWN_HOSTS_FILE"))
}

func resolveSSHBinary() (string, error) {
	if path := strings.TrimSpace(os.Getenv("SPACES_SSH_BIN")); path != "" {
		return path, nil
	}
	return exec.LookPath("ssh")
}

func parseIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func printSSHUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: spaces ssh <subcommand> [flags]

Subcommands:
  add-key          Register an SSH public key
  list-keys        List registered SSH keys
  remove-key       Unregister an SSH key by fingerprint
  issue-cert       Issue a short-lived SSH certificate
  connect          Connect to a Room via SSH
  client-config    Generate an OpenSSH config block

Use "spaces ssh <subcommand> -h" for flag details.
`)
}
