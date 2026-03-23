package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSanitizeTerminalText(t *testing.T) {
	got := sanitizeTerminalText(" hello\x1b[31m\nworld\t ")
	if got != "hello[31m world" {
		t.Fatalf("sanitizeTerminalText = %q", got)
	}
}

func TestDoJSONSanitizesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("{\"error\":\"bad\\u001b[31m\\nnews\"}"))
	}))
	defer server.Close()

	err := apiClient{BaseURL: server.URL}.doJSON("GET", "/boom", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "\x1b") || strings.Contains(err.Error(), "\n") {
		t.Fatalf("error was not sanitized: %q", err.Error())
	}
}

func TestPrintTableSanitizesCells(t *testing.T) {
	var buf bytes.Buffer
	printTable(&buf, []string{"id", "name"}, [][]string{{"1", "bad\x1b[31m\nname"}})

	output := buf.String()
	if strings.Contains(output, "\x1b") {
		t.Fatalf("table output still contains escape sequence: %q", output)
	}
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header + row after sanitization, got %d lines: %q", len(lines), output)
	}
}
