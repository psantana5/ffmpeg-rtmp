# FFmpeg RTMP Power Monitoring
[![Docker Build](https://github.com/psantana5/ffmpeg-rtmp/actions/workflows/ci.yml/badge.svg)](https://github.com/psantana5/ffmpeg-rtmp/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Python 3.10+](https://img.shields.io/badge/python-3.10+-blue.svg)](https://www.python.org/downloads/)
[![Go 1.21+](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/docker-optional-blue.svg)](https://www.docker.com/)
[![Code style: ruff](https://img.shields.io/badge/code%20style-ruff-000000.svg)](https://github.com/astral-sh/ruff)

A comprehensive streaming test and power monitoring stack for analyzing energy consumption during video transcoding. Features **high-performance Go exporters**, **VictoriaMetrics** for production-grade telemetry, and **distributed compute capabilities** for scaling workloads across multiple nodes.

**Production deployment uses master-agent architecture (no Docker required). Docker Compose available for local development only.**

## Quick Start (Production - Distributed Mode)

The **recommended way** to deploy for production workloads is **Distributed Compute Mode** with master and agent nodes.

### Prerequisites

- **Go 1.21+** (for building binaries)
- Python 3.10+ (for agent analysis scripts)
- FFmpeg (for transcoding)
- Linux with kernel 4.15+ (for RAPL power monitoring)

### Deploy Master Node

```bash
# Clone and build
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
make build-master

# Set API key (required for production)
export MASTER_API_KEY=$(openssl rand -base64 32)

# Start master service with production defaults
# TLS enabled (auto-generates cert)
# SQLite persistence (master.db)
# Job retry (3 attempts)
# Prometheus metrics (:9090)
./bin/master --port 8080 &

# Start monitoring stack (VictoriaMetrics + Grafana)
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

# Register and start agent (uses HTTPS with TLS)
./bin/agent --register --master https://MASTER_IP:8080 --api-key "$MASTER_API_KEY"
```

### Submit and Run Job

```bash
# Submit job to master (requires API key)
curl -X POST https://MASTER_IP:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "1080p-test",
    "confidence": "auto",
    "parameters": {"duration": 300, "bitrate": "5000k"}
  }'

# Agent automatically picks up and executes job
# Failed jobs auto-retry up to 3 times
```

### Access Dashboards

- **Grafana**: http://MASTER_IP:3000 (admin/admin)
- **VictoriaMetrics**: http://MASTER_IP:8428
- **Master API**: https://MASTER_IP:8080/nodes (view registered nodes)
- **Prometheus Metrics**: http://MASTER_IP:9090/metrics

### Production Deployment with Systemd

See [deployment/README.md](deployment/README.md) for systemd service templates and production setup.

---

## 🔬 Quick Start (Development - Local Testing Mode)

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
# Run a simple streaming test
python3 scripts/run_tests.py single --name "test1" --bitrate 2000k --duration 60

# View dashboards at http://localhost:3000
```

**See [docs/DEPLOYMENT_MODES.md](docs/DEPLOYMENT_MODES.md) for detailed comparison and setup instructions.**

## What's New: Production-Ready v2.2

**Distributed mode now production-ready with enterprise features:**

- **✅ TLS/HTTPS** - Enabled by default with auto-generated certificates
- **✅ API Authentication** - Required via `MASTER_API_KEY` environment variable
- **✅ SQLite Persistence** - Default storage, survives restarts
- **✅ Automatic Job Retry** - Failed jobs retry up to 3 times
- **✅ Prometheus Metrics** - Built-in metrics endpoint on port 9090
- **✅ Structured Logging** - Production-grade logging support

See [docs/PRODUCTION_FEATURES.md](docs/PRODUCTION_FEATURES.md) for complete feature guide.

## What's New: Go Exporters + VictoriaMetrics (v2.0)

This project now features **production-ready Go exporters** that have replaced Python exporters for all critical telemetry:

- **70%+ CPU reduction** vs Python exporters
- **1-second scrape granularity** with minimal jitter
- **VictoriaMetrics** as primary TSDB for 10x storage efficiency
- **30-day retention** by default (vs 7 days)
- **Zero missing metrics** under high load
- **ARM64 support** for edge deployment
- **FFmpeg stats exporter** for real-time encoding metrics
- **Automated benchmarking** with 4 workload profiles

See [CHANGELOG.md](CHANGELOG.md) for full v2.0 release notes.

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

See [docs/DEPLOYMENT_MODES.md](docs/DEPLOYMENT_MODES.md) for detailed comparison and architecture diagrams.

## Documentation

Documentation organized by topic:

### Deployment & Operations
- **[Production Features](docs/PRODUCTION_FEATURES.md)** - Production-ready features guide (TLS, auth, retry, metrics)
- **[Deployment Modes](docs/DEPLOYMENT_MODES.md)** - Production vs development deployment guide
- **[Internal Architecture](docs/INTERNAL_ARCHITECTURE.md)** - Complete runtime model and operations reference
- **[Distributed Architecture](docs/distributed_architecture_v1.md)** - Distributed compute details
- **[Production Deployment](deployment/README.md)** - Systemd service templates and setup
- **[Getting Started Guide](docs/getting-started.md)** - Initial setup walkthrough

### Development & Testing
- **[Running Tests](scripts/README.md)** - Test scenarios and batch execution
- **[Go Exporters Quick Start](docs/QUICKSTART_GO_EXPORTERS.md)** - One-command Go exporter deployment
- **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions

### Technical Reference
- **[Architecture Overview](docs/architecture.md)** - System design and data flow
- **[Exporters Overview](src/exporters/README.md)** - Metrics collectors
- **[Go Exporters Details](src/exporters/README_GO.md)** - Go exporter API and internals
- **[Energy Advisor](advisor/README.md)** - ML models and efficiency scoring
- **[Go Exporters Migration](docs/go-exporters-migration.md)** - Python to Go migration guide

## Common Commands

### Distributed Mode (Production)
```bash
# Build binaries
make build-master          # Build master node binary
make build-agent           # Build compute agent binary
make build-distributed     # Build both

# Run services
./bin/master --port 8080                        # Start master
./bin/agent --register --master http://MASTER_IP:8080  # Start agent

# Production with systemd
sudo systemctl start ffmpeg-master    # Start master service
sudo systemctl start ffmpeg-agent     # Start agent service
sudo systemctl status ffmpeg-master   # Check status

# Monitor
curl http://localhost:8080/nodes      # List registered agents
curl http://localhost:8080/jobs       # List jobs
journalctl -u ffmpeg-master -f        # View master logs
journalctl -u ffmpeg-agent -f         # View agent logs
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
# Submit multiple jobs to master
curl -X POST http://master:8080/jobs -H "Content-Type: application/json" -d '{
  "scenario": "4K-h265", "confidence": "auto",
  "parameters": {"duration": 3600, "bitrate": "15000k"}
}'

# Agents automatically pick up and execute jobs in parallel
# View results in Grafana at http://master:3000
```

### Development: Find Energy-Efficient Encoding Settings

Use local testing mode to iterate quickly:

```bash
# Start local stack
make up-build

# Run batch tests with different configurations
python3 scripts/run_tests.py batch --file batch_stress_matrix.json

# Analyze results and get recommendations
python3 scripts/analyze_results.py
```

The analyzer ranks configurations by energy efficiency and recommends optimal settings.

### Development: Compare H.264 vs H.265 Power Consumption

Create batch configuration testing codecs:

```bash
# Edit batch_stress_matrix.json with h264 and h265 scenarios
# Run tests locally
python3 scripts/run_tests.py batch --file codec_comparison.json

# Compare results in Grafana dashboards
```

### Production: Continuous CI/CD Benchmarking

Deploy distributed mode with agents on your build servers:

```bash
# CI/CD pipeline submits jobs to master after each release
curl -X POST http://master:8080/jobs -d @benchmark_config.json

# Results automatically aggregated and visualized
# Alerts fire if performance regressions detected
```

## Contributing

Contributions are welcome! See the detailed documentation for development guidelines.

## License

See [LICENSE](LICENSE) file for details.

## Quick Links

- [Full Documentation](docs/)
- [Test Runner Guide](scripts/README.md)
- [Exporter Documentation](src/exporters/README.md)
- [Energy Advisor](advisor/README.md)
