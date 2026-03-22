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

Use another deployment with:

```sh
export SPACES_BASE_URL=https://your-deployment.example.com
spaces <command> [options]
spaces help
```

## Development

```sh
go test ./...
specdown run
./scripts/sync-contract.sh
```
