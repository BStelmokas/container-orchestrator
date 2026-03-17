package orchestrator

import (
	"log"
	"strings"
	"sync"
	"time"

	"orchestrator/internal/manager"
)

// ServiceSpec defines a desired deployment state for a service.
// It includes a name, the Docker image to use, and how many replicas should be running.
type ServiceSpec struct {
	Name     string // logical service name (e.g., "auth-service")
	Image    string // Docker image (e.g., "nginx:latest")
	Replicas int    // number of desired container instances
}

// DeploymentController is responsible for ensuring that a given service
// always has the desired number of containers running.
type DeploymentController struct {
	manager     *manager.ContainerManager // handles container operations
	specs       map[string]ServiceSpec    // desired service specs by name
	running     map[string][]string       // containerIDs currently running per service
	mu          sync.Mutex                // protects access to specs and running maps
	checkPeriod time.Duration             // how often to reconcile state
}

// NewDeploymentController creates a new deployment controller instance
func NewDeploymentController(m *manager.ContainerManager, checkPeriod time.Duration) *DeploymentController {
	return &DeploymentController{
		manager:     m,
		specs:       make(map[string]ServiceSpec),
		running:     make(map[string][]string),
		checkPeriod: checkPeriod,
	}
}

// Deploy registers a desired spec and triggers reconciliation
func (d *DeploymentController) Deploy(spec ServiceSpec) {
	d.mu.Lock()
	defer d.mu.Unlock()

	log.Printf("Deploying service %q with image=%q, replicas=%d", spec.Name, spec.Image, spec.Replicas)
	d.specs[spec.Name] = spec
}

// Start begins the reconciliation loop as a background routine
func (d *DeploymentController) Start() {
	go func() {
		for {
			d.reconcileAll()
			time.Sleep(d.checkPeriod)
		}
	}()
}

// reconcileAll loops through all registered services and ensures desired state
func (d *DeploymentController) reconcileAll() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, spec := range d.specs {
		d.reconcileService(spec)
	}
}

// reconcileService checks the actual state for a single service
// and starts or restarts containers as needed.
func (d *DeploymentController) reconcileService(spec ServiceSpec) {
	// Fetch all containers only once to avoid repeated Docker calls
	containers, err := d.manager.ListContainers()
	if err != nil {
		log.Printf("[Deployer] Failed to list containers: %v", err)
		return
	}

	// Track healthy containers for this service
	var healthy []string

	// Loop through all containers and collect those that match this service
	for _, c := range containers {
		// Docker container names start with "/" - check for name prefix
		if len(c.Names) > 0 && strings.HasPrefix(c.Names[0], "/"+spec.Name) {
			healthy = append(healthy, c.ID)
		}
	}

	// Calculate how many replicas are missing
	missing := spec.Replicas - len(healthy)

	if missing > 0 {
		log.Printf("[Deployer] %d replicas missing for service %q", missing, spec.Name)
	}

	// When needing more containers, start them
	for i := 0; i < missing; i++ {
		containerName := spec.Name + "-" + randomSuffix() // ensure unique name
		id, err := d.manager.StartContainer(spec.Image, containerName)
		if err != nil {
			log.Printf("Failed to start container for service %q: %v", spec.Name, err)
			continue
		}
		log.Printf("Started container %s for service %q", id[:12], spec.Name)
		healthy = append(healthy, id)
	}

	// When needing fewer containers, stop them
	if missing < 0 {
		log.Printf("[Deployer] %d replicas too many for service %q", -missing, spec.Name)

		for i := 0; i < -missing; i++ {
			containerId := healthy[len(healthy)-1]
			err := d.manager.StopContainer(containerId)
			if err != nil {
				log.Printf("Failed to stop container for service %q: %v", spec.Name, err)
				continue
			}
			log.Printf("Stopped container %s for service %q", containerId[:12], spec.Name)
			healthy = healthy[:len(healthy)-1]
		}
	}


	// Save updated healthy container list
	d.running[spec.Name] = healthy
}

// randomSuffix generates a short random string to differentiate container names
func randomSuffix() string {
	return time.Now().Format("150405") // e.g. "153201" or 15:32:01
}
