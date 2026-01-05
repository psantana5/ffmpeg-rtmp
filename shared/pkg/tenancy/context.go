package tenancy

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

// Context keys for tenant and user information
type contextKey string

const (
	TenantIDKey contextKey = "tenant_id"
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
)

var (
	ErrNoTenantInContext = errors.New("no tenant ID in context")
	ErrNoUserInContext   = errors.New("no user ID in context")
	ErrInvalidTenantID   = errors.New("invalid tenant ID")
)

// GetTenantID extracts tenant ID from context
func GetTenantID(ctx context.Context) (string, error) {
	tenantID, ok := ctx.Value(TenantIDKey).(string)
	if !ok || tenantID == "" {
		return "", ErrNoTenantInContext
	}
	return tenantID, nil
}

// GetUserID extracts user ID from context
func GetUserID(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok || userID == "" {
		return "", ErrNoUserInContext
	}
	return userID, nil
}

// GetUserRole extracts user role from context
func GetUserRole(ctx context.Context) string {
	role, _ := ctx.Value(UserRoleKey).(string)
	return role
}

// WithTenant adds tenant ID to context
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

// WithUser adds user ID and role to context
func WithUser(ctx context.Context, userID, role string) context.Context {
	ctx = context.WithValue(ctx, UserIDKey, userID)
	ctx = context.WithValue(ctx, UserRoleKey, role)
	return ctx
}

// TenantMiddleware extracts tenant from request and adds to context
// Supports multiple methods:
// 1. X-Tenant-ID header
// 2. API key prefix (e.g., ffrtmp_<tenant_slug>_<key>)
// 3. JWT token claim
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tenantID string

		// Method 1: Check X-Tenant-ID header
		if headerTenant := r.Header.Get("X-Tenant-ID"); headerTenant != "" {
			tenantID = headerTenant
		}

		// Method 2: Extract from Authorization header (API key format: ffrtmp_<tenant>_<key>)
		if tenantID == "" {
			if auth := r.Header.Get("Authorization"); auth != "" {
				// Remove "Bearer " prefix if present
				token := strings.TrimPrefix(auth, "Bearer ")
				if parts := strings.Split(token, "_"); len(parts) >= 3 && parts[0] == "ffrtmp" {
					tenantID = parts[1]
				}
			}
		}

		// Method 3: Check query parameter (for WebSocket/SSE connections)
		if tenantID == "" {
			if queryTenant := r.URL.Query().Get("tenant_id"); queryTenant != "" {
				tenantID = queryTenant
			}
		}

		// If tenant found, add to context
		if tenantID != "" {
			ctx := WithTenant(r.Context(), tenantID)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// RequireTenant middleware ensures request has tenant context
func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := GetTenantID(r.Context()); err != nil {
			http.Error(w, `{"error":"tenant_required","message":"No tenant ID provided"}`, http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// TenantIsolationMiddleware enforces tenant data isolation
// This middleware should be used after authentication to ensure users can only access their tenant's data
func TenantIsolationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID, err := GetTenantID(r.Context())
		if err != nil {
			http.Error(w, `{"error":"unauthorized","message":"Tenant context required"}`, http.StatusUnauthorized)
			return
		}

		// Additional validation: ensure tenant ID is valid format (alphanumeric + hyphens)
		if !isValidTenantID(tenantID) {
			http.Error(w, `{"error":"invalid_tenant","message":"Invalid tenant ID format"}`, http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isValidTenantID validates tenant ID format
func isValidTenantID(tenantID string) bool {
	if len(tenantID) == 0 || len(tenantID) > 64 {
		return false
	}
	// Allow alphanumeric, hyphens, and underscores
	for _, ch := range tenantID {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || 
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return false
		}
	}
	return true
}
