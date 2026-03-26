package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"orchestrator/internal/registry"
)

// LoadBalancer holds round-robin state per logical service.
type LoadBalancer struct {
	registry *registry.Client
	mu       sync.Mutex
	next     map[string]int
}

// NewLoadBalancer creates a new load balancer instance.
func NewLoadBalancer(registryBaseURL string) *LoadBalancer {
	return &LoadBalancer{
		registry: registry.NewClient(registryBaseURL),
		next:     make(map[string]int),
	}
}

/*
selectBackend fetches all known backends for the service and returns the next one according to round-robin ordering.
*/
func (lb *LoadBalancer) selectBackend(serviceName string) (*registry.ServiceInstance, error) {
	instances, err := lb.registry.LookupAll(serviceName)
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, nil
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	index := lb.next[serviceName] % len(instances)
	lb.next[serviceName] = (lb.next[serviceName] + 1) % len(instances)

	instance := instances[index]
	return &instance, nil
}

func main() {
	lb := NewLoadBalancer("http://localhost:8000")

	r := gin.Default()

	r.Any("/proxy/:serviceName", func(c *gin.Context) {
		serviceName := c.Param("serviceName")

		instance, err := lb.selectBackend(serviceName)
		if err != nil {
			log.Printf("[LoadBalancer] Failed to lookup service %q: %v", serviceName, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to resolve backend"})
			return
		}

		if instance == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}

		targetURL := "http://" + instance.IP + ":" + strconv.Itoa(instance.Port)
		target, err := url.Parse(targetURL)
		if err != nil {
			log.Printf("[LoadBalancer] Invalid target URL %q for service %q: %v", targetURL, serviceName, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "invalid backend target"})
			return
		}

		log.Printf(
			"[LoadBalancer] Routing service %q to instance %s at %s",
			serviceName,
			instance.InstanceID[:12],
			targetURL,
		)

		proxy := httputil.NewSingleHostReverseProxy(target)

		// Ensure reverse-proxy failures become clear HTTP 502 responses.
		proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
			log.Printf("[LoadBalancer] Proxy error for service %q via instance %s: %v", serviceName, instance.InstanceID[:12], proxyErr)
			rw.WriteHeader(http.StatusBadGateway)
			_, _ = rw.Write([]byte(`{"error":"backend request failed"}`))
		}

		proxy.ServeHTTP(c.Writer, c.Request)
	})

	// Start the load balancer on port 9000
	log.Println("Load balancer listening on :9000")
	if err := r.Run(":9000"); err != nil {
		log.Fatalf("failed to start load balancer: %v", err)
	}
}
