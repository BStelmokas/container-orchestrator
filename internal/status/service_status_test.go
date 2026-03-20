package status

import (
	"testing"

	"orchestrator/internal/health"

	"github.com/docker/docker/api/types"
)

// Verify rollout-aware status marks outdated replicas and returns overallStatus=rolling.
func TestBuildServiceStatusDetectsRollingReplicas(t *testing.T) {
	tracker := health.NewTracker()

	builder := &Builder{
		tracker: tracker,
	}

	containers := []types.Container{
		{
			ID: "aaaaaaaaaaaa111111111111",
			Names: []string{"/web-old"},
			Status: "Up 10 seconds",
			Labels: map[string]string{
				"orchestrator.generation": "1",
			},
		},
		{
			ID: "bbbbbbbbbbbb222222222222",
			Names: []string{"/web-new"},
			Status: "Up 10 seconds",
			Labels: map[string]string{
				"orchestrator.generation": "2",
			},
		},
	}

	// Report both replicas healthy so rollout state is isolated from health state.
	tracker.Register("aaaaaaaaaaaa111111111111", "web", "web-old", "http://127.0.0.1:8081")
	tracker.Register("bbbbbbbbbbbb222222222222", "web", "web-new", "http://127.0.0.1:8082")
	tracker.ReportHealthy("aaaaaaaaaaaa111111111111")
	tracker.ReportHealthy("bbbbbbbbbbbb222222222222")

	status := builder.buildServiceStatus("web", "nginx:latest", 2, 2, containers)

	if status.OutdatedReplicas != 1 {
		t.Fatalf("expected 1 outdated replica, got %d", status.OutdatedReplicas)
	}

	if status.OverallStatus != "rolling" {
		t.Fatalf("expected overallStatus=rolling, got %s", status.OverallStatus)
	}

	if len(status.Containers) != 2 {
		t.Fatalf("expected 2 container statuses, got %d", len(status.Containers))
	}
}
