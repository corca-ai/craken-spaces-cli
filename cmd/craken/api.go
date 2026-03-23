package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type apiClient struct {
	BaseURL      string
	SessionToken string
	HTTPClient   *http.Client
}

func (c apiClient) doJSON(method, path string, requestBody, responseBody any) error {
	return c.doJSONQuery(method, path, nil, requestBody, responseBody)
}

func (c apiClient) doJSONQuery(method, path string, query url.Values, requestBody, responseBody any) error {
	if strings.TrimSpace(c.BaseURL) == "" {
		return errors.New("base URL is required")
	}
	baseURL, err := url.Parse(strings.TrimSpace(c.BaseURL))
	if err != nil {
		return err
	}
	joinedPath := joinAPIPath(baseURL.EscapedPath(), path)
	baseURL.Path = joinedPath
	baseURL.RawPath = joinedPath
	if decodedPath, decodeErr := url.PathUnescape(joinedPath); decodeErr == nil {
		baseURL.Path = decodedPath
	}
	baseURL.RawQuery = query.Encode()

	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, baseURL.String(), body)
	if err != nil {
		return err
	}
	if strings.TrimSpace(c.SessionToken) != "" {
		req.Header.Set("Authorization", "Bearer "+c.SessionToken)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on read path

	if resp.StatusCode >= 400 {
		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
			if message, ok := payload["error"].(string); ok && strings.TrimSpace(message) != "" {
				return errors.New(sanitizeTerminalText(message))
			}
		}
		return fmt.Errorf("server returned %s", resp.Status)
	}
	if responseBody == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(responseBody)
}

func joinAPIPath(basePath, path string) string {
	switch {
	case strings.TrimSpace(basePath) == "":
		return "/" + strings.TrimLeft(path, "/")
	case strings.TrimSpace(path) == "":
		return basePath
	default:
		return strings.TrimRight(basePath, "/") + "/" + strings.TrimLeft(path, "/")
	}
}
