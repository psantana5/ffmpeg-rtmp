package store

import (
	"time"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
)

// Store defines the interface for data persistence
// Both SQLite and PostgreSQL implement this interface
type Store interface {
	// Node operations
	RegisterNode(node *models.Node) error
	GetNode(id string) (*models.Node, error)
	GetNodeByAddress(address string) (*models.Node, error)
	GetAllNodes() []*models.Node
	UpdateNodeStatus(id, status string) error
	UpdateNodeHeartbeat(id string) error
	DeleteNode(id string) error

	// Job operations
	CreateJob(job *models.Job) error
	GetJob(id string) (*models.Job, error)
	GetJobBySequenceNumber(seqNum int) (*models.Job, error)
	GetAllJobs() []*models.Job
	GetNextJob(nodeID string) (*models.Job, error)
	UpdateJobStatus(id string, status models.JobStatus, errorMsg string) error
	UpdateJobProgress(id string, progress int) error
	UpdateJobActivity(id string) error
	UpdateJobFailureReason(id string, reason models.FailureReason, errorMsg string) error
	UpdateJob(job *models.Job) error
	DeleteJob(id string) error
	GetJobs(status string) ([]models.Job, error)

	// Job state management
	AddStateTransition(id string, from, to models.JobStatus, reason string) error
	PauseJob(id string) error
	ResumeJob(id string) error
	CancelJob(id string) error
	RetryJob(jobID string, errorMsg string) error
	TryQueuePendingJob(jobID string) (bool, error)
	GetQueuedJobs(queue string, priority string) []*models.Job

	// FSM operations (for production scheduler)
	TransitionJobState(jobID string, toState models.JobStatus, reason string) (bool, error)
	AssignJobToWorker(jobID, nodeID string) (bool, error)
	CompleteJob(jobID, nodeID string) (bool, error)
	UpdateJobHeartbeat(jobID string) error
	GetJobsInState(state models.JobStatus) ([]*models.Job, error)
	GetOrphanedJobs(workerTimeout time.Duration) ([]*models.Job, error)
	GetTimedOutJobs() ([]*models.Job, error)

	// Tenant operations (multi-tenancy)
	CreateTenant(tenant *models.Tenant) error
	GetTenant(id string) (*models.Tenant, error)
	GetTenantByName(name string) (*models.Tenant, error)
	ListTenants() ([]*models.Tenant, error)
	UpdateTenant(tenant *models.Tenant) error
	DeleteTenant(id string) error
	UpdateTenantUsage(id string, usage *models.TenantUsage) error
	GetTenantStats(id string) (*models.TenantUsage, error)

	// Tenant-aware operations
	GetJobsByTenant(tenantID string) ([]*models.Job, error)
	GetNodesByTenant(tenantID string) ([]*models.Node, error)

	// Lifecycle
	Close() error
	HealthCheck() error
	Vacuum() error

	// Metrics operations (optimized for large datasets)
	GetJobMetrics() (*JobMetrics, error)
}

// JobMetrics contains aggregated job statistics for metrics endpoint
type JobMetrics struct {
	JobsByState       map[models.JobStatus]int
	JobsByEngine      map[string]int
	CompletedByEngine map[string]int
	QueueByPriority   map[string]int
	QueueByType       map[string]int
	ActiveJobs        int
	QueueLength       int
	AvgDuration       float64
	TotalJobs         int
}

// ExtendedStore includes FSM operations for production scheduler
// Kept for backward compatibility - all stores now implement FSM methods
type ExtendedStore interface {
	Store
}

// Config holds database configuration
type Config struct {
	Type string // "sqlite" or "postgres"
	DSN  string // Connection string

	// PostgreSQL specific
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// SQLite specific (for backward compatibility)
	Path string
}

// NewStore creates a store based on configuration
func NewStore(config Config) (Store, error) {
	switch config.Type {
	case "postgres", "postgresql":
		return NewPostgreSQLStore(config)
	case "sqlite", "":
		// Default to SQLite for backward compatibility
		path := config.Path
		if path == "" {
			path = config.DSN
		}
		if path == "" {
			path = "master.db"
		}
		return NewSQLiteStore(path)
	default:
		return nil, ErrUnsupportedDatabase
	}
}

var (
	ErrUnsupportedDatabase = NewError("unsupported database type")
)

// NewError creates a new error with message
func NewError(message string) error {
	return &storeError{message: message}
}

type storeError struct {
	message string
}

func (e *storeError) Error() string {
	return e.message
}
