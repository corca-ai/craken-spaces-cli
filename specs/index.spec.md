---
type: guide
---

# craken-spaces-cli

## What is Craken Spaces?

Craken Spaces is a managed runtime where AI agents and humans work
together as a team.

A **Space** is your team's private, isolated environment. Inside a Space,
every member and every agent gets their own **Computer** -- a fully
independent machine with its own filesystem, processes, and network.
Computers are isolated from each other, so one member or agent can never
access another's environment.

### Features

- **Isolated Spaces** -- Each Space is hardware-isolated from every other
  Space. Your team's data and processes are completely separated from
  other teams.
- **A Computer for everyone** -- Every human and every AI agent in a
  Space gets their own Computer -- a full machine where you can install
  anything and run anything. Run Codex CLI, Claude Code, Gemini CLI, or
  any tool you need, safely. Each Computer has its own SSH access, API
  access, and dedicated resource budget.
- **Team collaboration** -- Invite members to your Space with scoped
  resource budgets. Work alongside your agents as equals.
- **Agent orchestration** -- Create agents that run persistently in the
  background. Agents can spawn sub-agents and coordinate work, all within
  strict resource limits you control.
- **Bring your own client** -- Connect Slack, Telegram, or any custom
  integration to interact with your agents in real time.
- **Credential management** -- Register API keys for GitHub, AWS, and
  other services once at the Space level. They are kept secure outside
  the runtime and injected transparently -- your tools work unmodified,
  and secrets never appear inside any Computer.

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
