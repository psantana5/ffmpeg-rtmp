# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
COPY shared/pkg/go.mod shared/pkg/go.sum* ./shared/pkg/

# Download dependencies
RUN go mod download

# Copy source code
COPY master/exporters/results_go/ ./master/exporters/results_go/
COPY shared/pkg/ ./shared/pkg/

# Build the exporter as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o results_exporter ./master/exporters/results_go/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/results_exporter .

EXPOSE 9502

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:${RESULTS_EXPORTER_PORT:-9502}/health || exit 1

ENTRYPOINT ["/app/results_exporter"]
