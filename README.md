# FFmpeg-RTMP: A Production-Validated Reference System

[![CI](https://github.com/psantana5/ffmpeg-rtmp/actions/workflows/ci.yml/badge.svg)](https://github.com/psantana5/ffmpeg-rtmp/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go 1.24+](https://img.shields.io/badge/go-1.24+-00ADD8.svg)](https://golang.org/)

A distributed video transcoding system documenting architectural patterns, design invariants, and failure semantics observed under real load. This reference implementation demonstrates master-worker coordination, state machine guarantees, and operational tradeoffs in production environments.

<img width="1899" height="963" alt="image" src="https://github.com/user-attachments/assets/a7434728-cefd-43c3-8940-1b6d1f7e4c52" />

## About This Reference System

**FFmpeg-RTMP documents architectural choices, invariants, and failure semantics observed under real load.** While the system is used in production and is available for reuse, its primary goal is to communicate design tradeoffs and operational lessons rather than to serve as a general-purpose or commercially supported platform.

### Research Goals

This reference implementation demonstrates:

- **Architectural patterns**: Pull-based coordination, state machine guarantees, idempotent operations
- **Design invariants**: What never changes, even under failure conditions
- **Failure semantics**: Explicit documentation of retry boundaries and terminal states
- **Operational tradeoffs**: Why certain design choices were made over alternatives
- **Performance characteristics**: Measured behavior under realistic workloads (45,000+ jobs tested)

### Key Contributions

1. **State Machine Correctness**: FSM with validated transitions and row-level locking prevents race conditions
2. **Failure Mode Documentation**: Explicit boundaries between transient (retry) and terminal (fail) errors
3. **Graceful Degradation**: Heartbeat-based failure detection with configurable recovery semantics
4. **Production Patterns**: Exponential backoff, connection pooling, graceful shutdown demonstrated at scale
5. **Transparency**: Design decisions documented with rationale and alternatives considered

### What This Is NOT

- **Not a commercial platform**: No support, SLAs, or stability guarantees across versions
- **Not general-purpose**: Optimized for batch transcoding workloads, not real-time streaming
- **Not plug-and-play**: Requires understanding of distributed systems concepts for deployment
- **Not feature-complete**: Focuses on core patterns; many production features deliberately omitted

### Intended Audience

- Systems researchers studying distributed coordination patterns
- Engineers evaluating architectural approaches for similar problems
- Students learning production distributed systems design
- Teams seeking a reference implementation to adapt for specific use cases

**This is a teaching tool backed by real operational data, not a turnkey solution.**

## Project Organization

This reference implementation is organized to clearly separate concerns:

- **[`master/`](master/)** - Orchestration: job scheduling, failure detection, state management
- **[`worker/`](worker/)** - Execution: job processing, FFmpeg integration, metrics collection
- **[`shared/`](shared/)** - Common libraries: FSM, retry semantics, database abstractions

See [ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed design discussion and [CODE_VERIFICATION_REPORT.md](CODE_VERIFICATION_REPORT.md) for implementation validation.

## Running the Reference Implementation

### Local Development Environment

For studying the system behavior locally:

```bash
# One-command setup: builds, runs, and verifies everything
./scripts/run_local_stack.sh
```

See [docs/LOCAL_STACK_GUIDE.md](docs/LOCAL_STACK_GUIDE.md) for details.

## Distributed Deployment (Research/Production Use)

The reference implementation can be deployed across multiple nodes to study distributed behavior patterns.

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

# Set API key for authentication
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

### Deploy Worker Node(s)

```bash
# On worker node(s)
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
make build-agent

# Set same API key as master
export MASTER_API_KEY="<same-key-as-master>"

# Register and start agent
# Concurrency settings affect failure mode behavior
./bin/agent \
  --register \
  --master https://MASTER_IP:8080 \
  --api-key "$MASTER_API_KEY" \
  --max-concurrent-jobs 4 \
  --poll-interval 3s \
  --insecure-skip-verify
  
# Note: --insecure-skip-verify only for self-signed certs in research environments
```

### Submit Jobs and Observe Behavior

```bash
# Submit via CLI
./bin/ffrtmp jobs submit \
  --master https://MASTER_IP:8080 \
  --scenario 1080p-h264 \
  --bitrate 5M \
  --duration 300

# Or via REST API
curl -X POST https://MASTER_IP:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "1080p-h264",
    "confidence": "auto",
    "parameters": {"duration": 300, "bitrate": "5M"}
  }'

# Workers poll master and execute jobs
# Observe state transitions and failure recovery patterns
# Monitor progress at https://MASTER_IP:8080/jobs
```

### Observability Endpoints

- **Master API**: https://MASTER_IP:8080/nodes (registered nodes, health status)
- **Prometheus Metrics**: http://MASTER_IP:9090/metrics
- **Grafana** (optional): http://MASTER_IP:3000 (admin/admin)
- **VictoriaMetrics** (optional): http://MASTER_IP:8428

For systemd service configuration, see [deployment/README.md](deployment/README.md).

---

## Experimental: Edge Workload Wrapper

The Edge Workload Wrapper demonstrates OS-level resource constraint patterns for compute workloads. This experimental component explores non-owning governance models where workloads survive wrapper crashes.

### Design Patterns Demonstrated

- **Non-owning supervision**: Workloads run independently of wrapper lifecycle
- **Attach semantics**: Govern already-running processes without restart
- **Graceful fallback**: OS-level constraints degrade gracefully without root/cgroups
- **Exit tracking**: Capture exit codes, reasons, and execution duration

### Example Usage

```bash
# Run FFmpeg with resource constraints
ffrtmp run \
  --job-id transcode-001 \
  --sla-eligible \
  --cpu-quota 200 \
  --memory-limit 4096 \
  -- ffmpeg -i input.mp4 -c:v h264_nvenc output.mp4

# Attach to existing process (demonstrates attach semantics)
ffrtmp attach \
  --pid 12345 \
  --job-id existing-job-042 \
  --cpu-weight 150 \
  --nice -5

# Auto-discovery watch daemon (NEW!)
ffrtmp watch \
  --scan-interval 10s \
  --enable-state \
  --enable-retry \
  --watch-config /etc/ffrtmp/watch-config.yaml
```

### Auto-Discovery Watch Daemon (Experimental)

Demonstrates automatic process discovery and governance patterns. Explores techniques for:
- Non-intrusive process discovery via /proc scanning
- State persistence across daemon restarts
- Configuration-driven process filtering and governance

**Example deployment**:
```bash
# Install experimental daemon
sudo ./deployment/install-edge.sh

# Configure discovery rules
sudo nano /etc/ffrtmp/watch-config.yaml

# Start service
sudo systemctl start ffrtmp-watch
```

See [deployment/WATCH_DEPLOYMENT.md](deployment/WATCH_DEPLOYMENT.md) for implementation details.

### Wrapper Documentation

- **[Wrapper Architecture](docs/WRAPPER_ARCHITECTURE.md)** - Design patterns and philosophy
- **[Wrapper Examples](docs/WRAPPER_EXAMPLES.md)** - Usage demonstrations

---

## System Architecture and Design Patterns

### Resource Management Patterns

The reference implementation demonstrates several resource management approaches:

**Running with privileged access** (research/production):

```bash
# Full cgroup support for resource isolation
sudo ./bin/agent \
  --register \
  --master https://MASTER_IP:8080 \
  --api-key "$MASTER_API_KEY" \
  --max-concurrent-jobs 4 \
  --poll-interval 3s
```

**Benefits of privileged execution:**
- Strict CPU quotas via cgroups (v1/v2)
- Hard memory limits with OOM protection
- Complete process isolation per job
- Resource exhaustion prevention

**Graceful degradation without privileges:**
- Disk space monitoring (always enforced)
- Timeout enforcement (always enforced)
- Process priority control via nice
- CPU/memory limits disabled (monitoring only)

### Per-Job Resource Constraints

Jobs support configurable limits for studying resource contention:

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

**Default constraints**:
- **CPU**: All available cores (numCPU × 100%)
- **Memory**: 2048 MB (2GB)
- **Disk**: 5000 MB (5GB)
- **Timeout**: 3600 seconds (1 hour)

### Resource Isolation Techniques

**1. CPU Limits (cgroup-based)**
- Demonstrates per-job CPU percentage allocation (100% = 1 core)
- Supports cgroup v1 and v2
- Fallback to nice priority without root

**2. Memory Limits (cgroup-based)**
- Hard memory caps via Linux cgroups
- OOM (Out of Memory) protection
- Automatic process termination if limits exceeded
- Requires root for enforcement

**3. Disk Space Monitoring**
**2. Memory Limits (cgroup-based)**
- Hard memory caps with OOM protection
- Automatic fallback to monitoring without enforcement (no root)

**3. Disk Space Monitoring**
- Pre-job validation (reject at 95% usage)
- Always enforced (no privileges required)
- Configurable cleanup policies for temporary files

**4. Timeout Enforcement**
- Per-job timeout with context-based cancellation
- SIGTERM → SIGKILL escalation
- Process group cleanup

**5. Process Priority**
- Nice value = 10 (lower than system services)
- Always enforced (no privileges required)

### Observability and Metrics

The system exports Prometheus metrics demonstrating:

- **Resource usage patterns**: CPU, memory, GPU utilization per job
- **Job lifecycle**: Active jobs, completion rates, latency distribution
- **Hardware monitoring**: GPU power, temperature (NVIDIA)
- **Encoder availability**: NVENC, QSV, VAAPI runtime detection
- **Bandwidth tracking**: Input/output bytes, compression ratios
- **SLA classification**: Intelligent job categorization (production vs test/debug)

**Metrics endpoint**: `http://worker:9091/metrics`

**Documentation:**
- [Auto-Attach Documentation](docs/AUTO_ATTACH.md) - Process discovery patterns
- [Bandwidth Metrics Guide](docs/BANDWIDTH_METRICS.md) - Bandwidth tracking implementation
- [SLA Tracking Guide](docs/SLA_TRACKING.md) - Service level monitoring approach
- [SLA Classification Guide](docs/SLA_CLASSIFICATION.md) - Job classification methodology (99.8% compliance with 45K+ jobs)
- [Alerting Guide](docs/ALERTING.md) - Prometheus alert configuration

### Measured Performance Characteristics

**Test Results (45,000+ jobs across 31 scenarios):**
- 99.8% SLA compliance observed
- Automatic retry recovers transient failures (network errors, node failures)
- FFmpeg failures terminal (codec errors, format issues)
- Heartbeat-based failure detection (90s timeout, 3 missed heartbeats)

See [CODE_VERIFICATION_REPORT.md](CODE_VERIFICATION_REPORT.md) for implementation validation and [docs/SLA_CLASSIFICATION.md](docs/SLA_CLASSIFICATION.md) for complete testing methodology.

### Configuration Examples by Workload Type

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
**System requirements:**
- Linux kernel 3.10+
- /tmp with 10GB+ free space
- 2GB+ RAM per worker

**Recommended (with privileged access):**
- Linux kernel 4.5+ (cgroup v2 support)
- /tmp with 50GB+ free space
- 8GB+ RAM per worker
- SSD storage for /tmp

**Additional documentation:**
- **[Resource Limits Guide](docs/RESOURCE_LIMITS.md)** - Configuration reference
- **[Production Features](shared/docs/PRODUCTION_FEATURES.md)** - Additional hardening patterns
- **[Troubleshooting](shared/docs/troubleshooting.md)** - Common issues

---

## Local Development Mode

For development and experimentation, Docker Compose provides a single-machine setup:

```bash
# Clone and start
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
make up-build

# Submit test jobs
make build-cli
./bin/ffrtmp jobs submit --scenario 1080p-h264 --bitrate 5M --duration 60

# View metrics at http://localhost:3000
```

**Note**: Docker Compose is for local testing only. For distributed deployment, see above.

See [shared/docs/DEPLOYMENT_MODES.md](shared/docs/DEPLOYMENT_MODES.md) for deployment comparisons.

## Key Design Patterns Demonstrated

### Version 2.4 (2026-01-06): Production Reliability Patterns

**Retry Semantics**:
- Transport-layer retry only (HTTP requests, heartbeats, polling)
- Exponential backoff: 1s → 30s, max 3 retries
- Context-aware (respects cancellation)
- Job execution never retried (FFmpeg failures terminal)

**Graceful Shutdown**:
- Worker: Stop accepting jobs, drain current jobs (30s timeout)
- Master: LIFO shutdown order (HTTP → metrics → scheduler → DB → logger)
- No workload interruption (jobs complete naturally or timeout)
- Async coordination via `shutdown.Done()` channel

**Readiness Checks**:
- FFmpeg validation before accepting work
- Disk space verification
- Master connectivity check
- HTTP 200 only when truly ready (Kubernetes-friendly)

**Centralized Logging**:
- Structured directory: `/var/log/ffrtmp/<component>/<subcomponent>.log`
- Multi-writer: file + stdout (systemd journald compatible)
- Automatic fallback to `./logs/` without privileges

**Documentation:**
- [Production Readiness Guide](docs/PRODUCTION_READINESS.md) - Complete pattern documentation
- [Security Review](docs/SECURITY_REVIEW.md) - Security audit
- [Audit Summary](AUDIT_COMPLETE.md) - Technical debt elimination

### Version 2.3: Distributed Coordination Patterns

**Concurrency**:
- Workers process multiple jobs simultaneously (`--max-concurrent-jobs`)
- Hardware-aware configuration tool: `ffrtmp config recommend`

**Reliability**:
- TLS/HTTPS enabled by default (auto-generated certificates)
- API authentication via `MASTER_API_KEY`
- SQLite persistence (jobs survive restarts)
- Automatic retry with exponential backoff

**Observability**:
- Built-in Prometheus metrics (port 9090)
- Dual engine support (FFmpeg/GStreamer)

See [docs/README.md](docs/README.md) for comprehensive documentation.

## Fault Tolerance Implementation

### Job Recovery Patterns

**Failure Detection**:
- Heartbeat-based (90s timeout, 3 missed heartbeats)
- Identifies dead nodes and orphaned jobs

**Automatic Reassignment**:
- Jobs from failed workers automatically reassigned
- Smart retry for transient failures (network errors, timeouts)
- FFmpeg failures terminal (not retried)
- Max 3 retry attempts with exponential backoff

**Stale Job Handling**:
- Batch jobs timeout after 30min
- Live jobs timeout after 5min inactivity

### Priority Queue Implementation

**Multi-level priorities**: Live > High > Medium > Low > Batch

**Queue-based scheduling**: `live`, `default`, `batch` queues with different SLAs

**FIFO within priority**: Fair scheduling for same-priority jobs

### Security Patterns

- TLS/mTLS between master and workers
- API key authentication required
- Certificate auto-generation support

```bash
# Example: Submit high-priority job
./bin/ffrtmp jobs submit \
    --scenario live-4k \
    --queue live \
    --priority high \
    --duration 3600

# Configure master
./bin/master \
    --port 8080 \
    --max-retries 5 \
    --scheduler-interval 10s \
    --api-key "$MASTER_API_KEY"
    
# Configure worker
./bin/agent \
    --master https://MASTER_IP:8080 \
    --max-concurrent-jobs 4 \
    --poll-interval 3s \
    --heartbeat-interval 30s
```

See [docs/README.md](docs/README.md) for complete implementation details.

## Dual Engine Support (FFmpeg + GStreamer)

Demonstrates engine selection patterns for different workload characteristics:

- **FFmpeg** (default): General-purpose file transcoding
- **GStreamer**: Low-latency live streaming
- **Auto-selection**: System chooses based on workload type
- **Hardware acceleration**: NVENC, QSV, VAAPI support for both

```bash
# Auto-select engine (default)
./bin/ffrtmp jobs submit --scenario live-stream --engine auto

# Force specific engine
./bin/ffrtmp jobs submit --scenario transcode --engine ffmpeg
./bin/ffrtmp jobs submit --scenario live-rtmp --engine gstreamer
```

**Auto-selection logic**:
- LIVE queue → GStreamer (low latency)
- FILE/batch → FFmpeg (better for offline)
- RTMP streaming → GStreamer
- GPU+NVENC+streaming → GStreamer

See [docs/DUAL_ENGINE_SUPPORT.md](docs/DUAL_ENGINE_SUPPORT.md) for details.

## Research Applications

This reference system can be used to study:

1. **Distributed coordination**: Master-worker patterns, state machine guarantees, failure detection
2. **Resource management**: CPU/memory limits, cgroup isolation, graceful degradation
3. **Retry semantics**: Transient vs terminal failures, exponential backoff, idempotent operations
4. **Observability patterns**: Metrics collection, distributed tracing, structured logging
5. **Energy efficiency**: Power consumption during video transcoding (Intel RAPL)
6. **Workload scaling**: Performance characteristics across multiple nodes

## System Architecture

### Distributed Deployment (Primary Use Case)

Master-worker architecture demonstrating coordination patterns:

- **Master Node**: Job orchestration, failure detection, metrics aggregation
  - HTTP API (Go)
  - VictoriaMetrics (30-day retention)
  - Grafana (visualization)
- **Worker Nodes**: Job execution, resource monitoring, heartbeat reporting
  - Hardware auto-detection
  - Pull-based job polling
  - Local metrics collection
  - Result reporting

### Local Development (Single Machine)

Docker Compose stack for experimentation:

- Nginx RTMP (streaming server)
- VictoriaMetrics (time-series database)
- Grafana (dashboards)
- Go Exporters (CPU/GPU metrics via RAPL/NVML)
- Python Exporters (QoE metrics, analysis)
- Alertmanager (alert routing)

See [shared/docs/DEPLOYMENT_MODES.md](shared/docs/DEPLOYMENT_MODES.md) for architecture diagrams.

## Documentation Index

**Primary documentation**: [docs/README.md](docs/README.md) - Complete reference guide

### Implementation Guides
- **[Configuration Tool](docs/CONFIGURATION_TOOL.md)** - Hardware-aware worker configuration
- **[Concurrent Jobs Guide](CONCURRENT_JOBS_IMPLEMENTATION.md)** - Parallel job processing
- **[Job Launcher Script](scripts/LAUNCH_JOBS_README.md)** - Batch job submission
### Academic Publications
- **[Deployment Success Report](DEPLOYMENT_SUCCESS.md)** - Real-world deployment case study

### Implementation Details
- **[Dual Engine Support](docs/DUAL_ENGINE_SUPPORT.md)** - FFmpeg + GStreamer selection patterns
- **[Production Features](shared/docs/PRODUCTION_FEATURES.md)** - Reliability patterns (TLS, auth, retry, metrics)
- **[Deployment Modes](shared/docs/DEPLOYMENT_MODES.md)** - Architecture comparison
- **[Internal Architecture](shared/docs/INTERNAL_ARCHITECTURE.md)** - Runtime model and operations
- **[Distributed Architecture](shared/docs/distributed_architecture_v1.md)** - Master-worker coordination
- **[Production Deployment](deployment/README.md)** - Systemd service configuration
- **[Getting Started Guide](shared/docs/getting-started.md)** - Initial setup

### Testing and Validation
- **[Running Tests](scripts/README.md)** - Test scenarios and execution
- **[Go Exporters Quick Start](shared/docs/QUICKSTART_GO_EXPORTERS.md)** - Metrics collection setup
- **[Troubleshooting](shared/docs/troubleshooting.md)** - Common issues

### Technical Reference
- **[Architecture Overview](shared/docs/architecture.md)** - System design and data flow
- **[Exporters Quick Reference](docs/EXPORTERS_QUICK_REFERENCE.md)** - Metrics collection patterns
- **[Exporters Overview](master/README.md#exporters)** - Master-side metrics
- **[Master Exporters Deployment](master/exporters/README.md)** - Master metrics setup
- **[Worker Exporters](worker/README.md#exporters)** - Worker-side metrics
- **[Worker Exporters Deployment](worker/exporters/DEPLOYMENT.md)** - Worker metrics setup
- **[Energy Advisor](shared/advisor/README.md)** - ML-based efficiency analysis
- **[Documentation Index](shared/docs/)** - Complete technical documentation

## Command Reference

### Distributed Deployment Commands
```bash
# Build components
make build-master          # Build master node binary
make build-agent           # Build worker agent binary
make build-cli             # Build ffrtmp CLI tool
make build-distributed     # Build all

# Get hardware-aware configuration
./bin/ffrtmp config recommend --environment production --output text

# Run services
./bin/master --port 8080 --api-key "$MASTER_API_KEY"
./bin/agent --register --master https://MASTER_IP:8080 \
  --api-key "$MASTER_API_KEY" \
  --max-concurrent-jobs 4 \
  --insecure-skip-verify

# Submit and manage jobs
./bin/ffrtmp jobs submit --scenario 1080p-h264 --bitrate 5M --duration 300
./bin/ffrtmp jobs status <job-id>
./bin/ffrtmp nodes list

# Systemd service management
sudo systemctl start ffmpeg-master
sudo systemctl start ffmpeg-agent
sudo systemctl status ffmpeg-master

# Monitor and observe
curl -k https://localhost:8080/nodes      # List registered workers
curl -k https://localhost:8080/jobs       # List jobs
curl http://localhost:9090/metrics        # Prometheus metrics
journalctl -u ffmpeg-master -f            # View master logs
journalctl -u ffmpeg-agent -f             # View worker logs
```

### Local Development Commands
```bash
# Stack management
make up-build              # Start Docker Compose stack
make down                  # Stop stack
make ps                    # Show container status
make logs SERVICE=victoriametrics  # View service logs

# Testing scenarios
make test-single           # Run single stream test
make test-batch            # Run batch test matrix
make run-benchmarks        # Run benchmark suite
make analyze               # Analyze results

# Development tools
make lint                  # Run linting
make format                # Format code
make test                  # Run test suite
```

## Example Research Scenarios

### Scenario 1: Studying Distributed Failure Recovery

Observe job reassignment after worker failure:

```bash
# Submit long-running jobs
./bin/ffrtmp jobs submit --scenario 4K-h265 --bitrate 15M --duration 3600
./bin/ffrtmp jobs submit --scenario 1080p-h264 --bitrate 5M --duration 1800

# Monitor initial assignment
curl -k https://master:8080/jobs

# Kill a worker mid-job (simulate failure)
sudo systemctl stop ffmpeg-agent  # On worker node

# Observe master detecting failure (90s timeout)
# Watch job reassignment to healthy workers
curl -k https://master:8080/jobs  # Check job state transitions

# Analyze recovery time and behavior
journalctl -u ffmpeg-master -f
```

**Observations to study**:
- Heartbeat failure detection timing (3 × 30s = 90s)
- Job state transitions (running → failed → queued)
- Reassignment latency
- Worker re-registration behavior

### Scenario 2: Analyzing Resource Isolation Effectiveness

Test cgroup-based resource limits under contention:

```bash
# Submit multiple jobs with different CPU limits
./bin/ffrtmp jobs submit --scenario 1080p-h264 --duration 600 \
  --cpu-limit 200   # 2 cores

./bin/ffrtmp jobs submit --scenario 1080p-h264 --duration 600 \
  --cpu-limit 100   # 1 core

# Monitor actual CPU usage via Prometheus metrics
curl http://worker:9091/metrics | grep cpu_usage

# Compare observed vs requested CPU allocation
# Study cgroup enforcement effectiveness
```

### Scenario 3: Energy Efficiency Analysis

Compare codec energy consumption patterns:

```bash
# Start local development stack
make up-build && make build-cli

# Test H.264 codec
./bin/ffrtmp jobs submit --scenario 4K60-h264 --bitrate 10M --duration 120
./bin/ffrtmp jobs submit --scenario 1080p60-h264 --bitrate 5M --duration 60

# Test H.265 codec
./bin/ffrtmp jobs submit --scenario 4K60-h265 --bitrate 10M --duration 120
./bin/ffrtmp jobs submit --scenario 1080p60-h265 --bitrate 5M --duration 60

# Analyze energy consumption via RAPL metrics
python3 scripts/analyze_results.py

# View power consumption dashboards
# Open Grafana at http://localhost:3000
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
