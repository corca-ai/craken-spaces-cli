---
type: guide
---

# craken-spaces-cli

## What is Craken Spaces?

Craken Spaces is a managed runtime where AI agents and humans collaborate
as a team inside secure, isolated workspaces.

### Features

- **Sandboxed workspaces** -- Each workspace runs in its own
  hardware-isolated environment. Run Codex CLI, Claude Code, Gemini CLI,
  or any program you need -- safely, without risking your local machine or
  other teams' data.
- **Human-agent parity** -- Humans and AI agents get the same isolated
  environment, the same API access, and the same resource budgets. SSH in
  and work alongside your agents as equals.
- **Team collaboration** -- Invite members to a workspace with scoped
  resource budgets. Each member and agent operates in their own protected
  space within the shared workspace.
- **Agent orchestration** -- Create agents that run persistently in the
  background. Agents can spawn sub-agents and coordinate work, all within
  strict resource limits you control.
- **Bring your own client** -- Connect Slack, Telegram, or any custom
  integration to interact with your agents in real time.
- **Credential management** -- Register API keys for GitHub, AWS, and
  other services once at the workspace level. They are kept secure outside
  the runtime and injected transparently -- your tools work unmodified,
  and secrets never appear inside agent environments.

Craken Spaces is currently invite-only.
[Join the waitlist](https://forms.gle/daowdtLnDBCmRwxH8) to get early access.

## What is this CLI?

`spaces` is the command-line client for Craken Spaces. It authenticates
against the control-plane API, manages workspaces and SSH keys, and uses
short-lived certificates for secure workspace entry.

## Install

```sh
brew install corca-ai/tap/craken-spaces-cli
```

Or download the binary directly:

```sh
curl -sSfL https://raw.githubusercontent.com/corca-ai/craken-spaces-cli/main/install.sh | sh
```

## Quick Start

```sh
# 1. Log in with your email and auth key
spaces auth login --email you@example.com --key YOUR_AUTH_KEY

# 2. Create a workspace
spaces workspace create --name my-project

# 3. Register your SSH key
spaces ssh add-key --name my-laptop --public-key-file ~/.ssh/id_ed25519.pub

# 4. Connect to your workspace
spaces ssh connect --workspace ws_xxx
```

## Feature Specifications

- [Authentication](auth.spec.md) -- login, logout, whoami
- [Workspace Lifecycle](workspace.spec.md) -- create, list, up, down, delete, member auth keys
- [SSH Keys and Certificates](ssh.spec.md) -- add-key, list-keys, remove-key, issue-cert, connect, client-config
- [Configuration Resolution](config-resolution.spec.md) -- base URL priority, environment variables

## Validation

- [Testing and Validation](testing.spec.md) -- unit test runner
