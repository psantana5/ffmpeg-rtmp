# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
COPY shared/pkg/go.mod shared/pkg/go.sum* ./shared/pkg/

# Download dependencies
RUN go mod download

# Copy source code
COPY master/exporters/qoe_go/ ./master/exporters/qoe_go/
COPY shared/pkg/ ./shared/pkg/

# Build the exporter as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o qoe_exporter ./master/exporters/qoe_go/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/qoe_exporter .

EXPOSE 9503

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:${QOE_EXPORTER_PORT:-9503}/health || exit 1

ENTRYPOINT ["/app/qoe_exporter"]
