package main

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

func sanitizeTerminalText(value string) string {
	value = strings.TrimSpace(value)
	var output strings.Builder
	for _, r := range value {
		switch r {
		case '\n', '\r', '\t':
			output.WriteByte(' ')
		default:
			if unicode.IsControl(r) {
				continue
			}
			output.WriteRune(r)
		}
	}
	return strings.TrimSpace(output.String())
}

func printCLIError(stderr io.Writer, err error) int {
	if err == nil {
		return 0
	}
	_, _ = fmt.Fprintf(stderr, "error: %s\n", sanitizeTerminalText(err.Error()))
	return 1
}
