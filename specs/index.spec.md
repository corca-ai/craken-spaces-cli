---
type: guide
---

# craken-spaces-cli

CLI client for Craken Spaces. It authenticates users against the
public control-plane HTTP API, manages workspaces and SSH keys, and uses
short-lived SSH certificates for secure Cell entry.

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
