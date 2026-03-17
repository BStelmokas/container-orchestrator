package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Client interacts with the service registry server.
type Client struct {
	baseURL string // e.g. "http://localhost:8000"
}

// NewClient creates a registry client.
func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL}
}

type registerRequest struct {
	ServiceName string `json:"serviceName"`
	InstanceID string `json:"instanceID"`
	IP string `json:"ip"`
	Port int `json:"port"`
}

type deregisterRequest struct {
	ServiceName string `json:"serviceName"`
	InstanceID string `json:"instanceID"`
}

// Register a service in the registry.
func (c *Client) Register(serviceName, instanceID, ip string, port int) error {
	body := registerRequest{
		ServiceName: serviceName,
		InstanceID: instanceID,
		IP: ip,
		Port: port,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode register request: %w", err)
	}

	resp, err := http.Post(c.baseURL+"/register", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to call register endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("registry register failed with status: %s", resp.Status)
	}

	return nil
}

// Deregister a service from the registry.
func (c *Client) Deregister(serviceName, instanceID string) error {
	body := deregisterRequest{
		ServiceName: serviceName,
		InstanceID: instanceID,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode deregister request: %w", err)
	}

	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/deregister", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to build deregister request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call deregister endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("registry deregister failed with status %d", resp.StatusCode)
	}

	return nil
}

// LookupAll returns every registered backend for a logical service.
func (c *Client) LookupAll(serviceName string) ([]ServiceInstance, error) {
	endpoint := c.baseURL + "/lookup-all?name=" + url.QueryEscape(serviceName)

	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to call lookup-all endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("registry lookup-all failed with status %d", resp.StatusCode)
	}

	var result []ServiceInstance
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode lookup-all response: %w", err)
	}

	return result, nil
}
