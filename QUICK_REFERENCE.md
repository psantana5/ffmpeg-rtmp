# Quick Reference - Production Features

## 🚀 Quick Start

### Submit a Job with Priority
```bash
ffrtmp jobs submit --scenario 4K60-h264 --queue live --priority high
```

### Monitor Job Progress
```bash
ffrtmp jobs status <job-id> --follow
```

### Control Job Execution
```bash
ffrtmp jobs pause <job-id>
ffrtmp jobs resume <job-id>
ffrtmp jobs cancel <job-id>
```

### Check Node Details
```bash
ffrtmp nodes describe <node-id>
```

---

## 📊 Queue & Priority System

### Queues (Workload Types)
- `live` - Real-time streaming (highest priority)
- `default` - Standard transcoding (medium priority)
- `batch` - Offline processing (lowest priority)

### Priority Levels
- `high` - Urgent jobs
- `medium` - Normal jobs (default)
- `low` - Background jobs

### Scheduling Order
1. Queue: live > default > batch
2. Priority: high > medium > low
3. FIFO within same class

---

## 📈 Prometheus Metrics

### Master Metrics (port 9090)
```
# Query examples
ffrtmp_jobs_total{state="queued"}
ffrtmp_queue_length
ffrtmp_active_jobs
ffrtmp_schedule_attempts_total{result="success"}
```

### Worker Metrics (port 9091)
```
# Query examples
ffrtmp_worker_cpu_usage{node_id="worker-1"}
ffrtmp_worker_gpu_usage{node_id="worker-1"}
ffrtmp_worker_memory_bytes{node_id="worker-1"}
ffrtmp_worker_temperature_celsius{node_id="worker-1"}
```

---

## 🔧 Configuration Flags

### Master
```bash
--port 8080              # API port
--metrics-port 9090      # Prometheus metrics port
--db master.db           # Database path
--max-retries 3          # Job retry limit
```

### Worker
```bash
--master https://...     # Master URL
--register               # Auto-register with master
--metrics-port 9091      # Prometheus metrics port
--poll-interval 10s      # Job polling interval
--heartbeat-interval 30s # Heartbeat frequency
```

### CLI
```bash
--output json            # JSON output
--follow                 # Follow/watch mode
```

---

## 🎯 Job States

```
pending → queued → assigned → processing → completed
              ↓         ↓          ↓
            canceled  paused    failed
                        ↓
                    processing (resumed)
```

**Terminal States:** `completed`, `failed`, `canceled`  
**Resumable State:** `paused`

---

## 🔍 Example Workflows

### High-Priority Live Streaming
```bash
# Submit
JOB_ID=$(ffrtmp jobs submit \
  --scenario 4K60-h264 \
  --queue live \
  --priority high \
  --output json | jq -r '.id')

# Follow progress
ffrtmp jobs status $JOB_ID --follow
```

### Batch Processing with Control
```bash
# Submit batch job
JOB_ID=$(ffrtmp jobs submit \
  --scenario 1080p30-h265 \
  --queue batch \
  --priority low \
  --output json | jq -r '.id')

# Pause if needed
ffrtmp jobs pause $JOB_ID

# Check node load
ffrtmp nodes describe $(ffrtmp jobs status $JOB_ID --output json | jq -r '.node_id')

# Resume when ready
ffrtmp jobs resume $JOB_ID
```

### Monitor Cluster Health
```bash
# List all nodes
ffrtmp nodes list

# Check specific node
ffrtmp nodes describe worker-1

# View metrics
curl http://localhost:9090/metrics | grep ffrtmp
```

---

## 📦 Build & Deploy

### Build All Components
```bash
make build
# Or manually:
go build -o bin/master ./master/cmd/master
go build -o bin/agent ./worker/cmd/agent
go build -o bin/ffrtmp ./cmd/ffrtmp
```

### Run Tests
```bash
go test ./...
# Or specific tests:
go test ./pkg/store -v
go test ./pkg/api -v
```

### Integration Tests
```bash
./tests/integration/run_all_tests.sh
```

---

## 🐛 Troubleshooting

### Job Stuck in Queue
```bash
# Check queue status
curl http://localhost:9090/metrics | grep ffrtmp_queue_length

# Check available workers
ffrtmp nodes list

# Check scheduling attempts
curl http://localhost:9090/metrics | grep ffrtmp_schedule_attempts
```

### Worker Not Picking Up Jobs
```bash
# Check worker status
ffrtmp nodes describe <node-id>

# Check worker logs
# Look for heartbeat messages and job polling

# Verify worker metrics
curl http://worker:9091/metrics
```

### High Resource Usage
```bash
# Check worker metrics
curl http://worker:9091/metrics | grep -E "cpu_usage|memory_bytes"

# Check active jobs
ffrtmp jobs status <job-id>

# Pause job if needed
ffrtmp jobs pause <job-id>
```

---

## 🔐 Security Notes

- API key required for all operations
- TLS recommended for production
- mTLS available for worker authentication
- Metrics endpoints don't require authentication (Prometheus pull model)

---

## 📚 Additional Resources

- Full documentation: `IMPLEMENTATION_COMPLETE.md`
- Test suite: `tests/integration/`
- Original requirements: (see original prompt)
- Metrics reference: Master/Worker exporter files

---

*Quick Reference - v1.0*  
*Updated: December 30, 2025*
