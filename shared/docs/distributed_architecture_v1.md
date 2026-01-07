# Distributed Architecture v1.0

## Overview

The FFmpeg RTMP power monitoring system now supports distributed compute capabilities, allowing multiple nodes to execute optimized FFmpeg workloads in parallel. Results are aggregated on a master node for storage, analytics, and visualization.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                          MASTER NODE                                 │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                     Master HTTP Service                       │  │
│  │                         (Port 8080)                           │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                 │                                    │
│    ┌────────────────────────────┼────────────────────────────┐      │
│    │                            │                            │      │
│    ▼                            ▼                            ▼      │
│  ┌────────────┐          ┌─────────────┐           ┌──────────────┐│
│  │   Node     │          │    Job      │           │ VictoriaMetrics││
│  │  Registry  │          │    Queue    │           │   + Grafana   ││
│  │            │          │   (FIFO)    │           │               ││
│  │ • CPU/GPU  │          │             │           │  (Local only) ││
│  │ • Memory   │          │ In-Memory   │           │               ││
│  │ • Labels   │          │   Store     │           │               ││
│  └────────────┘          └─────────────┘           └──────────────┘│
│                                                                      │
└──────────────────────────────┬───────────────────────────────────────┘
                               │
                  ┌────────────┴────────────┐
                  │     HTTP/JSON           │
                  │   (HTTPS in prod)       │
                  └────────────┬────────────┘
                               │
        ┏━━━━━━━━━━━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━━━━━━━┓
        ┃                                              ┃
        ▼                                              ▼
┌───────────────────┐                         ┌───────────────────┐
│  COMPUTE NODE 1   │                         │  COMPUTE NODE N   │
│                   │                         │                   │
│ ┌───────────────┐ │                         │ ┌───────────────┐ │
│ │ Agent Service │ │                         │ │ Agent Service │ │
│ │  (Go Binary)  │ │                         │ │  (Go Binary)  │ │
│ └───────────────┘ │                         │ └───────────────┘ │
│         │         │                         │         │         │
│         │         │                         │         │         │
│    ┌────┴────┐    │                         │    ┌────┴────┐    │
│    │         │    │                         │    │         │    │
│    ▼         ▼    │                         │    ▼         ▼    │
│  ┌────┐  ┌─────┐  │                         │  ┌────┐  ┌─────┐  │
│  │HW  │  │Poll │  │                         │  │HW  │  │Poll │  │
│  │Det │  │Jobs │  │                         │  │Det │  │Jobs │  │
│  └────┘  └─────┘  │                         │  └────┘  └─────┘  │
│                   │                         │                   │
│  On Job Received: │                         │  On Job Received: │
│  ┌─────────────┐  │                         │  ┌─────────────┐  │
│  │  FFmpeg +   │  │                         │  │  FFmpeg +   │  │
│  │  Exporters  │  │                         │  │  Exporters  │  │
│  │  Analyzer   │  │                         │  │  Analyzer   │  │
│  └─────────────┘  │                         │  └─────────────┘  │
│         │         │                         │         │         │
│         └─────────┼─────────────────────────┼─────────┘         │
│      Results      │                         │      Results      │
│      Batch JSON   │                         │      Batch JSON   │
└───────────────────┘                         └───────────────────┘
```

## Components

### Master Node

The master node runs a lightweight HTTP service that coordinates distributed workloads.

**Key Responsibilities:**
- Accept node registrations
- Maintain node registry with hardware capabilities
- Queue and dispatch jobs
- Receive and aggregate results
- Host VictoriaMetrics and Grafana (local only)

**REST API Endpoints:**

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/nodes/register` | Register a compute node |
| GET | `/nodes` | List all registered nodes |
| POST | `/nodes/{id}/heartbeat` | Update node heartbeat |
| POST | `/jobs` | Create a new job |
| GET | `/jobs` | List all jobs |
| GET | `/jobs/next?node_id=X` | Get next available job |
| POST | `/results` | Receive job results |
| GET | `/health` | Health check |

**Starting the Master:**

```bash
# Build the master
go build -o bin/master ./cmd/master

# Run the master
./bin/master --port 8080
```

### Compute Node Agent

The compute node agent is a Go binary that runs on worker machines. It detects hardware, registers with the master, polls for jobs, and executes workloads.

**Key Responsibilities:**
- Detect and report hardware capabilities (CPU, GPU, RAM)
- Register with master node
- Send periodic heartbeats
- Poll for available jobs
- Execute FFmpeg workloads with optimal parameters
- Collect metrics during execution
- Send results back to master

**Starting an Agent:**

```bash
# Build the agent
go build -o bin/agent ./cmd/agent

# Register and start the agent
./bin/agent --register --master http://master-ip:8080

# For development: Register master as worker (with confirmation)
./bin/agent --register --master http://localhost:8080 --allow-master-as-worker
```

**Agent Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--master` | `http://localhost:8080` | Master node URL |
| `--register` | `false` | Register with master node |
| `--poll-interval` | `10s` | Job polling interval |
| `--heartbeat-interval` | `30s` | Heartbeat interval |
| `--allow-master-as-worker` | `false` | Allow master as worker (dev mode) |

### Data Models

**Node Registration:**
```json
{
  "address": "worker-01",
  "type": "server",
  "cpu_threads": 16,
  "cpu_model": "Intel(R) Xeon(R) CPU E5-2680 v4",
  "has_gpu": true,
  "gpu_type": "NVIDIA Tesla T4",
  "ram_bytes": 34359738368,
  "labels": {
    "node_type": "server",
    "os": "linux",
    "arch": "amd64"
  }
}
```

**Job Request:**
```json
{
  "scenario": "4K60-h264",
  "confidence": "auto",
  "parameters": {
    "duration": 300,
    "bitrate": "15000k"
  }
}
```

**Job Result:**
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "node_id": "660e8400-e29b-41d4-a716-446655440001",
  "status": "completed",
  "metrics": {
    "cpu_usage": 75.5,
    "memory_usage": 2048,
    "duration": 5.0
  },
  "analyzer_output": {
    "scenario": "4K60-h264",
    "recommendation": "optimal"
  },
  "completed_at": "2025-12-29T19:45:00Z"
}
```

## Node Types

Nodes are automatically classified based on hardware:

| Type | Criteria |
|------|----------|
| **laptop** | Has battery |
| **server** | >16 threads AND >32GB RAM AND no battery |
| **desktop** | Everything else |

## Job Scheduling

**Current Implementation (v1.0):**
- Simple FIFO (First-In-First-Out) queue
- Jobs dispatched to first available node
- No resource-based scheduling

**Future Enhancements (v1.5+):**
- Priority queues
- Resource-aware scheduling (match workload to node capabilities)
- Load balancing
- Retry logic with exponential backoff

## Security

**Current (v1.0):**
- Trust-on-first-register
- HTTP communication
- UUID-based node identification

**Recommended for Production:**
- mTLS between nodes and master
- API authentication tokens
- HTTPS/TLS for all communication
- Node authentication via shared secrets or certificates

## Network Communication

- **Protocol:** JSON over HTTP (HTTPS recommended for production)
- **Push Model:** Compute nodes push results to master (no master-initiated scraping)
- **Batching:** Results sent in single JSON payload
- **Minimal Bandwidth:** Metrics aggregated locally before transmission

## Deployment Patterns

### Single Machine (Development)

Master and worker on same machine:

```bash
# Terminal 1: Start master
./bin/master --port 8080

# Terminal 2: Start worker (with confirmation)
./bin/agent --register --master http://localhost:8080 --allow-master-as-worker
```

 **Warning:** This configuration is for development only. Master and worker compete for resources.

### Multi-Node (Production)

Master on dedicated machine, workers on separate nodes:

```bash
# On master node (e.g., 192.168.1.100)
./bin/master --port 8080

# On worker node 1
./bin/agent --register --master http://192.168.1.100:8080

# On worker node 2
./bin/agent --register --master http://192.168.1.100:8080
```

## Metrics and Monitoring

**Master Node Only:**
- VictoriaMetrics and Grafana remain on master
- No Prometheus on compute nodes
- Exporters run only during active jobs on workers
- Results pushed to master for storage

**Advantages:**
- Reduced overhead on compute nodes
- Centralized metrics storage
- Simplified deployment

## Example Workflow

1. **Start Master:**
   ```bash
   ./bin/master --port 8080
   ```

2. **Register Workers:**
   ```bash
   # On each worker node
   ./bin/agent --register --master http://master-ip:8080
   ```

3. **Create Job:**
   ```bash
   curl -X POST http://master-ip:8080/jobs \
     -H "Content-Type: application/json" \
     -d '{
       "scenario": "4K60-h264",
       "confidence": "auto"
     }'
   ```

4. **Monitor Progress:**
   ```bash
   # List nodes
   curl http://master-ip:8080/nodes

   # List jobs
   curl http://master-ip:8080/jobs
   ```

5. **View Results:**
   - Check logs on master for received results
   - View metrics in Grafana (http://master-ip:3000)

## Building from Source

```bash
# Build master
go build -o bin/master ./cmd/master

# Build agent
go build -o bin/agent ./cmd/agent

# Or build both
make build-distributed
```

## Testing

### Test Node Registration

```bash
# Start master
./bin/master --port 8080

# In another terminal, register agent
./bin/agent --register --master http://localhost:8080 --allow-master-as-worker

# Verify registration
curl http://localhost:8080/nodes | jq
```

### Test Job Dispatch

```bash
# Create a test job
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-1080p",
    "confidence": "auto"
  }'

# Agent will pick it up automatically
# Check logs for execution
```

## Limitations (v1.0)

What's **NOT** included in this release:

- Multi-master setup
- Live streaming logs/metrics
- Kubernetes orchestration
- Hardware profiler database persistence
- Authentication UI
- Advanced scheduling algorithms

These features are planned for future releases (v1.5+).

## Troubleshooting

### Agent can't connect to master

```bash
# Check master is running
curl http://master-ip:8080/health

# Check network connectivity
ping master-ip

# Check firewall allows port 8080
```

### Node not receiving jobs

- Verify node status is "available": `curl http://master-ip:8080/nodes`
- Check heartbeat is working (logs should show periodic heartbeats)
- Ensure jobs exist in queue: `curl http://master-ip:8080/jobs`

### Master not starting

- Check port 8080 is not already in use: `lsof -i :8080`
- Try different port: `./bin/master --port 8081`

## Success Criteria

 **Node Registration:** Nodes appear via `GET /nodes`  
 **Job Dispatch:** Jobs assigned and executed on workers  
 **Results Collection:** Results recorded and visible in logs  
 **Single-Node Workflow:** Existing workflow unchanged  
 **Master-as-Worker:** Development mode with safety warnings  

## Next Steps

After v1.0 stabilizes, planned enhancements include:

1. **v1.1:** Retry logic and error recovery
2. **v1.2:** Resource-aware scheduling
3. **v1.3:** mTLS and authentication
4. **v1.4:** Results persistence to database
5. **v1.5:** Multi-master support
6. **v2.0:** Kubernetes integration

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines.

## License

See [LICENSE](../LICENSE) for details.
