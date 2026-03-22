---
type: guide
---

# craken-spaces-cli

## What is Craken Spaces?

Craken Spaces is a managed runtime where multiple AI agents and humans
collaborate as a team in a shared workspace. Each workspace is an isolated
server environment that can safely run tools like Codex CLI, Claude Code, and
Gemini CLI side by side. Connect your preferred client -- Slack, Telegram, or
any custom integration -- to interact with agents in real time.

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
