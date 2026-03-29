---
type: spec
---

# Configuration Resolution

The CLI resolves the control-plane base URL from multiple sources with a clear
priority order:

1. **`--base-url` flag** (highest priority)
2. **`SPACES_BASE_URL` environment variable**
3. **Saved session file** (from the last login)
4. **Default** `https://spaces.borca.ai` (lowest priority)

All resolved control-plane URLs must use `https://`, except local development
endpoints on `localhost` or a loopback IP, which may use `http://`.

The session file path is resolved similarly:

1. **`--session-file` flag** (highest)
2. **`SPACES_SESSION_FILE` environment variable**
3. **`SPACES_CONFIG_DIR`** / `session.json`
4. **Default** `~/.config/spaces/session.json`

If none of those sources are available because the home directory cannot be
resolved, the CLI returns an error instead of writing a session file into the
current working directory.

This spec exercises the precedence rules directly, so it uses the raw binary
with explicit environment variables instead of the wrapper.

```run:shell -> $bin, $url, $tmp
# Test harness -- loads raw binary without the wrapper.
. .specdown/test-env
tmp=$(mktemp -d)
export SPACES_BASE_URL
export SPACES_SESSION_FILE="$tmp/session.json"
spaces_issue_auth_key alice@example.com admin > "$tmp/auth.key"
chmod 600 "$tmp/auth.key"
bob_auth_key="$(spaces_issue_auth_key bob@example.com admin)"
printf '%s\n' "$bob_auth_key" > "$tmp/bob.auth.key"
chmod 600 "$tmp/bob.auth.key"
$SPACES auth login --email alice@example.com --key-file "$tmp/auth.key" >/dev/null
ssh-keygen -q -t ed25519 -N '' -f "$tmp/id_test"
printf '%s\n' "$SPACES" "$SPACES_BASE_URL" "$tmp"
```

> teardown

```run:shell
rm -rf ${tmp}
```

## Flag overrides environment

When both `--base-url` and `SPACES_BASE_URL` are set, the flag wins.
Here we set the env var to a bogus URL and pass the correct URL via the flag;
login succeeds because the flag takes priority:

```run:shell
$ SPACES_BASE_URL=http://wrong:9999 SPACES_SESSION_FILE=${tmp}/session.json ${bin} --base-url ${url} auth login --email bob@example.com --key-file ${tmp}/bob.auth.key | head -1
authenticated as bob@example.com
```

## Environment overrides the saved session

When a session already exists, authenticated commands still honor
`SPACES_BASE_URL`. Here we set the env var to a different host and confirm the
generated SSH config uses the env override instead of the saved session host.
The CLI also prints a warning because the saved session token came from a
different deployment and may be rejected there:

```run:shell
$ out="$(SPACES_BASE_URL=https://staging.example.test SPACES_SESSION_FILE=${tmp}/session.json ${bin} ssh client-config --space sp_1 --identity-file ${tmp}/id_test 2>&1)" && printf '%s\n' "$out" | grep -F 'warning: using https://staging.example.test from SPACES_BASE_URL, but the saved session was issued by http://127.0.0.1:' >/dev/null && printf '%s\n' "$out" | grep -F '  HostName staging.example.test' >/dev/null && echo ok
ok
```

## Explicit flag overrides the environment

Passing `--base-url` remains the highest-priority override, even when a session
is already saved and `SPACES_BASE_URL` is set. That explicit override also
prints the same origin-mismatch warning:

```run:shell
$ out="$(SPACES_BASE_URL=https://wrong.example.test SPACES_SESSION_FILE=${tmp}/session.json ${bin} --base-url https://staging.example.test ssh client-config --space sp_1 --identity-file ${tmp}/id_test 2>&1)" && printf '%s\n' "$out" | grep -F 'warning: using https://staging.example.test from --base-url, but the saved session was issued by http://127.0.0.1:' >/dev/null && printf '%s\n' "$out" | grep -F '  HostName staging.example.test' >/dev/null && echo ok
ok
```

## Default base URL

Without any override, the CLI falls back to the production URL. We verify
the help text mentions `spaces.borca.ai` as the default:

```run:shell
$ ${bin} help 2>&1 | grep SPACES_BASE_URL | grep -c spaces.borca.ai
1
```

## SSH environment variables

SSH-related environment variables override defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `SPACES_SSH_HOST` | derived from base URL | SSH entry host |
| `SPACES_SSH_PORT` | `22` | SSH entry port |
| `SPACES_SSH_LOGIN_USER` | `spaces-user` | SSH login user |
| `SPACES_SSH_KNOWN_HOSTS_FILE` | `~/.ssh/spaces_known_hosts` | known_hosts file used for strict host verification |
| `SPACES_SSH_BIN` | `ssh` from PATH | local ssh binary |

Setting `SPACES_SSH_LOGIN_USER` overrides the default login user in
the generated SSH config:

```run:shell
$ SPACES_SSH_LOGIN_USER=custom-user SPACES_SESSION_FILE=${tmp}/session.json ${bin} ssh client-config --space sp_1 --identity-file ${tmp}/id_test --host test.example.com | grep '^  User '
  User custom-user
```

Setting `SPACES_SSH_KNOWN_HOSTS_FILE` adds an explicit `UserKnownHostsFile`
entry to the generated SSH config:

```run:shell
$ SPACES_SSH_KNOWN_HOSTS_FILE=/tmp/spaces-known-hosts SPACES_SESSION_FILE=${tmp}/session.json ${bin} ssh client-config --space sp_1 --identity-file ${tmp}/id_test --host test.example.com | grep UserKnownHostsFile
  UserKnownHostsFile /tmp/spaces-known-hosts
```

Unsafe SSH config values are rejected instead of being emitted into the config:

```run:shell
$ ! SPACES_SSH_LOGIN_USER="$(printf 'spaces-room\nProxyCommand whoami')" SPACES_SESSION_FILE=${tmp}/session.json ${bin} ssh client-config --space sp_1 --identity-file ${tmp}/id_test --host test.example.com 2>&1
error: user must not contain whitespace or control characters
```
