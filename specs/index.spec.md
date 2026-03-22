---
type: guide
---

# craken-cli

CLI client for the Craken Managed Runtime. It authenticates users against the
public control-plane HTTP API, manages workspaces and SSH keys, and uses
short-lived SSH certificates for secure Cell entry.

From the repo root:

- `specdown run` executes the full specification suite against a local fake API
- `specdown run -dry-run` validates spec syntax without executing
- `go test ./...` runs contract-validated unit tests

## Feature Specifications

- [Authentication](auth.spec.md) -- login, logout, whoami
- [Workspace Lifecycle](workspace.spec.md) -- create, list, up, down, delete, member auth keys
- [SSH Keys and Certificates](ssh.spec.md) -- add-key, list-keys, remove-key, issue-cert, connect, client-config
- [Configuration Resolution](config-resolution.spec.md) -- base URL priority, environment variables

## Validation

- [Testing and Validation](testing.spec.md) -- unit test runner
