# Build stage
FROM golang:1.24-alpine AS builder

RUN apk --no-cache add ca-certificates git

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
COPY shared/pkg/go.mod shared/pkg/go.sum* ./shared/pkg/

# Download dependencies
RUN go mod download

# Copy source code
COPY worker/exporters/docker_stats_go/ ./worker/exporters/docker_stats_go/
COPY shared/pkg/ ./shared/pkg/

# Build the exporter as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o docker_stats_exporter ./worker/exporters/docker_stats_go/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/docker_stats_exporter .

EXPOSE 9501

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:${DOCKER_STATS_PORT:-9501}/health || exit 1

ENTRYPOINT ["/app/docker_stats_exporter"]
