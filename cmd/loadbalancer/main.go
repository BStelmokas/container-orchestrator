package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

/////////////////////////////////////////////////////////
// Structs and Clients

// ServiceEntry represents a single registered instance of a service,
// as returned by the service registry.
// This structure must match the JSON schema the registry sends back.
type ServiceEntry struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	LastCheck string `json:"last_check"` // ignored here
}

// RegistryClient is a client that talks to the service registry
// over HTTP to look up service instances.
type RegistryClient struct {
	BaseURL string // e.g., "http://localhost:8000"
}

// Lookup queries the registry for all healthy instances of a given service.
// It assumes the registry exposes an endpoint like /lookup-all?name=...
func (rc *RegistryClient) Lookup(name string) ([]ServiceEntry, error) {
	url := fmt.Sprintf("%s/lookup-all?name=%s", rc.BaseURL, name) // build full query URL
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("lookup failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lookup failed: %s", resp.Status)
	}

	// Decode the response into a slice of ServiceEntry structs
	var services []ServiceEntry
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	return services, nil
}

/////////////////////////////////////////////////////////
// Round-Robin Load Balancer

// RoundRobin holds counters to track the next backend to use per service name.
// It is safe for concurrent use by multiple goroutines.
type RoundRobin struct {
	mu       sync.Mutex     // protects the counters map
	counters map[string]int // maps serviceName -> next index to use
}

// NewRoundRobin creates and initializes a RoundRobin instance.
func NewRoundRobin() *RoundRobin {
	return &RoundRobin{
		counters: make(map[string]int),
	}
}

// Next returns the next backend to use for a given service name,
// cycling through the available instances using round-robin logic.
func (rr *RoundRobin) Next(name string, backends []ServiceEntry) *ServiceEntry {
	if len(backends) == 0 {
		return nil // no available instances
	}

	rr.mu.Lock()
	defer rr.mu.Unlock()

	// Get the current index and update it for next time
	index := rr.counters[name] % len(backends)
	rr.counters[name] = rr.counters[name] + 1

	return &backends[index]
}

/////////////////////////////////////////////////////////
// HTTP Handler: /proxy/{serviceName}

// makeProxyHandler returns an HTTP handler that acts as a reverse proxy.
// It looks up service instances and forwards the request to one of them.
func makeProxyHandler(reg *RegistryClient, rr *RoundRobin) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract service name from the URL path: /proxy/{serviceName}
		serviceName := r.URL.Path[len("/proxy/"):]
		if serviceName == "" {
			http.Error(w, "Missing service name", http.StatusBadRequest)
			return
		}

		// Look up all available backends for that service
		backends, err := reg.Lookup(serviceName)
		if err != nil || len(backends) == 0 {
			http.Error(w, "Service not available", http.StatusServiceUnavailable)
			return
		}

		// Choose a backend using round-robin logic
		target := rr.Next(serviceName, backends)
		if target == nil {
			http.Error(w, "No backend available", http.StatusServiceUnavailable)
			return
		}

		// Build the target service URL (e.g., http://172.17.0.2:8080)
		targetURL := fmt.Sprintf("http://%s:%d", target.IP, target.Port)
		remote, err := url.Parse(targetURL)
		if err != nil {
			http.Error(w, "Invalid backend URL", http.StatusInternalServerError)
			return
		}

		// Use Go's built-in reverse proxy to forward the request
		proxy := httputil.NewSingleHostReverseProxy(remote)

		// Custom error handler for failed upstream requests
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
			log.Printf("[Proxy Error] %v", e)
			http.Error(w, "Upstream error", http.StatusBadGateway)
		}

		// Forward the incoming request to the selected backend
		proxy.ServeHTTP(w, r)
	}
}

func main() {
	// Initialize the service registry client
	reg := &RegistryClient{
		BaseURL: "http://localhost:8000", // change if needed
	}

	// Create a new round-robin router instance
	rr := NewRoundRobin()

	// Route: /proxy/{serviceName} -> forwards to one of that service's backends
	http.HandleFunc("/proxy/", makeProxyHandler(reg, rr))

	// Start the load balancer on port 9000
	log.Println("Load balancer listening on :9000")
	if err := http.ListenAndServe(":9000", nil); err != nil {
		log.Fatalf("Load balancer error: %v", err)
	}
}
