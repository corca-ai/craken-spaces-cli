---
type: spec
---

# Authentication

The CLI uses email-based authentication against the public control-plane API.
A successful login stores a session token in a local JSON file; subsequent
commands read that token automatically. Logging out removes the file.

The authentication flow is:

1. `spaces auth login --email EMAIL --key KEY` sends credentials to the
   control plane and saves the returned session token locally.
2. `spaces whoami` reads the saved session and calls `/api/v1/whoami` to
   confirm the authenticated identity.
3. `spaces auth logout` invalidates the server-side session and deletes the
   local session file.

```run:shell -> $cli, $tmp
# Create a wrapper that bakes in base URL and session file
. .specdown/test-env
tmp=$(mktemp -d)
cat > "$tmp/spaces" <<WRAPPER
#!/bin/sh
export CRAKEN_BASE_URL=$CRAKEN_BASE_URL
export CRAKEN_SESSION_FILE=$tmp/session.json
exec $SPACES "\$@"
WRAPPER
chmod +x "$tmp/spaces"
printf '%s\n' "$tmp/spaces" "$tmp"
```

> teardown

```run:shell
rm -rf ${tmp}
```

## Login

On success, `auth login` prints the authenticated email and the session file
path.

```run:shell
$ ${cli} auth login --email alice@example.com --key test-key
authenticated as alice@example.com
...
```

The session file is created:

```run:shell
$ test -f ${tmp}/session.json && echo exists
exists
```

## Who Am I

`whoami` uses the saved session to query the control plane:

```run:shell
$ ${cli} whoami
alice@example.com
```

## Login requires both flags

Omitting `--email` or `--key` is an error:

```run:shell
# Missing --key must fail
! ${cli} auth login --email alice@example.com 2>/dev/null
```

```run:shell
# Missing --email must fail
! ${cli} auth login --key test-key 2>/dev/null
```

## Logout

`auth logout` removes the local session file:

```run:shell
$ ${cli} auth logout
...
```

```run:shell
$ test -f ${tmp}/session.json && echo exists || echo removed
removed
```
