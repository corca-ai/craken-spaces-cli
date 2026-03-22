---
type: spec
---

# SSH Key and Certificate Management

The CLI manages SSH public keys and short-lived certificates for secure Room
entry. The typical flow is:

1. **Register** your SSH public key once with `ssh add-key`.
2. **Connect** to a Room with `ssh connect` -- the CLI automatically
   issues a short-lived certificate and invokes `ssh`.

For advanced use, you can issue certificates manually with `ssh issue-cert`
or generate an OpenSSH config block with `ssh client-config`.

Certificates default to a 5-minute TTL, keeping the attack surface minimal.

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

## Connecting to a Room

### Quick connect

`ssh connect` is the easiest way to enter a Room. It handles certificate
issuance automatically. The `--room` flag takes the Room ID (e.g. `ws_xxx`)
shown by `room create` or `room list`:

```sh
spaces ssh connect --room ws_xxx
```

Behind the scenes, the CLI:

1. Reads your local private key (defaults to `~/.ssh/id_ed25519`)
2. Sends the public key to the control plane to get a short-lived certificate
3. Writes the certificate next to the private key
4. Runs `ssh` with the certificate, identity file, and Room target

### OpenSSH config

If you prefer to use `ssh` directly, generate an OpenSSH config block and
paste it into `~/.ssh/config`:

```run:shell
$ ${cli} ssh client-config --room ws_1 --identity-file ${tmp}/id_ed25519 --host cell.example.com | head -2
Host craken-ws_1
  HostName cell.example.com
```

After adding this to your SSH config, you can connect with just
`ssh craken-ws_1`.

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
