package resources

import (
	"fmt"
	"sync"
)

// ResourceType represents the type of resource
type ResourceType string

const (
	ResourceCPU ResourceType = "cpu"
	ResourceGPU ResourceType = "gpu"
	ResourceRAM ResourceType = "ram"
)

// Reservation represents a resource reservation
type Reservation struct {
	JobID        string
	NodeID       string
	ResourceType ResourceType
	Amount       float64 // CPU cores, GPU count, or RAM in GB
}

// Manager manages resource reservations and allocations
type Manager struct {
	mu           sync.RWMutex
	reservations map[string]*Reservation       // jobID -> reservation
	nodeResources map[string]*NodeResources     // nodeID -> available resources
}

// NodeResources tracks available resources for a node
type NodeResources struct {
	TotalCPU      float64
	TotalGPU      int
	TotalRAMGB    float64
	AvailableCPU  float64
	AvailableGPU  int
	AvailableRAMGB float64
}

// NewManager creates a new resource manager
func NewManager() *Manager {
	return &Manager{
		reservations:  make(map[string]*Reservation),
		nodeResources: make(map[string]*NodeResources),
	}
}

// RegisterNode registers a node's resources
func (m *Manager) RegisterNode(nodeID string, cpuCores float64, gpuCount int, ramGB float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodeResources[nodeID] = &NodeResources{
		TotalCPU:       cpuCores,
		TotalGPU:       gpuCount,
		TotalRAMGB:     ramGB,
		AvailableCPU:   cpuCores,
		AvailableGPU:   gpuCount,
		AvailableRAMGB: ramGB,
	}
}

// UnregisterNode removes a node from resource tracking
func (m *Manager) UnregisterNode(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.nodeResources, nodeID)
	
	// Release any reservations for this node
	for jobID, res := range m.reservations {
		if res.NodeID == nodeID {
			delete(m.reservations, jobID)
		}
	}
}

// Reserve attempts to reserve resources for a job
func (m *Manager) Reserve(jobID, nodeID string, cpuCores float64, gpuCount int, ramGB float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if node exists
	node, exists := m.nodeResources[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	// Check if resources are available
	if node.AvailableCPU < cpuCores {
		return fmt.Errorf("insufficient CPU: need %.2f, available %.2f", cpuCores, node.AvailableCPU)
	}
	if node.AvailableGPU < gpuCount {
		return fmt.Errorf("insufficient GPU: need %d, available %d", gpuCount, node.AvailableGPU)
	}
	if node.AvailableRAMGB < ramGB {
		return fmt.Errorf("insufficient RAM: need %.2f GB, available %.2f GB", ramGB, node.AvailableRAMGB)
	}

	// Check if job already has a reservation
	if _, exists := m.reservations[jobID]; exists {
		return fmt.Errorf("job %s already has a reservation", jobID)
	}

	// Reserve resources
	node.AvailableCPU -= cpuCores
	node.AvailableGPU -= gpuCount
	node.AvailableRAMGB -= ramGB

	// Create reservation record
	m.reservations[jobID] = &Reservation{
		JobID:        jobID,
		NodeID:       nodeID,
		ResourceType: ResourceCPU, // Primary resource type
		Amount:       cpuCores,
	}

	return nil
}

// Release releases resources reserved for a job
func (m *Manager) Release(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	res, exists := m.reservations[jobID]
	if !exists {
		return fmt.Errorf("no reservation found for job %s", jobID)
	}

	node, exists := m.nodeResources[res.NodeID]
	if !exists {
		// Node was removed, just clean up the reservation
		delete(m.reservations, jobID)
		return nil
	}

	// Release resources back to node
	node.AvailableCPU += res.Amount
	// Note: In a full implementation, we'd track GPU and RAM separately

	delete(m.reservations, jobID)
	return nil
}

// GetNodeResources returns the current resource state for a node
func (m *Manager) GetNodeResources(nodeID string) (*NodeResources, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.nodeResources[nodeID]
	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	// Return a copy to prevent external modifications
	return &NodeResources{
		TotalCPU:       node.TotalCPU,
		TotalGPU:       node.TotalGPU,
		TotalRAMGB:     node.TotalRAMGB,
		AvailableCPU:   node.AvailableCPU,
		AvailableGPU:   node.AvailableGPU,
		AvailableRAMGB: node.AvailableRAMGB,
	}, nil
}

// GetAvailableNodes returns nodes that can satisfy the resource requirements
func (m *Manager) GetAvailableNodes(cpuCores float64, gpuCount int, ramGB float64) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var availableNodes []string
	for nodeID, node := range m.nodeResources {
		if node.AvailableCPU >= cpuCores &&
			node.AvailableGPU >= gpuCount &&
			node.AvailableRAMGB >= ramGB {
			availableNodes = append(availableNodes, nodeID)
		}
	}

	return availableNodes
}

// GetReservation returns the reservation for a job
func (m *Manager) GetReservation(jobID string) (*Reservation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res, exists := m.reservations[jobID]
	if !exists {
		return nil, false
	}

	// Return a copy
	return &Reservation{
		JobID:        res.JobID,
		NodeID:       res.NodeID,
		ResourceType: res.ResourceType,
		Amount:       res.Amount,
	}, true
}

// GetAllReservations returns all current reservations
func (m *Manager) GetAllReservations() []*Reservation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reservations := make([]*Reservation, 0, len(m.reservations))
	for _, res := range m.reservations {
		reservations = append(reservations, &Reservation{
			JobID:        res.JobID,
			NodeID:       res.NodeID,
			ResourceType: res.ResourceType,
			Amount:       res.Amount,
		})
	}

	return reservations
}
