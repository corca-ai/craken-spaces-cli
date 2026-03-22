# craken-agents-cli

CLI client for Craken Agents.

See [AGENTS.md](AGENTS.md) for architecture and development details.

## Install

```sh
brew install corca-ai/tap/craken
```

## Usage

```sh
craken <command> [options]
craken help
```

The CLI defaults to:

```sh
https://agents.borca.ai
```

Use the dev host or another deployment with:

```sh
export CRAKEN_BASE_URL=https://agents-dev.borca.ai
craken <command> [options]
craken help
```

## Development

```sh
go test ./...
specdown run
./scripts/sync-contract.sh
```
