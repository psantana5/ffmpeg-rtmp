# Grafana Dashboard Metrics Guide

Complete guide to understanding and populating data in each Grafana dashboard.

## Quick Start - Populate All Dashboards

```bash
# 1. Ensure services are running
docker compose up -d

# 2. Start master and worker
./bin/master --port 8080 --db master.db --metrics &
./bin/worker --master https://localhost:8080 --metrics-port 9091 &

# 3. Run test jobs to generate metrics
./scripts/test-dashboard-metrics.sh

# 4. View dashboards
open http://localhost:3000
```

## Dashboard-by-Dashboard Guide

### 1. Production Monitoring (`ffmpeg-rtmp-prod`)

**Purpose:** Primary operational dashboard for daily monitoring

**Panels & Data Requirements:**

| Panel | Metric Name | Data Source | How to Populate |
|-------|-------------|-------------|-----------------|
| SLA Compliance Rate | `worker_job_sla_compliant_total` | Worker | Run jobs through system |
| Job Success Rate | `worker_jobs_completed_total`, `worker_jobs_failed_total` | Worker | Process jobs (success/fail) |
| Bandwidth Usage | `worker_bandwidth_bytes_total` | Worker | Jobs with file I/O |
| Active Jobs | `worker_current_jobs` OR `ffrtmp_worker_active_jobs` | Worker | Submit jobs |
| Exporter Health | `up` | All exporters | Auto-populated (✓ working) |
| SLA Compliance Trend | `worker_job_sla_compliant_total` | Worker | Run jobs over time |
| Job Completion Rates | `worker_jobs_completed_total`, `worker_jobs_failed_total`, `worker_job_cancellations_total` | Worker | Process/fail/cancel jobs |
| CPU Usage by Worker | `worker_cpu_usage_percent` OR `ffrtmp_worker_cpu_usage` | Worker | Worker running (✓ working) |
| Memory Usage by Worker | `worker_memory_usage_percent` OR `ffrtmp_worker_memory_bytes` | Worker | Worker running (✓ working) |
| Worker Bandwidth Utilization | `worker_bandwidth_bytes_total`, `worker_bandwidth_capacity_bytes` | Worker | Jobs with bandwidth tracking |
| Cancellation Stats | `worker_job_cancellations_graceful_total`, `worker_job_cancellations_forceful_total` | Worker | Cancel jobs (DELETE /jobs/:id) |
| Top 10 SLA Violations | `worker_job_sla_violation_total` | Worker | Jobs exceeding SLA |

**How to Populate:**

```bash
# Submit jobs
for i in {1..10}; do
  curl -k -X POST https://localhost:8080/jobs \
    -H "Content-Type: application/json" \
    -d '{
      "scenario": "1080p30-h264",
      "parameters": {
        "input": "test.mp4",
        "output": "output.mp4",
        "duration": "5"
      }
    }'
done

# Wait for processing
sleep 30

# Check metrics
curl http://localhost:9091/metrics | grep ffrtmp_worker
```

**Current Status:**
-  **Exporter Health**: Working (shows 9 exporters)
-  **CPU/Memory**: Working (basic system metrics)
- ⏳ **Job metrics**: Need Phase 1 worker updates (SLA, bandwidth, cancellations)

**Why "No Data":**
The Phase 1 metrics (SLA tracking, bandwidth monitoring, cancellation stats) were added to the codebase but need:
1. Worker to be rebuilt with updated exporter code
2. Jobs to be processed to generate metrics
3. Time for metrics to accumulate (not instant)

---

### 2. Job Scheduler & Queue Management (`job-scheduler`)

**Purpose:** Detailed job flow and queue monitoring

**Panels & Data Requirements:**

| Panel | Metric Name | Data Source | How to Populate |
|-------|-------------|-------------|-----------------|
| Active Jobs | `ffrtmp_active_jobs` | Master | Submit jobs |
| Jobs by State | `ffrtmp_jobs_total` | Master | Jobs in various states |
| Queue Length | `ffrtmp_queue_length` | Master | Queued jobs |
| Queue by Priority | `ffrtmp_queue_by_priority` | Master | Submit jobs with priority |
| Job Duration | Job metrics | Master/Worker | Complete jobs |
| Worker Nodes | `ffrtmp_nodes_total`, `ffrtmp_nodes_by_status` | Master | Register workers |
| Scheduling Attempts | `ffrtmp_schedule_attempts_total` | Master | Scheduler activity |

**How to Populate:**

```bash
# Check current metrics
curl http://localhost:9090/metrics | grep ffrtmp_

# Submit jobs with different priorities
curl -k -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{"scenario": "1080p60-h264", "priority": "high"}'

curl -k -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{"scenario": "720p30-h264", "priority": "low"}'
```

**Current Status:**
-  **Master metrics**: Available (`ffrtmp_jobs_total`, `ffrtmp_active_jobs`, etc.)
-  **Queue metrics**: Available (`ffrtmp_queue_length`, `ffrtmp_queue_by_priority`)
-  **Node metrics**: Available when workers register

---

### 3. Worker Node Monitoring (`worker-monitoring`)

**Purpose:** Deep-dive into individual worker performance

**Panels & Data Requirements:**

| Panel | Metric Name | Data Source | How to Populate |
|-------|-------------|-------------|-----------------|
| Worker CPU Usage | `ffrtmp_worker_cpu_usage` | Worker | Auto-populated (✓) |
| Worker GPU Usage | GPU metrics | GPU exporters | GPU available on worker |
| Worker Memory Usage | `ffrtmp_worker_memory_bytes` | Worker | Auto-populated (✓) |
| Active Jobs per Worker | `ffrtmp_worker_active_jobs` | Worker | Auto-populated (✓) |
| GPU Temperature | GPU exporter metrics | DCGM/nvidia-smi | GPU in system |
| GPU Power | GPU exporter metrics | DCGM | GPU in system |
| Worker Heartbeat | `ffrtmp_worker_heartbeats_total` | Worker | Auto-populated (✓) |
| GPU Availability | `ffrtmp_worker_has_gpu`, encoder availability | Worker | Auto-populated (✓) |

**How to Populate:**

```bash
# Check worker metrics
curl http://localhost:9091/metrics | grep ffrtmp_worker

# GPU metrics (if GPU present)
curl http://localhost:9400/metrics | grep dcgm  # DCGM exporter
curl http://localhost:9505/metrics | grep gpu   # GPU exporter
```

**Current Status:**
-  **CPU/Memory/Active Jobs**: Working
-  **Heartbeats/Uptime**: Working
- ⏳ **GPU metrics**: Only if GPU hardware present
-  **Encoder availability**: Working (NVENC/QSV/VAAPI checks)

---

### 4. Quality & Performance Metrics (`quality-metrics`)

**Purpose:** Video quality analysis and scenario comparison

**Panels & Data Requirements:**

| Panel | Metric Name | Data Source | How to Populate |
|-------|-------------|-------------|-----------------|
| VMAF Score | QoE exporter metrics | qoe-exporter:9503 | Jobs with quality analysis |
| PSNR Score | QoE exporter metrics | qoe-exporter:9503 | Jobs with quality analysis |
| SSIM Score | QoE exporter metrics | qoe-exporter:9503 | Jobs with quality analysis |
| Frame Statistics | Results exporter | results-exporter:9502 | Completed jobs |
| Scenario Results | Results exporter | results-exporter:9502 | Multiple scenarios |

**How to Populate:**

```bash
# Check QoE metrics
curl http://localhost:9503/metrics | grep qoe

# Check results metrics
curl http://localhost:9502/metrics | grep result

# Run jobs that generate quality metrics
./scripts/load_test.sh quick
```

**Current Status:**
-  **QoE exporter**: Running (available on :9503)
-  **Results exporter**: Running (available on :9502)
- ⏳ **Data**: Need jobs with quality analysis enabled

---

### 5. Cost Analysis (`cost-analysis`)

**Purpose:** Financial tracking and cost optimization

**Panels & Data Requirements:**

| Panel | Metric Name | Data Source | How to Populate |
|-------|-------------|-------------|-----------------|
| Total Cost | Cost exporter metrics | cost-exporter:9504 | Jobs with cost tracking |
| Cost Breakdown | Energy + compute costs | cost-exporter + cpu-exporter | Jobs running |
| Cost per Pixel | Cost exporter | cost-exporter:9504 | Processed jobs |
| Cost per Watch Hour | Cost exporter | cost-exporter:9504 | Streaming scenarios |

**How to Populate:**

```bash
# Check cost metrics
curl http://localhost:9504/metrics | grep cost

# Check energy metrics
curl http://localhost:9500/metrics | grep energy

# Run jobs to accumulate costs
./scripts/load_test.sh standard
```

**Current Status:**
-  **Cost exporter**: Running (available on :9504)
-  **CPU/Energy exporter**: Running (available on :9500)
- ⏳ **Data**: Accumulates as jobs run

---

### 6. ML Predictions (`ml-predictions`)

**Purpose:** Machine learning model performance tracking

**Panels & Data Requirements:**

| Panel | Metric Name | Data Source | How to Populate |
|-------|-------------|-------------|-----------------|
| Predicted VMAF/PSNR | ML exporter metrics | ml-predictions-exporter:9505 | ML predictions enabled |
| Predicted Cost | ML exporter metrics | ml-predictions-exporter:9505 | Cost predictions |
| Model Status | ML exporter metrics | ml-predictions-exporter:9505 | Auto-populated (✓) |
| Training Drift | ML exporter metrics | ml-predictions-exporter:9505 | Over time |

**How to Populate:**

```bash
# Check ML metrics
curl http://localhost:9505/metrics | grep ml

# ML predictions require trained model and inference
# See ML documentation for model training
```

**Current Status:**
-  **ML exporter**: Running (available on :9505)
- ⏳ **Predictions**: Need trained model and inference runs

---

## Common Issues & Solutions

### "No data" in all panels

**Cause:** VictoriaMetrics not scraping metrics

**Solution:**
```bash
# Check VictoriaMetrics
curl http://localhost:8428/api/v1/query?query=up

# Restart if needed
docker compose restart victoriametrics

# Wait 30 seconds for scraping
```

### "No data" in job-related panels only

**Cause:** No jobs have been processed yet

**Solution:**
```bash
# Run test script
./scripts/test-dashboard-metrics.sh

# Or submit jobs manually
curl -k -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{"scenario": "1080p30-h264", "parameters": {...}}'
```

### Metric names don't match dashboard queries

**Cause:** Dashboard uses Phase 1 metric names, worker exports different names

**Current metric mapping:**
- Dashboard: `worker_cpu_usage_percent` → Actual: `ffrtmp_worker_cpu_usage`
- Dashboard: `worker_jobs_completed_total` → Actual: Not yet exported
- Dashboard: `worker_bandwidth_bytes_total` → Actual: Not yet exported

**Solution:** Metrics will be exported when jobs are processed with Phase 1 code active.

### GPU metrics missing

**Cause:** No GPU hardware or GPU exporters not running

**Solution:**
```bash
# Check if GPU present
nvidia-smi

# Check GPU exporters
curl http://localhost:9400/metrics  # DCGM
curl http://localhost:9505/metrics  # GPU exporter

# Start if needed
docker compose up -d dcgm-exporter
```

## Metrics Export Timing

Different metrics populate at different times:

| Metric Type | When Available | Refresh Rate |
|-------------|----------------|--------------|
| System (CPU/Memory) | Immediate | 5-10s |
| Exporter Health | Immediate | 10s |
| Job Counts | After job submission | 10s |
| Job Completion | After job finishes | 10s |
| SLA/Bandwidth | After job with tracking | 10s |
| Quality (VMAF/PSNR) | After quality analysis | 30s-1min |
| Cost Accumulation | Continuous during jobs | 10s |
| ML Predictions | After model inference | Variable |

## Testing Checklist

Use this checklist to verify each dashboard:

```bash
# 1. System basics
curl http://localhost:9090/metrics | grep ffrtmp_  # Master
curl http://localhost:9091/metrics | grep ffrtmp_  # Worker

# 2. Submit test jobs
./scripts/test-dashboard-metrics.sh

# 3. Check VictoriaMetrics
curl http://localhost:8428/api/v1/query?query=ffrtmp_active_jobs

# 4. Open each dashboard and verify panels populate
open http://localhost:3000/d/ffmpeg-rtmp-prod/
open http://localhost:3000/d/job-scheduler/
open http://localhost:3000/d/worker-monitoring/
open http://localhost:3000/d/quality-metrics/
open http://localhost:3000/d/cost-analysis/
open http://localhost:3000/d/ml-predictions/

# 5. Wait 30-60 seconds and refresh
```

## Next Steps

1. **Run test script**: `./scripts/test-dashboard-metrics.sh`
2. **Submit real jobs**: Use your actual transcoding jobs
3. **Run load tests**: `./scripts/load_test.sh` for comprehensive data
4. **Monitor over time**: Let system run to accumulate metrics
5. **Check specific exporters**: Query each exporter directly to debug

## References

- **Prometheus Query Language**: https://prometheus.io/docs/prometheus/latest/querying/basics/
- **VictoriaMetrics UI**: http://localhost:8428/vmui
- **Master Metrics**: http://localhost:9090/metrics
- **Worker Metrics**: http://localhost:9091/metrics
- **All Exporters**: See docker-compose.yml for ports
