---
type: spec
---

# Space Lifecycle

A **Space** is your private environment. Each user gets their own Space
when a platform admin approves their access request. Inside that Space,
you create **Rooms** (fully isolated sandboxes) and launch agents.

The Space lifecycle is: **approve** (auto-created, running) -> **down** (stopped) -> **up** (running) -> **delete**.

```run:shell -> $cli, $tmp
# Test harness -- in normal use, just run "spaces" directly.
. .specdown/test-env
tmp=$(mktemp -d)
spaces_issue_auth_key alice@example.com admin > "$tmp/auth.key"
chmod 600 "$tmp/auth.key"
spaces_create_space alice@example.com my-room >/dev/null
cat > "$tmp/spaces" <<WRAPPER
#!/bin/sh
export SPACES_BASE_URL=$SPACES_BASE_URL
: "\${SPACES_SESSION_FILE:=$tmp/session.json}"
export SPACES_SESSION_FILE
exec $SPACES "\$@"
WRAPPER
chmod +x "$tmp/spaces"
"$tmp/spaces" login alice@example.com --key-file "$tmp/auth.key" >/dev/null
printf '%s\n' "$tmp/spaces" "$tmp"
```

> teardown

```run:shell
rm -rf ${tmp}
```

## Managing Spaces

### List

View all Spaces you have access to. The full table has columns:
id, name, role, state, cpu, memory, disk, net, llm_tokens, and
created_at. Here we show just the key columns:

Commands that take `--space` accept either the exact `sp_...` Space ID or the
exact Space name when that name is unique among Spaces you can access.

```run:shell
$ ${cli} list | awk '{print $1, $2, $4}'
id name state
sp_1 my-room running
```

### Up and Down

`space up` ensures a stopped Space is running:

```run:shell
$ ${cli} space up --space my-room
space sp_1 is running
```

Stop a Space when you're done to free resources:

```run:shell
$ ${cli} space down --space my-room
space sp_1 is stopped
```

### Delete

Permanently remove a Space and all its data:

```run:shell
$ ${cli} space delete --space my-room
deleted space sp_1
```

## Getting Started

You receive an auth key from a platform admin when your access request
is approved. A Space is automatically created for you. Here is the
typical flow:

1. Log in with the auth key you received
2. Use `list` to confirm your Space
3. Run `connect` to enter your Space

```sh
# 1. Log in with the auth key you received
spaces login you@example.com

# 2. List your Spaces
spaces list

# 3. Connect to your Space
spaces connect
```

Once inside, you have a full machine with your own filesystem, processes,
and network -- isolated from every other user and agent.
