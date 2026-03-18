package domain

import (
	"fmt"
	"strings"
)

type ServiceSpec struct {
	Name     string
	Image    string
	Replicas int
}

// Normalize returns a cleaned copy of the spec.
func (s ServiceSpec) Normalize() ServiceSpec {
	s.Name = strings.TrimSpace(s.Name)
	s.Image = strings.TrimSpace(s.Image)
	return s
}

// Validate enforces the minimum invariants needed by the orchestrator.
func (s ServiceSpec) Validate() error {
	s = s.Normalize()

	if s.Name == "" {
		return fmt.Errorf("service name is required")
	}

	if strings.Contains(s.Name, " ") {
		return fmt.Errorf("service name cannot contain spaces")
	}

	if s.Image == "" {
		return fmt.Errorf("service image is required")
	}

	if s.Replicas < 1 {
		return fmt.Errorf("replicas must be at least 1")
	}

	return nil
}
