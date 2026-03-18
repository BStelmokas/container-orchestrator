package state

import (
	"sort"
	"sync"

	"orchestrator/internal/domain"
)

// ServiceStore is the control-plane source of truth for desired service state.
type ServiceStore struct {
	mu       sync.RWMutex
	services map[string]domain.ServiceSpec
}

// NewServiceStore constructs an empty in-memory desired-state store.
func NewServiceStore() *ServiceStore {
	return &ServiceStore{
		services: make(map[string]domain.ServiceSpec),
	}
}

// Upsert inserts a new service spec or replaces an existing one.
func (s *ServiceStore) Upsert(spec domain.ServiceSpec) error {
	spec = spec.Normalize()

	if err := spec.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.services[spec.Name] = spec // Desired state is keyed by logical service name
	return nil
}

// Get returns a copy of one stored service spec.
func (s *ServiceStore) Get(name string) (domain.ServiceSpec, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	spec, found := s.services[name]
	return spec, found
}

// List returns all stored service specs in a stable order.
func (s *ServiceStore) List() []domain.ServiceSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.services))
	for name := range s.services {
		names = append(names, name)
	}
	sort.Strings(names) // For deterministic iteration

	result := make([]domain.ServiceSpec, 0, len(names))
	for _, name := range names {
		result = append(result, s.services[name])
	}

	return result
}

// Scale updates only the replica count of an existing service.
func (s *ServiceStore) Scale(name string, replicas int) (domain.ServiceSpec, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	spec, found := s.services[name]
	if !found {
		return domain.ServiceSpec{}, false, nil
	}

	spec.Replicas = replicas
	if err := spec.Validate(); err != nil {
		return domain.ServiceSpec{}, true, err
	}

	s.services[name] = spec
	return spec, true, nil
}

// Delete removes desired state for a service.
func (s *ServiceStore) Delete(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.services[name]; !found {
		return false
	}

	delete(s.services, name)
	return true
}
