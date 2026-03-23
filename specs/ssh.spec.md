---
type: spec
---

# SSH Key and Certificate Management

The CLI manages SSH public keys and short-lived certificates for secure Room
entry. The typical flow is:

1. **Connect** to a Space with `ssh connect`.
2. The CLI ensures a dedicated local SSH key exists, registers that public key
   if needed, fetches pinned host trust material, issues a short-lived
   certificate, and invokes `ssh`.

For advanced use, you can still register keys manually with `ssh add-key`,
issue certificates manually with `ssh issue-cert`, or generate an OpenSSH
config block with `ssh client-config`.

Certificates default to a 5-minute TTL, keeping the attack surface minimal.

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
ssh-keygen -q -t ed25519 -N '' -f "$tmp/id_ed25519"
printf '%s\n' "$tmp/spaces" "$tmp"
```

> teardown

```run:shell
rm -rf ${tmp}
```

## Registering SSH Keys

### Add a key

Register your SSH public key with a friendly name:

```run:shell
$ ${cli} ssh add-key --name my-laptop --public-key-file ${tmp}/id_ed25519.pub
registered ssh key SHA256:fake1
```

You can also pass the key material inline with `--public-key` instead of
`--public-key-file`.

### List registered keys

```run:shell
$ ${cli} ssh list-keys | awk '{print $1, $2, $3}'
id name fingerprint
1 my-laptop SHA256:fake1
```

### Remove a key

Unregister a key by its fingerprint:

```run:shell
$ ${cli} ssh remove-key --fingerprint SHA256:fake1
removed ssh key SHA256:fake1
```

## Connecting to a Space

### Quick connect

`ssh connect` is the easiest way to enter a Space. It handles certificate
issuance automatically. The `--space` flag accepts either the Space ID
(e.g. `sp_xxx`) or the exact Space name when that name is unique among your
visible Spaces:

```sh
spaces ssh connect --space my-project
```

Behind the scenes, the CLI:

1. Ensures a dedicated local private key exists (defaults to `~/.ssh/id_ed25519_spaces`)
2. Registers the matching public key if the control plane has not seen it yet
3. Fetches a pinned `known_hosts` line for the SSH entry host
4. Sends the public key to the control plane to get a short-lived certificate
5. Writes the certificate next to the private key
6. Runs `ssh` with strict host-key checking, the certificate, the identity file, and the Space target

### OpenSSH config

If you prefer to use `ssh` directly, generate an OpenSSH config block and
paste it into `~/.ssh/config`. The CLI rejects `client-config` inputs that
contain whitespace or control characters, because those values would change the
meaning of the generated `ssh_config` directives:

```run:shell
$ ${cli} ssh client-config --space sp_1 --identity-file ${tmp}/id_ed25519 --host cell.example.com | grep -E 'HostName|StrictHostKeyChecking'
  HostName cell.example.com
  StrictHostKeyChecking yes
```

After adding this to your SSH config, you can connect with just
`ssh spaces-sp_1`.

## Manual Certificate Issuance

For scripting or debugging, you can issue a certificate without connecting:

```run:shell
$ ${cli} ssh issue-cert --identity-file ${tmp}/id_ed25519 | head -1
issued ssh certificate ${tmp}/id_ed25519-cert.pub
```

The certificate is written next to the private key:

```run:shell
$ test -f ${tmp}/id_ed25519-cert.pub && echo exists
exists
```
