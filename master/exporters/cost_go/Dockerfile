# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
COPY shared/pkg/go.mod shared/pkg/go.sum* ./shared/pkg/

# Download dependencies
RUN go mod download

# Copy source code
COPY master/exporters/cost_go/ ./master/exporters/cost_go/
COPY shared/pkg/ ./shared/pkg/

# Build the exporter as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" \
    -o cost_exporter ./master/exporters/cost_go/

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/cost_exporter .

EXPOSE 9504

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -q -O - http://localhost:9504/health || exit 1

ENTRYPOINT ["/app/cost_exporter"]
