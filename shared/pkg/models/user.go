package models

import (
	"time"
)

// Role represents a user role in the system
type Role string

const (
	RoleAdmin    Role = "admin"    // Full system access
	RoleOperator Role = "operator" // Can manage jobs and nodes
	RoleViewer   Role = "viewer"   // Read-only access
	RoleDeveloper Role = "developer" // Can submit jobs, view results
)

// Permission represents a specific permission
type Permission string

const (
	// Job permissions
	PermJobCreate Permission = "job:create"
	PermJobRead   Permission = "job:read"
	PermJobUpdate Permission = "job:update"
	PermJobDelete Permission = "job:delete"
	PermJobCancel Permission = "job:cancel"

	// Node permissions
	PermNodeRegister Permission = "node:register"
	PermNodeRead     Permission = "node:read"
	PermNodeUpdate   Permission = "node:update"
	PermNodeDelete   Permission = "node:delete"

	// Tenant permissions
	PermTenantRead   Permission = "tenant:read"
	PermTenantUpdate Permission = "tenant:update"
	PermTenantDelete Permission = "tenant:delete"

	// User permissions
	PermUserCreate Permission = "user:create"
	PermUserRead   Permission = "user:read"
	PermUserUpdate Permission = "user:update"
	PermUserDelete Permission = "user:delete"

	// API key permissions
	PermAPIKeyCreate Permission = "apikey:create"
	PermAPIKeyRead   Permission = "apikey:read"
	PermAPIKeyRevoke Permission = "apikey:revoke"

	// Metrics permissions
	PermMetricsRead Permission = "metrics:read"
)

// User represents a user in the system
type User struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never expose in JSON
	FullName     string    `json:"full_name"`
	Role         Role      `json:"role"`
	Status       string    `json:"status"` // "active", "suspended", "deleted"
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserRequest represents a request to create or update a user
type UserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password,omitempty"` // Only for creation
	FullName string `json:"full_name"`
	Role     Role   `json:"role"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

// Session represents an authenticated session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TenantID  string    `json:"tenant_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
}

// RolePermissions maps roles to their permissions
var RolePermissions = map[Role][]Permission{
	RoleAdmin: {
		// All permissions
		PermJobCreate, PermJobRead, PermJobUpdate, PermJobDelete, PermJobCancel,
		PermNodeRegister, PermNodeRead, PermNodeUpdate, PermNodeDelete,
		PermTenantRead, PermTenantUpdate, PermTenantDelete,
		PermUserCreate, PermUserRead, PermUserUpdate, PermUserDelete,
		PermAPIKeyCreate, PermAPIKeyRead, PermAPIKeyRevoke,
		PermMetricsRead,
	},
	RoleOperator: {
		// Job and node management, read-only for tenant/users
		PermJobCreate, PermJobRead, PermJobUpdate, PermJobCancel,
		PermNodeRegister, PermNodeRead, PermNodeUpdate,
		PermTenantRead,
		PermUserRead,
		PermAPIKeyRead,
		PermMetricsRead,
	},
	RoleDeveloper: {
		// Can create and manage own jobs, read nodes
		PermJobCreate, PermJobRead, PermJobCancel,
		PermNodeRead,
		PermMetricsRead,
	},
	RoleViewer: {
		// Read-only access
		PermJobRead,
		PermNodeRead,
		PermTenantRead,
		PermMetricsRead,
	},
}

// HasPermission checks if a role has a specific permission
func (r Role) HasPermission(perm Permission) bool {
	perms, ok := RolePermissions[r]
	if !ok {
		return false
	}

	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// GetPermissions returns all permissions for a role
func (r Role) GetPermissions() []Permission {
	return RolePermissions[r]
}

// IsValid checks if a role is valid
func (r Role) IsValid() bool {
	_, ok := RolePermissions[r]
	return ok
}
