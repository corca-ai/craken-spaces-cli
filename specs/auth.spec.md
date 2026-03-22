---
type: spec
---

# Authentication

The CLI uses email-based authentication against the public control-plane API.
A successful login stores a session token in a local JSON file; subsequent
commands read that token automatically. Logging out removes the file.

```run:shell -> $cli, $tmp
# Test harness -- in normal use, just run "spaces" directly.
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

Authenticate with your email and a one-time auth key provided by your
Space admin:

```run:shell
$ ${cli} auth login --email alice@example.com --key test-key
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
