package manager

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.41"), // explicitly use Docker API v1.41 (previous compatibility issues)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	log.Printf("[Docker] Client version: %s", cli.ClientVersion())

	reg := registry.NewClient("http://localhost:8000")

	return &ContainerManager{
		cli:      cli,
		registry: reg,
	}, nil
}

// StartContainer starts a container with the specified image name
func (m *ContainerManager) StartContainer(serviceName, imageName, containerName string) (string, error) {
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

	// Expose container port 80 internally
	exposedPorts := nat.PortSet{
		"80/tcp": struct{}{},
	}

	// Use dynamic host port: let Docker bind an available port
	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"80/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0", // Bind on all interfaces
					HostPort: "",        // Let Docker pick a random available host port
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

	/*
	Inspect container to extract the actual host-mapped port
	*/

	// Retry ContainerInspect to give Docker time to assign ports
	var inspection types.ContainerJSON
	for i := 0; i < 5; i++ {
		inspection, err = m.cli.ContainerInspect(ctx, resp.ID)
		if err != nil {
			log.Printf("[StartContainer] Inspect attempt %d failed: %v", i+1, err)
			time.Sleep(300 * time.Millisecond)
			continue
		}
		// If port mapping is available, break early
		if len(inspection.NetworkSettings.Ports["80/tcp"]) > 0 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Docker maps "80/tcp" -> list of host port bindings
	hostPort := ""
	bindings := inspection.NetworkSettings.Ports["80/tcp"]
	if len(bindings) > 0 {
		hostPort = bindings[0].HostPort
	}

	if hostPort == "" {
		log.Printf("[Registry] No host port found for container %s", resp.ID[:12])
		return resp.ID, nil // stop it to avoid the empty string flowing into strconv.Atoi()
	}

	port, err := strconv.Atoi(hostPort)
	if err != nil {
		log.Printf("[Registry] Invalid port for container %s: %v", resp.ID[:12], err)
		return resp.ID, nil
	}

	ip := "127.0.0.1" // All ports are on localhost

	// To generate dynamic health check URL
	healthURL := fmt.Sprintf("http://%s:%d", ip, port)

	// Register using host-assigned port
	if err := m.registry.Register(serviceName, resp.ID, ip, port); err != nil {
		log.Printf("[Registry] Failed to register service %q instance %s: %v", serviceName, resp.ID[:12], err)
	} else {
		log.Printf("[Registry] Registered service %q instance %s at %s:%d", serviceName, resp.ID[:12], ip, port)
	}

	// To health-check each container
	m.StartHealthMonitor(resp.ID, serviceName, containerName, imageName, healthURL)

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

	// Resolve full container metadata so deregistration can target the exact service + instance.
	inspection, inspectErr := m.cli.ContainerInspect(ctx, containerID)
	if inspectErr != nil {
		log.Printf("[Registry] Could not inspect container %s before stop: %v", containerID[:12], inspectErr)
	}

	containerName := ""
	if inspectErr == nil {
		containerName = strings.TrimPrefix(inspection.Name, "/")
	}

	// Derive the logical service name from the managed naming convention.
	serviceName := managedServiceNameFromContainerName(containerName)

	timeout := 10 // Timeout in seconds before force-killing the container
	timeoutDuration := time.Duration(timeout) * time.Second

	// Using an older version of ContainerStop to be compatible with Docker API v1.41
	// In Docker SDK <= v20.10.x, ContainerStop uses a *duration instead of StopOptions
	err := m.cli.ContainerStop(ctx, containerID, &timeoutDuration)
	if err != nil {
		// If a container is already stopped, continue to removal
		if !strings.Contains(err.Error(), "is not running") {
			return fmt.Errorf("failed to stop container: %w", err)
		}
		log.Printf("[Docker] Container %s was already stopped; continuing with cleanup", containerID[:12])
	}

	// Deregister before removal when having a resolved name
	if inspectErr == nil && serviceName != "" {
		if err := m.registry.Deregister(serviceName, inspection.ID); err != nil {
			log.Printf("[Registry] Failed to deregister service %q instance %s: %v", serviceName, inspection.ID[:12], err)
		} else {
			log.Printf("[Registry] Deregistered service %q instance %s", serviceName, inspection.ID[:12])
		}
	}

	// Remove the container
	if err := m.cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
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

// ListRunningContainers exists for explicit live-runtime listing for reconciliation and operational APIs.
func (m *ContainerManager) ListRunningContainers() ([]types.Container, error) {
	ctx := context.Background()

	containers, err := m.cli.ContainerList(ctx, types.ContainerListOptions{All: false})
	if err != nil {
		return nil, fmt.Errorf("failed to list running containers: %w", err)
	}

	return containers, nil
}


// StartHealthMonitor starts a background goroutine that checks the health of a container's HTTP endpoint
// every 30 seconds. If the check fails, the container is restarted.
func (m *ContainerManager) StartHealthMonitor(containerID, serviceName, containerName, imageName, healthURL string) {
	go func() {
		for {
			time.Sleep(30 * time.Second)

			log.Printf("[HealthCheck] Checking %s...", healthURL)

			resp, err := http.Get(healthURL)

			// Handle failures first: prevent panic by checking if resp is nil
			if err != nil {
				log.Printf("[HealthCheck] FAILED (unreachable, err: %v)", err)
			} else if resp.StatusCode >= 400 {
				log.Printf("[HealthCheck] FAILED (bad status: %d)", resp.StatusCode)
			} else {
				log.Printf("[HealthCheck] OK (status: %d)", resp.StatusCode)
			}

			// If health check failed, restart container
			if err != nil || (resp != nil && resp.StatusCode >= 400) {
				// Before trying to stop or restart, confirm the container still exists
				ctx := context.Background()
				_, err := m.cli.ContainerInspect(ctx, containerID)
				if err != nil {
					log.Printf("[HealthCheck] Skipping - container %s no longer exits", containerID[:12])
					continue
				}

				m.mu.Lock()

				if stopErr := m.StopContainer(containerID); stopErr != nil {
					log.Printf("[HealthCheck] Failed to stop container: %v", stopErr)
				}

				newID, startErr := m.StartContainer(serviceName, imageName, containerName)
				if startErr != nil {
					log.Printf("[HealthCheck] Failed to restart container: %v", startErr)
				} else {
					containerID = newID
					log.Printf("[HealthCheck] Container restarted successfully. New ID: %s", newID[:12])
				}

				m.mu.Unlock()
			}

			// Only close response if it was returned
			if resp != nil {
				resp.Body.Close()
			}
		}
	}()
}

// FetchLogs retrieves the logs for the specified container ID.
func (m *ContainerManager) FetchLogs(containerID string) (string, error) {
	ctx := context.Background()

	// Request container logs (both stdout and stderr, no timestamps)
	reader, err := m.cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Tail:       "100", // limit to last 100 lines
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch logs: %w", err)
	}
	defer reader.Close()

	// Prepare output buffers for stdout and stderr
	var stdoutBuf, stderrBuf io.Writer
	out := new(strings.Builder)
	errOut := new(strings.Builder)

	stdoutBuf = out
	stderrBuf = errOut

	// Copy and demultiplex using Docker's stdcopy helper
	if _, err := stdcopy.StdCopy(stdoutBuf, stderrBuf, reader); err != nil {
		return "", fmt.Errorf("failed to decode container logs: %w", err)
	}

	// Combine both stdout and stderr cleanly
	return out.String() + errOut.String(), nil
}

/*
managedServiceNameFromContainerName extracts the logical service name from the managed naming convention "<service-name>-<random-suffix>".
*/
func managedServiceNameFromContainerName(containerName string) string {
	if containerName == "" {
		return ""
	}

	idx := strings.LastIndex(containerName, "-")
	if idx == -1 {
		return containerName
	}

	return containerName[:idx]
}
