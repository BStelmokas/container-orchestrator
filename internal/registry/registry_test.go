package registry

import "testing"

// Verify that one logical service can hold many concrete instances.
func TestRegistrySupportsMultipleInstancesPerService(t *testing.T) {
	reg := NewRegistry()

	reg.Register("api", "instance-a", "127.0.0.1", 8081)
	reg.Register("api", "instance-b", "127.0.0.1", 8082)

	all := reg.LookupAll("api")
	if len(all) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(all))
	}

	one, found := reg.LookupOne("api")
	if !found {
		t.Fatalf("expected LookupOne to find one instance")
	}

	if one.ServiceName != "api" {
		t.Fatalf("unexpected service name: %s", one.ServiceName)
	}
}

// Verify that deleting one instance does not remove sibling replicas.
func TestRegistryDeleteRemovesOnlyOneInstance(t *testing.T) {
	reg := NewRegistry()

	reg.Register("api", "instance-a", "127.0.0.1", 8081)
	reg.Register("api", "instance-b", "127.0.0.1", 8082)

	deleted := reg.Delete("api", "instance-a")
	if !deleted {
		t.Fatalf("expected instance delete to succeed")
	}

	all := reg.LookupAll("api")
	if len(all) != 1 {
		t.Fatalf("expected 1 remaining instance, got %d", len(all))
	}

	if all[0].InstanceID != "instance-b" {
		t.Fatalf("expected remaining instance to be instance-b, got %s", all[0].InstanceID)
	}
}
