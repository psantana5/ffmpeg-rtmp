# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src/exporters/gpu_exporter/ ./src/exporters/gpu_exporter/

# Build the exporter as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o gpu_exporter ./src/exporters/gpu_exporter/

# Runtime stage - use NVIDIA CUDA base for nvidia-smi
FROM nvidia/cuda:12.3.1-base-ubuntu22.04

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/gpu_exporter .

EXPOSE 9505

ENTRYPOINT ["/app/gpu_exporter"]
