package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// doJSONRequest is the shared HTTP helper for the service-oriented CLI.
func doJSONRequest(method, path string, requestBody any, responseBody any) error {
	fullURL := strings.TrimRight(serverURL, "/") + path

	var bodyReader io.Reader
	if requestBody != nil {
		data, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		return fmt.Errorf("API %s %s failed with status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	if responseBody != nil && len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, responseBody); err != nil {
			return fmt.Errorf("failed to decode response JSON: %w", err)
		}
	}

	return nil
}
