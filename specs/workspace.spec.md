---
type: spec
---

# Workspace Lifecycle

Workspaces are the primary resource in Craken Spaces. Each workspace
represents an isolated compute environment (a "Cell") with configurable
resource limits for CPU, memory, disk, network egress, and LLM tokens.

The workspace lifecycle is: **create** -> **up** (running) -> **down** (stopped) -> **delete**.

```run:shell -> $cli, $tmp
# Create wrapper and authenticate
. .specdown/test-env
tmp=$(mktemp -d)
cat > "$tmp/spaces" <<WRAPPER
#!/bin/sh
export CRAKEN_BASE_URL=$CRAKEN_BASE_URL
export CRAKEN_SESSION_FILE=$tmp/session.json
exec $SPACES "\$@"
WRAPPER
chmod +x "$tmp/spaces"
"$tmp/spaces" auth login --email alice@example.com --key test-key >/dev/null
printf '%s\n' "$tmp/spaces" "$tmp"
```

> teardown

```run:shell
rm -rf ${tmp}
```

## Create

Create a new workspace with a name. Default resource limits apply unless
overridden with flags like `--cpu-millis`, `--memory-mib`, `--llm-tokens-limit`:

```run:shell
$ ${cli} workspace create --name my-workspace
created workspace ws_1 (my-workspace)
```

## List

View all workspaces you have access to. The table includes resource limits,
runtime state, and LLM token usage:

```run:shell
$ ${cli} workspace list | awk 'NR==1{print $1}'
id
```

## Up and Down

Start a workspace to make its Cell available for SSH connections:

```run:shell
$ ${cli} workspace up --workspace ws_1
workspace ws_1 is running
```

Stop a workspace when you're done to free resources:

```run:shell
$ ${cli} workspace down --workspace ws_1
workspace ws_1 is stopped
```

## Delete

Permanently remove a workspace and all its data:

```run:shell
$ ${cli} workspace delete --workspace ws_1
deleted workspace ws_1
```

## Member Auth Keys

Workspace admins can invite other users by issuing scoped auth keys. Each
key grants the invitee access to the workspace with delegated resource limits.

### Issuing a key

Issue an auth key for a new member. The key is printed once and should be
shared with the invitee securely:

```run:shell
# Create a workspace for member key tests
${cli} workspace create --name team-project >/dev/null
```

```run:shell
$ ${cli} workspace issue-member-auth-key --workspace ws_2 --email bob@example.com | head -1
issued workspace member auth key 1 for bob@example.com
```

The invitee can then log in with `spaces auth login --email bob@example.com --key <received-key>`.

### Listing keys

View all issued keys for a workspace, including their status:

```run:shell
$ ${cli} workspace member-auth-keys --workspace ws_2 | grep bob@example.com | awk '{print $2}'
bob@example.com
```

### Revoking a key

Revoke a key to immediately deny the member's access:

```run:shell
$ ${cli} workspace revoke-member-auth-key --workspace ws_2 --id 1
revoked workspace member auth key 1
```

## Required flags

Most workspace commands require `--workspace` to identify the target:

```run:shell
# Missing --workspace must fail
! ${cli} workspace up 2>/dev/null
```

`workspace create` requires `--name`:

```run:shell
# Missing --name must fail
! ${cli} workspace create 2>/dev/null
```
