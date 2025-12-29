# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src/exporters/gpu_exporter/ ./src/exporters/gpu_exporter/

# Build the exporter
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o gpu_exporter ./src/exporters/gpu_exporter/

# Runtime stage
FROM nvidia/cuda:12.3.1-base-ubuntu22.04

# Install nvidia-smi (part of CUDA driver)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/gpu_exporter .

EXPOSE 9505

ENTRYPOINT ["/app/gpu_exporter"]
