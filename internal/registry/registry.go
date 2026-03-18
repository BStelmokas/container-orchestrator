package registry

import (
	"sync"
)

// ServiceRegistry keeps track of registered services.
type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]map[string]ServiceInstance
}

// NewRegistry returns a new in-memory service registry.
func NewRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]map[string]ServiceInstance),
	}
}

// Register adds or updates a service instance in the registry.
func (r *ServiceRegistry) Register(serviceName, instanceID, ip string, port int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create the per-service instance bucket if it does not already exist.
	if _, found := r.services[serviceName]; !found {
		r.services[serviceName] = make(map[string]ServiceInstance)
	}

	r.services[serviceName][instanceID] = ServiceInstance{
		ServiceName: serviceName,
		InstanceID:  instanceID,
		IP:          ip,
		Port:        port,
	}
}

// LookupAll returns every registered backend instance for one logical service.
func (r *ServiceRegistry) LookupAll(serviceName string) []ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, found := r.services[serviceName]
	if !found {
		return nil
	}

	result := make([]ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		result = append(result, instance)
	}

	return result
}

// LookupOne is kept as a convenience/backward-compatible helper for callers that only want one available instance.
func (r *ServiceRegistry) LookupOne(serviceName string) (ServiceInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, found := r.services[serviceName]
	if !found {
		return ServiceInstance{}, false
	}

	for _, instance := range instances {
		return instance, true
	}

	return ServiceInstance{}, false
}

// Delete removes an instance from a service by name.
func (r *ServiceRegistry) Delete(serviceName, instanceID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, found := r.services[serviceName]
	if !found {
		return false
	}

	if _, found := instances[instanceID]; !found {
		return false
	}

	delete(instances, instanceID)

	// Remove the empty service bucket once its last instance is gone.
	if len(instances) == 0 {
		delete(r.services, serviceName)
	}

	return true
}
