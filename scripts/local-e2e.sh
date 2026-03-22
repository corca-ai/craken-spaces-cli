#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "missing required command: $1" >&2
		exit 1
	}
}

for cmd in awk curl go grep mktemp python3 sed ssh-keygen; do
	require_cmd "$cmd"
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
managed_root="${SPACES_MANAGED_AGENTS_DIR:-${CRAKEN_MANAGED_AGENTS_DIR:-${repo_root}/../craken-managed-agents}}"
if [[ ! -f "${managed_root}/go.mod" || ! -f "${managed_root}/cmd/spaces/main.go" ]]; then
	echo "managed-agents checkout not found: ${managed_root}" >&2
	exit 1
fi

reserve_port() {
	python3 - <<'PY'
import socket
sock = socket.socket()
sock.bind(("127.0.0.1", 0))
print(sock.getsockname()[1])
sock.close()
PY
}

wait_for_http() {
	local url="$1"
	for _ in $(seq 1 80); do
		if curl -fsS "${url}" >/dev/null 2>&1; then
			return 0
		fi
		sleep 0.25
	done
	echo "timed out waiting for ${url}" >&2
	return 1
}

tmp_dir="$(mktemp -d)"
db_path="${tmp_dir}/control-plane.sqlite3"
proxy_port="${PROXY_PORT:-$(reserve_port)}"
proxy_base_url="http://127.0.0.1:${proxy_port}"
proxy_log="${tmp_dir}/proxy.log"
alice_session="${tmp_dir}/alice.session.json"
bob_session="${tmp_dir}/bob.session.json"
charlie_session="${tmp_dir}/charlie.session.json"
alice_key="${tmp_dir}/alice_ed25519"
ca_key="${tmp_dir}/ssh-user-ca"
spaces_control_bin="${tmp_dir}/spaces-control"
spaces_cli_bin="${tmp_dir}/craken-spaces-cli"

cleanup() {
	set +e
	if [[ -n "${proxy_pid:-}" ]]; then
		kill "${proxy_pid}" 2>/dev/null || true
		wait "${proxy_pid}" 2>/dev/null || true
	fi
	rm -rf "${tmp_dir}"
}
trap cleanup EXIT

(cd "${managed_root}" && go build -o "${spaces_control_bin}" ./cmd/spaces)
(cd "${repo_root}" && go build -o "${spaces_cli_bin}" ./cmd/craken)

export SPACES_EMAIL_MODE=stdout
export SPACES_SSH_CA_KEY="${ca_key}"

"${spaces_control_bin}" --db "${db_path}" init-db --set-admin-key admin-secret >/dev/null
"${spaces_control_bin}" --db "${db_path}" ssh ca-init --ca-key "${ca_key}" >/dev/null

"${spaces_control_bin}" --db "${db_path}" proxy serve \
	--listen "127.0.0.1:${proxy_port}" \
	--upstream-base-url "http://127.0.0.1:9" \
	--upstream-api-key "disabled-no-upstream-key" >"${proxy_log}" 2>&1 &
proxy_pid=$!
wait_for_http "${proxy_base_url}/healthz"

alice_request="$("${spaces_control_bin}" --db "${db_path}" request-access --email alice@example.com --name Alice | awk '/^created access request / {print $4}')"
alice_approval="$("${spaces_control_bin}" --db "${db_path}" --admin-key admin-secret admin approve --application-id "${alice_request}")"
alice_auth_key="$(printf '%s\n' "${alice_approval}" | awk -F': ' '/^Auth key: / {print $2}')"
if [[ -z "${alice_auth_key}" ]]; then
	echo "failed to parse Alice auth key" >&2
	printf '%s\n' "${alice_approval}" >&2
	exit 1
fi

ssh-keygen -q -t ed25519 -N '' -f "${alice_key}"

"${spaces_cli_bin}" --base-url "${proxy_base_url}" --session-file "${alice_session}" auth login \
	--email alice@example.com \
	--key "${alice_auth_key}" >/dev/null

if [[ "$("${spaces_cli_bin}" --session-file "${alice_session}" whoami)" != "alice@example.com" ]]; then
	echo "whoami did not return alice@example.com" >&2
	exit 1
fi

add_key_output="$("${spaces_cli_bin}" --session-file "${alice_session}" ssh add-key --name alice-laptop --public-key-file "${alice_key}.pub")"
alice_fingerprint="$(printf '%s\n' "${add_key_output}" | awk '/^registered ssh key / {print $4}')"
if [[ -z "${alice_fingerprint}" ]]; then
	echo "failed to parse Alice SSH key fingerprint" >&2
	printf '%s\n' "${add_key_output}" >&2
	exit 1
fi
if ! "${spaces_cli_bin}" --session-file "${alice_session}" ssh list-keys | grep -q "alice-laptop"; then
	echo "ssh list-keys did not contain alice-laptop" >&2
	exit 1
fi

create_output="$("${spaces_cli_bin}" --session-file "${alice_session}" room create --name cli-smoke)"
room_id="$(printf '%s\n' "${create_output}" | awk '/^created room / {print $3}')"
if [[ -z "${room_id}" ]]; then
	echo "failed to parse room id" >&2
	printf '%s\n' "${create_output}" >&2
	exit 1
fi

if ! "${spaces_cli_bin}" --session-file "${alice_session}" room list | grep -q "${room_id}"; then
	echo "Alice room list does not contain ${room_id}" >&2
	exit 1
fi

"${spaces_cli_bin}" --session-file "${alice_session}" room up --room "${room_id}" >/dev/null
"${spaces_cli_bin}" --session-file "${alice_session}" room down --room "${room_id}" >/dev/null

issue_output="$("${spaces_cli_bin}" --session-file "${alice_session}" room issue-member-auth-key --room "${room_id}" --email bob@example.com)"
bob_auth_key="$(printf '%s\n' "${issue_output}" | awk -F'=' '/^auth key=/ {print $2}')"
if [[ -z "${bob_auth_key}" ]]; then
	echo "failed to parse Bob auth key" >&2
	printf '%s\n' "${issue_output}" >&2
	exit 1
fi

if ! "${spaces_cli_bin}" --session-file "${alice_session}" room member-auth-keys --room "${room_id}" | grep -q "bob@example.com"; then
	echo "member-auth-keys did not contain bob@example.com" >&2
	exit 1
fi

charlie_issue="$("${spaces_cli_bin}" --session-file "${alice_session}" room issue-member-auth-key --room "${room_id}" --email charlie@example.com)"
charlie_key_id="$(printf '%s\n' "${charlie_issue}" | awk '/^issued room member auth key / {print $6}')"
charlie_auth_key="$(printf '%s\n' "${charlie_issue}" | awk -F'=' '/^auth key=/ {print $2}')"
if [[ -z "${charlie_key_id}" || -z "${charlie_auth_key}" ]]; then
	echo "failed to parse Charlie auth key metadata" >&2
	printf '%s\n' "${charlie_issue}" >&2
	exit 1
fi

"${spaces_cli_bin}" --session-file "${alice_session}" room revoke-member-auth-key --room "${room_id}" --id "${charlie_key_id}" >/dev/null

if "${spaces_cli_bin}" --base-url "${proxy_base_url}" --session-file "${charlie_session}" auth login \
	--email charlie@example.com \
	--key "${charlie_auth_key}" >/dev/null 2>&1; then
	echo "Charlie unexpectedly logged in with a revoked room member auth key" >&2
	exit 1
fi

"${spaces_cli_bin}" --base-url "${proxy_base_url}" --session-file "${bob_session}" auth login \
	--email bob@example.com \
	--key "${bob_auth_key}" >/dev/null

if ! "${spaces_cli_bin}" --session-file "${bob_session}" room list | grep -q "${room_id}"; then
	echo "Bob room list does not contain ${room_id}" >&2
	exit 1
fi

if "${spaces_cli_bin}" --session-file "${bob_session}" room create --name should-fail >/dev/null 2>&1; then
	echo "Bob unexpectedly created a room with no top-level grant" >&2
	exit 1
fi

if "${spaces_cli_bin}" --session-file "${bob_session}" room issue-member-auth-key --room "${room_id}" --email eve@example.com >/dev/null 2>&1; then
	echo "Bob unexpectedly issued a room member auth key" >&2
	exit 1
fi

"${spaces_cli_bin}" --session-file "${alice_session}" ssh issue-cert --identity-file "${alice_key}" >/dev/null
test -f "${alice_key}-cert.pub"
grep -q "ssh-ed25519-cert-v01@openssh.com" "${alice_key}-cert.pub"

"${spaces_cli_bin}" --session-file "${alice_session}" ssh remove-key --fingerprint "${alice_fingerprint}" >/dev/null
if "${spaces_cli_bin}" --session-file "${alice_session}" ssh list-keys | grep -q "alice-laptop"; then
	echo "ssh remove-key did not remove alice-laptop" >&2
	exit 1
fi

"${spaces_cli_bin}" --session-file "${bob_session}" auth logout >/dev/null
if [[ -f "${bob_session}" ]]; then
	echo "logout did not remove Bob session file" >&2
	exit 1
fi
