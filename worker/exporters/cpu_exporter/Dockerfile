# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
COPY shared/pkg/go.mod shared/pkg/go.sum* ./shared/pkg/

# Download dependencies
RUN go mod download

# Copy source code
COPY worker/exporters/cpu_exporter/ ./worker/exporters/cpu_exporter/
COPY shared/pkg/ ./shared/pkg/

# Build the exporter as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o cpu_exporter ./worker/exporters/cpu_exporter/

# Runtime stage - use scratch for minimal image
FROM scratch

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/cpu_exporter .

EXPOSE 9500

ENTRYPOINT ["/app/cpu_exporter"]
