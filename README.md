# craken-spaces-cli

CLI client for Craken Spaces.

See [AGENTS.md](AGENTS.md) for architecture and development details.

## Install

```sh
brew install corca-ai/tap/craken-spaces-cli
```

## Usage

```sh
spaces <command> [options]
spaces help
```

The CLI defaults to:

```sh
https://spaces.borca.ai
```

Use another deployment with an HTTPS control-plane URL:

```sh
export SPACES_BASE_URL=https://your-deployment.example.com
spaces <command> [options]
spaces help
```

For local development only, loopback `http://` URLs such as
`http://127.0.0.1:8080` are also accepted.

## Development

```sh
go test ./...
specdown run
./scripts/sync-contract.sh
```
