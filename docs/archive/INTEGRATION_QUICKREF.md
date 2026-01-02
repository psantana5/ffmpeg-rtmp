# Integration Quick Reference

## Features Integrated ✅

### 1. Fault Tolerance
- **What**: Automatic job recovery and node failure handling
- **Where**: `shared/pkg/scheduler/recovery.go`
- **How to use**: Enabled by default in scheduler (runs every 5s)
- **Config**: `--max-retries 3` (default)

### 2. Priority Queue
- **What**: Jobs scheduled by queue type and priority level
- **Where**: `shared/pkg/scheduler/priority_queue.go` + SQL in `sqlite.go`
- **How to use**: `./bin/ffrtmp jobs submit --queue live --priority high`
- **Priority order**: live/high → live/medium → default/high → ... → batch/low

### 3. Security/Authentication
- **What**: API key authentication for all endpoints
- **Where**: `master/cmd/master/main.go` (middleware)
- **How to use**: 
  ```bash
  export MASTER_API_KEY=$(openssl rand -base64 32)
  ./bin/master
  ```
- **Worker**: `export FFMPEG_RTMP_API_KEY=$MASTER_API_KEY`

### 4. Distributed Tracing
- **What**: OpenTelemetry integration for request tracing
- **Where**: `shared/pkg/tracing/` + agent client + master/worker main
- **How to use**:
  ```bash
  # Start Jaeger
  docker run -d -p 16686:16686 -p 4318:4318 jaegertracing/all-in-one
  
  # Enable tracing
  ./bin/master --tracing --tracing-endpoint localhost:4318
  ./bin/agent --tracing --tracing-endpoint localhost:4318
  
  # View: http://localhost:16686
  ```

### 5. Job Logs CLI
- **What**: Retrieve execution logs from database
- **Where**: `cmd/ffrtmp/cmd/jobs.go` + API endpoint
- **How to use**: `./bin/ffrtmp jobs logs <job-id>`
- **Formats**: table (default) or json (`--output json`)

### 6. Duplicate Node Prevention
- **What**: Prevents same address from registering twice
- **Where**: `shared/pkg/api/master.go` (RegisterNode)
- **How to verify**: Try registering same worker twice
- **Error**: HTTP 409 Conflict

## Quick Start Commands

```bash
# 1. Build everything
go build -o bin/master ./master/cmd/master
go build -o bin/agent ./worker/cmd/agent
go build -o bin/ffrtmp ./cmd/ffrtmp

# 2. Generate API key
export MASTER_API_KEY=$(openssl rand -base64 32)

# 3. Start master (with all features)
./bin/master \
  --db master.db \
  --port 8080 \
  --max-retries 3 \
  --tracing \
  --tracing-endpoint localhost:4318

# 4. Start worker (with all features)
export FFMPEG_RTMP_API_KEY=$MASTER_API_KEY
./bin/agent \
  --register \
  --master https://localhost:8080 \
  --tracing \
  --tracing-endpoint localhost:4318

# 5. Submit prioritized jobs
./bin/ffrtmp jobs submit \
  --scenario "live-stream" \
  --queue live \
  --priority high

./bin/ffrtmp jobs submit \
  --scenario "batch-transcode" \
  --queue batch \
  --priority low

# 6. Check job logs
./bin/ffrtmp jobs logs 1
./bin/ffrtmp jobs logs 2 --output json

# 7. Monitor jobs
./bin/ffrtmp jobs status
./bin/ffrtmp jobs status 1 --follow  # Live updates
```

## Testing

```bash
# Run all scheduler tests
cd shared/pkg/scheduler && go test -v

# Verify integrations
./bin/master --help | grep -E "(tracing|max-retries)"
./bin/agent --help | grep tracing
./bin/ffrtmp jobs logs --help
```

## Architecture Summary

```
Master Node
├── Auth Middleware (API Key)
├── Tracing Middleware (OpenTelemetry)
├── Priority Queue Scheduler
│   ├── Live queue (priority 10)
│   ├── Default queue (priority 5)
│   └── Batch queue (priority 1)
├── Recovery Manager
│   ├── Dead node detection (2min timeout)
│   ├── Job reassignment
│   └── Failed job retry (max 3x)
└── API Handlers
    ├── Register (duplicate check)
    ├── GetJobLogs (returns logs from DB)
    └── ... (all endpoints traced)

Worker Node
├── Agent Client (with tracing)
│   ├── Register
│   ├── Heartbeat
│   ├── GetNextJob
│   └── SendResults (with logs)
└── Job Execution
    ├── Capture logs
    └── Send to master
```

## Files Modified

1. `master/cmd/master/main.go` - Added tracing support
2. `worker/cmd/agent/main.go` - Added tracing support
3. `shared/pkg/agent/client.go` - Added tracing to all HTTP calls
4. Existing files (already had features):
   - `shared/pkg/scheduler/recovery.go` - Fault tolerance
   - `shared/pkg/scheduler/priority_queue.go` - Priority queue
   - `shared/pkg/api/master.go` - Auth + duplicate check + logs endpoint
   - `cmd/ffrtmp/cmd/jobs.go` - Job logs command

## Test Results

```
✅ Priority queue tests: PASS (8/8 tests)
✅ Recovery tests: PASS (6/6 tests)
✅ Scheduler tests: PASS (4/4 tests)
✅ Master binary builds successfully
✅ Worker binary builds successfully
✅ CLI binary builds successfully
✅ Master has tracing flags
✅ Worker has tracing flags
✅ Job logs command exists
```

## Common Issues & Solutions

### Issue: "Node with address X is already registered"
**Solution**: This is expected behavior (duplicate prevention). Remove the node first if you need to re-register.

### Issue: Jobs not being assigned
**Check**: 
- Worker heartbeat status
- Job priority settings
- Recovery manager logs

### Issue: Tracing not showing spans
**Check**:
- OTLP endpoint is running (Jaeger on port 4318)
- Both master and worker have `--tracing` flag
- Check logs for tracing initialization

### Issue: API authentication fails
**Check**:
- `MASTER_API_KEY` matches `FFMPEG_RTMP_API_KEY`
- Authorization header format: `Bearer <key>`
- Master logs for auth errors

## Metrics to Monitor

1. **Job scheduling**:
   - `ffrtmp_master_schedule_attempts_total{result="success"}`
   - Queue depths by priority

2. **Node health**:
   - `ffrtmp_master_nodes_total{status="offline"}` (should be 0)
   - `ffrtmp_worker_heartbeats_total` (increasing)

3. **Fault tolerance**:
   - Recovery manager logs
   - Job retry counts
   - Node failure detections

4. **Tracing**:
   - Span durations
   - Error rates per operation
   - Request throughput

## Next Steps

1. **Production Deployment**:
   - Enable TLS with real certificates
   - Set up mTLS for worker authentication
   - Configure persistent storage (SQLite WAL or PostgreSQL)
   - Deploy OTLP collector (Jaeger, Tempo, etc.)

2. **Monitoring Setup**:
   - Prometheus for metrics
   - Grafana dashboards
   - Alert rules for node failures

3. **Scale Testing**:
   - Test with multiple workers
   - Test priority queue under load
   - Test fault tolerance with node failures

For complete details, see `INTEGRATION_SUMMARY.md`.
