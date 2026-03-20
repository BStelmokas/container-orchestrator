package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"orchestrator/internal/domain"
)

// persistedState is the on-disk representation of desired service state.
type persistedState struct {
	Services []domain.ServiceSpec `json:"services"`
}

// SaveToFile writes the entire desired-state store to disk.
func (s *ServiceStore) SaveToFile(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.services))
	for name := range s.services {
		names = append(names, name)
	}
	sort.Strings(names)

	state := persistedState{
		Services: make([]domain.ServiceSpec, 0, len(names)),
	}

	for _, name := range names {
		state.Services = append(state.Services, s.services[name])
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// LoadFromFile restores desired service state from disk.
func (s *ServiceStore) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Clean first boot with no persisted state.
		}
		return err
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Replace the in-memory store with the restored snapshot so boot is deterministic.
	s.services = make(map[string]domain.ServiceSpec)

	for _, spec := range state.Services {
		spec = spec.Normalize()

		if err := spec.Validate(); err != nil {
			continue
		}
		s.services[spec.Name] = spec
	}

	return nil
}
