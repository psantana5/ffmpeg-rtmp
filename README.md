# FFmpeg RTMP Power Monitoring
[![Docker Build](https://github.com/psantana5/ffmpeg-rtmp/actions/workflows/ci.yml/badge.svg)](https://github.com/psantana5/ffmpeg-rtmp/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Python 3.10+](https://img.shields.io/badge/python-3.10+-blue.svg)](https://www.python.org/downloads/)
[![Go 1.21+](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/docker-required-blue.svg)](https://www.docker.com/)
[![Code style: ruff](https://img.shields.io/badge/code%20style-ruff-000000.svg)](https://github.com/astral-sh/ruff)

A comprehensive streaming test and power monitoring stack for analyzing energy consumption during video transcoding. Features **high-performance Go exporters**, **VictoriaMetrics** for production-grade telemetry, and **distributed compute capabilities** for scaling workloads across multiple nodes.

## ⚡ Quick Start

### Prerequisites

- **Docker + Docker Compose** (required)
- Python 3.11+ (for test automation)
- FFmpeg (for running tests)
- Intel CPU with RAPL support (for power monitoring)

**Note**: Go installation is **NOT required** - exporters build inside Docker automatically.

### Start the Stack

```bash
# IMPORTANT: If upgrading from v1.x, clean up old containers first
docker compose down -v

# Build and start all services (Go exporters build automatically)
make up-build

# Or manually
docker compose up -d --build
```

**Note for v2.0 upgrade**: The `docker compose down -v` command removes old containers (prometheus, rapl-exporter) and volumes. This is required for a clean migration to VictoriaMetrics.

### Access the Dashboards

- **Grafana**: http://localhost:3000 (admin/admin)
- **VictoriaMetrics**: http://localhost:8428 (primary TSDB with 30-day retention)
- **Alertmanager**: http://localhost:9093

### Run Your First Test

```bash
# Run a simple streaming test
python3 scripts/run_tests.py single --name "test1" --bitrate 2000k --duration 60

# Analyze the results
python3 scripts/analyze_results.py
```

## 🚀 What's New: Go Exporters + VictoriaMetrics (v2.0)

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

The stack includes:

- **Nginx RTMP**: Streaming server for RTMP ingest
- **VictoriaMetrics**: Production-grade time-series database (30-day retention, primary TSDB)
- **Grafana**: Visualization dashboards including Benchmark History
- **Go Exporters**: 
  - CPU power monitoring (RAPL)
  - GPU metrics (NVML/nvidia-smi)
  - FFmpeg encoding stats (encoder load, dropped frames, bitrate, latency)
- **Python Exporters**: QoE metrics, Cost analysis, Results tracking
- **Energy Advisor**: ML-based recommendations for optimal configurations
- **Benchmark Automation**: 4 workload profiles for performance testing
- **Distributed Compute**: Master-worker architecture for scaling across multiple nodes (NEW in v2.1)

## Documentation

Detailed documentation is organized by topic:

- **[Distributed Architecture](docs/distributed_architecture_v1.md)** - 🆕 Scale workloads across multiple nodes
- **[Go Exporters Quick Start](docs/QUICKSTART_GO_EXPORTERS.md)** - ⚡ One-command deployment
- **[Go Exporters Migration Guide](docs/go-exporters-migration.md)** - Python to Go migration
- **[Getting Started Guide](docs/getting-started.md)** - Complete setup and first steps
- **[Running Tests](scripts/README.md)** - How to run different test scenarios
- **[Exporters](src/exporters/README.md)** - Understanding the metrics collectors
- **[Go Exporters](src/exporters/README_GO.md)** - Go exporter details and API
- **[Energy Advisor](advisor/README.md)** - ML models and efficiency scoring
- **[Architecture](docs/architecture.md)** - System design and data flow
- **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions

## Common Commands

```bash
# Stack management
make up-build          # Start stack with rebuild
make down              # Stop stack
make ps                # Show container status
make logs SERVICE=prometheus  # View logs

# Distributed compute
make build-distributed # Build master and agent binaries
./bin/master          # Start master node
./bin/agent --help    # See agent options

# Testing
make test-single       # Run single stream test
make test-batch        # Run batch test matrix
make run-benchmarks    # Run automated benchmark suite
make analyze           # Analyze latest results

# Development
make lint              # Run code linting
make format            # Format code
make test              # Run test suite
```

## Example Use Cases

### Find the Most Energy-Efficient Bitrate

```bash
python3 scripts/run_tests.py batch --file batch_stress_matrix.json
python3 scripts/analyze_results.py
```

The analyzer will rank all configurations by energy efficiency and recommend the best settings for your hardware.

### Compare H.264 vs H.265 Power Consumption

Create a batch configuration testing both codecs at the same bitrates, run the tests, and compare results in Grafana.

### Monitor Production Streaming Power Usage

Set up the stack on your streaming server and configure Prometheus alerts to notify you when power consumption exceeds thresholds.

## Contributing

Contributions are welcome! See the detailed documentation for development guidelines.

## License

See [LICENSE](LICENSE) file for details.

## Quick Links

- [Full Documentation](docs/)
- [Test Runner Guide](scripts/README.md)
- [Exporter Documentation](src/exporters/README.md)
- [Energy Advisor](advisor/README.md)
