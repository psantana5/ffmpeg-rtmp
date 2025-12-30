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

⚠️ **Note:** Certificate generation only creates the files. You must restart the master with `--tls` to use them (see next step).

### Start Master with TLS

**After generating certificates**, start (or restart) the master with TLS enabled:

```bash
# Option 1: Using environment variable (recommended for security)
export FFMPEG_RTMP_API_KEY="your-secure-api-key"
./bin/master --port 8080 --tls \
  --cert certs/master.crt \
  --key certs/master.key

# Option 2: Using command-line flag (visible in process list)
./bin/master --port 8080 --tls \
  --cert certs/master.crt \
  --key certs/master.key \
  --api-key "your-secure-api-key"
```

**Important:** 
- The API key set here must be provided by all agents that connect to this master.
- If the master was already running, **stop it and restart** with the new certificate.
- Using the `FFMPEG_RTMP_API_KEY` environment variable is more secure than the command-line flag.

### Connect Agent with HTTPS

For development with self-signed certificates:

```bash
# Option 1: Using environment variable (recommended for security)
export FFMPEG_RTMP_API_KEY="your-secure-api-key"
./bin/agent --register \
  --master https://192.168.0.51:8080 \
  --insecure-skip-verify

# Option 2: Using command-line flag (visible in process list)
./bin/agent --register \
  --master https://192.168.0.51:8080 \
  --api-key "your-secure-api-key" \
  --insecure-skip-verify
```

⚠️ **Warning:** `--insecure-skip-verify` disables certificate validation. Only use in development!

For production with proper CA certificates:

```bash
# Using environment variable (recommended)
export FFMPEG_RTMP_API_KEY="your-secure-api-key"
./bin/agent --register \
  --master https://192.168.0.51:8080 \
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
# Solution 1: Regenerate certificate with correct hostname/IP
./bin/master --generate-cert --cert-hosts "your-hostname" --cert-ips "your-ip"

# Solution 2: IMPORTANT - Restart the master with --tls flag to use the new certificate
./bin/master --port 8080 --tls --cert certs/master.crt --key certs/master.key

# Error: "certificate signed by unknown authority"
# Solution: Use --insecure-skip-verify for development, or provide --ca for production
./bin/agent --register --master https://server:8080 --insecure-skip-verify
```

**Authentication errors:**
```bash
# Error: "Missing Authorization header" or "registration failed with status 401"
# Solution: Provide the same API key that the master was started with
export FFMPEG_RTMP_API_KEY="your-secure-api-key"
./bin/agent --register --master https://server:8080 --insecure-skip-verify

# Or using command-line flag:
./bin/agent --register --master https://server:8080 --api-key "your-secure-api-key" --insecure-skip-verify

# Error: "Invalid API key"
# Solution: Make sure the agent's API key matches the master's API key exactly
```

**Connection successful checklist:**
When the agent successfully registers, you should see:
```
✓ Registered successfully!
  Node ID: <uuid>
  Status: active
```
