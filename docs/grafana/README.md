# Grafana Dashboards - FFmpeg-RTMP Monitoring

## Overview

Consolidated Grafana dashboards for comprehensive system monitoring. Dashboards are **automatically provisioned** when starting the stack with docker-compose.

## Quick Start

```bash
# Start entire stack (Grafana + dashboards auto-load)
docker-compose up -d

# Or use deployment script
./full-stack-deploy.sh

# Access Grafana
http://localhost:3000  # admin/admin
```

## Dashboard Structure (6 Dashboards)

### 1. Production Monitoring (`ffmpeg-rtmp-prod`)
**Primary operational dashboard** - Start here for system health overview

**Panels (12):**
- SLA Compliance Rate (95% target)
- Job Success Rate (90% target) 
- Bandwidth Usage (MB/s per worker)
- Active Jobs count
- Exporter Health Status ← NEW
- SLA Compliance Trend
- Job Completion Rates (success/fail/cancel)
- CPU Usage by Worker
- Memory Usage by Worker
- Worker Bandwidth Utilization %
- Cancellation Stats (graceful vs forceful)
- Top 10 SLA Violations

**URL:** http://localhost:3000/d/ffmpeg-rtmp-prod/

### 2. Job Scheduler & Queue Management (`job-scheduler`)
**Detailed scheduler and queue monitoring** - Use for job flow analysis

**Panels (16):**
- Active Jobs, Queue Length
- Jobs by State, Priority, Type
- Job Duration distribution
- Scheduling Attempts
- Worker Node Status
- Engine Preference vs Usage
- Scheduler Bandwidth tracking
- Request/Response Size distribution

**URL:** http://localhost:3000/d/job-scheduler/

### 3. Worker Node Monitoring (`worker-monitoring`)
**Individual worker deep-dive** - Use for worker troubleshooting

**Panels (8):**
- Worker CPU Usage
- Worker GPU Usage
- Worker Memory Usage
- Active Jobs per Worker
- GPU Temperature
- GPU Power Consumption
- Worker Heartbeat Rate
- GPU Availability

**URL:** http://localhost:3000/d/worker-monitoring/

### 4. Quality & Performance Metrics (`quality-metrics`)
**Quality scores and scenario results** - Consolidated QoE + Results

**Panels (11):**
- VMAF Quality Score
- PSNR Quality Score
- SSIM Quality Score
- Quality per Watt Efficiency
- QoE Efficiency Score
- Frame Drop Rate
- Total Scenarios
- Scenario Duration
- Average FPS
- Frame Statistics
- Quality Scores by Scenario

**URL:** http://localhost:3000/d/quality-metrics/

### 5. Cost Analysis (`cost-analysis`)
**Financial tracking** - Cost per scenario, energy costs

**Panels (5):**
- Total Cost by Scenario
- Current Cost by Scenario
- Cost Breakdown: Energy vs Compute
- Cost Efficiency: Cost per Pixel
- Cost per Viewer Watch Hour

**URL:** http://localhost:3000/d/cost-analysis/

### 6. ML Predictions (`ml-predictions`)
**Machine learning predictions** - Model predictions and confidence

**Panels (12):**
- Predicted VMAF, PSNR
- Predicted Cost (USD)
- Predicted CO2 Emissions
- Prediction Confidence
- Recommended Bitrate
- Model Version, Status
- Time Since Last Model Update
- Prediction Summary Table
- Historical vs Predicted VMAF
- Training Drift Indicator

**URL:** http://localhost:3000/d/ml-predictions/

## How It Works

Dashboards are automatically loaded via Grafana provisioning:

```yaml
# docker-compose.yml
grafana:
  volumes:
    - ./master/monitoring/grafana/provisioning:/etc/grafana/provisioning:ro
```

Grafana scans `/etc/grafana/provisioning/dashboards/*.json` on startup and loads all dashboards automatically.

## Datasource

**VictoriaMetrics** (Prometheus-compatible) - Auto-configured
- **URL:** http://victoriametrics:8428
- **UID:** DS_VICTORIAMETRICS
- No manual setup required

## Adding/Updating Dashboards

1. **Edit JSON file:**
   ```bash
   vim master/monitoring/grafana/provisioning/dashboards/production-monitoring.json
   ```

2. **Validate JSON:**
   ```bash
   python3 -m json.tool production-monitoring.json > /dev/null && echo "Valid"
   ```

3. **Reload Grafana:**
   ```bash
   docker restart grafana
   ```

## Dashboard Guidelines

### When to Use Each Dashboard

- **Production Monitoring**: Daily operations, on-call monitoring, SLA tracking
- **Job Scheduler**: Debugging queue issues, analyzing job distribution
- **Worker Monitoring**: Worker performance issues, GPU problems
- **Quality Metrics**: Quality analysis, scenario comparisons
- **Cost Analysis**: Financial reporting, cost optimization
- **ML Predictions**: Model performance, prediction accuracy

### Dashboard Organization

**Operational (Daily Use):**
- Production Monitoring ← Primary
- Job Scheduler
- Worker Monitoring

**Analytical (Periodic Review):**
- Quality Metrics
- Cost Analysis
- ML Predictions

## Troubleshooting

**Dashboard not appearing?**
```bash
# Check container
docker ps | grep grafana

# Check logs
docker logs grafana | grep -i dashboard

# Restart stack
docker-compose restart grafana
```

**No data in panels?**
```bash
# Check worker metrics
curl http://localhost:9091/metrics

# Check VictoriaMetrics
curl http://localhost:8428/api/v1/query?query=up
```

**Dashboard changes not reflected?**
```bash
# Hard restart
docker-compose down
docker-compose up -d
```

## Consolidation Changes

**Removed (consolidated into other dashboards):**
- ❌ scheduler-jobs → Merged into job-scheduler
- ❌ qoe-metrics + results-overview → Merged into quality-metrics
- ❌ exporter-health → Integrated into production-monitoring
- ❌ production-scheduler → Deleted (empty)

**Before:** 10 dashboards with overlap  
**After:** 6 dashboards, clear purpose, no duplication

## Files

Dashboard JSON files:
```
master/monitoring/grafana/provisioning/dashboards/
├── production-monitoring.json  (12 panels, 28K)
├── job-scheduler.json          (16 panels, 24K)
├── worker-monitoring.json      (8 panels, 16K)
├── quality-metrics.json        (11 panels, 24K)
├── cost-analysis.json          (5 panels, 12K)
└── ml-predictions.json         (12 panels, 32K)
```

Provisioning config:
```
master/monitoring/grafana/provisioning/
├── datasources/
│   └── prometheus.yml          (VictoriaMetrics)
└── dashboards/
    ├── default.yml             (Dashboard provider)
    └── *.json                  (Dashboard files)
```

## References

- **docker-compose.yml** - Grafana service configuration
- **full-stack-deploy.sh** - Complete deployment script
- **VictoriaMetrics UI** - http://localhost:8428/vmui
