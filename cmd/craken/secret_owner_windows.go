//go:build windows

package main

import "os"

func currentUserOwnsFileInfo(info os.FileInfo) bool {
	_ = info
	// Windows ownership semantics do not map cleanly to Unix uid checks.
	return true
}
