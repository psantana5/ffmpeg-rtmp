# FFmpeg-RTMP Distributed Transcoding System

## Overview

A production-grade distributed system for real-time video transcoding and streaming using FFmpeg and GStreamer. The system implements intelligent job scheduling, concurrent processing, hardware optimization, and comprehensive monitoring.

## Project Status

**Version**: 1.0.0  
**Status**: Production Ready  
**Last Updated**: 2026-01-05

### Key Features

- **Distributed Architecture**: Master-worker topology with fault tolerance
- **Concurrent Processing**: Workers support multiple simultaneous jobs
- **Hardware Optimization**: Automatic detection and optimization for CPU/GPU
- **Multi-Engine Support**: FFmpeg and GStreamer with intelligent selection
- **Advanced Scheduling**: Priority queues, retry logic, and failure recovery
- **Comprehensive Monitoring**: Prometheus metrics with Grafana dashboards
- **Bandwidth Tracking**: Real-time HTTP bandwidth monitoring
- **TLS Security**: Full TLS support with optional mTLS
- **CLI Tools**: Complete command-line interface for job management

## Architecture

### Components

1. **Master Node** (`master/cmd/master`)
   - Job scheduling and distribution
   - Worker registration and health monitoring
   - HTTP API for job submission and status
   - SQLite/PostgreSQL persistence
   - Prometheus metrics exporter

2. **Worker Node** (`worker/cmd/agent`)
   - Job execution with FFmpeg/GStreamer
   - Automatic input video generation
   - Hardware capability detection
   - Concurrent job processing
   - Metrics and health reporting

3. **CLI Client** (`cmd/ffrtmp`)
   - Job submission and management
   - Node monitoring
   - Status queries and logs
   - Authentication support

4. **Monitoring Stack**
   - VictoriaMetrics for time-series storage
   - Grafana for visualization
   - Custom exporters for cost, QoE, results
   - AlertManager integration

### Technology Stack

- **Language**: Go 1.24
- **Database**: SQLite (default), PostgreSQL (optional)
- **Metrics**: Prometheus/VictoriaMetrics
- **Visualization**: Grafana
- **Encoding**: FFmpeg, GStreamer
- **Container**: Docker, Docker Compose

## Quick Start

### Prerequisites

- Go 1.24 or later
- FFmpeg 4.4+ with libx264, libx265
- GStreamer 1.20+ (optional)
- Docker and Docker Compose (for monitoring stack)

### Installation

```bash
# Clone repository
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# Build binaries
make build

# Generate TLS certificates (development)
./bin/master --generate-cert

# Start monitoring stack (optional)
docker compose up -d
```

### Running the System

#### 1. Start Master Node

```bash
./bin/master \
  --port 8080 \
  --db master.db \
  --tls \
  --cert certs/master.crt \
  --key certs/master.key \
  --metrics \
  --metrics-port 9090
```

#### 2. Start Worker Node

```bash
./bin/agent \
  --master https://localhost:8080 \
  --register \
  --insecure-skip-verify \
  --max-concurrent-jobs 4 \
  --metrics-port 9091
```

#### 3. Submit a Job

```bash
./bin/ffrtmp jobs submit \
  --scenario 720p30-h264 \
  --duration 120 \
  --bitrate 2M \
  --priority medium \
  --queue default
```

#### 4. Check Status

```bash
# List all jobs
./bin/ffrtmp jobs status

# List workers
./bin/ffrtmp nodes list

# Follow specific job
./bin/ffrtmp jobs status <job-id> --follow
```

## Configuration

### Master Node Options

| Flag | Description | Default |
|------|-------------|---------|
| `--port` | HTTP API port | 8080 |
| `--db` | SQLite database path | master.db |
| `--db-type` | Database type (sqlite/postgres) | sqlite |
| `--db-dsn` | PostgreSQL connection string | - |
| `--tls` | Enable TLS | true |
| `--cert` | TLS certificate file | certs/master.crt |
| `--key` | TLS key file | certs/master.key |
| `--mtls` | Require client certificates | false |
| `--api-key` | API authentication key | - |
| `--metrics` | Enable Prometheus metrics | true |
| `--metrics-port` | Metrics endpoint port | 9090 |
| `--max-retries` | Job retry limit | 3 |
| `--scheduler-interval` | Scheduler check interval | 5s |

### Worker Node Options

| Flag | Description | Default |
|------|-------------|---------|
| `--master` | Master node URL | http://localhost:8080 |
| `--register` | Register with master | false |
| `--max-concurrent-jobs` | Concurrent job limit | 1 |
| `--poll-interval` | Job polling interval | 10s |
| `--heartbeat-interval` | Heartbeat interval | 30s |
| `--metrics-port` | Metrics endpoint port | 9091 |
| `--generate-input` | Auto-generate input videos | true |
| `--api-key` | Authentication key | - |
| `--cert` | TLS client certificate | - |
| `--key` | TLS client key | - |
| `--ca` | CA certificate file | - |
| `--insecure-skip-verify` | Skip TLS verification | false |

## Performance Tuning

### Concurrent Jobs

Workers support processing multiple jobs simultaneously. The optimal value depends on available CPU cores:

- **Laptop (4-8 cores)**: 2-3 concurrent jobs
- **Desktop (8-16 cores)**: 4-6 concurrent jobs
- **Server (16+ cores)**: 8-12 concurrent jobs

Example:
```bash
./bin/agent --max-concurrent-jobs 4
```

### Hardware Optimization

The system automatically detects and optimizes for available hardware:

- **CPU-only**: Uses libx264/libx265 with appropriate presets
- **NVIDIA GPU**: Attempts h264_nvenc/hevc_nvenc with fallback
- **Intel QSV**: Attempts h264_qsv/hevc_qsv with fallback
- **AMD VAAPI**: Attempts h264_vaapi/hevc_vaapi with fallback

### Codec Selection

For CPU-only systems, use lightweight codecs:

- **VP8/VP9**: Good CPU efficiency, modern codec support
- **Low-resolution H.264**: 720p or below, suitable for CPU encoding
- **Avoid**: 4K H.265 on CPU (very slow)

## Monitoring and Metrics

### Prometheus Endpoints

- Master: `http://localhost:9090/metrics`
- Worker: `http://localhost:9091/metrics`

### Key Metrics

**Master Metrics**:
- `ffrtmp_jobs_total` - Total jobs by status
- `ffrtmp_jobs_completed_by_engine` - Completions by engine
- `scheduler_http_bandwidth_bytes_total` - Total bandwidth
- `scheduler_jobs_by_queue` - Jobs per queue type
- `scheduler_nodes_total` - Worker node count

**Worker Metrics**:
- `ffrtmp_worker_active_jobs` - Currently processing jobs
- `ffrtmp_worker_jobs_total` - Total jobs handled
- `ffrtmp_worker_heartbeat_total` - Heartbeat count
- `ffrtmp_worker_job_duration_seconds` - Job execution time

### Grafana Dashboards

Access Grafana at `http://localhost:3000` (if using docker-compose):

- **Distributed Job Scheduler**: Main dashboard with job status, bandwidth, and worker health
- **Energy Efficiency**: Power consumption and cost analysis
- **Node Performance**: Per-worker resource utilization

## API Reference

### Job Submission

```bash
POST /jobs
Content-Type: application/json

{
  "scenario": "720p30-h264",
  "confidence": "auto",
  "engine": "ffmpeg",
  "queue": "default",
  "priority": "medium",
  "parameters": {
    "duration": 120,
    "bitrate": "2M"
  }
}
```

### Job Status

```bash
GET /jobs
GET /jobs/{id}
```

### Worker Registration

```bash
POST /nodes/register
Content-Type: application/json

{
  "name": "worker-01",
  "type": "server",
  "cpu_threads": 16,
  "cpu_model": "Intel Xeon",
  "has_gpu": true,
  "gpu_type": "NVIDIA RTX 3090",
  "ram_total_bytes": 34359738368
}
```

### Health Check

```bash
GET /health
```

## Security

### TLS Configuration

Generate self-signed certificates for development:

```bash
./bin/master --generate-cert \
  --cert-hosts localhost,master.local \
  --cert-ips 127.0.0.1,192.168.1.100
```

For production, use proper certificates from a CA.

### API Authentication

Set an API key to require authentication:

```bash
# Master
export MASTER_API_KEY="your-secure-random-key"
./bin/master

# Worker
./bin/agent --api-key "your-secure-random-key"

# CLI
export FFMPEG_RTMP_API_KEY="your-secure-random-key"
./bin/ffrtmp jobs submit ...
```

### mTLS (Mutual TLS)

Require client certificates for enhanced security:

```bash
# Master
./bin/master --mtls --ca ca.crt

# Worker
./bin/agent --cert worker.crt --key worker.key --ca ca.crt
```

## Troubleshooting

### Worker Registration Fails

**Error**: `401 Missing Authorization header`

**Solution**: Ensure master and worker have matching authentication:
- Check for environment variable `MASTER_API_KEY`
- Use `--api-key` flag on worker
- Or disable auth on master (development only)

### GStreamer Jobs Fail

**Error**: `gst-launch-1.0 execution failed: exit status 1`

**Solution**: Force FFmpeg engine for affected jobs:
```bash
./bin/ffrtmp jobs submit --scenario 720p30-h264 --engine ffmpeg
```

### High CPU Usage

**Issue**: Worker consuming 100% CPU

**Solution**: Reduce concurrent jobs:
```bash
./bin/agent --max-concurrent-jobs 2
```

### Jobs Queue But Don't Process

**Check**:
1. Worker registered: `./bin/ffrtmp nodes list`
2. Worker logs: Check for errors in worker output
3. Job status: Verify jobs are in "queued" not "failed" state

## Development

### Building from Source

```bash
# Build all binaries
make build

# Build specific component
go build -o bin/master ./master/cmd/master
go build -o bin/agent ./worker/cmd/agent
go build -o bin/ffrtmp ./cmd/ffrtmp

# Run tests
make test

# Run with race detector
go test -race ./...
```

### Project Structure

```
ffmpeg-rtmp/
├── master/              # Master node implementation
│   ├── cmd/master/      # Master binary
│   ├── exporters/       # Custom Prometheus exporters
│   └── monitoring/      # Grafana dashboards
├── worker/              # Worker node implementation
│   ├── cmd/agent/       # Worker binary
│   └── exporters/       # Worker-specific exporters
├── cmd/ffrtmp/          # CLI client
├── shared/              # Shared libraries
│   └── pkg/
│       ├── models/      # Data models
│       ├── store/       # Database layer
│       ├── scheduler/   # Job scheduling
│       └── agent/       # Worker logic
├── scripts/             # Utility scripts
└── docs/                # Documentation
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines.

## License

See [LICENSE](LICENSE) for details.

## Support

For issues and questions:
- GitHub Issues: https://github.com/psantana5/ffmpeg-rtmp/issues
- Documentation: https://github.com/psantana5/ffmpeg-rtmp/tree/main/docs

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history and release notes.
