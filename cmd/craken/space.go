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
	case "create":
		return cmdCreateWithClient(cfg, client, session, "space create", argv[1:], stdout, stderr)

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

type spaceCreateRequest struct {
	Name      string
	CPUMillis int
	MemoryMiB int
	DiskMB    int
	NetworkMB int
	LLMTokens int
}

func cmdCreateCommand(cfg cliConfig, commandName string, argv []string, stdout, stderr io.Writer) int {
	client, session, err := cfg.requireAuthenticatedClient()
	if err != nil {
		return printCLIError(stderr, err)
	}
	warnAuthenticatedBaseURLOverride(stderr, cfg, session)
	return cmdCreateWithClient(cfg, client, session, commandName, argv, stdout, stderr)
}

func cmdCreateWithClient(cfg cliConfig, client apiClient, session *localSession, commandName string, argv []string, stdout, stderr io.Writer) int {
	if len(argv) > 0 && isHelpWord(argv[0]) {
		printCreateUsage(stdout, commandName)
		return 0
	}
	request, code, done := parseSpaceCreateRequest(commandName, argv, stderr)
	if done {
		return code
	}
	return runSpaceCreate(cfg, client, session, request, stdout, stderr)
}

func parseSpaceCreateRequest(commandName string, argv []string, stderr io.Writer) (spaceCreateRequest, int, bool) {
	fs := flag.NewFlagSet(commandName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printCreateUsage(fs.Output(), commandName)
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}
	name := fs.String("name", "", "space name")
	cpuMillis := fs.Int("cpu-millis", 4000, "space CPU ceiling in millicores")
	memoryMiB := fs.Int("memory-mib", 8192, "space memory ceiling in MiB")
	diskMB := fs.Int("disk-mb", 10240, "space writable disk ceiling in MB")
	networkMB := fs.Int("network-egress-mb", 1024, "space cumulative network egress ceiling in MB")
	llmTokens := fs.Int("llm-tokens-limit", 100000, "space LLM token ceiling")
	if len(argv) > 0 && !strings.HasPrefix(argv[0], "-") && !isHelpWord(argv[0]) {
		argv = append([]string{"--name", argv[0]}, argv[1:]...)
	}
	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return spaceCreateRequest{}, 0, true
		}
		return spaceCreateRequest{}, 2, true
	}
	if extra := fs.Args(); len(extra) > 0 {
		fmt.Fprintf(stderr, "error: unexpected arguments: %s\n\n", strings.Join(extra, " "))
		fs.Usage()
		return spaceCreateRequest{}, 2, true
	}
	request := spaceCreateRequest{
		Name:      *name,
		CPUMillis: *cpuMillis,
		MemoryMiB: *memoryMiB,
		DiskMB:    *diskMB,
		NetworkMB: *networkMB,
		LLMTokens: *llmTokens,
	}
	return request, 0, false
}

func runSpaceCreate(cfg cliConfig, client apiClient, session *localSession, request spaceCreateRequest, stdout, stderr io.Writer) int {
	if strings.TrimSpace(request.Name) == "" {
		fmt.Fprintln(stderr, "error: --name is required")
		return 2
	}
	var response struct {
		OK    bool        `json:"ok"`
		Error string      `json:"error"`
		Space spaceRecord `json:"space"`
	}
	if err := client.doJSON("POST", "/api/v1/spaces", map[string]any{
		"name":              request.Name,
		"cpu_millis":        request.CPUMillis,
		"memory_mib":        request.MemoryMiB,
		"disk_mb":           request.DiskMB,
		"network_egress_mb": request.NetworkMB,
		"llm_tokens_limit":  request.LLMTokens,
	}, &response); err != nil {
		return printCLIError(stderr, err)
	}
	if response.Space.ID != "" && response.Space.RuntimeState != "running" {
		escapedSpaceID := url.PathEscape(strings.TrimSpace(response.Space.ID))
		if err := client.doJSON("POST", "/api/v1/spaces/"+escapedSpaceID+"/up", nil, &response); err != nil {
			return printCLIError(stderr, err)
		}
	}
	warnSessionUpdate(stderr, "failed to save default space", setSessionDefaultSpace(cfg.SessionFile, session, response.Space.ID))
	fmt.Fprintf(stdout, "created space %s (%s)\n", sanitizeTerminalText(response.Space.ID), sanitizeTerminalText(response.Space.Name))
	if strings.TrimSpace(response.Space.RuntimeState) != "" {
		fmt.Fprintf(stdout, "space %s is %s\n", sanitizeTerminalText(response.Space.ID), sanitizeTerminalText(response.Space.RuntimeState))
	}
	return 0
}

func printCreateUsage(w io.Writer, commandName string) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintf(w, "  spaces %s SPACE [flags]\n", commandName)
	fmt.Fprintf(w, "  spaces %s --name SPACE [flags]\n", commandName)
}

func printSpaceUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: spaces space <subcommand> [flags]

Subcommands:
  create                    Create a new Space (--name required)
  list                      List Spaces you have access to
  up                        Start a Space (--space ID or exact name required)
  down                      Stop a Space (--space ID or exact name required)
  delete                    Permanently delete a Space (--space ID or exact name required)

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
