package registry

import (
	"sync"
	"time"
)

// ServiceEntry represents a registered service.
type ServiceEntry struct {
	Name      string    `json:"name"`
	IP        string    `json:"ip"`
	Port      int       `json:"port"`
	LastCheck time.Time `json:"last_check"` // Used for TTL or heartbeat expiration
}

// ServiceRegistry keeps track of registered services.
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]ServiceEntry
}

// NewRegistry returns a new in-memory service registry.
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
		Name:      name,
		IP:        ip,
		Port:      port,
		LastCheck: time.Now(),
	}
}

// Lookup retrieves a service by name.
func (r *ServiceRegistry) Lookup(name string) (ServiceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.services[name]
	return entry, ok
}

// Delete removes a service from the registry by name.
func (r *ServiceRegistry) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.services, name)
}
