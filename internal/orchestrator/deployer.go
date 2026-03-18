package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"orchestrator/internal/domain"
	"orchestrator/internal/events"
	"orchestrator/internal/health"
	"orchestrator/internal/manager"
	"orchestrator/internal/state"

	"github.com/docker/docker/api/types"
)

// DeploymentController is responsible for reconciling desired service state with actual running containers.
type DeploymentController struct {
	manager     *manager.ContainerManager // handles container operations
	store       *state.ServiceStore       // desired state source of truth
	tracker     *health.Tracker
	recorder 		*events.Recorder
	running     map[string][]string // containerIDs currently running per service
	mu          sync.Mutex          // protects access to specs and running maps
	checkPeriod time.Duration       // how often to reconcile state
}

// NewDeploymentController creates a new deployment controller instance
func NewDeploymentController(
	m *manager.ContainerManager,
	store *state.ServiceStore,
	tracker *health.Tracker,
	recorder *events.Recorder,
	checkPeriod time.Duration,
	) *DeploymentController {
	return &DeploymentController{
		manager:     m,
		store:       store,
		tracker:     tracker,
		recorder: recorder,
		running:     make(map[string][]string),
		checkPeriod: checkPeriod,
	}
}

// ReconcileNow gives API handlers a way to trigger immediate convergence after changing desired state.
func (d *DeploymentController) ReconcileNow() {
	d.reconcileAll()
}

// Start begins the reconciliation loop as a background routine.
func (d *DeploymentController) Start() {
	go func() {
		for {
			d.reconcileAll()
			time.Sleep(d.checkPeriod)
		}
	}()
}

// reconcileAll loops through all registered services and ensures desired state.
func (d *DeploymentController) reconcileAll() {
	specs := d.store.List()

	for _, spec := range specs {
		d.reconcileService(spec)
	}
}

// reconcileService checks the actual state for a single service
// and starts or restarts containers as needed.
func (d *DeploymentController) reconcileService(spec domain.ServiceSpec) {
	// Fetch all containers only once to avoid repeated Docker calls
	containers, err := d.manager.ListRunningContainers()
	if err != nil {
		log.Printf("[Deployer] Failed to list running containers: %v", err)
		return
	}

	// Track healthy containers for this service
	var available []string // Replicas that are running and not explicitly unhealthy.
	var unhealthy []string // Replicas the health tracker has explicitly marked unhealthy.

	// Track old-generation replicas separately so restart can roll forward gradually.
	var outdated []types.Container

	for _, c := range containers {
		// Docker container names start with "/" - check for name prefix
		if len(c.Names) == 0 || !matchesServiceContainerName(c.Names[0], spec.Name) {
			continue
		}

		// Containers with an older generation are considered outdated.
		// They are still running, but they no longer match desired state.
		if containerGeneration(c) != spec.Generation {
			outdated = append(outdated, c)
			continue
		}

		report, found := d.tracker.Get(c.ID)
		if found && report.HasCheck && !report.Healthy {
			// Only explicitly unhealthy replicas are quarantined for replacement.
			unhealthy = append(unhealthy, c.ID)
			continue
		}

		available = append(available, c.ID)
	}

	// Replacement.
	for _, containerID := range unhealthy {
		log.Printf("[Deployer] Replacing unhealthy replica %s for service %q", containerID[:12], spec.Name)

		if d.recorder != nil {
			d.recorder.Add(
			"reconcile",
			fmt.Sprintf("replacing unhealthy replica %s for service=%s", containerID[:12], spec.Name),
			)
		}

		if err := d.manager.StopContainer(containerID); err != nil {
			log.Printf("[Deployer] Failed to remove unhealthy container %s for service %q: %v", containerID[:12], spec.Name, err)
			continue
		}
	}

	// Rolling restart behavior.
	if len(outdated) > 0 {
		if len(available) < spec.Replicas {
			containerName := spec.Name + "-" + randomSuffix()

			id, err := d.manager.StartContainer(spec.Name, spec.Image, containerName, spec.Generation)
			if err != nil {
				log.Printf("[Deployer] Failed to start rollout replica for service %q: %v", spec.Name, err)
			} else {
				log.Printf("[Deployer] Started rollout replica %s for service %q generation %d", id[:12], spec.Name, spec.Generation)

				if d.recorder != nil {
					d.recorder.Add(
						"reconcile",
						fmt.Sprintf("started rollout replica %s for service=%s generation=%d", id[:12], spec.Name, spec.Generation),
					)
				}

				available = append(available, id)
			}

			d.mu.Lock()
			d.running[spec.Name] = available
			d.mu.Unlock()
			return
		}

		outdatedContainer := outdated[0]
		log.Printf(
			"[Deployer] Removing outdated replica %s for service %q oldGeneration=%d desiredGeneration=%d",
			outdatedContainer.ID[:12],
			spec.Name,
			containerGeneration(outdatedContainer),
			spec.Generation,
		)

		if d.recorder != nil {
			d.recorder.Add(
				"reconcile",
				fmt.Sprintf(
					"removing outdated replica %s for service=%s oldGeneration=%d desiredGeneration=%d",
					outdatedContainer.ID[:12],
					spec.Name,
					containerGeneration(outdatedContainer),
					spec.Generation,
				),
			)
		}

		if err := d.manager.StopContainer(outdatedContainer.ID); err != nil {
			log.Printf("[Deployer] Failed to remove outdated container %s for service %q: %v", outdatedContainer.ID[:12], spec.Name, err)
		}

		d.mu.Lock()
		d.running[spec.Name] = available
		d.mu.Unlock()
		return
	}

	// Calculate how many replicas are missing
	missing := spec.Replicas - len(available)

	if missing > 0 {
		log.Printf("[Deployer] %d replicas missing for service %q", missing, spec.Name)
	}

	// When needing more containers, start them
	for i := 0; i < missing; i++ {
		containerName := spec.Name + "-" + randomSuffix() // ensure unique name
		id, err := d.manager.StartContainer(spec.Name, spec.Image, containerName, spec.Generation)
		if err != nil {
			log.Printf("Failed to start container for service %q: %v", spec.Name, err)
			continue
		}

		if d.recorder != nil {
			d.recorder.Add(
				"reconcile",
				fmt.Sprintf("started replica %s for service=%s generation=%d", id[:12], spec.Name, spec.Generation),
			)
		}

		log.Printf("Started container %s for service %q", id[:12], spec.Name)
		available = append(available, id)
	}

	// When needing fewer containers, stop them
	if missing < 0 {
		log.Printf("[Deployer] %d replicas too many for service %q", -missing, spec.Name)

		for i := 0; i < -missing; i++ {
			containerId := available[len(available)-1]

			if d.recorder != nil {
				d.recorder.Add(
					"reconcile",
					fmt.Sprintf("scaling down replica %s for service=%s", containerId[:12], spec.Name),
				)
			}

			err := d.manager.StopContainer(containerId)
			if err != nil {
				log.Printf("Failed to stop container for service %q: %v", spec.Name, err)
				continue
			}
			log.Printf("Stopped container %s for service %q", containerId[:12], spec.Name)
			available = available[:len(available)-1]
		}
	}

	d.mu.Lock()
	// Save updated healthy container list
	d.running[spec.Name] = available
	d.mu.Unlock()
}

// matchesServiceContainerName is a helper for explicit service matching.
func matchesServiceContainerName(containerDockerName, serviceName string) bool {
	trimmed := strings.TrimPrefix(containerDockerName, "/")
	return trimmed == serviceName || strings.HasPrefix(trimmed, serviceName+"-")
}

// containerGeneration reads the generation label attached at container creation time.
func containerGeneration(c types.Container) int64 {
	if c.Labels == nil {
		return 0
	}

	raw := c.Labels["orcherstrator.generation"]
	if raw == "" {
		return 0
	}

	generation, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}

	return generation
}

// randomSuffix generates a short random string to differentiate container names
func randomSuffix() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback to keep uniqueness
		return time.Now().Format("150405.000000000")
	}

	return hex.EncodeToString(b)
}
