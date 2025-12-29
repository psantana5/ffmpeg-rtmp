# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src/exporters/cpu_exporter/ ./src/exporters/cpu_exporter/

# Build the exporter
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o cpu_exporter ./src/exporters/cpu_exporter/

# Runtime stage
FROM alpine:3.19

# Add ca-certificates for HTTPS support
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/cpu_exporter .

# Run as non-root user (but needs privileged for RAPL access)
USER nobody

EXPOSE 9500

ENTRYPOINT ["/app/cpu_exporter"]
