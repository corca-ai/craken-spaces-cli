---
type: spec
---

# Configuration Resolution

The CLI resolves the control-plane base URL from multiple sources with a clear
priority order:

1. **`--base-url` flag** (highest priority)
2. **`CRAKEN_BASE_URL` environment variable**
3. **Saved session file** (from the last `auth login`)
4. **Default** `https://agents.borca.ai` (lowest priority)

This allows per-invocation overrides while keeping the common case zero-config
after the initial login.

```run:shell -> $url, $cli, $tmp
# Load test environment
. .specdown/test-env
tmp=$(mktemp -d)
printf '%s\n' "$FAKE_URL" "$CRAKEN_BIN" "$tmp"
```

> teardown

```run:shell
rm -rf ${tmp}
```

## Flag overrides environment

When both `--base-url` and `CRAKEN_BASE_URL` are set, the flag wins.
We verify this by logging in with the flag pointing to the fake server:

```run:shell
$ CRAKEN_BASE_URL=http://should-not-be-used:9999 ${cli} --base-url ${url} --session-file ${tmp}/flag-test.json auth login --email alice@example.com --key test-key | head -1
authenticated as alice@example.com
```

## Environment overrides session

When `CRAKEN_BASE_URL` is set, it takes precedence over the base URL saved in
the session file. We verify by checking `ssh client-config` host resolution:

```run:shell
# First login saves the fake URL in the session
${cli} --base-url ${url} --session-file ${tmp}/env-test.json auth login --email alice@example.com --key test-key >/dev/null
ssh-keygen -q -t ed25519 -N '' -f ${tmp}/id_env_test
```

```run:shell
$ CRAKEN_BASE_URL=https://agents-dev.borca.ai ${cli} --session-file ${tmp}/env-test.json ssh client-config --workspace ws_1 --identity-file ${tmp}/id_env_test | grep HostName
  HostName agents-dev.borca.ai
```

## Default base URL

Without any override, the CLI falls back to the production URL.
We can observe this in the help output:

```run:shell
$ ${cli} help 2>&1 | grep CRAKEN_BASE_URL | grep -c agents.borca.ai
1
```

## SSH environment variables

SSH-related environment variables override defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAKEN_SSH_HOST` | derived from base URL | Cell-entry SSH host |
| `CRAKEN_SSH_PORT` | `22` | Cell-entry SSH port |
| `CRAKEN_SSH_LOGIN_USER` | `craken-cell` | SSH login user |
| `CRAKEN_SSH_BIN` | `ssh` from PATH | local ssh binary |

The `ssh client-config` command reflects these overrides:

```run:shell
$ CRAKEN_SSH_LOGIN_USER=custom-user ${cli} --session-file ${tmp}/env-test.json ssh client-config --workspace ws_1 --identity-file ${tmp}/id_env_test --host test.example.com | grep User
  User custom-user
```
