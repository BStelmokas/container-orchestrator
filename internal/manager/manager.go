package manager

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"orchestrator/internal/registry"
)

// ContainerManager manages Docker containers via the Docker API.
type ContainerManager struct {
	cli      *client.Client
	mu       sync.Mutex // Protects container restart logic to avoid race conditions
	registry *registry.Client
}

// NewContainerManager creates a new Docker client using environment configuration.
func NewContainerManager() (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	reg := registry.NewClient("http://localhost:8000")

	return &ContainerManager{
		cli:      cli,
		registry: reg,
	}, nil
}

// StartContainer starts a container with the specified image name.
func (m *ContainerManager) StartContainer(imageName, containerName string) (string, error) {
	ctx := context.Background()

	// Pull image from Docker Hub if not available
	reader, err := m.cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader) // drain the response body to complete the pull

	// Remove any existing container with the same name if exists
	_ = m.cli.ContainerRemove(ctx, containerName, types.ContainerRemoveOptions{
		Force: true, // Kill if running
	})

	// Port binding setup: expose container port 80 to host port 80 on localhost

	// Define which ports the container exposes internally
	exposedPorts := nat.PortSet{
		"80/tcp": struct{}{},
	}
	// Define how container ports map to host machine (localhost)
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"80/tcp": []nat.PortBinding{
				{
					HostIP:   "127.0.0.1", // Limit binding to localhost only
					HostPort: "80",        // Expose as http://localhost:80
				},
			},
		},
	}

	// Create container with networking setup
	resp, err := m.cli.ContainerCreate(ctx, &container.Config{
		Image:        imageName,
		ExposedPorts: exposedPorts, // So Docker knows port 80 is in use
	}, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := m.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container %w", err)
	}

	// After starting the container, register it with the service registry
	ip := "127.0.0.1" // Binding to localhost
	port := 80

	if err := m.registry.Register(containerName, ip, port); err != nil {
		log.Printf("[Registry] Failed to register service %q: %v", containerName, err)
	} else {
		log.Printf("[Registry] Registered service %q at %s:%d", containerName, ip, port)
	}

	return resp.ID, nil
}

// GetContainerName looks up the container's name using its ID via Docker Inspect.
// It returns the name without leading "/" (e.g., "my-nginx" instead of "/my-nginx").
func (m *ContainerManager) GetContainerName(containerID string) (string, error) {
	ctx := context.Background()

	// Use Docker API to inspect the container by ID
	containerJSON, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	// Docker names are returned like "/my-nginx" -> remove leading "/"
	name := containerJSON.Name
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}

	return name, nil
}

// StopContainer gracefully stops a running container by ID or name.
func (m *ContainerManager) StopContainer(containerID string) error {
	ctx := context.Background()

	timeout := 10 // Timeout in seconds before force-killing the container
	err := m.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Resolve container name from its ID
	name, err := m.GetContainerName(containerID)
	if err != nil {
		log.Printf("[Registry] Could not resolve name for container ID %s: %v", containerID[:12], err)
		return nil // Still return nil so StopContainer isn't considered a failure
	}

	// Deregister the service from the registry
	if err := m.registry.Deregister(name); err != nil {
		log.Printf("[Registry] Failed to deregister service %q: %v", name, err)
	} else {
		log.Printf("[Registry] Deregistered service %q", name)
	}

	return nil
}

// ListContainers lists all containers.
func (m *ContainerManager) ListContainers() ([]types.Container, error) {
	ctx := context.Background()
	containers, err := m.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return containers, nil
}

// StartHealthMonitor starts a background goroutine that checks the health of a container's HTTP endpoint
// every 30 seconds. If the check fails, the container is restarted.
func (m *ContainerManager) StartHealthMonitor(containerID, containerName, imageName, healthURL string) {
	go func() {
		for {
			time.Sleep(30 * time.Second)

			log.Printf("[HealthCheck] Checking %s...", healthURL)

			resp, err := http.Get(healthURL)
			if err != nil || resp.StatusCode >= 400 {
				log.Printf("[HealthCheck] FAILED (status: %v, err: %v)", resp.StatusCode, err)

				m.mu.Lock()

				if stopErr := m.StopContainer(containerID); stopErr != nil {
					log.Printf("[HealthCheck] Failed to stop container: %v", stopErr)
				}

				newID, startErr := m.StartContainer(imageName, containerName)
				if startErr != nil {
					log.Printf("[HealthCheck] Failed to restart container: %v", startErr)
				} else {
					containerID = newID
					log.Printf("[HealthCheck] Container restarted successfully. New ID: %s", newID[:12])
				}

				m.mu.Unlock()
			} else {
				log.Printf("[HealthCheck] OK (status: %d)", resp.StatusCode)
			}

			if resp != nil {
				resp.Body.Close()
			}
		}
	}()
}
