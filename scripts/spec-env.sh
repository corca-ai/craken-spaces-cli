#!/bin/sh
# Global setup for specdown specs.
#
# Builds the CLI binary and starts a fake API server.
# Writes .specdown/test-env sourced by individual spec files.
set -eu

mkdir -p .specdown

# Build CLI once
go build -o .specdown/spaces ./cmd/craken

# Start fake API server in background
python3 scripts/fake-api.py > .specdown/fake-url.txt &
fake_pid=$!

# Wait for the server to print its URL (up to 5 seconds)
i=0
while [ $i -lt 50 ]; do
	if [ -s .specdown/fake-url.txt ]; then
		break
	fi
	sleep 0.1
	i=$((i + 1))
done

if [ ! -s .specdown/fake-url.txt ]; then
	echo "fake API server did not start" >&2
	kill "$fake_pid" 2>/dev/null || true
	exit 1
fi

fake_url="$(cat .specdown/fake-url.txt)"

# Write env file for specs to source.
# Sets SPACES_BASE_URL so specs
# never need --base-url or --session-file flags.
cat > .specdown/test-env <<EOF
FAKE_PID=${fake_pid}
SPACES=.specdown/spaces
SPACES_FAKE_API_URL=${fake_url}
spaces_issue_auth_key() {
  if [ "\$#" -lt 2 ] || [ "\$#" -gt 3 ]; then
    echo "usage: spaces_issue_auth_key EMAIL ROLE [SPACE_ID]" >&2
    return 2
  fi
  email="\$1"
  role="\$2"
  space_id="\${3:-}"
  if [ -n "\$space_id" ]; then
    curl -fsS -G --data-urlencode "email=\$email" --data-urlencode "role=\$role" --data-urlencode "space_id=\$space_id" "\$SPACES_FAKE_API_URL/__test/issue-auth-key"
    return
  fi
  curl -fsS -G --data-urlencode "email=\$email" --data-urlencode "role=\$role" "\$SPACES_FAKE_API_URL/__test/issue-auth-key"
}
spaces_create_space() {
  if [ "\$#" -ne 2 ]; then
    echo "usage: spaces_create_space EMAIL NAME" >&2
    return 2
  fi
  curl -fsS -G --data-urlencode "email=\$1" --data-urlencode "name=\$2" "\$SPACES_FAKE_API_URL/__test/create-space" >/dev/null
}
export SPACES_BASE_URL=${fake_url}
export SPACES_FAKE_API_URL
EOF
