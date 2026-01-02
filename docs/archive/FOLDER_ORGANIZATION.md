# Folder Organization Guide

This document explains the logical separation of components in the FFmpeg RTMP Power Monitoring system based on whether they run on the **master node** or **worker nodes**.

## Directory Structure Overview

```
ffmpeg-rtmp/
├── master/                    # Master node components (orchestration, monitoring, visualization)
│   ├── cmd/                   # Master binary entry point
│   ├── exporters/             # Master-side exporters (results, qoe, cost aggregation)
│   ├── monitoring/            # VictoriaMetrics, Grafana, Alertmanager configs
│   └── deployment/            # Master-specific deployment configs
│
├── worker/                    # Worker/Agent node components (transcoding workloads)
│   ├── cmd/                   # Agent binary entry point
│   ├── exporters/             # Worker-side exporters (CPU, GPU, FFmpeg stats)
│   └── deployment/            # Worker-specific deployment configs
│
├── shared/                    # Shared components used by both master and workers
│   ├── pkg/                   # Shared Go packages (API, models, auth, etc.)
│   ├── scripts/               # Shared scripts (test runners, analyzers)
│   └── docs/                  # Documentation
│
└── [root files]               # Top-level configs (Makefile, README, docker-compose, etc.)
```

## Component Classification

### Master Node Components

**Purpose**: Orchestration, job queuing, results aggregation, metrics visualization

**Components**:
- `cmd/master/` → `master/cmd/` - Master HTTP service binary
- `src/exporters/results/` → `master/exporters/results/` - Test results exporter
- `src/exporters/qoe/` → `master/exporters/qoe/` - Quality of Experience metrics
- `src/exporters/cost/` → `master/exporters/cost/` - Cost calculation metrics
- `src/exporters/health_checker/` → `master/exporters/health_checker/` - Exporter health monitoring
- `grafana/` → `master/monitoring/grafana/` - Grafana dashboards and provisioning
- `alertmanager/` → `master/monitoring/alertmanager/` - Alert configuration
- `victoriametrics.yml` → `master/monitoring/victoriametrics.yml` - Metrics scrape config
- `alert-rules.yml` → `master/monitoring/alert-rules.yml` - Alert rules
- `deployment/ffmpeg-master.service` → `master/deployment/ffmpeg-master.service` - Master systemd service

**Why Master?**:
- These components aggregate data from all workers
- They provide centralized visualization and alerting
- They run only on the master node in production

### Worker Node Components

**Purpose**: Execute transcoding workloads, collect local metrics

**Components**:
- `cmd/agent/` → `worker/cmd/` - Agent binary (worker service)
- `src/exporters/cpu_exporter/` → `worker/exporters/cpu_exporter/` - CPU power monitoring (RAPL)
- `src/exporters/gpu_exporter/` → `worker/exporters/gpu_exporter/` - GPU power monitoring (NVML)
- `src/exporters/ffmpeg_exporter/` → `worker/exporters/ffmpeg_exporter/` - FFmpeg stats
- `src/exporters/docker_stats/` → `worker/exporters/docker_stats/` - Docker container stats
- `deployment/ffmpeg-agent.service` → `worker/deployment/ffmpeg-agent.service` - Worker systemd service

**Why Worker?**:
- These components collect metrics during job execution
- They monitor local hardware (CPU, GPU, memory)
- They run on each compute node that executes transcoding jobs

### Shared Components

**Purpose**: Common functionality used by both master and workers

**Components**:
- `pkg/` → `shared/pkg/` - Shared Go packages
  - `pkg/api/` - HTTP API definitions and handlers
  - `pkg/models/` - Data models (Job, Node, Result, etc.)
  - `pkg/auth/` - Authentication middleware
  - `pkg/agent/` - Agent logic (hardware detection, job execution)
  - `pkg/store/` - Storage interfaces (SQLite, in-memory)
  - `pkg/tls/` - TLS utilities
  - `pkg/metrics/` - Prometheus metrics utilities
  - `pkg/logging/` - Structured logging
- `scripts/` → `shared/scripts/` - Test runners, analyzers, benchmark scripts
- `advisor/` → `shared/advisor/` - ML models for efficiency scoring
- `models/` → `shared/models/` - Trained ML model files
- `docs/` → `shared/docs/` - All documentation

**Why Shared?**:
- Code reused by both master and worker binaries
- Common data models ensure API compatibility
- Scripts can be run from either master or worker for testing

### Development-Only Components

**Purpose**: Local development and testing (not used in production distributed mode)

**Components** (remain at root):
- `docker-compose.yml` - Local testing stack (all components on one machine)
- `nginx.conf` - RTMP server for local testing
- `Makefile` - Build and deployment commands
- Root-level docs (`README.md`, `CONTRIBUTING.md`, etc.)
- Test configurations (`batch_stress_matrix.json`, etc.)

**Why Root?**:
- Docker Compose is development-only, runs everything locally
- Makefile orchestrates both master and worker builds
- Root README is the entry point for all users

## Migration Benefits

### 1. **Clarity**
- Developers immediately understand which components run where
- New contributors can quickly find relevant code
- Documentation references are clearer

### 2. **Simplified Deployment**
- Master-only deployments don't need worker exporter dependencies
- Worker-only deployments don't need visualization stack
- Smaller Docker images and binaries

### 3. **Better Maintenance**
- Changes to worker exporters don't affect master
- Master monitoring updates don't require worker rebuilds
- Easier to test components in isolation

### 4. **Scalability**
- Workers can be deployed independently
- Master can be updated without worker downtime
- Shared packages ensure compatibility

## Deployment Scenarios

### Scenario 1: Production Distributed Mode

**Master Node** (e.g., 192.168.1.100):
```bash
# Deploy only master components
cd master/
make deploy-master
# Starts: master HTTP service, VictoriaMetrics, Grafana, Alertmanager
```

**Worker Nodes** (e.g., 192.168.1.101-110):
```bash
# Deploy only worker components
cd worker/
make deploy-worker MASTER_URL=https://192.168.1.100:8080
# Starts: agent binary, CPU/GPU exporters, FFmpeg exporter
```

### Scenario 2: Development Mode (Docker Compose)

**Single Machine**:
```bash
# Use root docker-compose.yml - runs everything locally
make up-build
# Starts: All master + worker components on localhost
```

### Scenario 3: Hybrid Mode

**Master on cloud, workers on-premise**:
```bash
# Master (AWS)
cd master/ && make deploy-master

# Workers (on-prem data center)
cd worker/ && make deploy-worker MASTER_URL=https://master.cloud.example.com:8080
```

## Implementation Plan

### Phase 1: Create New Structure ✅
- [x] Create `master/`, `worker/`, `shared/` directories
- [ ] Document the organization (this file)

### Phase 2: Move Components
- [ ] Move master components to `master/`
- [ ] Move worker components to `worker/`
- [ ] Move shared components to `shared/`
- [ ] Create symlinks at root for backward compatibility (optional)

### Phase 3: Update Build System
- [ ] Update Makefile with new paths
- [ ] Update docker-compose.yml with new paths
- [ ] Update Go module imports
- [ ] Update systemd service files

### Phase 4: Update Documentation
- [ ] Update README.md with new structure
- [ ] Update deployment guides
- [ ] Update architecture diagrams
- [ ] Add migration guide for existing deployments

### Phase 5: Validation
- [ ] Test master build: `make build-master`
- [ ] Test agent build: `make build-agent`
- [ ] Test docker-compose: `make up-build`
- [ ] Run integration tests: `./test_distributed.sh`
- [ ] Verify systemd deployments

## Backward Compatibility

To maintain backward compatibility during transition:

1. **Keep root-level docker-compose.yml** - Development mode unchanged
2. **Keep Makefile at root** - Build commands work as before
3. **Update import paths gradually** - Use Go module replace directives
4. **Document migration** - Provide clear upgrade path for existing deployments

## Questions & Decisions

### Why not `agent/` instead of `worker/`?
- **Decision**: Use `worker/` in folder name for clarity
- The binary is still called `agent`, but the folder represents "worker node components"
- Aligns with "master-worker" architectural pattern
- "Agent" is the software, "Worker" is the role

### Why keep docker-compose.yml at root?
- **Decision**: Docker Compose is for development only
- Keeps the quick-start experience simple
- Production uses master/worker separation, not Docker Compose
- Root Makefile orchestrates everything

### What about exporters used by both?
- **Decision**: Classify by primary use case
- Results/QoE/Cost exporters aggregate worker data → Master
- CPU/GPU/FFmpeg exporters collect local metrics → Worker
- If truly shared, put in `shared/exporters/` (currently none)

## Related Documentation

- [README.md](../README.md) - Main project documentation
- [DEPLOYMENT_MODES.md](docs/DEPLOYMENT_MODES.md) - Production vs development deployment
- [distributed_architecture_v1.md](docs/distributed_architecture_v1.md) - Distributed system architecture
- [deployment/README.md](deployment/README.md) - Systemd deployment guide

## Questions?

If you're unsure where a component belongs:

1. **Does it aggregate data from multiple nodes?** → Master
2. **Does it collect local hardware metrics?** → Worker
3. **Is it used by both master and worker binaries?** → Shared
4. **Is it development/testing only?** → Root

For questions or suggestions, open an issue on GitHub.
