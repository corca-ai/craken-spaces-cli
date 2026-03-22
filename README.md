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

Use the dev host or another deployment with:

```sh
export CRAKEN_BASE_URL=https://spaces-dev.borca.ai
spaces <command> [options]
spaces help
```

## Development

```sh
go test ./...
specdown run
./scripts/sync-contract.sh
```
