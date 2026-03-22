---
type: spec
---

# Configuration Resolution

The CLI resolves the control-plane base URL from multiple sources with a clear
priority order:

1. **`--base-url` flag** (highest priority)
2. **`CRAKEN_BASE_URL` environment variable**
3. **Saved session file** (from the last login)
4. **Default** `https://agents.borca.ai` (lowest priority)

The session file path is resolved similarly:

1. **`--session-file` flag** (highest)
2. **`CRAKEN_SESSION_FILE` environment variable**
3. **`CRAKEN_CONFIG_DIR`** / `session.json`
4. **Default** `~/.config/craken/session.json`

This spec exercises the precedence rules directly, so it uses the raw binary
with explicit environment variables instead of the wrapper.

```run:shell -> $bin, $url, $tmp
# Test harness -- loads raw binary without the wrapper.
. .specdown/test-env
tmp=$(mktemp -d)
export CRAKEN_BASE_URL
export CRAKEN_SESSION_FILE="$tmp/session.json"
$SPACES auth login --email alice@example.com --key test-key >/dev/null
ssh-keygen -q -t ed25519 -N '' -f "$tmp/id_test"
printf '%s\n' "$SPACES" "$CRAKEN_BASE_URL" "$tmp"
```

> teardown

```run:shell
rm -rf ${tmp}
```

## Flag overrides environment

When both `--base-url` and `CRAKEN_BASE_URL` are set, the flag wins.
Here we set the env var to a bogus URL and pass the correct URL via the flag;
login succeeds because the flag takes priority:

```run:shell
$ CRAKEN_BASE_URL=http://wrong:9999 CRAKEN_SESSION_FILE=${tmp}/session.json ${bin} --base-url ${url} auth login --email bob@example.com --key test-key | head -1
authenticated as bob@example.com
```

## Environment overrides session

When `CRAKEN_BASE_URL` is set, it takes precedence over the base URL saved in
the session file. Here we point the env var to a dev URL and confirm the
generated SSH config picks it up as the hostname:

```run:shell
$ CRAKEN_BASE_URL=https://agents-dev.borca.ai CRAKEN_SESSION_FILE=${tmp}/session.json ${bin} ssh client-config --room ws_1 --identity-file ${tmp}/id_test | grep HostName
  HostName agents-dev.borca.ai
```

## Default base URL

Without any override, the CLI falls back to the production URL. We verify
the help text mentions `agents.borca.ai` as the default:

```run:shell
$ ${bin} help 2>&1 | grep CRAKEN_BASE_URL | grep -c agents.borca.ai
1
```

## SSH environment variables

SSH-related environment variables override defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAKEN_SSH_HOST` | derived from base URL | Room-entry SSH host |
| `CRAKEN_SSH_PORT` | `22` | Room-entry SSH port |
| `CRAKEN_SSH_LOGIN_USER` | `craken-cell` | SSH login user |
| `CRAKEN_SSH_BIN` | `ssh` from PATH | local ssh binary |

Setting `CRAKEN_SSH_LOGIN_USER` overrides the default login user in
the generated SSH config:

```run:shell
$ CRAKEN_SSH_LOGIN_USER=custom-user CRAKEN_SESSION_FILE=${tmp}/session.json ${bin} ssh client-config --room ws_1 --identity-file ${tmp}/id_test --host test.example.com | grep User
  User custom-user
```
