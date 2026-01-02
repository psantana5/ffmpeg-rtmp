package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

type contextKey string

const TenantContextKey contextKey = "tenant_id"

// TenantMiddleware validates and injects tenant context
func TenantMiddleware(s store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip tenant validation for certain endpoints
			path := r.URL.Path
			if path == "/health" || path == "/metrics" || path == "/api/v1/tenants" {
				next.ServeHTTP(w, r)
				return
			}

			// Extract tenant ID from X-Tenant-ID header
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				// For backward compatibility, allow requests without tenant ID
				// to use a default tenant (can be configured)
				tenantID = "default"
			}

			// Validate tenant exists and is active
			tenant, err := s.GetTenant(tenantID)
			if err != nil {
				log.Printf("Invalid tenant %s: %v", tenantID, err)
				http.Error(w, fmt.Sprintf("Invalid tenant: %v", err), http.StatusUnauthorized)
				return
			}

			if !tenant.IsActive {
				log.Printf("Tenant %s is inactive", tenantID)
				http.Error(w, "Tenant is inactive", http.StatusForbidden)
				return
			}

			// Inject tenant ID into request context
			ctx := context.WithValue(r.Context(), TenantContextKey, tenant.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetTenantID extracts tenant ID from request context
func GetTenantID(r *http.Request) string {
	if tenantID, ok := r.Context().Value(TenantContextKey).(string); ok {
		return tenantID
	}
	return ""
}

// RequireTenant ensures a tenant ID is present in the context
func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := GetTenantID(r)
		if tenantID == "" {
			http.Error(w, "Tenant ID required", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}
