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
# Sets CRAKEN_BASE_URL and CRAKEN_SESSION_FILE so specs
# never need --base-url or --session-file flags.
cat > .specdown/test-env <<EOF
FAKE_PID=${fake_pid}
SPACES=.specdown/spaces
export CRAKEN_BASE_URL=${fake_url}
EOF
