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
managed_root="${SPACES_SERVER_REPO_DIR:-${SPACES_MANAGED_AGENTS_DIR:-${CRAKEN_MANAGED_AGENTS_DIR:-${repo_root}/../craken-spaces}}}"
if [[ ! -f "${managed_root}/go.mod" || ! -f "${managed_root}/cmd/spaces/main.go" ]]; then
	echo "server checkout not found: ${managed_root}" >&2
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
alice_auth_key_file="${tmp_dir}/alice.authkey"
bob_auth_key_file="${tmp_dir}/bob.authkey"
charlie_auth_key_file="${tmp_dir}/charlie.authkey"
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
printf '%s\n' "${alice_auth_key}" > "${alice_auth_key_file}"
chmod 600 "${alice_auth_key_file}"

ssh-keygen -q -t ed25519 -N '' -f "${alice_key}"

"${spaces_cli_bin}" --base-url "${proxy_base_url}" --session-file "${alice_session}" auth login \
	--email alice@example.com \
	--key-file "${alice_auth_key_file}" >/dev/null

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

create_output="$("${spaces_cli_bin}" --session-file "${alice_session}" space create --name cli-smoke)"
space_name="cli-smoke"
space_id="$(printf '%s\n' "${create_output}" | awk '/^created space / {print $3}')"
if [[ -z "${space_id}" ]]; then
	echo "failed to parse space id" >&2
	printf '%s\n' "${create_output}" >&2
	exit 1
fi

if ! "${spaces_cli_bin}" --session-file "${alice_session}" space list | grep -q "${space_id}"; then
	echo "Alice space list does not contain ${space_id}" >&2
	exit 1
fi

"${spaces_cli_bin}" --session-file "${alice_session}" space down --space "${space_name}" >/dev/null

issue_output="$("${spaces_cli_bin}" --session-file "${alice_session}" space issue-member-auth-key --space "${space_name}" --email bob@example.com --auth-key-file "${bob_auth_key_file}")"
if [[ ! -f "${bob_auth_key_file}" ]]; then
	echo "failed to write Bob auth key file" >&2
	printf '%s\n' "${issue_output}" >&2
	exit 1
fi

if ! "${spaces_cli_bin}" --session-file "${alice_session}" space member-auth-keys --space "${space_name}" | grep -q "bob@example.com"; then
	echo "member-auth-keys did not contain bob@example.com" >&2
	exit 1
fi

charlie_issue="$("${spaces_cli_bin}" --session-file "${alice_session}" space issue-member-auth-key --space "${space_name}" --email charlie@example.com --auth-key-file "${charlie_auth_key_file}")"
charlie_key_id="$(printf '%s\n' "${charlie_issue}" | awk '/^issued space member auth key / {print $6}')"
if [[ -z "${charlie_key_id}" || ! -f "${charlie_auth_key_file}" ]]; then
	echo "failed to parse Charlie auth key metadata" >&2
	printf '%s\n' "${charlie_issue}" >&2
	exit 1
fi

"${spaces_cli_bin}" --session-file "${alice_session}" space revoke-member-auth-key --space "${space_name}" --id "${charlie_key_id}" >/dev/null

if "${spaces_cli_bin}" --base-url "${proxy_base_url}" --session-file "${charlie_session}" auth login \
	--email charlie@example.com \
	--key-file "${charlie_auth_key_file}" >/dev/null 2>&1; then
	echo "Charlie unexpectedly logged in with a revoked space member auth key" >&2
	exit 1
fi

"${spaces_cli_bin}" --base-url "${proxy_base_url}" --session-file "${bob_session}" auth login \
	--email bob@example.com \
	--key-file "${bob_auth_key_file}" >/dev/null

if ! "${spaces_cli_bin}" --session-file "${bob_session}" space list | grep -q "${space_id}"; then
	echo "Bob space list does not contain ${space_id}" >&2
	exit 1
fi

if "${spaces_cli_bin}" --session-file "${bob_session}" space create --name should-fail >/dev/null 2>&1; then
	echo "Bob unexpectedly created a space with no top-level grant" >&2
	exit 1
fi

if "${spaces_cli_bin}" --session-file "${bob_session}" space issue-member-auth-key --space "${space_name}" --email eve@example.com >/dev/null 2>&1; then
	echo "Bob unexpectedly issued a space member auth key" >&2
	exit 1
fi

ssh_config_output="$("${spaces_cli_bin}" --session-file "${alice_session}" ssh client-config --space "${space_name}" --host spaces.example.test)"
if ! printf '%s\n' "${ssh_config_output}" | grep -q "RemoteCommand ${space_id}"; then
	echo "ssh client-config did not resolve exact space name to ${space_id}" >&2
	printf '%s\n' "${ssh_config_output}" >&2
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
