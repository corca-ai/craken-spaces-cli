package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

type spaceRecord struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	RuntimeState    string `json:"runtime_state"`
	CPUMillis       int    `json:"cpu_millis"`
	MemoryMiB       int    `json:"memory_mib"`
	DiskMB          int    `json:"disk_mb"`
	NetworkEgressMB int    `json:"network_egress_mb"`
	LLMTokensUsed   int    `json:"llm_tokens_used"`
	LLMTokensLimit  int    `json:"llm_tokens_limit"`
	CreatedAt       string `json:"created_at"`
}

type memberAuthKeyRecord struct {
	ID              int64  `json:"id"`
	InviteeEmail    string `json:"invitee_email"`
	IssuedAt        string `json:"issued_at"`
	ExpiresAt       string `json:"expires_at"`
	RedeemedAt      string `json:"redeemed_at"`
	RevokedAt       string `json:"revoked_at"`
	CPUMillis       int    `json:"cpu_millis"`
	MemoryMiB       int    `json:"memory_mib"`
	DiskMB          int    `json:"disk_mb"`
	NetworkEgressMB int    `json:"network_egress_mb"`
	LLMTokensLimit  int    `json:"llm_tokens_limit"`
}

func cmdSpace(cfg cliConfig, argv []string, stdout, stderr io.Writer) int { //nolint:gocognit // CLI command dispatcher
	if len(argv) == 0 || isHelpWord(argv[0]) {
		printSpaceUsage(stdout)
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
	case "create":
		fs := flag.NewFlagSet("space create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		name := fs.String("name", "", "space name")
		cpuMillis := fs.Int("cpu-millis", 4000, "space CPU ceiling in millicores")
		memoryMiB := fs.Int("memory-mib", 8192, "space memory ceiling in MiB")
		diskMB := fs.Int("disk-mb", 10240, "space writable disk ceiling in MB")
		networkMB := fs.Int("network-egress-mb", 1024, "space cumulative network egress ceiling in MB")
		llmTokens := fs.Int("llm-tokens-limit", 100000, "space LLM token ceiling")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*name) == "" {
			fmt.Fprintln(stderr, "error: --name is required")
			return 2
		}
		var response struct {
			OK    bool        `json:"ok"`
			Error string      `json:"error"`
			Space spaceRecord `json:"space"`
		}
		if err := client.doJSON("POST", "/api/v1/spaces", map[string]any{
			"name":              *name,
			"cpu_millis":        *cpuMillis,
			"memory_mib":        *memoryMiB,
			"disk_mb":           *diskMB,
			"network_egress_mb": *networkMB,
			"llm_tokens_limit":  *llmTokens,
		}, &response); err != nil {
			return printCLIError(stderr, err)
		}
		if response.Space.ID != "" && response.Space.RuntimeState != "running" {
			escapedSpaceID := url.PathEscape(strings.TrimSpace(response.Space.ID))
			if err := client.doJSON("POST", "/api/v1/spaces/"+escapedSpaceID+"/up", nil, &response); err != nil {
				return printCLIError(stderr, err)
			}
		}
		fmt.Fprintf(stdout, "created space %s (%s)\n", sanitizeTerminalText(response.Space.ID), sanitizeTerminalText(response.Space.Name))
		if strings.TrimSpace(response.Space.RuntimeState) != "" {
			fmt.Fprintf(stdout, "space %s is %s\n", sanitizeTerminalText(response.Space.ID), sanitizeTerminalText(response.Space.RuntimeState))
		}
		return 0

	case "list":
		spaces, err := listSpaces(client)
		if err != nil {
			return printCLIError(stderr, err)
		}
		rows := make([][]string, 0, len(spaces))
		for i := range spaces {
			s := &spaces[i]
			rows = append(rows, []string{
				s.ID,
				s.Name,
				s.Role,
				s.RuntimeState,
				strconv.Itoa(s.CPUMillis),
				strconv.Itoa(s.MemoryMiB),
				strconv.Itoa(s.DiskMB),
				strconv.Itoa(s.NetworkEgressMB),
				fmt.Sprintf("%d/%d", s.LLMTokensUsed, s.LLMTokensLimit),
				s.CreatedAt,
			})
		}
		printTable(stdout, []string{"id", "name", "role", "state", "cpu", "memory", "disk", "net", "llm_tokens", "created_at"}, rows)
		return 0

	case "up", "down":
		fs := flag.NewFlagSet("space "+argv[0], flag.ContinueOnError)
		fs.SetOutput(stderr)
		spaceRef := fs.String("space", "", "space ID or exact space name")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*spaceRef) == "" {
			fmt.Fprintln(stderr, "error: --space is required")
			return 2
		}
		space, err := resolveSpaceRef(client, *spaceRef)
		if err != nil {
			return printCLIError(stderr, err)
		}
		action := "up"
		if argv[0] == "down" {
			action = "down"
		}
		escapedSpaceID := url.PathEscape(strings.TrimSpace(space.ID))
		var response struct {
			OK    bool        `json:"ok"`
			Error string      `json:"error"`
			Space spaceRecord `json:"space"`
		}
		if err := client.doJSON("POST", "/api/v1/spaces/"+escapedSpaceID+"/"+action, nil, &response); err != nil {
			return printCLIError(stderr, err)
		}
		fmt.Fprintf(stdout, "space %s is %s\n", sanitizeTerminalText(response.Space.ID), sanitizeTerminalText(response.Space.RuntimeState))
		return 0

	case "delete":
		fs := flag.NewFlagSet("space delete", flag.ContinueOnError)
		fs.SetOutput(stderr)
		spaceRef := fs.String("space", "", "space ID or exact space name")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*spaceRef) == "" {
			fmt.Fprintln(stderr, "error: --space is required")
			return 2
		}
		space, err := resolveSpaceRef(client, *spaceRef)
		if err != nil {
			return printCLIError(stderr, err)
		}
		escapedSpaceID := url.PathEscape(strings.TrimSpace(space.ID))
		if err := client.doJSON("DELETE", "/api/v1/spaces/"+escapedSpaceID+"/delete", nil, nil); err != nil {
			return printCLIError(stderr, err)
		}
		fmt.Fprintf(stdout, "deleted space %s\n", sanitizeTerminalText(space.ID))
		return 0

	case "issue-member-auth-key":
		fs := flag.NewFlagSet("space issue-member-auth-key", flag.ContinueOnError)
		fs.SetOutput(stderr)
		spaceRef := fs.String("space", "", "space ID or exact space name")
		email := fs.String("email", "", "space member email address")
		authKeyFile := fs.String("auth-key-file", "", "path to securely write the issued auth key")
		expiresHours := fs.Int("expires-hours", 24*7, "lifetime in hours")
		cpuMillis := fs.Int("cpu-millis", 1000, "delegated CPU ceiling")
		memoryMiB := fs.Int("memory-mib", 1024, "delegated memory ceiling")
		diskMB := fs.Int("disk-mb", 1024, "delegated disk ceiling")
		networkMB := fs.Int("network-egress-mb", 256, "delegated network ceiling")
		llmTokens := fs.Int("llm-tokens-limit", 10000, "delegated monthly LLM token budget")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*spaceRef) == "" || strings.TrimSpace(*email) == "" || strings.TrimSpace(*authKeyFile) == "" {
			fmt.Fprintln(stderr, "error: --space, --email, and --auth-key-file are required")
			return 2
		}
		space, err := resolveSpaceRef(client, *spaceRef)
		if err != nil {
			return printCLIError(stderr, err)
		}
		escapedSpaceID := url.PathEscape(strings.TrimSpace(space.ID))
		var response struct {
			OK      bool                `json:"ok"`
			Error   string              `json:"error"`
			AuthKey memberAuthKeyRecord `json:"auth_key"`
			Key     string              `json:"key"`
		}
		if err := client.doJSON("POST", "/api/v1/spaces/"+escapedSpaceID+"/member-auth-keys", map[string]any{
			"email":             *email,
			"expires_hours":     *expiresHours,
			"cpu_millis":        *cpuMillis,
			"memory_mib":        *memoryMiB,
			"disk_mb":           *diskMB,
			"network_egress_mb": *networkMB,
			"llm_tokens_limit":  *llmTokens,
		}, &response); err != nil {
			return printCLIError(stderr, err)
		}
		if err := writeSecretFile(*authKeyFile, response.Key); err != nil {
			return printCLIError(stderr, err)
		}
		fmt.Fprintf(stdout, "issued space member auth key %d for %s\n", response.AuthKey.ID, sanitizeTerminalText(response.AuthKey.InviteeEmail))
		fmt.Fprintf(stdout, "auth_key_file=%s\n", sanitizeTerminalText(*authKeyFile))
		fmt.Fprintf(stdout, "expires_at=%s\n", sanitizeTerminalText(response.AuthKey.ExpiresAt))
		return 0

	case "member-auth-keys":
		fs := flag.NewFlagSet("space member-auth-keys", flag.ContinueOnError)
		fs.SetOutput(stderr)
		spaceRef := fs.String("space", "", "space ID or exact space name")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*spaceRef) == "" {
			fmt.Fprintln(stderr, "error: --space is required")
			return 2
		}
		space, err := resolveSpaceRef(client, *spaceRef)
		if err != nil {
			return printCLIError(stderr, err)
		}
		escapedSpaceID := url.PathEscape(strings.TrimSpace(space.ID))
		var response struct {
			OK       bool                  `json:"ok"`
			Error    string                `json:"error"`
			AuthKeys []memberAuthKeyRecord `json:"auth_keys"`
		}
		if err := client.doJSON("GET", "/api/v1/spaces/"+escapedSpaceID+"/member-auth-keys", nil, &response); err != nil {
			return printCLIError(stderr, err)
		}
		rows := make([][]string, 0, len(response.AuthKeys))
		for i := range response.AuthKeys {
			k := &response.AuthKeys[i]
			status := "active"
			switch {
			case strings.TrimSpace(k.RevokedAt) != "":
				status = "revoked"
			case strings.TrimSpace(k.RedeemedAt) != "":
				status = "redeemed"
			}
			rows = append(rows, []string{
				strconv.FormatInt(k.ID, 10),
				k.InviteeEmail,
				status,
				k.ExpiresAt,
				k.IssuedAt,
			})
		}
		printTable(stdout, []string{"id", "email", "status", "expires_at", "issued_at"}, rows)
		return 0

	case "revoke-member-auth-key":
		fs := flag.NewFlagSet("space revoke-member-auth-key", flag.ContinueOnError)
		fs.SetOutput(stderr)
		spaceRef := fs.String("space", "", "space ID or exact space name")
		authKeyID := fs.Int64("id", 0, "auth key ID")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*spaceRef) == "" || *authKeyID <= 0 {
			fmt.Fprintln(stderr, "error: --space and --id are required")
			return 2
		}
		space, err := resolveSpaceRef(client, *spaceRef)
		if err != nil {
			return printCLIError(stderr, err)
		}
		escapedSpaceID := url.PathEscape(strings.TrimSpace(space.ID))
		if err := client.doJSON("DELETE", fmt.Sprintf("/api/v1/spaces/%s/member-auth-keys/%d", escapedSpaceID, *authKeyID), nil, nil); err != nil {
			return printCLIError(stderr, err)
		}
		fmt.Fprintf(stdout, "revoked space member auth key %d\n", *authKeyID)
		return 0

	default:
		fmt.Fprintf(stderr, "error: unknown space subcommand %q\n\n", argv[0])
		printSpaceUsage(stderr)
		return 2
	}
}

func printSpaceUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: spaces space <subcommand> [flags]

Subcommands:
  create                    Create a new Space (--name required)
  list                      List Spaces you have access to
  up                        Start a Space (--space ID or exact name required)
  down                      Stop a Space (--space ID or exact name required)
  delete                    Permanently delete a Space (--space ID or exact name required)
  issue-member-auth-key     Invite a member with a scoped auth key
  member-auth-keys          List issued member auth keys for a Space
  revoke-member-auth-key    Revoke a member auth key

Use "spaces space <subcommand> -h" for flag details.
`)
}

func printTable(w io.Writer, header []string, rows [][]string) {
	safeHeader := make([]string, len(header))
	for i, cell := range header {
		safeHeader[i] = sanitizeTerminalText(cell)
	}
	safeRows := make([][]string, len(rows))
	for i, row := range rows {
		safeRows[i] = make([]string, len(row))
		for j, cell := range row {
			safeRows[i][j] = sanitizeTerminalText(cell)
		}
	}
	widths := make([]int, len(safeHeader))
	for i, cell := range safeHeader {
		widths[i] = len(cell)
	}
	for _, row := range safeRows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	for i, cell := range safeHeader {
		fmt.Fprintf(w, "%-*s", widths[i], cell)
		if i+1 < len(header) {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)
	for _, row := range safeRows {
		for i, cell := range row {
			fmt.Fprintf(w, "%-*s", widths[i], cell)
			if i+1 < len(row) {
				fmt.Fprint(w, "  ")
			}
		}
		fmt.Fprintln(w)
	}
}
