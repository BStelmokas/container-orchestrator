package main

import (
	"log"
	"net/http"
	"sync"

	"orchestrator/internal/registry"

	"github.com/gin-gonic/gin"
)

// RegistryStore models one logical service with many concrete instances.
type RegistryStore struct {
	mu sync.RWMutex
	services map[string]map[string]registry.ServiceInstance
}

// NewRegistryStore creates an empty in-memory registry.
func NewRegistryStore() *RegistryStore {
	return &RegistryStore{
		services: make(map[string]map[string]registry.ServiceInstance),
	}
}

// Register stores one concrete instance under a logical service name.
func (r *RegistryStore) Register(serviceName, instanceID, ip string, port int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, found := r.services[serviceName]; !found {
		r.services[serviceName] = make(map[string]registry.ServiceInstance)
	}

	r.services[serviceName][instanceID] = registry.ServiceInstance{
		ServiceName: serviceName,
		InstanceID: instanceID,
		IP: ip,
		Port: port,
	}
}

// Deregister removes one exact instance from one logical service.
func (r *RegistryStore) Deregister(serviceName, instanceID string) bool {
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

	// Delete empty service buckets to keep the registry tidy.
	if len(instances) == 0 {
		delete(r.services, serviceName)
	}

	return true
}

// LookupAll returns all concrete backends for one logical service.
func (r *RegistryStore) LookupAll(serviceName string) []registry.ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, found := r.services[serviceName]
	if !found {
		return nil
	}

	result := make([]registry.ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		result = append(result, instance)
	}

	return result
}

// LookupOne keeps backward compatibility for callers that still expect a single instance.
func (r *RegistryStore) LookupOne(serviceName string) (registry.ServiceInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instances, found := r.services[serviceName]
	if !found {
		return registry.ServiceInstance{}, false
	}

	for _, instance := range instances {
		return instance, true
	}

	return registry.ServiceInstance{}, false
}

func main() {
	store := NewRegistryStore()

	r := gin.Default()

	// POST /register
	r.POST("/register", func(c *gin.Context) {
		var req struct {
			ServiceName string `json:"serviceName"`
			InstanceID string `json:"instanceID"`
			IP string `json:"ip"`
			Port int `json:"port"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		// Validate the full service-instance identity.
		if req.ServiceName == "" || req.InstanceID == "" || req.IP == "" || req.Port <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName, instanceID, ip, and positive port are required"})
			return
		}

		store.Register(req.ServiceName, req.InstanceID, req.IP, req.Port)
		log.Printf("[Registry] Registered as %s/%s -> %s:%d", req.ServiceName, req.InstanceID, req.IP, req.Port)

		c.JSON(http.StatusOK, gin.H{"status": "registered"})
	})

	// DELETE /deregister
	r.DELETE("/deregister", func(c *gin.Context) {
		var req struct {
			ServiceName string `json:"serviceName"`
			InstanceID string `json:"instanceID"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		// Deregistration is now specific to one instance.
		if req.ServiceName == "" || req.InstanceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "serviceName and instanceID are required"})
			return
		}

		if !store.Deregister(req.ServiceName, req.InstanceID) {
			c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
			return
		}

		log.Printf("[Registry] Deregistered %s/%s", req.ServiceName, req.InstanceID)
		c.JSON(http.StatusOK, gin.H{"status": "deregistered"})
	})

	// GET /lookup?name=service-name
	r.GET("/lookup", func(c *gin.Context) {
		name := c.Query("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing service name"})
			return
		}

		// Preserve backward compatibility by returning one instance.
		instance, found := store.LookupOne(name)
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}

		c.JSON(http.StatusOK, instance)
	})

	// GET /lookup-all?name=service-name
	r.GET("/lookup-all", func(c *gin.Context) {
		name := c.Query("name")
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing service name"})
			return
		}

		instances := store.LookupAll(name)
		if len(instances) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}

		// Return the full backend set for a logical service.
		c.JSON(http.StatusOK, instances)
	})

	log.Println("Service registry running on :8000")
	if err := r.Run(":8000"); err != nil {
		log.Fatalf("failed to start registry: %v", err)
	}
}
