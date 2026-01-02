# API Reference

## Master Node API

The master node exposes a REST API for job management, node registration, and cluster operations.

**Base URL:** `https://master-host:8080` (or `http://master-host:8080` without TLS)

**Authentication:** All endpoints require either:
- API Key in header: `X-API-Key: your-api-key`
- mTLS client certificates

---

## Jobs API

### Create Job

Submit a new transcoding job to the cluster.

```http
POST /jobs
Content-Type: application/json
X-API-Key: your-api-key

{
  "scenario": "4K60-h264",
  "confidence": "auto",
  "engine": "auto",
  "queue": "default",
  "priority": "medium",
  "parameters": {
    "duration": 30,
    "bitrate": "10M",
    "output_mode": "file"
  }
}
```

**Response:**
```json
{
  "id": "uuid-string",
  "sequence_number": 1,
  "scenario": "4K60-h264",
  "status": "pending",
  "created_at": "2026-01-02T10:00:00Z"
}
```

### Get Job Status

```http
GET /jobs/{id}
X-API-Key: your-api-key
```

**Response:**
```json
{
  "id": "uuid",
  "sequence_number": 1,
  "scenario": "4K60-h264",
  "status": "running",
  "progress": 45,
  "node_id": "worker-1",
  "node_name": "worker-1.local",
  "created_at": "2026-01-02T10:00:00Z",
  "started_at": "2026-01-02T10:00:05Z"
}
```

### List Jobs

```http
GET /jobs
X-API-Key: your-api-key
```

**Response:**
```json
{
  "jobs": [...],
  "count": 10
}
```

### Get Job Logs

```http
GET /jobs/{id}/logs
X-API-Key: your-api-key
```

**Response:**
```json
{
  "job_id": "uuid",
  "logs": "=== Command Execution ===\nCommand: ffmpeg -i input.mp4...\n\n=== STDERR ===\nffmpeg version..."
}
```

### Cancel Job

```http
POST /jobs/{id}/cancel
X-API-Key: your-api-key
```

### Pause Job

```http
POST /jobs/{id}/pause
X-API-Key: your-api-key
```

### Resume Job

```http
POST /jobs/{id}/resume
X-API-Key: your-api-key
```

### Retry Job

```http
POST /jobs/{id}/retry
X-API-Key: your-api-key
```

---

## Nodes API

### Register Node

```http
POST /nodes/register
Content-Type: application/json
X-API-Key: your-api-key

{
  "address": "https://worker-1.local:9091",
  "type": "worker",
  "cpu_threads": 8,
  "cpu_model": "Intel Xeon",
  "has_gpu": true,
  "gpu_type": "NVIDIA RTX 4090",
  "gpu_capabilities": "nvenc,cuda",
  "ram_total_bytes": 17179869184
}
```

### List Nodes

```http
GET /nodes
X-API-Key: your-api-key
```

### Get Node Details

```http
GET /nodes/{id}
X-API-Key: your-api-key
```

### Remove Node

```http
DELETE /nodes/{id}
X-API-Key: your-api-key
```

### Node Heartbeat

```http
POST /nodes/{id}/heartbeat
X-API-Key: your-api-key
```

---

## Job Parameters

### Common Parameters

- **scenario** (required): Preset scenario name (e.g., "4K60-h264", "1080p-h265")
- **confidence**: `auto`, `high`, `medium`, `low` (default: `auto`)
- **engine**: `auto`, `ffmpeg`, `gstreamer` (default: `auto`)
- **queue**: `live`, `default`, `batch` (default: `default`)
- **priority**: `high`, `medium`, `low` (default: `medium`)

### Job-Specific Parameters

```json
{
  "duration": 30,              // Duration in seconds
  "bitrate": "10M",            // Target bitrate
  "input": "/path/to/input",   // Input file path
  "output_mode": "file",       // "file" or "rtmp"
  "rtmp_url": "rtmp://...",    // For streaming mode
  "codec": "h264",             // Video codec
  "preset": "medium"           // Encoding preset
}
```

---

## Job States (FSM)

Jobs follow a finite state machine:

```
QUEUED → ASSIGNED → RUNNING → COMPLETED
         ↓           ↓
         CANCELED    FAILED → RETRYING → QUEUED
                     ↓
                     TIMED_OUT → RETRYING
```

**Valid States:**
- `queued`: Waiting for a worker
- `assigned`: Assigned to a worker
- `running`: Currently executing
- `completed`: Successfully finished
- `failed`: Execution failed
- `canceled`: Manually canceled
- `timed_out`: Exceeded timeout
- `retrying`: Being retried after failure

---

## Error Responses

All errors return:

```json
{
  "error": "Error message",
  "status": 400
}
```

**Common Status Codes:**
- `200`: Success
- `201`: Created
- `400`: Bad Request
- `401`: Unauthorized
- `404`: Not Found
- `409`: Conflict
- `500`: Internal Server Error

---

## CLI Usage

The `ffrtmp` CLI provides a convenient interface to the API:

```bash
# Submit job
./bin/ffrtmp jobs submit --scenario 4K60-h264 --duration 30

# Check status
./bin/ffrtmp jobs status 123

# Get logs
./bin/ffrtmp jobs logs 123

# List all jobs
./bin/ffrtmp jobs status

# Cancel job
./bin/ffrtmp jobs cancel 123
```

See `./bin/ffrtmp --help` for all commands.

---

## Authentication

### API Key Authentication

Set environment variable:
```bash
export FFMPEG_RTMP_API_KEY="your-api-key-here"
```

Or pass in CLI:
```bash
./bin/ffrtmp --api-key "your-key" jobs status
```

### mTLS Authentication

Generate certificates:
```bash
./scripts/generate-certs.sh
```

Use with CLI:
```bash
./bin/ffrtmp --cert certs/client.crt --key certs/client.key jobs status
```

---

## Metrics Endpoints

### Prometheus Metrics

Worker nodes expose metrics on port 9091:
```
http://worker-host:9091/metrics
```

**Available Metrics:**
- `ffmpeg_jobs_total`: Total jobs processed
- `ffmpeg_job_duration_seconds`: Job execution duration
- `ffmpeg_power_watts`: Current power consumption
- `ffmpeg_cpu_usage_percent`: CPU utilization
- `ffmpeg_memory_bytes`: Memory usage

### VictoriaMetrics

If deployed, metrics are aggregated at:
```
http://master-host:8428
```

---

## Rate Limiting

API endpoints are rate-limited:
- **Default**: 100 requests/minute per API key
- **Burst**: 20 requests

Exceeded limits return `429 Too Many Requests`.

---

## Webhooks (Future)

Webhook support for job status notifications is planned for future releases.

---

For implementation details, see [ARCHITECTURE.md](ARCHITECTURE.md).
For deployment instructions, see [DEPLOYMENT.md](../DEPLOYMENT.md).
