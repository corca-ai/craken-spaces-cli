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

func cmdSpace(cfg cliConfig, argv []string, stdout, stderr io.Writer) int { //nolint:gocognit // CLI command dispatcher
	if len(argv) == 0 || isHelpWord(argv[0]) {
		printSpaceUsage(stdout)
		if len(argv) == 0 {
			return 2
		}
		return 0
	}
	var client apiClient
	var session *localSession
	if !containsHelpFlag(argv) {
		c, s, err := cfg.requireAuthenticatedClient()
		if err != nil {
			return printCLIError(stderr, err)
		}
		client = c
		session = s
		warnAuthenticatedBaseURLOverride(stderr, cfg, session)
	}

	switch argv[0] {
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
		if session != nil && strings.TrimSpace(session.DefaultSpace) == strings.TrimSpace(space.ID) {
			warnSessionUpdate(stderr, "failed to clear default space", clearSessionDefaultSpace(cfg.SessionFile, session))
		}
		fmt.Fprintf(stdout, "deleted space %s\n", sanitizeTerminalText(space.ID))
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
  list                      List Spaces you have access to
  up                        Start a Space (--space ID or exact name required)
  down                      Stop a Space (--space ID or exact name required)
  delete                    Permanently delete a Space (--space ID or exact name required)

Spaces are created automatically when a platform admin approves your access request.

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
