# Build stage for Rust ML library
FROM rust:1.83-alpine AS rust-builder

RUN apk --no-cache add musl-dev

WORKDIR /build

# Copy Rust library source
COPY ml_rust/ ./ml_rust/

# Build Rust library with musl target
RUN cd ml_rust && \
    rustup target add x86_64-unknown-linux-musl && \
    cargo build --release --target x86_64-unknown-linux-musl

# Build stage for Go exporter
FROM golang:1.24-alpine AS go-builder

RUN apk --no-cache add ca-certificates git musl-dev gcc

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
COPY shared/pkg/go.mod shared/pkg/go.sum* ./shared/pkg/

# Download dependencies
RUN go mod download

# Copy Rust library from previous stage (use static library for musl)
COPY --from=rust-builder /build/ml_rust/target/x86_64-unknown-linux-musl/release/libffmpeg_ml.a /usr/local/lib/

# Copy source code
COPY master/exporters/ml_predictions/ ./master/exporters/ml_predictions/
COPY shared/pkg/ ./shared/pkg/

# Build the exporter with CGO enabled
ENV CGO_ENABLED=1
ENV CGO_LDFLAGS="-L/usr/local/lib -lffmpeg_ml"
RUN cd master/exporters/ml_predictions && \
    go build -a -installsuffix cgo -ldflags="-w -s" \
    -o ml_exporter .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl libgcc

WORKDIR /app

# Copy binary from builder
COPY --from=go-builder /build/master/exporters/ml_predictions/ml_exporter .

EXPOSE 9505

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:${ML_EXPORTER_PORT:-9505}/health || exit 1

ENTRYPOINT ["/app/ml_exporter"]
