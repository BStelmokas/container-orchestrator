package registry

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Client interacts with the service registry server.
type Client struct {
	BaseURL string // e.g. "http://localhost:8000"
}

// NewClient creates a registry client.
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL}
}

// Register a service in the registry.
func (c *Client) Register(name, ip string, port int) error {
	endpoint := fmt.Sprintf("%s/register", c.BaseURL)
	values := url.Values{}
	values.Set("name", name)
	values.Set("ip", ip)
	values.Set("port", strconv.Itoa(port))

	resp, err := http.Get(endpoint + "?" + values.Encode())
	if err != nil {
		return fmt.Errorf("registry register error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register failed: %s", resp.Status)
	}

	return nil
}

// Deregister a service from the registry.
func (c *Client) Deregister(name string) error {
	endpoint := c.BaseURL + "/deregister"
	values := url.Values{}
	values.Set("name", name)

	resp, err := http.Get(endpoint + "?" + values.Encode())
	if err != nil {
		return fmt.Errorf("deregister failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deregister returned %s", resp.Status)
	}

	return nil
}
