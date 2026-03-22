---
type: spec
---

# Authentication

The CLI uses email-based authentication against the public control-plane API.
A successful login stores a session token in a local JSON file; subsequent
commands read that token automatically. Logging out removes the file.

The authentication flow is:

1. `auth login --email EMAIL --key KEY` sends credentials to the control plane
   and saves the returned session token locally.
2. `whoami` reads the saved session and calls `/api/v1/whoami` to confirm the
   authenticated identity.
3. `auth logout` invalidates the server-side session and deletes the local
   session file.

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

## Login

`auth login` requires `--email` and `--key`. On success it prints the
authenticated email and the session file path.

```run:shell
$ ${cli} --base-url ${url} --session-file ${tmp}/session.json auth login --email alice@example.com --key test-key
authenticated as alice@example.com
...
```

The session file is created with the base URL, email, and token:

```run:shell
$ test -f ${tmp}/session.json && echo exists
exists
```

## Who Am I

`whoami` uses the saved session to query the control plane:

```run:shell
$ ${cli} --session-file ${tmp}/session.json whoami
alice@example.com
```

## Login requires both flags

Omitting `--email` or `--key` is an error:

```run:shell !fail
${cli} --base-url ${url} --session-file ${tmp}/no.json auth login --email alice@example.com 2>/dev/null
```

```run:shell !fail
${cli} --base-url ${url} --session-file ${tmp}/no.json auth login --key test-key 2>/dev/null
```

## Logout

`auth logout` removes the local session file:

```run:shell
$ ${cli} --session-file ${tmp}/session.json auth logout
logged out; session removed from ${tmp}/session.json
```

```run:shell
$ test -f ${tmp}/session.json && echo exists || echo removed
removed
```
