# Distributed Compute Quick Start

This guide shows how to quickly set up and test the distributed compute system.

## Quick Test (Development Mode)

### 1. Build the binaries

```bash
make build-distributed
```

This creates:
- `bin/master` - Master node service
- `bin/agent` - Compute agent

### 2. Start the master node

```bash
./bin/master --port 8080
```

You should see:
```
Starting FFmpeg RTMP Distributed Master Node
Port: 8080
Master node listening on :8080
API endpoints:
  POST   /nodes/register
  GET    /nodes
  ...
```

### 3. Register a worker (development mode)

In a new terminal:

```bash
./bin/agent --register --master http://localhost:8080 --allow-master-as-worker
```

⚠️ You'll see a warning about registering master as worker. Type `yes` to continue.

The agent will:
- Detect your hardware (CPU, GPU, RAM)
- Register with the master
- Start polling for jobs

### 4. Create a test job

In a third terminal:

```bash
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-1080p",
    "confidence": "auto",
    "parameters": {
      "duration": 60,
      "bitrate": "5000k"
    }
  }'
```

The agent will pick up and execute the job automatically!

### 5. Monitor progress

```bash
# List registered nodes
curl http://localhost:8080/nodes | jq

# List jobs
curl http://localhost:8080/jobs | jq

# Check health
curl http://localhost:8080/health
```

## Production Deployment

For production, run workers on separate machines:

### On Master Node (e.g., 192.168.1.100)

```bash
./bin/master --port 8080
```

### On Worker Node 1

```bash
./bin/agent --register --master http://192.168.1.100:8080
```

### On Worker Node 2

```bash
./bin/agent --register --master http://192.168.1.100:8080
```

No `--allow-master-as-worker` flag needed in production!

## Testing with Script

Run the integration test:

```bash
./test_distributed.sh
```

This tests:
- Node registration
- Job creation
- Job dispatch
- Results submission

## Architecture

```
Master (port 8080)
    ↓
    ├─ Node Registry
    ├─ Job Queue
    └─ Results Collection
    
Workers (poll every 10s)
    ↓
    ├─ Hardware Detection
    ├─ Job Execution
    └─ Results Upload
```

## Next Steps

- See [docs/distributed_architecture_v1.md](docs/distributed_architecture_v1.md) for full documentation
- Add mTLS for production security (see below)
- Implement advanced scheduling
- Connect to existing FFmpeg workflows

## TLS/HTTPS Setup (Production)

For secure production deployments, use HTTPS with TLS certificates:

### Generate Self-Signed Certificate

On the master node, generate a certificate with your server's IP address and hostname:

```bash
./bin/master --generate-cert \
  --cert-ips "192.168.0.51,10.0.0.5" \
  --cert-hosts "depa,master-node" \
  --cert certs/master.crt \
  --key certs/master.key
```

This creates a certificate valid for:
- DNS names: `master`, `localhost`, `depa`, `master-node`
- IP addresses: `127.0.0.1`, `::1`, `192.168.0.51`, `10.0.0.5`

### Start Master with TLS

```bash
./bin/master --port 8080 --tls \
  --cert certs/master.crt \
  --key certs/master.key \
  --api-key "your-secure-api-key"
```

### Connect Agent with HTTPS

For development with self-signed certificates:

```bash
./bin/agent --register \
  --master https://192.168.0.51:8080 \
  --api-key "your-secure-api-key" \
  --insecure-skip-verify
```

⚠️ **Warning:** `--insecure-skip-verify` disables certificate validation. Only use in development!

For production with proper CA certificates:

```bash
./bin/agent --register \
  --master https://192.168.0.51:8080 \
  --api-key "your-secure-api-key" \
  --ca certs/ca.crt
```

### Mutual TLS (mTLS)

For maximum security, require client certificates:

1. Generate client certificates for each agent
2. Start master with `--mtls --ca certs/ca.crt`
3. Connect agents with `--cert agent.crt --key agent.key`

## Troubleshooting

**Agent can't connect:**
```bash
# Check master is running
curl http://localhost:8080/health

# Check network
ping master-ip
```

**Master-as-worker blocked:**
```bash
# Use the allow flag in development
./bin/agent --register --master http://localhost:8080 --allow-master-as-worker
```

**TLS certificate errors:**
```bash
# Error: "certificate is valid for X, not Y"
# Solution: Regenerate certificate with correct hostname/IP
./bin/master --generate-cert --cert-hosts "your-hostname" --cert-ips "your-ip"

# Error: "certificate signed by unknown authority"
# Solution: Use --insecure-skip-verify for development, or provide --ca for production
./bin/agent --register --master https://server:8080 --insecure-skip-verify
```
