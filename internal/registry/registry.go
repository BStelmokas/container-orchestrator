package registry

import (
	"sync"
	"time"
)

// ServiceEntry represents a registered service.
type ServiceEntry struct {
	Name string
	IP string
	Port int
	LastCheck time.Time // Used for TTL or heartbeat epiration
}

// ServiceRegistry keeps track of registered services.
type ServiceRegistry struct {
	mu sync.RWMutex
	services map[string]ServiceEntry
}

// NewRegistry creates a new in-memory service registry.
func NewRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]ServiceEntry),
	}
}

// Register adds or updates a service in the registry.
func (r *ServiceRegistry) Register(name, ip string, port int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.services[name] = ServiceEntry{
		Name: name,
		IP: ip,
		Port: port,
		LastCheck: time.Now(),
	}
}

// Lookup retrieves a service by name.
func (r *ServiceRegistry) Lookup(name string) (ServiceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.services[name]
	return entry, exists
}
