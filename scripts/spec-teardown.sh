#!/bin/sh
# Global teardown for specdown specs.
# Kills the fake API server and cleans up build artifacts.
set -eu

if [ -f .specdown/test-env ]; then
	. .specdown/test-env
	kill "$FAKE_PID" 2>/dev/null || true
fi

rm -f .specdown/test-env .specdown/fake-url.txt .specdown/spaces
