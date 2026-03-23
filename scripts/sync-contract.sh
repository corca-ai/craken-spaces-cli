#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
server_root="${SPACES_SERVER_REPO_DIR:-${SPACES_MANAGED_AGENTS_DIR:-${CRAKEN_MANAGED_AGENTS_DIR:-${repo_root}/../craken-spaces}}}"
source_path="${server_root}/protocol/public-api-v1.openapi.yaml"
dest_path="${repo_root}/protocol/public-api-v1.openapi.yaml"

if [[ ! -f "${source_path}" ]]; then
	echo "server public API contract not found: ${source_path}" >&2
	exit 1
fi

mkdir -p "$(dirname "${dest_path}")"
cp "${source_path}" "${dest_path}"
