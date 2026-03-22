package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type workspaceRecord struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Role            string `json:"role"`
	RuntimeDriver   string `json:"runtime_driver"`
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

func cmdRoom(cfg cliConfig, argv []string, stdout, stderr io.Writer) int { //nolint:gocognit // CLI command dispatcher
	if len(argv) == 0 || isHelpWord(argv[0]) {
		printRoomUsage(stdout)
		if len(argv) == 0 {
			return 2
		}
		return 0
	}
	client, _, err := cfg.requireAuthenticatedClient()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	switch argv[0] {
	case "create":
		fs := flag.NewFlagSet("room create", flag.ContinueOnError)
		fs.SetOutput(stderr)
		name := fs.String("name", "", "room name")
		runtimeDriver := fs.String("runtime-driver", "mock", "runtime driver")
		cpuMillis := fs.Int("cpu-millis", 4000, "room CPU ceiling in millicores")
		memoryMiB := fs.Int("memory-mib", 8192, "room memory ceiling in MiB")
		diskMB := fs.Int("disk-mb", 10240, "room writable disk ceiling in MB")
		networkMB := fs.Int("network-egress-mb", 1024, "room cumulative network egress ceiling in MB")
		llmTokens := fs.Int("llm-tokens-limit", 100000, "room LLM token ceiling")
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
			OK        bool            `json:"ok"`
			Error     string          `json:"error"`
			Workspace workspaceRecord `json:"space"`
		}
		if err := client.doJSON("POST", "/api/v1/spaces", map[string]any{
			"name":              *name,
			"runtime_driver":    *runtimeDriver,
			"cpu_millis":        *cpuMillis,
			"memory_mib":        *memoryMiB,
			"disk_mb":           *diskMB,
			"network_egress_mb": *networkMB,
			"llm_tokens_limit":  *llmTokens,
		}, &response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "created room %s (%s)\n", response.Workspace.ID, response.Workspace.Name)
		return 0

	case "list":
		var response struct {
			OK         bool              `json:"ok"`
			Error      string            `json:"error"`
			Workspaces []workspaceRecord `json:"spaces"`
		}
		if err := client.doJSON("GET", "/api/v1/spaces", nil, &response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		rows := make([][]string, 0, len(response.Workspaces))
		for i := range response.Workspaces {
			w := &response.Workspaces[i]
			rows = append(rows, []string{
				w.ID,
				w.Name,
				w.Role,
				w.RuntimeDriver,
				w.RuntimeState,
				strconv.Itoa(w.CPUMillis),
				strconv.Itoa(w.MemoryMiB),
				strconv.Itoa(w.DiskMB),
				strconv.Itoa(w.NetworkEgressMB),
				fmt.Sprintf("%d/%d", w.LLMTokensUsed, w.LLMTokensLimit),
				w.CreatedAt,
			})
		}
		printTable(stdout, []string{"id", "name", "role", "driver", "state", "cpu", "memory", "disk", "net", "llm_tokens", "created_at"}, rows)
		return 0

	case "up", "down":
		fs := flag.NewFlagSet("room "+argv[0], flag.ContinueOnError)
		fs.SetOutput(stderr)
		workspaceID := fs.String("room", "", "room ID")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*workspaceID) == "" {
			fmt.Fprintln(stderr, "error: --room is required")
			return 2
		}
		action := "up"
		if argv[0] == "down" {
			action = "down"
		}
		var response struct {
			OK        bool            `json:"ok"`
			Error     string          `json:"error"`
			Workspace workspaceRecord `json:"space"`
		}
		if err := client.doJSON("POST", "/api/v1/spaces/"+*workspaceID+"/"+action, nil, &response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "room %s is %s\n", response.Workspace.ID, response.Workspace.RuntimeState)
		return 0

	case "delete":
		fs := flag.NewFlagSet("room delete", flag.ContinueOnError)
		fs.SetOutput(stderr)
		workspaceID := fs.String("room", "", "room ID")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*workspaceID) == "" {
			fmt.Fprintln(stderr, "error: --room is required")
			return 2
		}
		if err := client.doJSON("DELETE", "/api/v1/spaces/"+*workspaceID+"/delete", nil, nil); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "deleted room %s\n", *workspaceID)
		return 0

	case "issue-member-auth-key":
		fs := flag.NewFlagSet("room issue-member-auth-key", flag.ContinueOnError)
		fs.SetOutput(stderr)
		workspaceID := fs.String("room", "", "room ID")
		email := fs.String("email", "", "room member email address")
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
		if strings.TrimSpace(*workspaceID) == "" || strings.TrimSpace(*email) == "" {
			fmt.Fprintln(stderr, "error: --room and --email are required")
			return 2
		}
		var response struct {
			OK      bool                `json:"ok"`
			Error   string              `json:"error"`
			AuthKey memberAuthKeyRecord `json:"auth_key"`
			Key     string              `json:"key"`
		}
		if err := client.doJSON("POST", "/api/v1/spaces/"+*workspaceID+"/member-auth-keys", map[string]any{
			"email":             *email,
			"expires_hours":     *expiresHours,
			"cpu_millis":        *cpuMillis,
			"memory_mib":        *memoryMiB,
			"disk_mb":           *diskMB,
			"network_egress_mb": *networkMB,
			"llm_tokens_limit":  *llmTokens,
		}, &response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "issued room member auth key %d for %s\n", response.AuthKey.ID, response.AuthKey.InviteeEmail)
		fmt.Fprintf(stdout, "auth key=%s\n", response.Key)
		fmt.Fprintf(stdout, "expires_at=%s\n", response.AuthKey.ExpiresAt)
		return 0

	case "member-auth-keys":
		fs := flag.NewFlagSet("room member-auth-keys", flag.ContinueOnError)
		fs.SetOutput(stderr)
		workspaceID := fs.String("room", "", "room ID")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*workspaceID) == "" {
			fmt.Fprintln(stderr, "error: --room is required")
			return 2
		}
		var response struct {
			OK       bool                  `json:"ok"`
			Error    string                `json:"error"`
			AuthKeys []memberAuthKeyRecord `json:"auth_keys"`
		}
		if err := client.doJSON("GET", "/api/v1/spaces/"+*workspaceID+"/member-auth-keys", nil, &response); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
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
		fs := flag.NewFlagSet("room revoke-member-auth-key", flag.ContinueOnError)
		fs.SetOutput(stderr)
		workspaceID := fs.String("room", "", "room ID")
		authKeyID := fs.Int64("id", 0, "auth key ID")
		if err := fs.Parse(argv[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if strings.TrimSpace(*workspaceID) == "" || *authKeyID <= 0 {
			fmt.Fprintln(stderr, "error: --room and --id are required")
			return 2
		}
		if err := client.doJSON("DELETE", fmt.Sprintf("/api/v1/spaces/%s/member-auth-keys/%d", *workspaceID, *authKeyID), nil, nil); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "revoked room member auth key %d\n", *authKeyID)
		return 0

	default:
		fmt.Fprintf(stderr, "error: unknown room subcommand %q\n\n", argv[0])
		printRoomUsage(stderr)
		return 2
	}
}

func printRoomUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  spaces room <create|list|up|down|delete|issue-member-auth-key|member-auth-keys|revoke-member-auth-key> [flags]")
}

func printTable(w io.Writer, header []string, rows [][]string) {
	widths := make([]int, len(header))
	for i, cell := range header {
		widths[i] = len(cell)
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	for i, cell := range header {
		fmt.Fprintf(w, "%-*s", widths[i], cell)
		if i+1 < len(header) {
			fmt.Fprint(w, "  ")
		}
	}
	fmt.Fprintln(w)
	for _, row := range rows {
		for i, cell := range row {
			fmt.Fprintf(w, "%-*s", widths[i], cell)
			if i+1 < len(row) {
				fmt.Fprint(w, "  ")
			}
		}
		fmt.Fprintln(w)
	}
}
