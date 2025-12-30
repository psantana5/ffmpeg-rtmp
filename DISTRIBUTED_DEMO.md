# Distributed Compute Demo

This guide demonstrates the complete distributed compute workflow.

## Architecture

```
┌─────────────────────────────────────────┐
│         Master Node (Port 8080)         │
│  • Job Queue                            │
│  • Node Registry                        │
│  • Results Collection                   │
└────────────┬────────────────────────────┘
             │
      ┌──────┴──────┐
      │             │
┌─────▼─────┐ ┌────▼──────┐
│  Worker 1 │ │  Worker N │
│  (Agent)  │ │  (Agent)  │
└───────────┘ └───────────┘
```

## Demo Scenario

We'll demonstrate:
1. Starting a master node
2. Registering multiple worker nodes
3. Creating and dispatching jobs
4. Collecting results
5. Monitoring the system

## Step-by-Step Demo

### Step 1: Build the Binaries

```bash
make build-distributed
```

Output:
```
Building master node...
✓ Master binary created: bin/master

Building compute agent...
✓ Agent binary created: bin/agent
```

### Step 2: Start the Master Node

```bash
./bin/master --port 8080
```

You'll see:
```
Starting FFmpeg RTMP Distributed Master Node
Port: 8080
Master node listening on :8080
API endpoints:
  POST   /nodes/register
  GET    /nodes
  POST   /nodes/{id}/heartbeat
  POST   /jobs
  GET    /jobs
  GET    /jobs/next?node_id=<id>
  POST   /results
  GET    /health
```

### Step 3: Verify Master Health

In a new terminal:
```bash
curl -k https://localhost:8080/health
```

Response:
```json
{"status":"healthy"}
```

### Step 4: Register Worker Nodes

#### Development Mode (Master as Worker)

```bash
./bin/agent --register --master https://localhost:8080 --allow-master-as-worker
```

You'll see a warning:
```
⚠️  WARNING: Master node detected as worker!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
You are attempting to register the master node as a compute worker.
This configuration is intended for DEVELOPMENT/TESTING ONLY.

Risks:
  • Master and worker compete for CPU/memory resources
  • Heavy workloads may impact master API responsiveness
  • Not recommended for production environments

Recommended: Run workers on separate machines in production.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Do you want to continue? (yes/no):
```

Type `yes` to continue.

#### Production Mode (Separate Workers)

On worker machine:
```bash
./bin/agent --register --master http://MASTER_IP:8080
```

No warning! Agent will register and start polling.

### Step 5: List Registered Nodes

```bash
curl -k https://localhost:8080/nodes | jq
```

Response:
```json
{
  "nodes": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "address": "worker-01",
      "type": "desktop",
      "cpu_threads": 8,
      "cpu_model": "AMD EPYC 7763 64-Core Processor",
      "has_gpu": false,
      "ram_bytes": 16777216000,
      "labels": {
        "arch": "amd64",
        "node_type": "desktop",
        "os": "linux"
      },
      "status": "available",
      "last_heartbeat": "2025-12-29T19:45:00Z",
      "registered_at": "2025-12-29T19:40:00Z"
    }
  ],
  "count": 1
}
```

### Step 6: Create Jobs

```bash
# Create a 1080p test job
curl -k -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "1080p-h264",
    "confidence": "auto",
    "parameters": {
      "duration": 60,
      "bitrate": "5000k"
    }
  }' | jq
```

Response:
```json
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "scenario": "1080p-h264",
  "confidence": "auto",
  "parameters": {
    "bitrate": "5000k",
    "duration": 60
  },
  "status": "pending",
  "created_at": "2025-12-29T19:46:00Z",
  "retry_count": 0
}
```

### Step 7: Create Multiple Jobs

```bash
# 720p job
curl -k -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "720p-h264",
    "confidence": "auto"
  }'

# 4K job
curl -k -X POST https://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "4K60-h264",
    "confidence": "high"
  }'
```

### Step 8: Watch Jobs Get Executed

Agents automatically poll and execute jobs. Check the agent logs to see:
```
Received job: 660e8400-e29b-41d4-a716-446655440001 (scenario: 1080p-h264)
Executing job 660e8400-e29b-41d4-a716-446655440001...
Job 660e8400-e29b-41d4-a716-446655440001 completed successfully
Results sent for job 660e8400-e29b-41d4-a716-446655440001 (status: completed)
```

### Step 9: Monitor Job Status

```bash
curl -k https://localhost:8080/jobs | jq
```

You'll see jobs with status:
- `pending` - waiting for worker
- `running` - currently executing
- `completed` - finished successfully
- `failed` - execution failed

### Step 10: Run Integration Test

Automated test that validates the entire workflow:
```bash
./test_distributed.sh
```

Output:
```
==================================
Distributed Compute Integration Test
==================================

1. Checking master health...
   ✓ Master is healthy

2. Registering test node...
   ✓ Node registered with ID: ...

3. Listing registered nodes...
   ✓ Found 1 registered node

4. Creating test job...
   ✓ Job created with ID: ...

5. Getting next job for node...
   ✓ Job assigned to node

6. Sending job results...
   ✓ Results sent successfully

7. Checking for additional jobs...
   ✓ No more jobs available (as expected)

==================================
✓ All integration tests passed!
==================================
```

## Production Deployment

### Master Node (e.g., 192.168.1.100)

```bash
# Start master
./bin/master --port 8080

# Or with systemd
sudo systemctl start ffmpeg-master
```

### Worker Nodes

```bash
# On each worker
./bin/agent \
  --register \
  --master http://192.168.1.100:8080 \
  --poll-interval 10s \
  --heartbeat-interval 30s

# Or with systemd
sudo systemctl start ffmpeg-agent
```

### With Docker (Future)

```bash
# Master
docker run -p 8080:8080 ffmpeg-rtmp/master

# Agent
docker run ffmpeg-rtmp/agent \
  --master https://master:8080 \
  --register
```

## Monitoring

### Check System Status

```bash
# Nodes
curl -k https://localhost:8080/nodes | jq '.count'

# Jobs
curl -k https://localhost:8080/jobs | jq '.count'

# Filter by status
curl -k https://localhost:8080/jobs | jq '.jobs[] | select(.status=="running")'
```

### Health Checks

```bash
# Master
curl -k https://localhost:8080/health

# Should return: {"status":"healthy"}
```

## Troubleshooting

### Agent Can't Connect

```bash
# Check master is running
curl http://MASTER_IP:8080/health

# Check network
ping MASTER_IP

# Check firewall
sudo ufw allow 8080/tcp
```

### No Jobs Being Executed

```bash
# Check nodes are registered and available
curl -k https://localhost:8080/nodes | jq '.nodes[] | {id, status}'

# Check jobs exist
curl -k https://localhost:8080/jobs | jq '.count'
```

### Master-as-Worker Blocked

Use the `--allow-master-as-worker` flag in development:
```bash
./bin/agent --register --master https://localhost:8080 --allow-master-as-worker
```

## Next Steps

- See [distributed_architecture_v1.md](docs/distributed_architecture_v1.md) for full architecture
- See [IMPLEMENTATION_NOTES.md](docs/IMPLEMENTATION_NOTES.md) for technical details
- See [QUICKSTART_DISTRIBUTED.md](QUICKSTART_DISTRIBUTED.md) for quick start

## Video Demo

(Future: Add link to screencast demonstration)
