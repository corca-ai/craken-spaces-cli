---
type: validation
timeout: 300000
---

# Testing and Validation

## Purpose

This document runs the Go unit test suite. The unit tests use contract-validated
fake servers (backed by the OpenAPI spec in `protocol/public-api-v1.openapi.yaml`)
to verify CLI behavior without any external dependencies.

## Repo preflight

```run:shell
# Verify essential source files exist
test -f go.mod
test -f cmd/craken/main.go
test -f cmd/craken/fake_server_test.go
test -f protocol/public-api-v1.openapi.yaml
command -v go >/dev/null 2>&1
```

## Unit tests

```run:shell
# Run Go unit tests with contract-validated fakes
# (The protocol sync test is skipped here; it only applies when the
#  sibling craken-managed-agents checkout is present.)
go test -run 'Test[^P]' ./...
```
