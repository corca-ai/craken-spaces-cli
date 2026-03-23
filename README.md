# craken-spaces-cli

CLI client for Craken Spaces.

See [AGENTS.md](AGENTS.md) for architecture and development details.

## Install

```sh
brew install corca-ai/tap/craken-spaces-cli
```

If you use the installer script, download it first and run it locally instead
of piping it straight to `sh`:

```sh
curl -sSfL -o install.sh https://raw.githubusercontent.com/corca-ai/craken-spaces-cli/main/install.sh
sh install.sh
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
