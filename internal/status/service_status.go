package status

import (
	"fmt"
	"strconv"
	"strings"

	"orchestrator/internal/health"
	"orchestrator/internal/manager"
	"orchestrator/internal/state"

	"github.com/docker/docker/api/types"
)

// ReplicaStatus describes one running replica as seen by the status layer.
type ReplicaStatus struct {
	ContainerID   string `json:"containerID"`
	ContainerName string `json:"containerName"`
	RuntimeStatus string `json:"runtimeStatus"`
	HealthStatus  string `json:"healthStatus"`
	LastError     string `json:"lastError,omitempty"`
	Generation    int64  `json:"generation"`
	Outdated    	bool   `json:"outdated"`
}

// ServiceStatus is the centralized status view exposed to the API and dashboard.
type ServiceStatus struct {
	Name              string          `json:"name"`
	Image             string          `json:"image"`
	DesiredReplicas   int             `json:"desiredReplicas"`
	RunningReplicas   int             `json:"runningReplicas"`
	HealthyReplicas   int             `json:"healthyReplicas"`
	UnhealthyReplicas int             `json:"unhealthyReplicas"`
	UnknownReplicas   int             `json:"unknownReplicas"`
	OverallStatus     string          `json:"overallStatus"`
	Containers        []ReplicaStatus `json:"containers"`
	Generation        int64 					`json:"generation"`
	OutdatedReplicas  int							`json:"outdatedReplicas"`
}

// Builder computes service status from desired state + runtime state + health state.
type Builder struct {
	store   *state.ServiceStore
	manager *manager.ContainerManager
	tracker *health.Tracker
}

// NewBuilder constructs a centralized service-status builder.
func NewBuilder(store *state.ServiceStore, manager *manager.ContainerManager, tracker *health.Tracker) *Builder {
	return &Builder{
		store:   store,
		manager: manager,
		tracker: tracker,
	}
}

// ListServices computes status for every desired service.
func (b *Builder) ListServices() ([]ServiceStatus, error) {
	specs := b.store.List()

	containers, err := b.manager.ListRunningContainers()
	if err != nil {
		return nil, err
	}

	result := make([]ServiceStatus, 0, len(specs))
	for _, spec := range specs {
		result = append(result, b.buildServiceStatus(spec.Name, spec.Image, spec.Replicas, spec.Generation, containers))
	}

	return result, nil
}

// GetService computes status for one desired service.
func (b *Builder) GetService(name string) (ServiceStatus, bool, error) {
	spec, found := b.store.Get(name)
	if !found {
		return ServiceStatus{}, false, nil
	}

	containers, err := b.manager.ListRunningContainers()
	if err != nil {
		return ServiceStatus{}, true, err
	}

	return b.buildServiceStatus(spec.Name, spec.Image, spec.Replicas, spec.Generation, containers), true, nil
}

// buildServiceStatus creates the status view for one service from a shared container snapshot.
func (b *Builder) buildServiceStatus(name, image string, desired int, generation int64, containers []types.Container) ServiceStatus {
	status := ServiceStatus{
		Name:            name,
		Image:           image,
		DesiredReplicas: desired,
		Generation: generation,
		Containers:      []ReplicaStatus{},
	}

	for _, cont := range containers {
		if len(cont.Names) == 0 || !matchesServiceName(cont.Names[0], name) {
			continue
		}

		containerName := strings.TrimPrefix(cont.Names[0], "/")

		/*
		Read the replica generation from Docker labels so status can detect whether a rolling restart is still in progress.
		*/
		replicaGeneration := containerGeneration(cont)
		isOutdated := replicaGeneration != generation

		replica := ReplicaStatus{
			ContainerID:   cont.ID[:12],
			ContainerName: containerName,
			RuntimeStatus: cont.Status,
			HealthStatus:  "unknown", // Default until the tracker gives an explicit health result.
			Generation: replicaGeneration,
			Outdated: isOutdated,
		}

		status.RunningReplicas++

		// Track rollout lag separately from health state.
		if isOutdated {
			status.OutdatedReplicas++
		}

		report, found := b.tracker.Get(cont.ID)
		if found && report.HasCheck {
			if report.Healthy {
				replica.HealthStatus = "healthy"
				status.HealthyReplicas++
			} else {
				replica.HealthStatus = "unhealthy"
				replica.LastError = report.LastError
				status.UnhealthyReplicas++
			}
		} else {
			status.UnknownReplicas++
		}

		status.Containers = append(status.Containers, replica)
	}

	// Derive one clear overall status for the service.
	switch {
	case status.OutdatedReplicas > 0:
		status.OverallStatus = "rolling"
	case status.RunningReplicas < status.DesiredReplicas:
		status.OverallStatus = "progressing"
	case status.UnhealthyReplicas > 0:
		status.OverallStatus = "degraded"
	case status.UnknownReplicas > 0:
		status.OverallStatus = "starting"
	default:
		status.OverallStatus = "healthy"
	}

	return status
}

// matchesServiceName keeps service grouping consistent with the rest of the control plane.
func matchesServiceName(containerDockerName, serviceName string) bool {
	trimmed := strings.TrimPrefix(containerDockerName, "/")
	return trimmed == serviceName || strings.HasPrefix(trimmed, serviceName+"-")
}

// containerGeneration reads the generation label attached at container creation time.
func containerGeneration(c types.Container) int64 {
	if c.Labels == nil {
		return 0
	}

	raw := c.Labels["orchestrator.generation"]
	if raw == "" {
		return 0
	}

	generation, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}

	return generation
}

// FormatServiceSummary is a small helper for logs, future UI use.
func FormatServiceSummary(s ServiceStatus) string {
	return fmt.Sprintf(
		"%s image=%s desired=%d running=%d healthy=%d unhealthy=%d unknown=%d outdated=%d generation=%d status=%s",
		s.Name,
		s.Image,
		s.DesiredReplicas,
		s.RunningReplicas,
		s.HealthyReplicas,
		s.UnhealthyReplicas,
		s.UnknownReplicas,
		s.OutdatedReplicas,
		s.Generation,
		s.OverallStatus,
	)
}
