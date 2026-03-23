---
type: spec
---

# Space Lifecycle

A **Space** is your team's private environment. Each member and agent gets
their own **Room** inside that Space, with isolated processes, filesystem,
and network.

There are two roles in a Space:

- **Admin** -- creates and manages Spaces, invites members, controls resource budgets.
- **Member** -- receives an auth key from an admin, logs in, and uses their Room.

The Space lifecycle is: **create** (running) -> **down** (stopped) -> **up** (running) -> **delete**.

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
"$tmp/spaces" auth login --email alice@example.com --key-file "$tmp/auth.key" >/dev/null
printf '%s\n' "$tmp/spaces" "$tmp"
```

> teardown

```run:shell
rm -rf ${tmp}
```

## Admin: Managing Spaces

These commands are available to Space admins who create and control Spaces.

### Create

Create a new Space with a name. Default resource limits apply unless
overridden with flags like `--cpu-millis`, `--memory-mib`, `--llm-tokens-limit`:

```run:shell
$ ${cli} space create --name my-room
created space sp_1 (my-room)
space sp_1 is running
```

### List

View all Spaces you have access to. The full table has columns:
id, name, role, driver, state, cpu, memory, disk, net, llm_tokens, and
created_at. Here we show just the key columns:

Commands that take `--space` accept either the exact `sp_...` Space ID or the
exact Space name when that name is unique among Spaces you can access.

```run:shell
$ ${cli} space list | awk '{print $1, $2, $5}'
id name state
sp_1 my-room running
```

### Up and Down

`space up` is still available and idempotent if you want to ensure a stopped
Space is running before SSH connections:

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

## Admin: Inviting Members

Space admins invite members by issuing scoped auth keys. Each key grants
the invitee access to a Space with delegated resource limits that the admin
controls. SSH then lands the member in their own Room inside that Space.

### Issuing a key

Issue an auth key for a new member. The key is printed once and should be
shared with the invitee securely:

```run:shell
# Create a Space for member key tests
${cli} space create --name team-project >/dev/null
```

```run:shell
$ ${cli} space issue-member-auth-key --space team-project --email bob@example.com --auth-key-file ${tmp}/bob.authkey | head -2
issued space member auth key 1 for bob@example.com
auth_key_file=${tmp}/bob.authkey
```

The invitee can then log in with `spaces auth login --email bob@example.com`
and paste the received auth key when prompted, or use
`--key-file /path/to/received-auth.key` for non-interactive shells.

### Listing keys

View all issued keys for a Space, including their status. The full table
has columns: id, email, status, expires_at, and issued_at. Here we show the key columns:

```run:shell
$ ${cli} space member-auth-keys --space team-project | awk '{print $1, $2, $3}'
id email status
1 bob@example.com active
```

### Member permissions

A member can see their delegated Space but cannot create Spaces or invite others:

```run:shell
$ SPACES_SESSION_FILE=${tmp}/bob.session.json ${cli} auth login --email bob@example.com --key-file ${tmp}/bob.authkey >/dev/null && SPACES_SESSION_FILE=${tmp}/bob.session.json ${cli} space list | awk '{print $2, $3}'
name role
team-project member
```

```run:shell
$ ! SPACES_SESSION_FILE=${tmp}/bob.session.json ${cli} space create --name should-fail 2>&1
error: forbidden
```

```run:shell
$ ! SPACES_SESSION_FILE=${tmp}/bob.session.json ${cli} space issue-member-auth-key --space team-project --email eve@example.com --auth-key-file ${tmp}/eve.authkey 2>&1
error: forbidden
```

### Revoking a key

Revoke a key to immediately deny the member's access:

```run:shell
$ ${cli} space revoke-member-auth-key --space team-project --id 1
revoked space member auth key 1
```

## Member: Getting Started

As a member, you receive an auth key from a Space admin. Here is the
typical flow to get into your Room:

```sh
# 1. Log in with the auth key you received
spaces auth login --email you@example.com

# 2. List your Spaces to find the Space ID or exact name
spaces space list

# 3. Connect to your Space
spaces ssh connect --space sp_xxx
```

Once inside, you have a full machine with your own filesystem, processes,
and network -- isolated from every other member and agent in the Space.
