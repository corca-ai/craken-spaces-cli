#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
managed_root="${CRAKEN_MANAGED_AGENTS_DIR:-${repo_root}/../craken-managed-agents}"
source_path="${managed_root}/protocol/public-api-v1.openapi.yaml"
dest_path="${repo_root}/protocol/public-api-v1.openapi.yaml"

if [[ ! -f "${source_path}" ]]; then
	echo "managed-agents public API contract not found: ${source_path}" >&2
	exit 1
fi

mkdir -p "$(dirname "${dest_path}")"
cp "${source_path}" "${dest_path}"
