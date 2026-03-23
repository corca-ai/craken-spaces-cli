package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAuthKeyFile(t *testing.T, dir, value string) string {
	t.Helper()
	path := filepath.Join(dir, "auth.key")
	if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
