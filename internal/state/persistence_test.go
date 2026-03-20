package state

import (
	"path/filepath"
	"testing"

	"orchestrator/internal/domain"
)

// Verify persisted desired state survives a save/load round trip.
func TestServiceStoreSaveAndLoadRoundTrip(t *testing.T) {
	store := NewServiceStore()

	err := store.Upsert(domain.ServiceSpec{
		Name:       "web",
		Image:      "nginx:latest",
		Replicas:   3,
		Generation: 2,
	})
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "services.json")

	if err := store.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	reloaded := NewServiceStore()
	if err := reloaded.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}

	spec, found := reloaded.Get("web")
	if !found {
		t.Fatalf("expected service to be reloaded")
	}

	if spec.Image != "nginx:latest" {
		t.Fatalf("unexpected image: %s", spec.Image)
	}
	if spec.Replicas != 3 {
		t.Fatalf("unexpected replicas: %d", spec.Replicas)
	}
	if spec.Generation != 2 {
		t.Fatalf("unexpected generation: %d", spec.Generation)
	}
}
