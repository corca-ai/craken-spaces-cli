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
	if strings.TrimSpace(c.BaseURL) == "" {
		return errors.New("base URL is required")
	}
	endpoint, err := url.JoinPath(strings.TrimSpace(c.BaseURL), path)
	if err != nil {
		return err
	}

	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, endpoint, body)
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
				return errors.New(message)
			}
		}
		return fmt.Errorf("server returned %s", resp.Status)
	}
	if responseBody == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(responseBody)
}
