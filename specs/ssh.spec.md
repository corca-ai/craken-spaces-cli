---
type: spec
---

# SSH Key and Certificate Management

The CLI manages SSH public keys and short-lived certificates for secure Cell
entry. The flow is:

1. **Register** a local SSH public key with the control plane (`ssh add-key`).
2. **Issue** a short-lived certificate signed by the platform CA (`ssh issue-cert`).
3. **Connect** to a Cell using the certificate (`ssh connect`), or generate an
   OpenSSH config block (`ssh client-config`).

Certificates default to a 5-minute TTL, keeping the attack surface minimal.

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

## Setup: authenticate and create SSH key pair

```run:shell
# Log in and generate a test SSH key pair
${cli} --base-url ${url} --session-file ${tmp}/session.json auth login --email alice@example.com --key test-key >/dev/null
ssh-keygen -q -t ed25519 -N '' -f ${tmp}/id_ed25519
```

## Add Key

`ssh add-key` registers a public key with the control plane:

```run:shell
$ ${cli} --session-file ${tmp}/session.json ssh add-key --name my-laptop --public-key-file ${tmp}/id_ed25519.pub
registered ssh key SHA256:fake1
```

## List Keys

`ssh list-keys` shows all registered public keys:

```run:shell
$ ${cli} --session-file ${tmp}/session.json ssh list-keys | grep my-laptop | awk '{print $2}'
my-laptop
```

## Remove Key

`ssh remove-key` unregisters a key by fingerprint:

```run:shell
$ ${cli} --session-file ${tmp}/session.json ssh remove-key --fingerprint SHA256:fake1
removed ssh key SHA256:fake1
```

## Issue Certificate

`ssh issue-cert` requests a short-lived certificate and writes it next to the
private key:

```run:shell
$ ${cli} --session-file ${tmp}/session.json ssh issue-cert --identity-file ${tmp}/id_ed25519 | head -1
issued ssh certificate ${tmp}/id_ed25519-cert.pub
```

The certificate file is created:

```run:shell
$ test -f ${tmp}/id_ed25519-cert.pub && echo exists
exists
```

## Connect

`ssh connect` issues a certificate and then invokes the local `ssh` binary.
We use a fake `ssh` script to capture the arguments:

```run:shell
# Create a fake ssh binary and a second key pair for connect test
printf '#!/bin/sh\nprintf "%%s\\n" "$@" > %s/ssh-args.txt\n' "${tmp}" > ${tmp}/fake-ssh.sh
chmod +x ${tmp}/fake-ssh.sh
ssh-keygen -q -t ed25519 -N '' -f ${tmp}/id_connect
```

```run:shell
# Connect using the fake ssh binary
CRAKEN_SSH_BIN=${tmp}/fake-ssh.sh ${cli} --session-file ${tmp}/session.json ssh connect --workspace ws_1 --host cell.example.com --identity-file ${tmp}/id_connect --command "echo hi" >/dev/null
```

The fake ssh was called with the expected arguments:

```run:shell
$ grep -c 'CertificateFile' ${tmp}/ssh-args.txt
1
$ grep 'craken-cell@cell.example.com' ${tmp}/ssh-args.txt
craken-cell@cell.example.com
```

## Client Config

`ssh client-config` generates an OpenSSH config block for manual use:

```run:shell
$ ${cli} --session-file ${tmp}/session.json ssh client-config --workspace ws_1 --identity-file ${tmp}/id_ed25519 --host cell.example.com | head -2
Host craken-ws_1
  HostName cell.example.com
```
