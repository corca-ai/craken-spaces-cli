package main

import (
	"os"
	"reflect"
	"testing"
)

func TestHasFlag(t *testing.T) {
	tests := []struct {
		args []string
		flag string
		want bool
	}{
		{[]string{"ssh", "connect", "--host", "h"}, "--host", true},
		{[]string{"ssh", "connect", "--host=h"}, "--host", true},
		{[]string{"ssh", "connect", "--workspace", "w"}, "--host", false},
		{[]string{"ssh", "connect"}, "--host", false},
	}
	for _, tt := range tests {
		if got := hasFlag(tt.args, tt.flag); got != tt.want {
			t.Errorf("hasFlag(%v, %q) = %v, want %v", tt.args, tt.flag, got, tt.want)
		}
	}
}

func TestRewriteArgs_InjectsHost(t *testing.T) {
	args := []string{"ssh", "connect", "--workspace", "ws_123"}
	got := rewriteArgs(args, "myhost")
	if !hasFlag(got, "--host") {
		t.Errorf("expected --host to be injected, got %v", got)
	}

	args = []string{"ssh", "client-config", "--workspace", "ws_123"}
	got = rewriteArgs(args, "myhost")
	if !hasFlag(got, "--host") {
		t.Errorf("expected --host to be injected for client-config, got %v", got)
	}
}

func TestRewriteArgs_PreservesExplicitHost(t *testing.T) {
	args := []string{"ssh", "connect", "--workspace", "ws_123", "--host", "explicit"}
	got := rewriteArgs(args, "myhost")

	hostCount := 0
	for _, a := range got {
		if a == "--host" {
			hostCount++
		}
	}
	if hostCount != 1 {
		t.Errorf("expected exactly 1 --host flag, got %d in %v", hostCount, got)
	}
}

func TestRewriteArgs_NoRewriteForOtherCommands(t *testing.T) {
	args := []string{"workspace", "list"}
	got := rewriteArgs(args, "myhost")
	if !reflect.DeepEqual(got, args) {
		t.Errorf("expected no rewrite, got %v", got)
	}
}

func TestRewritePublicKeyFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "pubkey-*.pub")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString("ssh-ed25519 AAAA testkey\n"); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	t.Run("separate flag and value", func(t *testing.T) {
		args := []string{"ssh", "add-key", "--name", "mykey", "--public-key-file", tmp.Name()}
		got := rewritePublicKeyFile(args)

		if hasFlag(got, "--public-key-file") {
			t.Errorf("--public-key-file should be removed, got %v", got)
		}
		if !hasFlag(got, "--public-key") {
			t.Errorf("--public-key should be added, got %v", got)
		}

		for i, a := range got {
			if a == "--public-key" && i+1 < len(got) {
				if got[i+1] != "ssh-ed25519 AAAA testkey" {
					t.Errorf("unexpected public key value: %q", got[i+1])
				}
			}
		}
	})

	t.Run("combined flag=value", func(t *testing.T) {
		args := []string{"ssh", "add-key", "--public-key-file=" + tmp.Name()}
		got := rewritePublicKeyFile(args)

		if hasFlag(got, "--public-key-file") {
			t.Errorf("--public-key-file should be removed, got %v", got)
		}
		if !hasFlag(got, "--public-key") {
			t.Errorf("--public-key should be added, got %v", got)
		}
	})
}

func TestIsInteractive(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"ssh", "connect"}, true},
		{[]string{"ssh", "list-keys"}, false},
		{[]string{"workspace", "list"}, false},
		{[]string{"version"}, false},
	}
	for _, tt := range tests {
		if got := isInteractive(tt.args); got != tt.want {
			t.Errorf("isInteractive(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}
