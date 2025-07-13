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
)

// Manages Docker containers via the Docker API
type ContainerManager struct {
	cli *client.Client
	mu sync.Mutex // Protects container restart logic to avoid race conditions
}

// Creates a new Docker client using environment configuration
func NewContainerManager() (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &ContainerManager{cli: cli}, nil
}

// Starts a container with the specified image name
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
					HostIP: "127.0.0.1", // Limit binding to localhost only
					HostPort: "80",     // Expose as http://localhost:80
				},
			},
		},
	}

	// Create container with networking setup
	resp, err := m.cli.ContainerCreate(ctx, &container.Config{
			Image: imageName,
			ExposedPorts: exposedPorts, // So Docker knows port 80 is in use
		}, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := m.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container %w", err)
	}

	return resp.ID, nil
}

// Gracefully stops a running container by ID or name
func (m *ContainerManager) StopContainer(containerID string) error {
	ctx := context.Background()

	timeout := 10 // Timeout in seconds before force-killing the container
	err := m.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

// Lists all containers
func (m *ContainerManager) ListContainers() ([]types.Container, error) {
	ctx := context.Background()
	containers, err := m.cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return containers, nil
}

// Starts a background goroutine that checks the health of a container's HTTP endpoint
// every 30 seconds. If the check fails, the container is restarted

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
