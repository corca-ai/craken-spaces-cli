---
type: spec
---

# Room Lifecycle

A **Room** is your isolated machine inside a Space. Each Room has
configurable resource limits for CPU, memory, disk, network egress,
and LLM tokens.

There are two roles in a Space:

- **Admin** -- creates and manages Rooms, invites members, controls resource budgets.
- **Member** -- receives an auth key from an admin, logs in, and uses their Room.

The Room lifecycle is: **create** -> **up** (running) -> **down** (stopped) -> **delete**.

```run:shell -> $cli, $tmp
# Test harness -- in normal use, just run "spaces" directly.
. .specdown/test-env
tmp=$(mktemp -d)
cat > "$tmp/spaces" <<WRAPPER
#!/bin/sh
export SPACES_BASE_URL=$SPACES_BASE_URL
export SPACES_SESSION_FILE=$tmp/session.json
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

## Admin: Managing Rooms

These commands are available to Space admins who create and control Rooms.

### Create

Create a new Room with a name. Default resource limits apply unless
overridden with flags like `--cpu-millis`, `--memory-mib`, `--llm-tokens-limit`:

```run:shell
$ ${cli} room create --name my-room
created room ws_1 (my-room)
```

### List

View all Rooms you have access to. The full table has columns:
id, name, role, driver, state, cpu, memory, disk, net, llm_tokens, and
created_at. Here we show just the key columns:

```run:shell
$ ${cli} room list | awk '{print $1, $2, $5}'
id name state
ws_1 my-room stopped
```

### Up and Down

Start a Room to make it available for SSH connections:

```run:shell
$ ${cli} room up --room ws_1
room ws_1 is running
```

Stop a Room when you're done to free resources:

```run:shell
$ ${cli} room down --room ws_1
room ws_1 is stopped
```

### Delete

Permanently remove a Room and all its data:

```run:shell
$ ${cli} room delete --room ws_1
deleted room ws_1
```

## Admin: Inviting Members

Space admins invite members by issuing scoped auth keys. Each key grants
the invitee access to a Room with delegated resource limits that the admin
controls.

### Issuing a key

Issue an auth key for a new member. The key is printed once and should be
shared with the invitee securely:

```run:shell
# Create a Room for member key tests
${cli} room create --name team-project >/dev/null
```

```run:shell
$ ${cli} room issue-member-auth-key --room ws_2 --email bob@example.com | head -1
issued room member auth key 1 for bob@example.com
```

The invitee can then log in with `spaces auth login --email bob@example.com --key <received-key>`.

### Listing keys

View all issued keys for a Room, including their status. The full table
has columns: id, email, status, expires_at, and issued_at. Here we show the key columns:

```run:shell
$ ${cli} room member-auth-keys --room ws_2 | awk '{print $1, $2, $3}'
id email status
1 bob@example.com active
```

### Revoking a key

Revoke a key to immediately deny the member's access:

```run:shell
$ ${cli} room revoke-member-auth-key --room ws_2 --id 1
revoked room member auth key 1
```

## Member: Getting Started

As a member, you receive an auth key from a Space admin. Here is the
typical flow to get into your Room:

```sh
# 1. Log in with the auth key you received
spaces auth login --email you@example.com --key AUTH_KEY_FROM_ADMIN

# 2. Register your SSH public key (one-time setup)
spaces ssh add-key --name my-laptop --public-key-file ~/.ssh/id_ed25519.pub

# 3. List your Rooms to find the Room ID
spaces room list

# 4. Connect to your Room
spaces ssh connect --room ws_xxx
```

Once inside, you have a full machine with your own filesystem, processes,
and network -- isolated from every other member and agent in the Space.
