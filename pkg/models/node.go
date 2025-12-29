package models

import (
	"time"
)

// NodeType represents the type of compute node
type NodeType string

const (
	NodeTypeServer  NodeType = "server"
	NodeTypeDesktop NodeType = "desktop"
	NodeTypeLaptop  NodeType = "laptop"
)

// Node represents a compute node in the distributed system
type Node struct {
	ID              string            `json:"id"`
	Address         string            `json:"address"`
	Type            NodeType          `json:"type"`
	CPUThreads      int               `json:"cpu_threads"`
	CPUModel        string            `json:"cpu_model"`
	HasGPU          bool              `json:"has_gpu"`
	GPUType         string            `json:"gpu_type,omitempty"`
	RAMBytes        uint64            `json:"ram_bytes"`
	Labels          map[string]string `json:"labels,omitempty"`
	Status          string            `json:"status"` // "available", "busy", "offline"
	LastHeartbeat   time.Time         `json:"last_heartbeat"`
	RegisteredAt    time.Time         `json:"registered_at"`
	CurrentJobID    string            `json:"current_job_id,omitempty"`
}

// NodeRegistration represents a node registration request
type NodeRegistration struct {
	Address      string            `json:"address"`
	Type         NodeType          `json:"type"`
	CPUThreads   int               `json:"cpu_threads"`
	CPUModel     string            `json:"cpu_model"`
	HasGPU       bool              `json:"has_gpu"`
	GPUType      string            `json:"gpu_type,omitempty"`
	RAMBytes     uint64            `json:"ram_bytes"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// NodeCapabilities represents the capabilities of a compute node
type NodeCapabilities struct {
	CPUThreads int               `json:"cpu_threads"`
	CPUModel   string            `json:"cpu_model"`
	HasGPU     bool              `json:"has_gpu"`
	GPUType    string            `json:"gpu_type,omitempty"`
	RAMBytes   uint64            `json:"ram_bytes"`
	Labels     map[string]string `json:"labels,omitempty"`
}
