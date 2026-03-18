package health

import(
	"sort"
	"sync"
	"time"
)

// Report represents the latest known health information for one concrete container replica.
type Report struct {
	ContainerID string
	ServiceName string
	ContainerName string
	HealthURL string
	Healthy bool
	HasCheck bool
	LastChecked time.Time
	LastError string
}

// Tracker is the centralized in-memory health state store.
type Tracker struct {
	mu sync.RWMutex
	reports map[string]Report
}

// NewTracker constructs an empty health tracker.
func NewTracker() *Tracker {
	return &Tracker{
		reports: make(map[string]Report),
	}
}

// Register seeds health state for a newly created replica.
func (t *Tracker) Register(containerID, serviceName, containerName, healthURL string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.reports[containerID] = Report{
		ContainerID: containerID,
		ServiceName: serviceName,
		ContainerName: containerName,
		HealthURL: healthURL,
		Healthy: false,
		HasCheck: false, // Prevents the reconciler from treating a brand-new replica as explicitly unhealthy.
		LastChecked: time.Time{},
		LastError: "",
	}
}

// ReportHealthy records a successful health check for a replica.
func (t *Tracker) ReportHealthy(containerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	report, found := t.reports[containerID]
	if !found {
		return
	}

	report.Healthy = true
	report.HasCheck = true
	report.LastChecked = time.Now()
	report.LastError = ""
	t.reports[containerID] = report
}

// ReportUnhealthy records a failed health check for a replica.
func (t *Tracker) ReportUnhealthy(containerID, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	report, found := t.reports[containerID]
	if !found {
		return
	}

	report.Healthy = false
	report.HasCheck = true
	report.LastChecked = time.Now()
	report.LastError = reason
	t.reports[containerID] = report
}

// Get returns the latest known health report for one container replica.
func (t *Tracker) Get(containerID string) (Report, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	report, found := t.reports[containerID]
	return report, found
}

// Remove deletes health state for a replica that has been stopped/removed.
func (t *Tracker) Remove(containerID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.reports, containerID)
}

// ListByService returns all health reports for one logical service in stable order.
func (t *Tracker) ListByService(serviceName string) []Report {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []Report
	for _, report := range t.reports {
		if report.ServiceName == serviceName {
			result = append(result, report)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ContainerID < result[j].ContainerID
	})

	return result
}
