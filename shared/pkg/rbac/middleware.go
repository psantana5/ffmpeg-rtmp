package rbac

import (
	"context"
	"errors"
	"net/http"

	"github.com/psantana5/ffmpeg-rtmp/pkg/models"
	"github.com/psantana5/ffmpeg-rtmp/pkg/tenancy"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
	ErrInvalidRole      = errors.New("invalid role")
)

// HasPermission checks if the current user has a specific permission
func HasPermission(ctx context.Context, perm models.Permission) bool {
	roleStr := tenancy.GetUserRole(ctx)
	if roleStr == "" {
		return false
	}

	role := models.Role(roleStr)
	return role.HasPermission(perm)
}

// RequirePermission middleware that checks for a specific permission
func RequirePermission(perm models.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !HasPermission(r.Context(), perm) {
				http.Error(w, `{"error":"forbidden","message":"Insufficient permissions"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission middleware that checks for any of the given permissions
func RequireAnyPermission(perms ...models.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, perm := range perms {
				if HasPermission(r.Context(), perm) {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden","message":"Insufficient permissions"}`, http.StatusForbidden)
		})
	}
}

// RequireAllPermissions middleware that checks for all given permissions
func RequireAllPermissions(perms ...models.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, perm := range perms {
				if !HasPermission(r.Context(), perm) {
					http.Error(w, `{"error":"forbidden","message":"Insufficient permissions"}`, http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole middleware that checks for a specific role
func RequireRole(role models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := tenancy.GetUserRole(r.Context())
			if models.Role(userRole) != role {
				http.Error(w, `{"error":"forbidden","message":"Insufficient role"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole middleware that checks for any of the given roles
func RequireAnyRole(roles ...models.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userRole := models.Role(tenancy.GetUserRole(r.Context()))
			for _, role := range roles {
				if userRole == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, `{"error":"forbidden","message":"Insufficient role"}`, http.StatusForbidden)
		})
	}
}

// AdminOnly middleware that allows only admin users
func AdminOnly(next http.Handler) http.Handler {
	return RequireRole(models.RoleAdmin)(next)
}

// OperatorOrAdmin middleware that allows operators and admins
func OperatorOrAdmin(next http.Handler) http.Handler {
	return RequireAnyRole(models.RoleAdmin, models.RoleOperator)(next)
}

// CheckPermission is a helper function to check permissions in handlers
func CheckPermission(ctx context.Context, perm models.Permission) error {
	if !HasPermission(ctx, perm) {
		return ErrPermissionDenied
	}
	return nil
}

// GetUserPermissions returns all permissions for the current user
func GetUserPermissions(ctx context.Context) []models.Permission {
	roleStr := tenancy.GetUserRole(ctx)
	if roleStr == "" {
		return nil
	}

	role := models.Role(roleStr)
	return role.GetPermissions()
}
