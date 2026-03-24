---
type: guide
---

# craken-spaces-cli

## What is Craken Spaces?

Craken Spaces is a managed runtime where AI agents and humans work
together as a team.

A **Space** is your team's private, isolated environment. Inside a Space,
every member and every agent gets their own **Room** -- a fully
independent machine with its own filesystem, processes, and network.
Rooms are isolated from each other, so one member or agent can never
access another's environment.

### Features

- **Isolated Spaces** -- Each Space is hardware-isolated from every other
  Space. Your team's data and processes are completely separated from
  other teams.
- **A Room for everyone** -- Every human and every AI agent in a
  Space gets their own Room -- a full machine where you can install
  anything and run anything. Run Codex CLI, Claude Code, Gemini CLI, or
  any tool you need, safely. Each Room has its own SSH access, API
  access, and dedicated resource budget.
- **Team collaboration** -- Invite members to your Space with scoped
  resource budgets. Work alongside your agents as equals.
- **Agent orchestration** -- Create agents that run persistently in the
  background. Agents can spawn sub-agents and coordinate work, all within
  strict resource limits you control.
- **Bring your own client** -- Connect Slack, Telegram, or any custom
  integration to interact with your agents in real time.
- **Credential management** -- Register API keys for GitHub, AWS, and
  other services once at the Space level. They are kept secure outside
  the runtime and injected transparently -- your tools work unmodified,
  and secrets never appear inside any Room.

Craken Spaces is currently invite-only.
[Join the waitlist](https://forms.gle/daowdtLnDBCmRwxH8) to get early access.

## What is this CLI?

`spaces` is the command-line client for Craken Spaces. It authenticates
against the control-plane API, manages Spaces, and uses local `ssh` for
final Room entry. Most users only need three commands:

- `spaces login you@example.com` -- log in and paste the auth key when prompted
- `spaces space list` -- see which Spaces you can access
- `spaces connect` -- enter your Room; the CLI uses your default Space (or the only Space you can access) and handles SSH key creation, registration, host trust material, and short-lived cert issuance automatically

## Install

```sh
brew install corca-ai/tap/craken-spaces-cli
```

Or download the installer script locally, review it, then run it:

```sh
curl -sSfL -o install.sh https://raw.githubusercontent.com/corca-ai/craken-spaces-cli/main/install.sh
sh install.sh
```

## Quick Start for Admins

Admins create Spaces, invite members, and control resource
budgets. To get your admin auth key,
[join the waitlist](https://forms.gle/daowdtLnDBCmRwxH8) and you will
receive one when your access is approved.

If you are a **Space admin**, your normal workflow is:

1. Log in with your admin auth key
2. Create a Space for your team
3. Connect to that Space yourself
4. Issue member auth keys for teammates
5. Stop or delete the Space when you are done

```sh
# 1. Log in and paste your auth key when prompted
spaces login you@example.com

# 2. Create a Space
spaces space create --name my-project
# → created space sp_xxx (my-project)
# → space sp_xxx is running  (create auto-starts the Space)

# 3. Connect
spaces connect

# 4. Invite a team member with scoped resource limits
spaces space issue-member-auth-key --space my-project --email teammate@example.com --auth-key-file ./teammate.authkey
# → share the auth key file contents with your teammate securely, then delete the file

# 5. Stop the Space later if you want to release runtime resources
spaces space down --space my-project
```

## Quick Start for Members

Members receive an auth key from a Space admin. That key grants access
to a Space with delegated resource limits, and SSH lands them in their
Room inside that Space.

If you are a **Space member**, your normal workflow is:

1. Log in with the auth key you received
2. Check which Spaces you can access
3. Connect to your own Room inside that Space

Members can use `space list` and `ssh connect`, but they cannot create
Spaces or issue member auth keys.

```sh
# 1. Log in and paste the auth key you received
spaces login you@example.com

# 2. Find your Space ID or exact Space name if you do not already know it
spaces space list

# 3. Connect to your Space
spaces connect
```

## Feature Specifications

- [Authentication](auth.spec.md) -- login, logout, whoami
- [Space Lifecycle](space.spec.md) -- admin space management, member invitations, member onboarding
- [SSH Keys and Certificates](ssh.spec.md) -- automatic `ssh connect` flow plus advanced key/cert commands

## Reference

- [Configuration Resolution](config-resolution.spec.md) -- base URL priority, environment variables

## Validation

- [Testing and Validation](testing.spec.md) -- unit test runner
