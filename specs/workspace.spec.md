---
type: spec
---

# Workspace Lifecycle

Workspaces are the primary resource in Craken. Each workspace represents an
isolated compute environment (a "Cell") with configurable resource limits for
CPU, memory, disk, network egress, and LLM tokens.

The workspace lifecycle is: **create** -> **up** (running) -> **down** (stopped) -> **delete**.

Workspace admins can also issue, list, and revoke member auth keys to grant
scoped access to other users.

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

## Setup: authenticate

All workspace commands require an active session.

```run:shell
# Log in as Alice
${cli} --base-url ${url} --session-file ${tmp}/session.json auth login --email alice@example.com --key test-key >/dev/null
```

## Create

`workspace create --name NAME` creates a new workspace with default resource
limits and prints the workspace ID:

```run:shell
$ ${cli} --session-file ${tmp}/session.json workspace create --name my-workspace
created workspace ws_1 (my-workspace)
```

## List

`workspace list` shows a table of all workspaces the user can access:

```run:shell
$ ${cli} --session-file ${tmp}/session.json workspace list | awk 'NR==1{print $1}'
id
```

## Up and Down

`workspace up` starts a workspace; `workspace down` stops it:

```run:shell
$ ${cli} --session-file ${tmp}/session.json workspace up --workspace ws_1
workspace ws_1 is running
```

```run:shell
$ ${cli} --session-file ${tmp}/session.json workspace down --workspace ws_1
workspace ws_1 is stopped
```

## Delete

`workspace delete` permanently removes a workspace:

```run:shell
$ ${cli} --session-file ${tmp}/session.json workspace delete --workspace ws_1
deleted workspace ws_1
```

## Member Auth Keys

Workspace admins can issue scoped auth keys that let other users join a
workspace with delegated resource limits.

### Issue

```run:shell
# Create a workspace for member key tests
${cli} --session-file ${tmp}/session.json workspace create --name key-test >/dev/null
```

```run:shell
$ ${cli} --session-file ${tmp}/session.json workspace issue-member-auth-key --workspace ws_2 --email bob@example.com | head -1
issued workspace member auth key 1 for bob@example.com
```

### List

```run:shell
$ ${cli} --session-file ${tmp}/session.json workspace member-auth-keys --workspace ws_2 | grep bob@example.com | awk '{print $2}'
bob@example.com
```

### Revoke

```run:shell
$ ${cli} --session-file ${tmp}/session.json workspace revoke-member-auth-key --workspace ws_2 --id 1
revoked workspace member auth key 1
```

## Required flags

Workspace subcommands that target a specific workspace require `--workspace`:

```run:shell !fail
${cli} --session-file ${tmp}/session.json workspace up 2>/dev/null
```

`workspace create` requires `--name`:

```run:shell !fail
${cli} --session-file ${tmp}/session.json workspace create 2>/dev/null
```
