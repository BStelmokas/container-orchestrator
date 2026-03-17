package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"orchestrator/internal/manager"
	"orchestrator/internal/orchestrator"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize the Docker-based container manager.
	manager, err := manager.NewContainerManager()
	if err != nil {
		log.Fatalf("Error creating container manager: %v", err)
	}

	// Initialize and start deployment controller (manages desired replica state).
	controller := orchestrator.NewDeploymentController(manager, 10)
	controller.Start()

	r := gin.Default()

	// Serve Static HTML Dashboard.
	r.StaticFile("/", "./dashboard/static/dashboard.html")

	// /api/services
	r.GET("/api/services", func(c *gin.Context) {
		containers, err := manager.ListRunningContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list running containers"})
			return
		}

		specs := controller.ListSpecs()

		// Group containers.
		serviceMap := make(map[string][]string)
		for _, spec := range specs {
			for _, cont := range containers {
				if len(cont.Names) == 0 {
					continue
				}

				if matchesServiceName(cont.Names[0], spec.Name) {
					label := fmt.Sprintf("%s (%s)", cont.ID[:12], cont.Status)
					serviceMap[spec.Name] = append(serviceMap[spec.Name], label)
				}
			}
		}

		// Return services in sorted order for a stable UI experience.
		names := make([]string, 0, len(serviceMap))
		for name := range serviceMap {
			names = append(names, name)
		}
		sort.Strings(names)

		var result []gin.H
		for _, name := range names {
			result = append(result, gin.H{
				"name": name,
				"containers": serviceMap[name],
			})
		}

		c.JSON(http.StatusOK, gin.H{"services": result})
	})


	// /api/scale/:name/:delta
	r.POST("/api/scale/:name/:delta", func(c *gin.Context) {
		name := c.Param("name")
		deltaStr := c.Param("delta")

		// Convert delta from string to integer
		delta, err := strconv.Atoi(deltaStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scale delta"})
			return
		}

		spec, found := controller.GetSpec(name)
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown service: " + name})
			return
		}

		// Compute new desired replica count.
		newReplicas := spec.Replicas + delta
		if newReplicas < 1 {
			newReplicas = 1 // Always keep at least one replica running
		}

		// Re-deploy service with updated replica count
		spec.Replicas = newReplicas
		controller.Deploy(spec)

		c.JSON(http.StatusOK, gin.H{
			"service":  name,
			"replicas": newReplicas,
			"status":   "scaled successfully",
		})
	})

	// /api/restart/:name
	r.POST("/api/restart/:name", func(c *gin.Context) {
		name := c.Param("name")

		// Get list of all containers
		spec, found := controller.GetSpec(name)
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown service: " + name})
			return
		}

		containers, err := manager.ListRunningContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list running containers"})
			return
		}

		stoppedAny := false
		for _, cont := range containers {
			if len(cont.Names) == 0 {
				continue
			}

			if matchesServiceName(cont.Names[0], name) {
				log.Printf("[Restart] Stopping container: %s", cont.ID[:12])
				if err := manager.StopContainer(cont.ID); err != nil {
					log.Printf("[Restart] Failed stopping container %s: %v", cont.ID[:12], err)
					continue
				}
				stoppedAny = true
			}
		}

		if !stoppedAny {
			c.JSON(http.StatusNotFound, gin.H{"error": "no running containers found for service " + name})
			return
		}

		// Re-deploy the service with the same number of replicas
		controller.Deploy(spec)

		c.JSON(http.StatusOK, gin.H{
			"service":  name,
			"replicas": spec.Replicas,
			"status":   "restart triggered",
		})
	})

  /*
	The REST API
	*/

	// POST /deploy
	r.POST("/deploy", func(c *gin.Context) {
		var req orchestrator.ServiceSpec
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
			return
		}
		controller.Deploy(req)
		c.JSON(http.StatusOK, gin.H{"status": "deployment started", "service": req.Name})
	})

	// POST /scale/:name/:replicas
	r.POST("/scale/:name/:replicas", func(c *gin.Context) {
		name := c.Param("name")
		replicas := c.Param("replicas")

		n, err := strconv.Atoi(replicas)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid replica count"})
			return
		}

		spec, found := controller.GetSpec(name)
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown service: " + name})
			return
		}

		spec.Replicas = n
		controller.Deploy(spec)

		c.JSON(http.StatusOK, gin.H{"status": "scaled", "replicas": n})
	})

	// GET /status/:name
	r.GET("/status/:name", func(c *gin.Context) {
		name := c.Param("name")

		containers, err := manager.ListRunningContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list running containers"})
			return
		}

		var result []string
		for _, cont := range containers {
			if len(cont.Names) == 0 {
				continue
			}

			if matchesServiceName(cont.Names[0], name) {
				result = append(result, cont.ID[:12]+" ("+cont.Status+")")
			}
		}

		c.JSON(http.StatusOK, gin.H{"containers": result})
	})

	// GET /logs/:name
	r.GET("/logs/:name", func(c *gin.Context) {
		name := c.Param("name")

		containers, err := manager.ListRunningContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list running containers"})
			return
		}

		logs := make(map[string]string)
		for _, cont := range containers {
			if len(cont.Names) == 0 {
				continue
			}

			if matchesServiceName(cont.Names[0], name) {
				out, err := manager.FetchLogs(cont.ID)
				filtered := filterNginxAccessLogs(out)
				if err == nil && strings.TrimSpace(filtered) != "" {
					logs[cont.ID[:12]] = filtered
				}
			}
		}

		c.JSON(http.StatusOK, logs)
	})

	// Start API server on port 8080.
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("failed to start HTTP server: %v", err)
	}
}

// matchesServiceName is a helper for explicit service matching.
func matchesServiceName(containerDockerName, serviceName string) bool {
	trimmed := strings.TrimPrefix(containerDockerName, "/")
	return trimmed == serviceName || strings.HasPrefix(trimmed, serviceName+"-")
}


// filterNginxAccessLogs makes the logs less verbose.
func filterNginxAccessLogs(raw string) string {
	lines := strings.Split(raw, "\n")
	var out []string

	for _, line := range lines {
		// Keep only real HTTP access logs that are NOT internal orchestrator health checks
		if strings.Contains(line, "HTTP/1.1") && !strings.Contains(line, "Go-http-client/1.1") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
