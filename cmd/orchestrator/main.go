package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"orchestrator/internal/domain"
	"orchestrator/internal/events"
	"orchestrator/internal/health"
	"orchestrator/internal/manager"
	"orchestrator/internal/orchestrator"
	"orchestrator/internal/state"
	"orchestrator/internal/status"

	"github.com/gin-gonic/gin"
)

// Persist desired services in a stable local file.
const desiredStateFilePath = "data/services.json"

// scaleServiceRequest is the canonical request payload for service scaling.
type scaleServiceRequest struct {
	Replicas int `json:"replicas"`
}

func main() {
	// Initialize the Docker-based container manager.
	manager, err := manager.NewContainerManager()
	if err != nil {
		log.Fatalf("Error creating container manager: %v", err)
	}

	// Create a dedicated desired-state store.
	serviceStore := state.NewServiceStore()

	// Restore desired state before the controller starts.
	if err := serviceStore.LoadFromFile(desiredStateFilePath); err != nil {
		log.Fatalf("failed to load desired service state: %v", err)
	}

	// Create the centralized health tracker.
	healthTracker := health.NewTracker()

	// Create a centralized event recorder for recent control-plane activity.
	eventRecorder := events.NewRecorder(200)

	// Inject the health tracker into the runtime layer.
	manager.SetHealthTracker(healthTracker)
	manager.SetEventRecorder(eventRecorder)

	// Initialize and start deployment controller (manages desired replica state).
	controller := orchestrator.NewDeploymentController(
		manager,
		serviceStore,
		healthTracker,
		eventRecorder,
		10*time.Second,
	)
	controller.Start()

	// Build a centralized status layer for the API/dashboard.
	statusBuilder := status.NewBuilder(serviceStore, manager, healthTracker)

	r := gin.Default()

	// Serve Static HTML Dashboard.
	r.StaticFile("/", "./dashboard/static/dashboard.html")

	// GET /api/services
	r.GET("/api/services", func(c *gin.Context) {
		services, err := statusBuilder.ListServices()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build service status"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"services": services})
	})

	// GET /api/services/:name
	r.GET("/api/services/:name", func(c *gin.Context) {
		name := c.Param("name")

		serviceStatus, found, err := statusBuilder.GetService(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build service status"})
			return
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown service: " + name})
			return
		}

		c.JSON(http.StatusOK, serviceStatus)
	})

	// GET /api/events
	r.GET("/api/events", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"events": eventRecorder.List(50),
		})
	})

	// POST /api/services
	r.POST("/api/services", func(c *gin.Context) {
		var req domain.ServiceSpec
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
			return
		}

		if err := serviceStore.Upsert(req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Persist desired state after mutation.
		if err := serviceStore.SaveToFile(desiredStateFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist desired state"})
			return
		}

		// Record desired-state writes as explicit API events.
		eventRecorder.Add(
			"api",
			fmt.Sprintf("stored service=%s image=%s replicas=%d", req.Name, req.Image, req.Replicas),
		)

		controller.ReconcileNow() // Trigger immediate convergence after desired-state mutation

		c.JSON(http.StatusOK, gin.H{
			"status":  "service stored",
			"service": req.Name,
		})
	})

	// POST /api/services/:name/scale
	r.POST("/api/services/:name/scale", func(c *gin.Context) {
		name := c.Param("name")

		var req scaleServiceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
			return
		}

		if req.Replicas < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "replicas must be at least 1"})
			return
		}

		updatedSpec, found, err := serviceStore.Scale(name, req.Replicas)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown service: " + name})
			return
		}

		if err := serviceStore.SaveToFile(desiredStateFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist desired state"})
			return
		}

		eventRecorder.Add(
			"api",
			fmt.Sprintf("scaled service=%s replicas=%d", name, updatedSpec.Replicas),
		)

		controller.ReconcileNow()

		c.JSON(http.StatusOK, gin.H{
			"service":  name,
			"replicas": updatedSpec.Replicas,
			"status":   "scaled successfully",
		})
	})

	// POST /api/services/:name/restart
	r.POST("/api/services/:name/restart", func(c *gin.Context) {
		name := c.Param("name")

		updatedSpec, found, err := serviceStore.RequestRestart(name)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown service: " + name})
			return
		}

		if err := serviceStore.SaveToFile(desiredStateFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist desired state"})
			return
		}

		eventRecorder.Add(
			"api",
			fmt.Sprintf("requested rolling restart for service=%s generation=%d", name, updatedSpec.Generation),
		)

		controller.ReconcileNow()

		c.JSON(http.StatusOK, gin.H{
			"service":    name,
			"generation": updatedSpec.Generation,
			"status":     "rolling restart requested",
		})
	})

	// DELETE /api/services/:name
	r.DELETE("/api/services/:name", func(c *gin.Context) {
		name := c.Param("name")

		if deleted := serviceStore.Delete(name); !deleted {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown service " + name})
			return
		}

		// Persist the delete before reconciliation so a process restart does not resurrect the service.
		if err := serviceStore.SaveToFile(desiredStateFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist desired state"})
			return
		}

		eventRecorder.Add("api", fmt.Sprintf("deleted service=%s", name))

		containers, err := manager.ListRunningContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list running containers"})
			return
		}

		for _, cont := range containers {
			if len(cont.Names) == 0 {
				continue
			}

			if matchesServiceName(cont.Names[0], name) {
				log.Printf("[Delete] Stopping container: %s", cont.ID[:12])
				if err := manager.StopContainer(cont.ID); err != nil {
					log.Printf("[Delete] Failed stopping container %s: %v", cont.ID[:12], err)
				}
			}
		}

		controller.ReconcileNow() // Ensure control plane converges immediately after delete.

		c.JSON(http.StatusOK, gin.H{
			"service": name,
			"status":  "deleted",
		})
	})

	// GET /api/services/:name/logs
	r.GET("/api/services/:name/logs", func(c *gin.Context) {
		name := c.Param("name")

		if _, found := serviceStore.Get(name); !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown service: " + name})
			return
		}

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
