package resources

import (
	"testing"
)

func TestResourceManager(t *testing.T) {
	manager := NewManager()

	// Register a node
	manager.RegisterNode("node1", 8.0, 1, 16.0)

	// Check node resources
	res, err := manager.GetNodeResources("node1")
	if err != nil {
		t.Fatalf("Failed to get node resources: %v", err)
	}
	if res.AvailableCPU != 8.0 {
		t.Errorf("Expected 8.0 available CPU, got %.2f", res.AvailableCPU)
	}

	// Reserve resources for a job
	err = manager.Reserve("job1", "node1", 4.0, 1, 8.0)
	if err != nil {
		t.Fatalf("Failed to reserve resources: %v", err)
	}

	// Check updated resources
	res, err = manager.GetNodeResources("node1")
	if err != nil {
		t.Fatalf("Failed to get node resources: %v", err)
	}
	if res.AvailableCPU != 4.0 {
		t.Errorf("Expected 4.0 available CPU after reservation, got %.2f", res.AvailableCPU)
	}

	// Try to reserve more resources than available
	err = manager.Reserve("job2", "node1", 5.0, 0, 4.0)
	if err == nil {
		t.Error("Expected error when reserving more CPU than available")
	}

	// Release resources
	err = manager.Release("job1")
	if err != nil {
		t.Fatalf("Failed to release resources: %v", err)
	}

	// Check resources after release
	res, err = manager.GetNodeResources("node1")
	if err != nil {
		t.Fatalf("Failed to get node resources: %v", err)
	}
	if res.AvailableCPU != 8.0 {
		t.Errorf("Expected 8.0 available CPU after release, got %.2f", res.AvailableCPU)
	}
}

func TestGetAvailableNodes(t *testing.T) {
	manager := NewManager()

	manager.RegisterNode("node1", 8.0, 1, 16.0)
	manager.RegisterNode("node2", 4.0, 0, 8.0)
	manager.RegisterNode("node3", 16.0, 2, 32.0)

	// Find nodes with at least 6 CPU cores
	nodes := manager.GetAvailableNodes(6.0, 0, 4.0)
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes with 6+ CPU cores, got %d", len(nodes))
	}

	// Find nodes with GPU
	nodes = manager.GetAvailableNodes(4.0, 1, 8.0)
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes with GPU, got %d", len(nodes))
	}

	// Reserve resources on node1
	manager.Reserve("job1", "node1", 6.0, 1, 12.0)

	// node1 should no longer have enough CPU
	nodes = manager.GetAvailableNodes(6.0, 0, 4.0)
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node with 6+ CPU cores after reservation, got %d", len(nodes))
	}
}

func TestUnregisterNode(t *testing.T) {
	manager := NewManager()

	manager.RegisterNode("node1", 8.0, 1, 16.0)
	manager.Reserve("job1", "node1", 4.0, 1, 8.0)

	// Unregister node
	manager.UnregisterNode("node1")

	// Node should no longer exist
	_, err := manager.GetNodeResources("node1")
	if err == nil {
		t.Error("Expected error when getting resources for unregistered node")
	}

	// Reservation should be cleaned up
	_, exists := manager.GetReservation("job1")
	if exists {
		t.Error("Expected reservation to be cleaned up when node is unregistered")
	}
}
