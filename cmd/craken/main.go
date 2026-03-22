package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

var version = "dev"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "version":
		fmt.Printf("craken %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		remoteExec(args)
	}
}

// remoteExec proxies a command to the remote host via SSH.
// It replaces the current process with ssh(1).
func remoteExec(args []string) {
	host := os.Getenv("CRAKEN_HOST")
	if host == "" {
		fmt.Fprintln(os.Stderr, "error: CRAKEN_HOST environment variable is required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  export CRAKEN_HOST=<hostname>")
		os.Exit(1)
	}

	args = rewriteArgs(args, host)

	interactive := isInteractive(args)

	var sshArgs []string
	if interactive {
		sshArgs = append(sshArgs, "-tt")
	}

	if port := os.Getenv("CRAKEN_SSH_PORT"); port != "" {
		sshArgs = append(sshArgs, "-p", port)
	}

	target := host
	if user := os.Getenv("CRAKEN_SSH_USER"); user != "" {
		target = user + "@" + host
	}
	sshArgs = append(sshArgs, target, "--")

	remoteBin := "craken"
	if bin := os.Getenv("CRAKEN_REMOTE_BIN"); bin != "" {
		remoteBin = bin
	}
	sshArgs = append(sshArgs, remoteBin)
	sshArgs = append(sshArgs, args...)

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: ssh not found in PATH")
		os.Exit(1)
	}

	execArgs := append([]string{"ssh"}, sshArgs...)
	if err := syscall.Exec(sshPath, execArgs, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "error: exec ssh: %v\n", err)
		os.Exit(1)
	}
}

// rewriteArgs adjusts arguments before remote execution:
//   - ssh connect / ssh client-config: injects --host from CRAKEN_HOST if absent
//   - ssh add-key --public-key-file: reads local file and converts to --public-key
func rewriteArgs(args []string, host string) []string {
	if len(args) < 2 || args[0] != "ssh" {
		return args
	}

	sub := args[1]

	if (sub == "connect" || sub == "client-config") && !hasFlag(args, "--host") {
		args = append(args, "--host", host)
	}

	if sub == "add-key" {
		args = rewritePublicKeyFile(args)
	}

	return args
}

// rewritePublicKeyFile replaces --public-key-file with --public-key by reading
// the local file. This is necessary because the file path is local and won't
// exist on the remote host.
func rewritePublicKeyFile(args []string) []string {
	var filePath string
	fileIdx := -1

	for i, a := range args {
		if a == "--public-key-file" && i+1 < len(args) {
			filePath = args[i+1]
			fileIdx = i
			break
		}
		if strings.HasPrefix(a, "--public-key-file=") {
			filePath = strings.TrimPrefix(a, "--public-key-file=")
			fileIdx = i
			break
		}
	}

	if fileIdx < 0 {
		return args
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading public key file: %v\n", err)
		os.Exit(1)
	}

	pubKey := strings.TrimSpace(string(data))

	var out []string
	for i, a := range args {
		if i == fileIdx {
			out = append(out, "--public-key", pubKey)
			if !strings.Contains(a, "=") {
				i++ // skip the next arg (the file path value)
			}
			continue
		}
		// skip the value of --public-key-file if it was a separate arg
		if fileIdx >= 0 && i == fileIdx+1 && !strings.Contains(args[fileIdx], "=") {
			continue
		}
		out = append(out, a)
	}

	return out
}

func isInteractive(args []string) bool {
	if len(args) < 2 {
		return false
	}
	return args[0] == "ssh" && args[1] == "connect"
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag || strings.HasPrefix(a, flag+"=") {
			return true
		}
	}
	return false
}

func printUsage() {
	fmt.Print(`Usage: craken <command> [options]

Environment:
  CRAKEN_HOST        Remote host (required for all commands except version)
  CRAKEN_SSH_USER    SSH user for host connection (default: current user)
  CRAKEN_SSH_PORT    SSH port for host connection (default: 22)

Commands:
  version              Print the craken CLI version
  auth login           Authenticate with the control plane
  auth logout          Remove session
  whoami               Show authenticated user
  request-access       Request alpha access

  workspace create     Create a workspace
  workspace list       List accessible workspaces
  workspace up         Start workspace runtime
  workspace down       Stop workspace runtime
  workspace delete     Delete a workspace
  workspace add-member Add a member to a workspace

  agent create         Create an agent
  agent list           List agents in a workspace
  agent start          Start an agent
  agent stop           Stop an agent
  agent delete         Delete an agent

  ssh add-key          Register an SSH public key
  ssh list-keys        List registered SSH keys
  ssh remove-key       Remove an SSH public key
  ssh connect          SSH into a workspace Cell
  ssh client-config    Print OpenSSH config for a workspace

  help                 Show this help message

All commands except version are executed on the remote host via SSH.
Pass --help to any subcommand for detailed usage.

https://github.com/corca-ai/craken-cli
`)
}
