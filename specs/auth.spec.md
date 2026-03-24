---
type: spec
---

# Authentication

The CLI uses email-based authentication against the public control-plane API.
A successful login stores a session token in a local JSON file; subsequent
commands read that token automatically. Logging out removes the file.

Both **Space admins** and **Space members** use the same login flow:

- Admins log in with an admin auth key, then create and manage Spaces
- Members log in with a member auth key they received from a Space admin, then list Spaces and connect to their Room

```run:shell -> $cli, $tmp
# Test harness -- in normal use, just run "spaces" directly.
. .specdown/test-env
tmp=$(mktemp -d)
spaces_issue_auth_key alice@example.com admin > "$tmp/auth.key"
chmod 600 "$tmp/auth.key"
cat > "$tmp/spaces" <<WRAPPER
#!/bin/sh
export SPACES_BASE_URL=$SPACES_BASE_URL
: "\${SPACES_SESSION_FILE:=$tmp/session.json}"
export SPACES_SESSION_FILE
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

Authenticate with your email and a one-time auth key provided by your
platform approval flow or by your Space admin. On a terminal,
`spaces login alice@example.com`
prompts for the auth key automatically and masks what you type with `*`.
For non-interactive shells, use `--key-file` or `--key-stdin`. The
executable example here stays non-interactive by using `--key-file`:

```run:shell
$ ${cli} login alice@example.com --key-file ${tmp}/auth.key
authenticated as alice@example.com
...
```

## Who Am I

Check which account is currently logged in:

```run:shell
$ ${cli} whoami
alice@example.com
```

## Logout

End the current session and remove the local session file:

```run:shell
$ ${cli} auth logout
...
```

After logout, commands that require authentication return an error:

```run:shell
$ ! ${cli} whoami 2>&1
error: not authenticated; run 'spaces auth login'
```
