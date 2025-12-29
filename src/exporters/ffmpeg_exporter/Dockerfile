# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src/exporters/ffmpeg_exporter/ ./src/exporters/ffmpeg_exporter/

# Build the exporter as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o ffmpeg_exporter ./src/exporters/ffmpeg_exporter/

# Runtime stage - use alpine for process monitoring tools
FROM alpine:latest

# Install procps for pgrep command
RUN apk add --no-cache procps

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/ffmpeg_exporter .

EXPOSE 9506

ENTRYPOINT ["/app/ffmpeg_exporter"]
