# Agents

CLI client for Craken Spaces. It stores local session state, talks to
the public control-plane HTTP API, and uses local `ssh` only for final Room
entry.

## Terminology

- **Space** -- a team's private, isolated environment.
- **Room** -- an individual machine inside a Space (one per member or agent).

## Primary specs

- Spec index and report entrypoint: [specs/index.spec.md](specs/index.spec.md)
- Testing and validation: [specs/testing.spec.md](specs/testing.spec.md)
- Public API contract snapshot: [protocol/public-api-v1.openapi.yaml](protocol/public-api-v1.openapi.yaml)

## Canonical validation

- Default repo-owned E2E entrypoint: `specdown run`
- Syntax-only spec validation: `specdown run -dry-run`
- Focused local E2E harness: [scripts/local-e2e.sh](scripts/local-e2e.sh)
- Contract sync helper: [scripts/sync-contract.sh](scripts/sync-contract.sh)

## Quick Reference

- Language: Go 1.26
- Entry point: `cmd/craken/main.go`
- Build/release: GoReleaser + Homebrew tap
- Docs guide: [docs/metadoc.md](docs/metadoc.md)

## Architecture

Local-state client:

- `auth`, `whoami`, `space`, and `ssh key/cert` commands call the public
  control-plane HTTP API
- session state lives in a local JSON file
- `ssh connect` first fetches a short-lived SSH cert from the control plane,
  writes it next to the chosen local private key, then runs local `ssh`

### Environment Variables

| Variable | Required | Description |
|---|---|---|
| `SPACES_BASE_URL` | no | Override the default public control-plane base URL (`https://spaces.borca.ai`) |
| `SPACES_SESSION_FILE` | no | Override local session file path |
| `SPACES_SSH_HOST` | no | Override SSH entry host |
| `SPACES_SSH_PORT` | no | Override SSH entry port (default: `22`) |
| `SPACES_SSH_LOGIN_USER` | no | Space-entry SSH login user (default: `spaces-user`) |
| `SPACES_SSH_KNOWN_HOSTS_FILE` | no | Override known_hosts file used for strict SSH host verification |
| `SPACES_SSH_BIN` | no | Override local `ssh` binary path |

## Development

```sh
go build -o spaces ./cmd/craken
go test ./...
specdown run
```
