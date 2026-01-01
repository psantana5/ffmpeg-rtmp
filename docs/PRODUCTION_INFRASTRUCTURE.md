# Production Infrastructure Guide

This guide covers the production-grade features implemented in the FFmpeg-RTMP distributed system.

## Table of Contents

1. [Rate Limiting](#rate-limiting)
2. [Distributed Tracing](#distributed-tracing)
3. [Resource Management](#resource-management)
4. [Fault Tolerance](#fault-tolerance)
5. [Security](#security)

## Rate Limiting

### Overview

Rate limiting protects the master node from being overwhelmed by excessive requests. The system uses token bucket algorithm for efficient rate limiting.

### Configuration

Enable rate limiting in master node:

```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/ratelimit"

// Create rate limiter: 100 requests per second, burst of 20
limiter := ratelimit.NewLimiter(100, 20)

// Add middleware to router
router.Use(limiter.Middleware(ratelimit.IPKeyFunc))
```

### Rate Limit Strategies

**By IP Address:**
```go
router.Use(limiter.Middleware(ratelimit.IPKeyFunc))
```

**By API Key:**
```go
router.Use(limiter.Middleware(ratelimit.APIKeyFunc))
```

**Custom Key Function:**
```go
router.Use(limiter.Middleware(func(r *http.Request) string {
    return r.Header.Get("X-Custom-ID")
}))
```

### Monitoring

When rate limited, clients receive HTTP 429 (Too Many Requests) response.

---

## Distributed Tracing

### Overview

OpenTelemetry-based distributed tracing provides end-to-end visibility into job execution across master and worker nodes.

### Prerequisites

Deploy an OpenTelemetry collector or compatible backend:
- **Jaeger** (recommended for development)
- **Tempo** (Grafana-native)
- **Any OTLP-compatible backend**

#### Quick Start with Jaeger

```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest
```

Access Jaeger UI: http://localhost:16686

### Master Configuration

```bash
./bin/master \
  --tracing-enabled=true \
  --tracing-endpoint=localhost:4318 \
  --tracing-service-name=ffrtmp-master \
  --tracing-environment=production
```

### Worker Configuration

```bash
./bin/agent \
  --tracing-enabled=true \
  --tracing-endpoint=localhost:4318 \
  --tracing-service-name=ffrtmp-worker \
  --tracing-environment=production
```

### What Gets Traced

- **HTTP requests**: All API calls
- **Job lifecycle**: Creation, assignment, execution, completion
- **Worker registration**: Node registration and heartbeats
- **Database operations**: Store interactions
- **FFmpeg/GStreamer execution**: Encoding jobs

### Trace Context Propagation

Trace context is automatically propagated via HTTP headers:
- `traceparent`: W3C Trace Context
- `tracestate`: Additional vendor-specific data

### Example Queries

In Jaeger UI:

1. **Find slow jobs**: Service=ffrtmp-worker, Tags=job.id
2. **Trace full workflow**: Service=ffrtmp-master, Operation=POST /jobs/submit
3. **Debug failures**: Tags=error=true

---

## Resource Management

### Overview

Resource management ensures fair allocation of CPU, GPU, and RAM across jobs.

### Architecture

```
┌─────────────┐
│   Master    │
│             │
│ Resource    │◄─── Job requests CPU/GPU/RAM
│ Manager     │
│             │
└─────────────┘
      │
      ├───► Node 1: 8 CPU, 1 GPU, 16GB RAM
      ├───► Node 2: 16 CPU, 2 GPU, 32GB RAM
      └───► Node 3: 4 CPU, 0 GPU, 8GB RAM
```

### Usage

**Register node resources:**
```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/resources"

resourceMgr := resources.NewManager()
resourceMgr.RegisterNode("node1", 8.0, 1, 16.0) // 8 CPU cores, 1 GPU, 16GB RAM
```

**Reserve resources for a job:**
```go
err := resourceMgr.Reserve("job123", "node1", 4.0, 1, 8.0)
if err != nil {
    // Handle insufficient resources
}
```

**Find available nodes:**
```go
nodes := resourceMgr.GetAvailableNodes(4.0, 1, 8.0) // Need 4 CPU, 1 GPU, 8GB RAM
fmt.Printf("Available nodes: %v\n", nodes)
```

**Release resources:**
```go
resourceMgr.Release("job123")
```

### Integration with Scheduler

The scheduler can use resource manager to make intelligent scheduling decisions:

```go
// Find nodes that can handle this job
availableNodes := resourceMgr.GetAvailableNodes(
    job.Parameters["cpu_cores"].(float64),
    job.Parameters["gpu_count"].(int),
    job.Parameters["ram_gb"].(float64),
)

if len(availableNodes) == 0 {
    // Queue job for later
    store.UpdateJobStatus(job.ID, models.JobStatusQueued, "Waiting for resources")
} else {
    // Assign to best node
    selectedNode := selectBestNode(availableNodes)
    resourceMgr.Reserve(job.ID, selectedNode, ...)
    store.AssignJob(job.ID, selectedNode)
}
```

---

## Fault Tolerance

### Job Recovery

The system automatically detects and recovers from job failures:

#### Stale Job Detection

**Batch Jobs** (default, VoD):
- Timeout: 30 minutes
- Trigger: Total processing time exceeds threshold
- Action: Mark as failed, available for retry

**Live Jobs** (streaming):
- Timeout: 5 minutes of inactivity
- Trigger: No heartbeat/progress update
- Action: Mark as failed, reassign if retry count allows

#### Heartbeat Mechanism

Workers send progress updates that reset `LastActivityAt`:

```bash
# Worker sends progress
POST /jobs/{job_id}/progress
{
  "progress": 45,
  "status": "processing"
}
```

Master updates `LastActivityAt` automatically.

#### Retry Logic

Jobs are retried up to `max-retries` times (default: 3):

```bash
./bin/master --max-retries=3
```

Retry behavior:
1. Job fails → `retry_count++`
2. If `retry_count < max_retries`: status → `queued`
3. If `retry_count >= max_retries`: status → `failed` (permanent)

### Worker Failure Handling

When a worker disconnects:
1. Scheduler detects stale jobs (no heartbeat for 5 min)
2. Jobs marked as failed
3. If retries available, jobs re-queued
4. Resource manager releases reservations

### Graceful Shutdown

Workers should implement graceful shutdown:

```go
// Handle SIGTERM/SIGINT
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

<-sigCh
log.Println("Shutting down gracefully...")

// Update job status to queued
client.UpdateJobProgress(jobID, models.JobStatusQueued, 0)

// Deregister from master
client.Deregister()

os.Exit(0)
```

---

## Security

### TLS/mTLS

**Master with TLS:**
```bash
./bin/master \
  --tls=true \
  --cert=certs/master.crt \
  --key=certs/master.key
```

**Master with mTLS (mutual authentication):**
```bash
./bin/master \
  --tls=true \
  --mtls=true \
  --cert=certs/master.crt \
  --key=certs/master.key \
  --ca=certs/ca.crt
```

**Worker with TLS client:**
```bash
./bin/agent \
  --master=https://master:8080 \
  --ca=certs/ca.crt \
  --cert=certs/worker.crt \
  --key=certs/worker.key
```

### API Authentication

**Generate secure API key:**
```bash
openssl rand -base64 32
```

**Master with API key:**
```bash
export MASTER_API_KEY=your-secure-key-here
./bin/master
```

**Worker with API key:**
```bash
export FFMPEG_RTMP_API_KEY=your-secure-key-here
./bin/agent --master=https://master:8080
```

**CLI with API key:**
```bash
export FFMPEG_RTMP_API_KEY=your-secure-key-here
ffrtmp nodes list --master=https://master:8080
```

### Rate Limiting

See [Rate Limiting](#rate-limiting) section above.

### Security Best Practices

1. **Always use TLS in production**
2. **Enable mTLS for worker-master communication**
3. **Rotate API keys regularly**
4. **Use strong API keys (32+ bytes entropy)**
5. **Enable rate limiting**
6. **Monitor for suspicious activity via traces**
7. **Run workers in isolated environments (containers)**
8. **Restrict network access (firewall rules)**

---

## Monitoring Dashboard

### Recommended Stack

```yaml
version: '3'
services:
  # Metrics
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin

  # Tracing
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"  # Jaeger UI
      - "4318:4318"    # OTLP HTTP
```

### Key Metrics to Monitor

**Master:**
- `ffrtmp_master_jobs_total{status}`: Job counts by status
- `ffrtmp_master_nodes_total{status}`: Node counts by status
- `ffrtmp_master_job_duration_seconds`: Job execution time
- Rate limit hits (HTTP 429 responses)

**Worker:**
- `ffrtmp_worker_nvenc_available`: GPU encoder availability
- `ffrtmp_worker_jobs_total`: Jobs processed
- `ffrtmp_worker_input_generation_duration_seconds`: Input gen time
- CPU/GPU utilization

---

## Troubleshooting

### Issue: Jobs stuck in "queued" status

**Diagnosis:**
```bash
ffrtmp nodes list
# Check if workers are available

ffrtmp jobs status
# Check resource requirements
```

**Solution:**
- Ensure workers are registered and available
- Check resource manager for capacity
- Verify network connectivity (TLS, firewall)

### Issue: Rate limit errors (HTTP 429)

**Diagnosis:**
Check rate limiter configuration and client request rate.

**Solution:**
- Increase rate limit: `limiter := ratelimit.NewLimiter(200, 50)`
- Add exponential backoff in client
- Use connection pooling

### Issue: Traces not appearing

**Diagnosis:**
1. Check OTLP endpoint connectivity
2. Verify tracing enabled on both master and worker
3. Check Jaeger/collector logs

**Solution:**
```bash
# Test OTLP endpoint
curl http://localhost:4318/v1/traces

# Check master logs for tracing initialization
grep "OpenTelemetry" master.log
```

### Issue: Worker crashes after job assignment

**Diagnosis:**
Check worker logs and enable tracing for detailed span errors.

**Solution:**
- Verify input file generation (if enabled)
- Check FFmpeg/GStreamer availability
- Ensure adequate resources (CPU/RAM)
- Review trace spans for failure point

---

## Performance Tuning

### Rate Limiting

Adjust based on master CPU capacity:
- **Small deployment**: 50 req/s, burst 10
- **Medium deployment**: 100 req/s, burst 20
- **Large deployment**: 500 req/s, burst 100

### Tracing Sampling

For high-traffic systems, use sampling:

```go
tp := sdktrace.NewTracerProvider(
    sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)), // Sample 10%
    // ...
)
```

### Resource Manager

Adjust reservation strategy:
- **Conservative**: Reserve full job requirements
- **Aggressive**: Allow oversubscription (1.5x resources)

---

## Future Enhancements

- [ ] Master clustering (Raft consensus)
- [ ] Automatic failover
- [ ] Advanced scheduling policies (fairness, priority preemption)
- [ ] RBAC (role-based access control)
- [ ] Multi-tenancy support
- [ ] Cost-based scheduling with resource pricing
- [ ] GPU sharing/fractionalization
- [ ] Job dependencies (DAG workflows)

---

## Support

For issues or questions:
- GitHub Issues: https://github.com/psantana5/ffmpeg-rtmp/issues
- Documentation: https://github.com/psantana5/ffmpeg-rtmp/docs
