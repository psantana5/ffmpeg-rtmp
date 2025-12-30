# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
COPY shared/pkg/go.mod shared/pkg/go.sum* ./shared/pkg/

# Download dependencies
RUN go mod download

# Copy source code
COPY worker/exporters/gpu_exporter/ ./worker/exporters/gpu_exporter/
COPY shared/pkg/ ./shared/pkg/

# Build the exporter as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o gpu_exporter ./worker/exporters/gpu_exporter/

# Runtime stage - use NVIDIA CUDA base for nvidia-smi
FROM nvidia/cuda:12.3.1-base-ubuntu22.04

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/gpu_exporter .

EXPOSE 9505

ENTRYPOINT ["/app/gpu_exporter"]
