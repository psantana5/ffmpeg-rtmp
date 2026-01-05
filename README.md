# FFmpeg RTMP Power Monitoring
[![CI](https://github.com/psantana5/ffmpeg-rtmp/actions/workflows/ci.yml/badge.svg)](https://github.com/psantana5/ffmpeg-rtmp/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go 1.24+](https://img.shields.io/badge/go-1.24+-00ADD8.svg)](https://golang.org/)
[![Test Coverage](https://img.shields.io/badge/coverage-60%25-brightgreen.svg)](#testing)
[![Code Quality](https://img.shields.io/badge/code%20quality-A-success.svg)](#)

A comprehensive streaming test and power monitoring stack for analyzing energy consumption during video transcoding. Features **high-performance Go exporters**, **VictoriaMetrics** for production-grade telemetry, and **distributed compute capabilities** for scaling workloads across multiple nodes.
<img width="1658" height="1020" alt="image" src="https://github.com/user-attachments/assets/12e560b2-1d60-407d-b856-f7a80dcfd02c" />

**Production deployment uses master-agent architecture (no Docker required). Docker Compose available for local development only.**

## Project Organization

This project is organized into three main directories for clarity:

- **[`master/`](master/)** - Master node components (orchestration, monitoring, visualization)
- **[`worker/`](worker/)** - Worker node components (transcoding, hardware metrics)
- **[`shared/`](shared/)** - Shared libraries, scripts, and documentation

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for system architecture and design.

## Quick Start (Local Development)

**For local testing**, use the automated script to run both master and agent on your machine:

```bash
# One-command setup: builds, runs, and verifies everything
./scripts/run_local_stack.sh
```

This will compile all binaries, start master+agent, and display helpful commands. See [docs/LOCAL_STACK_GUIDE.md](docs/LOCAL_STACK_GUIDE.md) for details.

## Quick Start (Production - Distributed Mode)

The **recommended way** to deploy for production workloads is **Distributed Compute Mode** with master and agent nodes.

### Prerequisites

- **Go 1.24+** (for building binaries)
- Python 3.10+ (optional, for analysis scripts)
- FFmpeg (for transcoding)
- Linux with kernel 4.15+ (for RAPL power monitoring)

### Deploy Master Node

```bash
# Clone and build
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
make build-master

# Set API key (recommended for production)
export MASTER_API_KEY=$(openssl rand -base64 32)

# Start master service
# - TLS enabled by default (auto-generates self-signed cert)
# - SQLite persistence (master.db)
# - Job retry (3 attempts default)
# - Prometheus metrics on port 9090
./bin/master --port 8080 --api-key "$MASTER_API_KEY"

# Optional: Start monitoring stack (VictoriaMetrics + Grafana)
make vm-up-build
```

### Deploy Compute Agent(s)

```bash
# On compute node(s)
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
make build-agent

# Set same API key as master
export MASTER_API_KEY="<same-key-as-master>"

# Get optimal configuration for this hardware
./bin/ffrtmp config recommend --environment production

# Register and start agent
# Recommended: Use hardware-appropriate concurrency settings
./bin/agent \
  --register \
  --master https://MASTER_IP:8080 \
  --api-key "$MASTER_API_KEY" \
  --max-concurrent-jobs 4 \
  --poll-interval 3s \
  --insecure-skip-verify
  
# Note: --insecure-skip-verify only needed for self-signed certs
# For production with proper TLS, use --ca flag instead
```

### Submit and Run Jobs

```bash
# Using CLI tool (recommended)
./bin/ffrtmp jobs submit \
  --master https://MASTER_IP:8080 \
  --scenario 1080p-h264 \
  --bitrate 5M \
  --duration 300

# Using curl (for automation)
curl -X POST https://MASTER_IP:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "1080p-h264",
    "confidence": "auto",
    "parameters": {"duration": 300, "bitrate": "5M"}
  }'

# Agent automatically picks up and executes jobs
# Failed jobs auto-retry up to 3 times
# Monitor progress at https://MASTER_IP:8080/jobs
```

### Access Dashboards

- **Master API**: https://MASTER_IP:8080/nodes (view registered nodes)
- **Prometheus Metrics**: http://MASTER_IP:9090/metrics
- **Grafana** (if enabled): http://MASTER_IP:3000 (admin/admin)
- **VictoriaMetrics** (if enabled): http://MASTER_IP:8428

### Production Deployment with Systemd

See [deployment/README.md](deployment/README.md) for systemd service templates and production setup.

---

## Resource Management & Production Best Practices

### Running as Root (Recommended for Production)

**For production deployments, running the worker agent as root is strongly recommended** to enable full resource management capabilities:

```bash
# Run worker with full cgroup support (recommended)
sudo ./bin/agent \
  --register \
  --master https://MASTER_IP:8080 \
  --api-key "$MASTER_API_KEY" \
  --max-concurrent-jobs 4 \
  --poll-interval 3s
```

**Benefits of running as root:**
- ✅ **Full CPU limits**: Enforce per-job CPU quotas via cgroups
- ✅ **Memory limits**: Hard memory caps with OOM protection
- ✅ **Resource isolation**: Complete process isolation per job
- ✅ **Production stability**: Prevent resource exhaustion and runaway jobs

**Without root privileges**, the system gracefully falls back to:
- ✅ Disk space monitoring (always enforced)
- ✅ Timeout enforcement (always enforced)  
- ✅ Process priority control via nice (always enforced)
- ⚠️ CPU/Memory limits disabled (soft limits only)

### Resource Limits Per Job

Every job supports configurable resource limits to ensure system stability:

```json
{
  "scenario": "1080p-h264",
  "parameters": {
    "bitrate": "4M",
    "duration": 300
  },
  "resource_limits": {
    "max_cpu_percent": 200,      // 200% = 2 CPU cores
    "max_memory_mb": 2048,        // 2GB memory limit
    "max_disk_mb": 5000,          // 5GB temp space required
    "timeout_sec": 600            // 10 minute timeout
  }
}
```

**Default limits** (if not specified):
- **CPU**: All available cores (numCPU × 100%)
- **Memory**: 2048 MB (2GB)
- **Disk**: 5000 MB (5GB)
- **Timeout**: 3600 seconds (1 hour)

### Resource Management Features

**1. CPU Limits (cgroup-based)**
- Prevent jobs from monopolizing CPU resources
- Support for cgroup v1 and v2
- Automatic fallback to nice priority without root
- Per-job CPU percentage allocation (100% = 1 core)

**2. Memory Limits (cgroup-based)**
- Hard memory caps via Linux cgroups
- OOM (Out of Memory) protection
- Automatic process termination if limits exceeded
- Requires root for enforcement

**3. Disk Space Monitoring**
- Pre-job disk space validation
- Reject jobs at 95% disk usage
- Warn at 90% disk usage
- Always enforced (no root required)

**4. Timeout Enforcement**
- Per-job timeout configuration
- Context-based cancellation
- SIGTERM → SIGKILL escalation
- Process group cleanup

**5. Process Priority**
- Nice value = 10 (lower priority than system services)
- Prevents worker from impacting other services
- Always enforced (no root required)

**6. Bandwidth Tracking** 🆕
- Per-job input/output file size tracking
- Real-time bandwidth utilization metrics (Mbps)
- Cumulative bandwidth statistics per worker
- Compression ratio analysis
- Capacity planning support

**7. SLA Tracking** 🆕
- Automatic SLA compliance monitoring (default: 10 min per job)
- Success vs failure rate tracking
- Per-worker SLA compliance percentage
- Real-time SLA status in job logs
- Trend analysis and alerting

### Monitoring & Metrics

**Prometheus Metrics Endpoint**: `http://worker:9091/metrics`

Available metrics include:
- **Resource Usage**: CPU, memory, GPU utilization
- **Job Metrics**: Active jobs, completion rate, latency, success/failure counts
- **Hardware**: GPU power, temperature (NVIDIA)
- **Encoder Availability**: NVENC, QSV, VAAPI runtime validation
- **Bandwidth**: Input/output bytes, bandwidth utilization (Mbps) 🆕
- **SLA Tracking**: Compliance rate, SLA violations, job completion stats 🆕

**Documentation:**
- [Bandwidth Metrics Guide](docs/BANDWIDTH_METRICS.md) - Bandwidth tracking and capacity planning
- [SLA Tracking Guide](docs/SLA_TRACKING.md) - Service level agreement monitoring
- [Alerting Guide](docs/ALERTING.md) - Prometheus alerts and notification setup

**3. Disk Space Monitoring**
- Pre-job disk space validation
- Reject jobs if < 5% disk space available
- Warning alerts at 90% disk usage
- Automatic cleanup of temporary files

**4. Timeout Enforcement**
- Configurable per-job timeouts
- Process group cleanup (SIGTERM → SIGKILL)
- Prevents runaway jobs
- Always enforced (no special permissions needed)

**5. Process Priority Management**
- Lower priority (nice=10) for transcoding jobs
- System remains responsive under heavy load
- Automatic background prioritization

### Best Practices by Workload

**720p Fast Encoding:**
```json
"resource_limits": {
  "max_cpu_percent": 150,     // 1.5 cores
  "max_memory_mb": 1024,      // 1GB
  "timeout_sec": 300          // 5 minutes
}
```

**1080p Standard Encoding:**
```json
"resource_limits": {
  "max_cpu_percent": 300,     // 3 cores
  "max_memory_mb": 2048,      // 2GB
  "timeout_sec": 900          // 15 minutes
}
```

**4K High Quality Encoding:**
```json
"resource_limits": {
  "max_cpu_percent": 600,     // 6 cores
  "max_memory_mb": 4096,      // 4GB
  "timeout_sec": 3600         // 1 hour
}
```

### System Requirements for Resource Limits

**Minimum (without root):**
- Linux kernel 3.10+
- /tmp with 10GB+ free space
- 2GB+ RAM per worker

**Recommended (with root):**
- Linux kernel 4.5+ (cgroup v2 support)
- /tmp with 50GB+ free space  
- 8GB+ RAM per worker
- SSD storage for /tmp
- Root or sudo access

**For complete documentation**, see:
- **[Resource Limits Guide](docs/RESOURCE_LIMITS.md)** - Complete API and configuration reference
- **[Production Features](shared/docs/PRODUCTION_FEATURES.md)** - Additional production hardening
- **[Troubleshooting](shared/docs/troubleshooting.md)** - Common issues and solutions

---

## Quick Start (Development - Local Testing Mode)

For **development and local testing only**, you can use Docker Compose to run all components on a single machine.

**Important**: Docker Compose mode is **NOT recommended for production**. Use Distributed Mode above for production workloads.

### Prerequisites

- Docker 20.10+ and Docker Compose 2.0+
- Python 3.10+
- FFmpeg

### Start Local Stack

```bash
# Clone repository
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# Start all services
make up-build
```

### Run Local Test

```bash
# Build the CLI tool
make build-cli

# Submit jobs with different configurations
./bin/ffrtmp jobs submit --scenario 4K60-h264 --bitrate 10M --duration 120 --engine auto
./bin/ffrtmp jobs submit --scenario 1080p60-h265 --bitrate 5M --duration 60 --engine ffmpeg
./bin/ffrtmp jobs submit --scenario 720p30-h264 --bitrate 2M --duration 60 --engine gstreamer

# View dashboards at http://localhost:3000
```

**See [shared/docs/DEPLOYMENT_MODES.md](shared/docs/DEPLOYMENT_MODES.md) for detailed comparison and setup instructions.**

**For running exporters without Docker**, see:
- **[Exporters Quick Reference](docs/EXPORTERS_QUICK_REFERENCE.md)** - Quick commands and setup
- **[Master Exporters Guide](master/exporters/README.md)** - Detailed Python exporter deployment
- **[Worker Exporters Guide](worker/exporters/DEPLOYMENT.md)** - Detailed Go exporter deployment

## What's New: Production-Ready v2.3

**Latest enhancements for distributed transcoding:**

- **Concurrent Job Processing** - Workers can process multiple jobs simultaneously with `--max-concurrent-jobs`
- **Hardware-Aware Configuration** - Built-in tool recommends optimal settings: `ffrtmp config recommend`
- **Production Job Launcher** - Batch submit 1000s of jobs with `scripts/launch_jobs.sh`
- **TLS/HTTPS** - Enabled by default with auto-generated certificates
- **API Authentication** - Secure via `MASTER_API_KEY` environment variable
- **SQLite Persistence** - Jobs survive restarts with master.db
- **Automatic Job Retry** - Failed jobs retry up to 3 times with exponential backoff
- **Prometheus Metrics** - Built-in metrics endpoint on port 9090
- **Dual Engine Support** - Choose between FFmpeg and GStreamer based on workload

See [docs/README.md](docs/README.md) for comprehensive documentation.

## NEW: Enterprise-Grade Fault Tolerance

**Production-ready reliability features for mission-critical workloads:**

### Automatic Job Recovery
- **Node Failure Detection** - Identifies dead nodes based on heartbeat timeout (2min default)
- **Automatic Job Reassignment** - Jobs from failed nodes automatically reassigned to healthy workers
- **Transient Failure Retry** - Smart retry for connection errors, timeouts, network issues
- **Configurable Max Retries** - Default 3 attempts with exponential backoff
- **Stale Job Detection** - Batch jobs timeout after 30min, live jobs after 5min inactivity

### Priority Queue Management
- **Multi-Level Priorities** - Live > High > Medium > Low > Batch
- **Queue-Based Scheduling** - `live`, `default`, `batch` queues with different SLAs
- **FIFO Within Priority** - Fair scheduling for same-priority jobs
- **Smart Job Selection** - Automatic priority-based job assignment

### Observability
- **Distributed Tracing** - OpenTelemetry integration for end-to-end visibility
- **Prometheus Metrics** - Comprehensive metrics for jobs, nodes, and system health
- **Structured Logging** - Production-grade logging with context
- **Rate Limiting** - Built-in per-IP rate limiting (100 req/s default)

### Security
- **TLS/mTLS** - Mutual TLS authentication between master and workers
- **API Key Authentication** - Required for all API operations
- **Certificate Management** - Auto-generation and rotation support

```bash
# Submit high-priority live stream job
./bin/ffrtmp jobs submit \
    --scenario live-4k \
    --queue live \
    --priority high \
    --duration 3600 \
    --engine gstreamer

# Configure master with custom settings
./bin/master \
    --port 8080 \
    --max-retries 5 \
    --scheduler-interval 10s \
    --api-key "$MASTER_API_KEY"
    
# Configure agent with optimal concurrency
./bin/ffrtmp config recommend --environment production
./bin/agent \
    --master https://MASTER_IP:8080 \
    --max-concurrent-jobs 4 \
    --poll-interval 3s \
    --heartbeat-interval 30s
```

See [docs/README.md](docs/README.md) for complete production deployment guide.

## Dual Transcoding Engine Support

**Choose the best transcoding engine for your workload:**

- **FFmpeg** (default) - Versatile, mature, excellent for file transcoding
- **GStreamer** - Optimized for low-latency live streaming
- **Intelligent Auto-Selection** - System picks the best engine automatically
- **Hardware Acceleration** - NVIDIA NVENC, Intel QSV/VAAPI support for both engines

```bash
# Auto-select best engine (default)
./bin/ffrtmp jobs submit --scenario live-stream --engine auto

# Force specific engine
./bin/ffrtmp jobs submit --scenario transcode --engine ffmpeg
./bin/ffrtmp jobs submit --scenario live-rtmp --engine gstreamer
```

**Auto-selection logic:**
- LIVE queue → GStreamer (low latency)
- FILE/batch → FFmpeg (better for offline)
- RTMP streaming → GStreamer
- GPU+NVENC+streaming → GStreamer

See **[docs/DUAL_ENGINE_SUPPORT.md](docs/DUAL_ENGINE_SUPPORT.md)** for complete documentation.

## What This Project Does

This project helps you:

1. **Run FFmpeg streaming tests** with various configurations (bitrate, resolution, codec)
2. **Monitor power consumption** in real-time using Intel RAPL
3. **Collect system metrics** (CPU, memory, network, Docker overhead)
4. **Analyze energy efficiency** and get recommendations for optimal transcoding settings
5. **Visualize results** in Grafana dashboards
6. **Set up alerts** for power thresholds
7. **Scale workloads** across multiple compute nodes (NEW in v2.1)

## Architecture

The system supports two deployment modes:

### 1. Distributed Compute Mode (Production)

Master-agent architecture for scaling across multiple nodes:

- **Master Node**: Job orchestration, metrics aggregation, dashboards
  - Master Service (Go HTTP API)
  - VictoriaMetrics (TSDB with 30-day retention)
  - Grafana (visualization)
- **Compute Agents**: Execute transcoding workloads
  - Hardware auto-detection
  - Job polling and execution
  - Local metrics collection
  - Results reporting

### 2. Local Testing Mode (Development Only)

Docker Compose stack on single machine:

- **Nginx RTMP**: Streaming server
- **VictoriaMetrics**: Time-series database
- **Grafana**: Dashboards
- **Go Exporters**: CPU (RAPL), GPU (NVML), FFmpeg stats
- **Python Exporters**: QoE metrics, cost analysis, results tracking
- **Alertmanager**: Alert routing

**Local Testing mode is for development only. Use Distributed Compute mode for production.**

See [shared/docs/DEPLOYMENT_MODES.md](shared/docs/DEPLOYMENT_MODES.md) for detailed comparison and architecture diagrams.

## Documentation

**NEW: [Complete Documentation Guide](docs/README.md)** - Comprehensive reference with architecture, configuration, API, security, and troubleshooting

### Quick Reference
- **[Configuration Tool](docs/CONFIGURATION_TOOL.md)** - Hardware-aware worker configuration (CLI: `ffrtmp config recommend`)
- **[Concurrent Jobs Guide](CONCURRENT_JOBS_IMPLEMENTATION.md)** - Parallel job processing implementation
- **[Job Launcher Script](scripts/LAUNCH_JOBS_README.md)** - Production-grade batch job submission
- **[Deployment Success Report](DEPLOYMENT_SUCCESS.md)** - Real-world production deployment results

### Deployment & Operations
- **[Dual Engine Support](docs/DUAL_ENGINE_SUPPORT.md)** - FFmpeg + GStreamer engine selection guide
- **[Production Features](shared/docs/PRODUCTION_FEATURES.md)** - Production-ready features guide (TLS, auth, retry, metrics)
- **[Deployment Modes](shared/docs/DEPLOYMENT_MODES.md)** - Production vs development deployment guide
- **[Internal Architecture](shared/docs/INTERNAL_ARCHITECTURE.md)** - Complete runtime model and operations reference
- **[Distributed Architecture](shared/docs/distributed_architecture_v1.md)** - Distributed compute details
- **[Production Deployment](deployment/README.md)** - Systemd service templates and setup
- **[Getting Started Guide](shared/docs/getting-started.md)** - Initial setup walkthrough

### Development & Testing
- **[Running Tests](scripts/README.md)** - Test scenarios and batch execution
- **[Go Exporters Quick Start](shared/docs/QUICKSTART_GO_EXPORTERS.md)** - One-command Go exporter deployment
- **[Troubleshooting](shared/docs/troubleshooting.md)** - Common issues and solutions

### Technical Reference
- **[Architecture Overview](shared/docs/architecture.md)** - System design and data flow
- **[Exporters Quick Reference](docs/EXPORTERS_QUICK_REFERENCE.md)** - Quick commands for deploying exporters without Docker
- **[Exporters Overview](master/README.md#exporters)** - Master exporters (results, qoe, cost)
- **[Master Exporters Manual Deployment](master/exporters/README.md)** - Running master exporters without Docker
- **[Worker Exporters](worker/README.md#exporters)** - Worker exporters (CPU, GPU, FFmpeg)
- **[Worker Exporters Manual Deployment](worker/exporters/DEPLOYMENT.md)** - Running worker exporters without Docker
- **[Energy Advisor](shared/advisor/README.md)** - ML models and efficiency scoring
- **[Documentation Index](shared/docs/)** - All technical documentation

## Common Commands

### Distributed Mode (Production)
```bash
# Build binaries
make build-master          # Build master node binary
make build-agent           # Build compute agent binary
make build-cli             # Build ffrtmp CLI tool
make build-distributed     # Build all

# Get hardware-aware configuration recommendations
./bin/ffrtmp config recommend --environment production --output text

# Run services
./bin/master --port 8080 --api-key "$MASTER_API_KEY"
./bin/agent --register --master https://MASTER_IP:8080 \
  --api-key "$MASTER_API_KEY" \
  --max-concurrent-jobs 4 \
  --insecure-skip-verify

# Submit jobs using CLI
./bin/ffrtmp jobs submit --scenario 1080p-h264 --bitrate 5M --duration 300
./bin/ffrtmp jobs status <job-id>
./bin/ffrtmp nodes list

# Production with systemd
sudo systemctl start ffmpeg-master    # Start master service
sudo systemctl start ffmpeg-agent     # Start agent service
sudo systemctl status ffmpeg-master   # Check status

# Monitor
curl -k https://localhost:8080/nodes      # List registered agents
curl -k https://localhost:8080/jobs       # List jobs
curl http://localhost:9090/metrics        # Prometheus metrics
journalctl -u ffmpeg-master -f            # View master logs
journalctl -u ffmpeg-agent -f             # View agent logs
```

### Local Testing Mode (Development)
```bash
# Stack management
make up-build              # Start Docker Compose stack
make down                  # Stop stack
make ps                    # Show container status
make logs SERVICE=victoriametrics  # View specific service logs

# Testing (local mode)
make test-single           # Run single stream test
make test-batch            # Run batch test matrix
make run-benchmarks        # Run automated benchmark suite
make analyze               # Analyze latest results

# Development
make lint                  # Run code linting
make format                # Format code
make test                  # Run test suite
```

## Example Use Cases

### Production: Distributed Transcoding Benchmarks

Run long-duration benchmarks across multiple compute nodes:

```bash
# Submit multiple jobs using CLI
./bin/ffrtmp jobs submit --scenario 4K-h265 --bitrate 15M --duration 3600
./bin/ffrtmp jobs submit --scenario 1080p-h264 --bitrate 5M --duration 1800
./bin/ffrtmp jobs submit --scenario 720p-vp9 --bitrate 2M --duration 900

# Or submit in batch using curl with authentication
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "4K-h265",
    "confidence": "auto",
    "parameters": {"duration": 3600, "bitrate": "15M"}
  }'

# Agents automatically pick up and execute jobs in parallel
# Monitor at https://master:8080/jobs
# View results in Grafana at http://master:3000
```

### Development: Find Energy-Efficient Encoding Settings

Use local testing mode to iterate quickly:

```bash
# Start local stack
make up-build

# Build CLI tool
make build-cli

# Submit multiple test jobs with different configurations
./bin/ffrtmp jobs submit --scenario 4K60-h264 --bitrate 10M --duration 120 --engine auto
./bin/ffrtmp jobs submit --scenario 1080p60-h265 --bitrate 5M --duration 60 --engine ffmpeg
./bin/ffrtmp jobs submit --scenario 720p30-vp9 --bitrate 2M --duration 60 --engine gstreamer

# Monitor job status
./bin/ffrtmp jobs status <job-id>

# Analyze results and get recommendations
python3 scripts/analyze_results.py
```

The analyzer ranks configurations by energy efficiency and recommends optimal settings.

### Development: Compare H.264 vs H.265 Power Consumption

Submit jobs to test different codecs:

```bash
# H.264 tests
./bin/ffrtmp jobs submit --scenario 4K60-h264 --bitrate 10M --duration 120
./bin/ffrtmp jobs submit --scenario 1080p60-h264 --bitrate 5M --duration 60

# H.265 tests
./bin/ffrtmp jobs submit --scenario 4K60-h265 --bitrate 10M --duration 120
./bin/ffrtmp jobs submit --scenario 1080p60-h265 --bitrate 5M --duration 60

# Compare results in Grafana dashboards at http://localhost:3000
```

### Production: Continuous CI/CD Benchmarking

Deploy distributed mode with agents on your build servers:

```bash
# CI/CD pipeline submits jobs to master after each release
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d @benchmark_config.json

# Results automatically aggregated and visualized
# Alerts fire if performance regressions detected
```

## Contributing

Contributions are welcome! See the detailed documentation for development guidelines.

## License

See [LICENSE](LICENSE) file for details.

## Quick Links

- [Master Node Setup](master/README.md)
- [Worker Node Setup](worker/README.md)
- [Shared Components](shared/README.md)
- [Full Documentation](shared/docs/)
- [Scripts Documentation](shared/scripts/README.md)

## Testing

The project includes comprehensive test coverage for critical components:

```bash
# Run all tests with race detector
cd shared/pkg
go test -v -race ./...

# Run tests with coverage report
go test -v -coverprofile=coverage.out ./models ./scheduler ./store
go tool cover -html=coverage.out
```

**Test Coverage:**
- **models**: 85% (FSM state machine fully tested)
- **scheduler**: 53% (priority queues, recovery logic)
- **store**: Comprehensive database operations tests
- **agent**: Engine selection, optimizers, encoders

**CI/CD:**
- Automated testing on every push
- Race condition detection
- Multi-architecture builds (amd64, arm64)
- Binary artifacts for master, worker, and CLI

See [CONTRIBUTING.md](CONTRIBUTING.md) for testing guidelines.

## Documentation

Core documentation has been streamlined for clarity:

- **[docs/README.md](docs/README.md)** - Complete system documentation (NEW)
- **[docs/CONFIGURATION_TOOL.md](docs/CONFIGURATION_TOOL.md)** - Hardware-aware config tool
- **[CONCURRENT_JOBS_IMPLEMENTATION.md](CONCURRENT_JOBS_IMPLEMENTATION.md)** - Parallel processing guide
- **[QUICKSTART.md](QUICKSTART.md)** - Get started in 5 minutes
- **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - System design and architecture
- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Production deployment guide
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contribution guidelines
- **[docs/LOCAL_STACK_GUIDE.md](docs/LOCAL_STACK_GUIDE.md)** - Local development setup
- **[CHANGELOG.md](CHANGELOG.md)** - Version history

Additional technical documentation is available in `docs/archive/` for reference.
