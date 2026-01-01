package tracing

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware creates an HTTP middleware that traces requests
func HTTPMiddleware(provider *Provider, serviceName string) func(http.Handler) http.Handler {
	tracer := provider.Tracer()
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from headers
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start span
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.host", r.Host),
					attribute.String("http.scheme", r.URL.Scheme),
					attribute.String("http.remote_addr", r.RemoteAddr),
					attribute.String("http.user_agent", r.Header.Get("User-Agent")),
				),
			)
			defer span.End()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Inject trace context into response headers
			propagator.Inject(ctx, propagation.HeaderCarrier(rw.Header()))

			// Call next handler
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Record response attributes
			span.SetAttributes(
				attribute.Int("http.status_code", rw.statusCode),
			)

			// Mark span as error if status code indicates failure
			if rw.statusCode >= 400 {
				span.SetAttributes(attribute.Bool("error", true))
			}
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// InjectHTTPHeaders injects trace context into HTTP request headers
func InjectHTTPHeaders(ctx context.Context, req *http.Request) {
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// ExtractHTTPHeaders extracts trace context from HTTP request headers
func ExtractHTTPHeaders(ctx context.Context, req *http.Request) context.Context {
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	return propagator.Extract(ctx, propagation.HeaderCarrier(req.Header))
}
