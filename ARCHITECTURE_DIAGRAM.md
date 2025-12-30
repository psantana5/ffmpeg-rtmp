# Master/Worker Folder Architecture

This document provides a visual representation of the master/worker separation in the project structure.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         FFMPEG-RTMP PROJECT                             │
│                                                                         │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐    │
│  │  MASTER NODE     │  │  WORKER NODE     │  │  SHARED          │    │
│  │  COMPONENTS      │  │  COMPONENTS      │  │  COMPONENTS      │    │
│  │                  │  │                  │  │                  │    │
│  │  • Orchestration │  │  • Job Execution │  │  • Go Packages   │    │
│  │  • Job Queue     │  │  • HW Monitoring │  │  • Data Models   │    │
│  │  • Visualization │  │  • FFmpeg Run    │  │  • Scripts       │    │
│  │  • Aggregation   │  │  • Result Report │  │  • ML Models     │    │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘    │
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                    DEVELOPMENT TOOLS (Root)                       │ │
│  │  docker-compose.yml | Makefile | README.md | Config Files        │ │
│  └───────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

## Detailed Directory Mapping

### Before (Current Structure)
```
ffmpeg-rtmp/
├── cmd/
│   ├── master/                 (Master binary)
│   └── agent/                  (Worker binary)
├── pkg/                        (Shared packages)
├── src/exporters/              (ALL exporters mixed together)
│   ├── results/                → Used by master
│   ├── qoe/                    → Used by master
│   ├── cost/                   → Used by master
│   ├── cpu_exporter/           → Used by worker
│   ├── gpu_exporter/           → Used by worker
│   ├── ffmpeg_exporter/        → Used by worker
│   ├── docker_stats/           → Used by worker
│   └── health_checker/         → Used by master
├── grafana/                    (Master monitoring)
├── alertmanager/               (Master monitoring)
├── deployment/                 (Mixed configs)
├── scripts/                    (Shared scripts)
└── docs/                       (Shared docs)
```

### After (Organized Structure)
```
ffmpeg-rtmp/
├── master/                     ⭐ MASTER NODE COMPONENTS
│   ├── cmd/master/             (Master binary)
│   ├── exporters/              (Master-side exporters)
│   │   ├── results/
│   │   ├── qoe/
│   │   ├── cost/
│   │   └── health_checker/
│   ├── monitoring/             (VictoriaMetrics, Grafana, Alerts)
│   │   ├── grafana/
│   │   ├── alertmanager/
│   │   ├── victoriametrics.yml
│   │   └── alert-rules.yml
│   ├── deployment/             (Master systemd service)
│   │   └── ffmpeg-master.service
│   └── README.md               (Master setup guide)
│
├── worker/                     ⭐ WORKER NODE COMPONENTS
│   ├── cmd/agent/              (Agent binary)
│   ├── exporters/              (Worker-side exporters)
│   │   ├── cpu_exporter/
│   │   ├── gpu_exporter/
│   │   ├── ffmpeg_exporter/
│   │   └── docker_stats/
│   ├── deployment/             (Worker systemd service)
│   │   └── ffmpeg-agent.service
│   └── README.md               (Worker setup guide)
│
├── shared/                     ⭐ SHARED COMPONENTS
│   ├── pkg/                    (Shared Go packages)
│   │   ├── api/
│   │   ├── models/
│   │   ├── auth/
│   │   ├── agent/
│   │   ├── store/
│   │   ├── tls/
│   │   ├── metrics/
│   │   └── logging/
│   ├── scripts/                (Utility scripts)
│   │   ├── run_tests.py
│   │   ├── analyze_results.py
│   │   └── run_benchmarks.sh
│   ├── advisor/                (ML models)
│   ├── models/                 (Trained models)
│   ├── docs/                   (Documentation)
│   └── README.md               (Shared components guide)
│
├── [ROOT - Development Tools]  ⭐ DEVELOPMENT ENVIRONMENT
│   ├── docker-compose.yml      (Local testing stack)
│   ├── Makefile                (Build orchestration)
│   ├── README.md               (Main project README)
│   ├── FOLDER_ORGANIZATION.md  (This guide!)
│   ├── go.mod                  (Go dependencies)
│   ├── nginx.conf              (RTMP server config)
│   └── [config files]          (Various configs)
│
└── [Legacy - for compatibility]
    ├── cmd/                    (Symlinks to master/worker cmd/)
    ├── deployment/             (Symlinks to master/worker deployment/)
    └── src/exporters/          (Symlinks to master/worker exporters/)
```

## Component Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           PRODUCTION DEPLOYMENT                         │
└─────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│                        MASTER NODE                                │
│                   (master/ components)                            │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Master HTTP Service (cmd/master)                       │    │
│  │  • Job Queue                                            │    │
│  │  • Node Registry                                        │    │
│  │  • Results Collection                                   │    │
│  │  Uses: shared/pkg/{api,models,store,auth,tls}          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                          │                                       │
│                          │ Stores results in                     │
│                          ▼                                       │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Master Exporters (exporters/)                          │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │    │
│  │  │ Results  │ │   QoE    │ │   Cost   │ │  Health  │  │    │
│  │  │ :9502    │ │  :9503   │ │  :9504   │ │  :9600   │  │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │    │
│  │  Uses: shared/{advisor,scripts}                        │    │
│  └─────────────────────────────────────────────────────────┘    │
│                          │                                       │
│                          │ Scraped by                            │
│                          ▼                                       │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Monitoring Stack (monitoring/)                         │    │
│  │  ┌──────────────┐  ┌─────────────┐  ┌──────────────┐  │    │
│  │  │VictoriaMetrics│  │   Grafana   │  │ Alertmanager │  │    │
│  │  │    :8428      │  │    :3000    │  │    :9093     │  │    │
│  │  └──────────────┘  └─────────────┘  └──────────────┘  │    │
│  └─────────────────────────────────────────────────────────┘    │
└───────────────────────────┬───────────────────────────────────────┘
                            │
              HTTP/JSON API │ (Job dispatch, Results collection)
                            │
         ┏━━━━━━━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━━━━━━┓
         ▼                                         ▼
┌─────────────────────────┐            ┌─────────────────────────┐
│    WORKER NODE 1        │            │    WORKER NODE N        │
│  (worker/ components)   │            │  (worker/ components)   │
│                         │            │                         │
│  ┌───────────────────┐  │            │  ┌───────────────────┐  │
│  │ Agent Service     │  │            │  │ Agent Service     │  │
│  │  (cmd/agent)      │  │            │  │  (cmd/agent)      │  │
│  │                   │  │            │  │                   │  │
│  │ • Poll for jobs   │  │            │  │ • Poll for jobs   │  │
│  │ • Execute FFmpeg  │  │            │  │ • Execute FFmpeg  │  │
│  │ • Report results  │  │            │  │ • Report results  │  │
│  │                   │  │            │  │                   │  │
│  │ Uses: shared/pkg/ │  │            │  │ Uses: shared/pkg/ │  │
│  │ {agent,models}    │  │            │  │ {agent,models}    │  │
│  └───────────────────┘  │            │  └───────────────────┘  │
│           │              │            │           │              │
│           │ During job   │            │           │ During job   │
│           ▼              │            │           ▼              │
│  ┌───────────────────┐  │            │  ┌───────────────────┐  │
│  │ Worker Exporters  │  │            │  │ Worker Exporters  │  │
│  │  (exporters/)     │  │            │  │  (exporters/)     │  │
│  │                   │  │            │  │                   │  │
│  │ ┌───┐ ┌───┐ ┌───┐│  │            │  │ ┌───┐ ┌───┐ ┌───┐│  │
│  │ │CPU│ │GPU│ │FFm││  │            │  │ │CPU│ │GPU│ │FFm││  │
│  │ │:10│ │:11│ │:06││  │            │  │ │:10│ │:11│ │:06││  │
│  │ └───┘ └───┘ └───┘│  │            │  │ └───┘ └───┘ └───┘│  │
│  └───────────────────┘  │            │  └───────────────────┘  │
└─────────────────────────┘            └─────────────────────────┘
```

## Development Mode (Docker Compose)

```
┌─────────────────────────────────────────────────────────────────┐
│               SINGLE MACHINE (Local Development)                │
│                    docker-compose.yml                           │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  ALL Master Components (from master/)                     │ │
│  │  • VictoriaMetrics, Grafana, Alertmanager                 │ │
│  │  • Results, QoE, Cost exporters                           │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  ALL Worker Components (from worker/)                     │ │
│  │  • CPU, GPU, FFmpeg exporters                             │ │
│  │  • Docker stats exporter                                  │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  Dev-Only Components                                      │ │
│  │  • Nginx RTMP server                                      │ │
│  │  • Node Exporter, cAdvisor                                │ │
│  └───────────────────────────────────────────────────────────┘ │
│                                                                 │
│  All on localhost, Docker bridge network                       │
└─────────────────────────────────────────────────────────────────┘
```

## Build System Flow

```
┌────────────────────────────────────────────────────────────────┐
│                         Makefile (Root)                        │
└────────────────────────────────────────────────────────────────┘
                            │
        ┏━━━━━━━━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━━━━┓
        ▼                                        ▼
┌─────────────────┐                    ┌─────────────────┐
│ make build-master                    │ make build-agent │
└─────────────────┘                    └─────────────────┘
        │                                        │
        ▼                                        ▼
┌─────────────────┐                    ┌─────────────────┐
│ go build        │                    │ go build        │
│ -o bin/master   │                    │ -o bin/agent    │
│ ./master/cmd/   │                    │ ./worker/cmd/   │
│                 │                    │                 │
│ Imports:        │                    │ Imports:        │
│ • shared/pkg/   │                    │ • shared/pkg/   │
│   api, store,   │                    │   agent, models,│
│   auth, models  │                    │   tls           │
└─────────────────┘                    └─────────────────┘
```

## Deployment Paths

### Master-Only Deployment
```bash
cd master/
make deploy-master
# Deploys: Master service + Monitoring stack
# Uses: master/cmd/, master/exporters/, master/monitoring/
```

### Worker-Only Deployment
```bash
cd worker/
make deploy-worker MASTER_URL=https://master:8080
# Deploys: Agent service + Worker exporters
# Uses: worker/cmd/, worker/exporters/
```

### Full Development Environment
```bash
cd /  # Root
make up-build
# Deploys: Everything in docker-compose.yml
# Uses: All components (master/, worker/, shared/)
```

## Benefits of This Organization

### 1. Clear Separation of Concerns
```
Master components  →  master/    (Orchestration, visualization)
Worker components  →  worker/    (Job execution, metrics)
Shared components  →  shared/    (Common code, models)
Dev tools          →  /          (docker-compose, Makefile)
```

### 2. Simplified Deployment
```
Master node:   Only needs master/ and shared/
Worker node:   Only needs worker/ and shared/
Dev machine:   Uses docker-compose.yml at root
```

### 3. Better Documentation
```
master/README.md   → How to deploy master
worker/README.md   → How to deploy workers
shared/README.md   → Shared component docs
FOLDER_ORGANIZATION.md → This overview
```

### 4. Easier Maintenance
```
Master changes     → Edit master/ only
Worker changes     → Edit worker/ only
Shared changes     → Edit shared/ (affects both)
Breaking changes   → Clearly visible in shared/
```

## Related Files

- [FOLDER_ORGANIZATION.md](FOLDER_ORGANIZATION.md) - Detailed organization guide
- [master/README.md](master/README.md) - Master node setup
- [worker/README.md](worker/README.md) - Worker node setup
- [shared/README.md](shared/README.md) - Shared components
- [README.md](README.md) - Main project README
