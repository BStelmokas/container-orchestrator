package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"orchestrator/internal/manager"
	"orchestrator/internal/orchestrator"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize the Docker-based container manager
	manager, err := manager.NewContainerManager()
	if err != nil {
		log.Fatalf("Error creating container manager: %v", err)
	}

	// Start a container for base testing/demo
	image := "nginx:latest"
	name := "my-nginx"
	fmt.Println("Starting container...")
	containerID, err := manager.StartContainer(image, name)
	if err != nil {
		log.Fatalf("Start error: %v", err)
	}
	fmt.Printf("Started container with ID: %s\n", containerID[:12])

	// Start monitoring health of that container manually
	healthURL := "http://localhost:80"
	manager.StartHealthMonitor(containerID, name, image, healthURL)

	// List containers to verify running state
	fmt.Println("Listing containers...")
	containers, err := manager.ListContainers()
	if err != nil {
		log.Fatalf("List error: %v", err)
	}
	for _, c := range containers {
		fmt.Printf("Container ID: %s, Image: %s, Status: %s\n", c.ID[:12], c.Image, c.Status)
	}

	//////////////////////////////////////////////////////////////
	// Initialize and start deployment controller (manages desired replica state)
	controller := orchestrator.NewDeploymentController(manager, 10*time.Second)
	controller.Start()

	// Define desired state for a service and deploy it
	spec := orchestrator.ServiceSpec{
		Name:     "nginx-service",
		Image:    "nginx:latest",
		Replicas: 3, // run 3 copies of this server at all times
	}
	controller.Deploy(spec)

	////////////////////////////////////////////////////////////////////////
	// Auto-scaling controller: monitors CPU and scales up/down accordingly
	go func() {
		for {
			time.Sleep(15 * time.Second) // Check every 15 seconds

			// List all containers again
			containers, err := manager.ListContainers()
			if err != nil {
				log.Printf("[AutoScale] Failed to list containers: %v", err)
				continue
			}

			// Filter containers that match our deployed service name
			var matching []types.Container
			for _, c := range containers {
				// Docker container names usually start with "/" - e.g. "/nginx-service-1"
				if len(c.Names) > 0 && strings.HasPrefix(c.Names[0], "/"+spec.Name) {
					matching = append(matching, c)
				}
			}

			// Nothing to scale if none found
			if len(matching) == 0 {
				log.Printf("[AutoScale] No containers found for %s", spec.Name)
				continue
			}

			// Calculate average CPU usage across replicas
			avgCPU, err := getAverageCPUUsage(matching)
			if err != nil {
				log.Printf("[AutoScale] Error calculating CPU usage: %v", err)
				continue
			}

			log.Printf("[AutoScale] Service=%s, Replicas=%d, AvgCPU=%.2f%%", spec.Name, len(matching), avgCPU)

			actualReplicas := len(matching)

			if avgCPU > 80.0 {
				// Scale up: add 1 more replica
				newSpec := spec
				newSpec.Replicas = actualReplicas + 1
				log.Printf("[AutoScale] Scaling UP to %d replicas", newSpec.Replicas)
				controller.Deploy(newSpec)
				spec = newSpec // update latest desired state
			} else if avgCPU < 20.0 && actualReplicas > 1 {
				// Scale down: remove 1 replica
				newSpec := spec
				newSpec.Replicas = actualReplicas - 1
				log.Printf("[AutoScale] Scaling DOWN to %d replicas", newSpec.Replicas)
				controller.Deploy(newSpec)
				spec = newSpec
			} else {
				log.Printf("[AutoScale] No scaling action needed")
			}
		}
	}()

	////////////////////////////////////////////////////////////////////////
	// REST API using Gin

	r := gin.Default()

	////////////////////////////////////////////////////////////
	// Serve Static HTML Dashboard

	// dashboard/static/dashboard.html
	r.StaticFile("/", "./dashboard/static/dashboard.html")

	// /api/services
	r.GET("/api/services", func(c *gin.Context) {
		containers, err := manager.ListContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list containers"})
			return
		}

		// Group containers by service prefix (e.g., "nginx-service")
		serviceMap := make(map[string][]string)

		for _, cont := range containers {
			if len(cont.Names) == 0 {
				continue
			}

			name := strings.TrimPrefix(cont.Names[0], "/")
			parts := strings.Split(name, "-")
			if len(parts) == 0 {
				continue
			}
			service := parts[0] // Extract logical service name
			label := fmt.Sprintf("%s (%s)", cont.ID[:12], cont.Status)
			serviceMap[service] = append(serviceMap[service], label)
		}

		// Build JSON response
		var result []gin.H
		for name, list := range serviceMap {
			result = append(result, gin.H{
				"name":       name,
				"containers": list,
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

		// List containers to count current replicas of the given service
		containers, err := manager.ListContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list containers"})
			return
		}

		currentReplicas := 0
		for _, cont := range containers {
			if strings.HasPrefix(cont.Names[0], "/"+name) {
				currentReplicas++
			}
		}

		// Compute new desired replica count
		newReplicas := currentReplicas + delta
		if newReplicas < 1 {
			newReplicas = 1 // Always keep at least one replica running
		}

		// Re-deploy service with updated replica count
		spec := orchestrator.ServiceSpec{
			Name:     name,
			Image:    "nginx:latest", // TODO: Extract from running containers or config
			Replicas: newReplicas,
		}
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
		containers, err := manager.ListContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list containers"})
			return
		}

		// Stop matching containers and count them
		replicaCount := 0
		for _, cont := range containers {
			if strings.HasPrefix(cont.Names[0], "/"+name) {
				log.Printf("[Restart] Stopping container: %s", cont.ID[:12])
				_ = manager.StopContainer(cont.ID)
				replicaCount++
			}
		}

		if replicaCount == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "no containers found for service " + name})
			return
		}

		// Re-deploy the service with the same number of replicas
		spec := orchestrator.ServiceSpec{
			Name:     name,
			Image:    "nginx:latest", // NOTE: Could be extracted dynamically in the future
			Replicas: replicaCount,
		}
		controller.Deploy(spec)

		c.JSON(http.StatusOK, gin.H{
			"service":  name,
			"replicas": replicaCount,
			"status":   "restart triggered",
		})
	})

	////////////////////////////////////////////////////////////
	// The REST API

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
		spec := orchestrator.ServiceSpec{
			Name:     name,
			Image:    "nginx:latest",
			Replicas: n,
		}
		controller.Deploy(spec)
		c.JSON(http.StatusOK, gin.H{"status": "scaled", "replicas": n})
	})

	// GET /status/:name
	r.GET("/status/:name", func(c *gin.Context) {
		name := c.Param("name")
		containers, err := manager.ListContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list containers"})
			return
		}
		var result []string
		for _, cont := range containers {
			if strings.HasPrefix(cont.Names[0], "/"+name) {
				result = append(result, cont.ID[:12]+" ("+cont.Status+")")
			}
		}
		c.JSON(http.StatusOK, gin.H{"containers": result})
	})

	// GET /logs/:name
	r.GET("/logs/:name", func(c *gin.Context) {
		name := c.Param("name")
		containers, err := manager.ListContainers()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list containers"})
			return
		}
		var logs map[string]string = make(map[string]string)
		for _, cont := range containers {
			if strings.HasPrefix(cont.Names[0], "/"+name) {
				out, err := manager.FetchLogs(cont.ID)
				// Filters the logs
				filtered := filterNginxAccessLogs(out)
				if err == nil && strings.TrimSpace(filtered) != "" {
					logs[cont.ID[:12]] = filtered
				}
			}
		}
		c.JSON(http.StatusOK, logs)
	})

	r.Run(":8080") // starts API server on port 8080

	// Block forever so the health monitor goroutine doesn't exit + controller keeps running
	select {}
}

// getAverageCPUUsage computes the average CPU usage of all given containers.
func getAverageCPUUsage(containers []types.Container) (float64, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return 0, err
	}
	defer cli.Close()

	var total float64
	var count int

	for _, container := range containers {
		// One-shot stats request
		resp, err := cli.ContainerStats(context.Background(), container.ID, false)
		if err != nil {
			log.Printf("[AutoScale] Failed to get stats for container %s: %v", container.ID[:12], err)
			continue
		}
		defer resp.Body.Close()

		// Decode stats JSON into Docker's native struct
		var stats types.StatsJSON
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			log.Printf("[AutoScale] Failed to decode stats for container %s: %v", container.ID[:12], err)
			continue
		}

		// Extract CPU usage and system CPU usage
		cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(stats.CPUStats.SystemUsage)

		// Count logical CPUs (percpu length)
		perCPU := len(stats.CPUStats.CPUUsage.PercpuUsage)
		if perCPU == 0 {
			perCPU = 1 // fallback for single-core or missing data
		}

		if cpuDelta == 0 || systemDelta == 0 {
			log.Printf("[AutoScale] Invalid CPU stats for container %s, skipping", container.ID[:12])
			continue
		}

		cpuPercent := (cpuDelta / systemDelta) * float64(perCPU) * 100.0

		total += cpuPercent
		count++
	}

	if count == 0 {
		return 0, fmt.Errorf("no valid CPU stats from containers")
	}

	avg := total / float64(count)
	return math.Round(avg*100) / 100, nil
}

// filterNginxAccessLogs makes the logs less verbose
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
