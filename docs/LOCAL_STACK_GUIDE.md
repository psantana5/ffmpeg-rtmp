# Local Stack Deployment Guide

This guide explains how to run the complete FFmpeg RTMP stack (master + agent) locally on a single machine for development and testing.

## Quick Start

```bash
# Run the automated script (recommended)
./scripts/run_local_stack.sh

# Or skip the test job submission
SKIP_TEST_JOB=true ./scripts/run_local_stack.sh
```

The script will:
1.  Check prerequisites (Go, Python, FFmpeg, curl)
2. 🔨 Build all binaries (master, agent, CLI)
3.  Start the master node with TLS
4.  Register and start the compute agent
5. ✔️ Verify the stack is running
6.  Submit a test job (optional)
7.  Display helpful commands and URLs

## What Gets Started

### Master Node
- **Port**: 8080 (HTTPS)
- **Endpoints**:
  - Health: `https://localhost:8080/health`
  - Nodes: `https://localhost:8080/nodes`
  - Jobs: `https://localhost:8080/jobs`
- **Metrics**: http://localhost:9090/metrics
- **Database**: SQLite (`master.db`)
- **Logs**: `logs/master.log`

### Agent Node
- **Registration**: Automatic with master
- **Mode**: Master-as-worker (development only)
- **Metrics**: http://localhost:9091/metrics
- **Logs**: `logs/agent.log`

## Prerequisites

The script checks for these automatically:

- **Go 1.21+** - For building binaries
- **Python 3.10+** - For helper scripts
- **FFmpeg** - For video transcoding
- **curl** - For API testing

## Environment Variables

The script respects these variables:

```bash
# Master API port (default: 8080)
export MASTER_PORT=8080

# Agent API port (default: 8081)
export AGENT_PORT=8081

# API key (auto-generated if not set)
export MASTER_API_KEY="your-secret-key"

# Skip test job submission
export SKIP_TEST_JOB=true
```

## Using the Stack

### Check Running Status

```bash
# List registered nodes
curl -s -k -H "Authorization: Bearer $MASTER_API_KEY" \
  https://localhost:8080/nodes | python3 -m json.tool

# List all jobs
curl -s -k -H "Authorization: Bearer $MASTER_API_KEY" \
  https://localhost:8080/jobs | python3 -m json.tool

# Check master health
curl -s -k https://localhost:8080/health
```

### Submit a Job

```bash
# Simple test job
curl -X POST -k \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  https://localhost:8080/jobs \
  -d '{
    "scenario": "test-720p",
    "confidence": "auto",
    "parameters": {
      "duration": 30,
      "bitrate": "2000k",
      "resolution": "1280x720",
      "fps": 30
    }
  }'

# Live stream job (high priority)
curl -X POST -k \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  https://localhost:8080/jobs \
  -d '{
    "scenario": "live-stream-1080p",
    "confidence": "auto",
    "queue": "live",
    "priority": "high",
    "parameters": {
      "duration": 60,
      "bitrate": "5000k"
    }
  }'
```

### Monitor Jobs

```bash
# Get specific job status
JOB_ID="<your-job-id>"
curl -s -k \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  https://localhost:8080/jobs/$JOB_ID | python3 -m json.tool

# Watch logs in real-time
tail -f logs/agent.log
tail -f logs/master.log
```

### View Prometheus Metrics

```bash
# Master metrics
curl -s http://localhost:9090/metrics | grep ffmpeg_

# Agent metrics  
curl -s http://localhost:9091/metrics | grep ffmpeg_
```

## Stopping the Stack

Press `Ctrl+C` in the terminal running the script. The cleanup handler will:
1. Stop the agent process
2. Stop the master process
3. Display cleanup confirmation

## Manual Setup (Alternative)

If you prefer to run components manually:

### 1. Build Binaries

```bash
make build-distributed
```

### 2. Start Master

```bash
export MASTER_API_KEY=$(openssl rand -base64 32)
./bin/master --port 8080 &
MASTER_PID=$!
```

### 3. Start Agent

```bash
./bin/agent \
  --register \
  --master https://localhost:8080 \
  --api-key "$MASTER_API_KEY" \
  --allow-master-as-worker \
  --skip-confirmation &
AGENT_PID=$!
```

### 4. Stop Services

```bash
kill $AGENT_PID
kill $MASTER_PID
```

## Troubleshooting

### Agent Not Registering

**Problem**: Agent fails to register with master

**Solutions**:
- Check master is running: `curl -k https://localhost:8080/health`
- Verify API key matches: `echo $MASTER_API_KEY`
- Check logs: `cat logs/agent.log`

### Port Already in Use

**Problem**: Port 8080 or 9090 already in use

**Solutions**:
```bash
# Use different ports
MASTER_PORT=8888 ./scripts/run_local_stack.sh

# Or kill existing process
lsof -ti:8080 | xargs kill -9
```

### TLS Certificate Errors

**Problem**: SSL certificate verification fails

**Solutions**:
- Use `-k` flag with curl: `curl -k https://localhost:8080/health`
- The agent automatically skips verification for localhost
- For production, use proper certificates with `--cert` and `--key` flags

### Job Not Processing

**Problem**: Jobs stay in "queued" status

**Solutions**:
- Check agent is running: `ps aux | grep agent`
- Verify node is available: `curl -k -H "Authorization: Bearer $MASTER_API_KEY" https://localhost:8080/nodes`
- Check agent logs: `tail -f logs/agent.log`

### Build Failures

**Problem**: Go build errors

**Solutions**:
```bash
# Update dependencies
go mod tidy
go mod download

# Clear Go cache
go clean -cache -modcache

# Rebuild
make build-distributed
```

## Log Files

All logs are written to the `logs/` directory:

```bash
logs/
├── master.log    # Master node logs
└── agent.log     # Agent node logs
```

View logs:
```bash
# Tail logs in real-time
tail -f logs/master.log
tail -f logs/agent.log

# Search logs
grep "error" logs/*.log
grep "job" logs/agent.log
```

## Performance Notes

### Resource Usage

Running master + agent on the same machine:
- **CPU**: Shared between orchestration and transcoding
- **Memory**: ~500MB for master, variable for agent (depends on job)
- **Disk**: Minimal (logs + SQLite database)

### Recommended Hardware

For local testing:
- **CPU**: 4+ cores
- **RAM**: 8GB+
- **Storage**: 10GB+ free space

For heavier workloads:
- **CPU**: 8+ cores
- **RAM**: 16GB+
- **GPU**: Optional (NVIDIA for hardware acceleration)

## Production Deployment

 **This local setup is for development/testing only**

For production:
1. Deploy master on dedicated server
2. Deploy agents on separate compute nodes
3. Use proper TLS certificates
4. Configure firewall rules
5. Set up monitoring (Grafana + VictoriaMetrics)
6. Use systemd services for auto-restart

See [deployment/README.md](../deployment/README.md) for production setup.

## Next Steps

- [Submit real workloads](../README.md#quick-start-production---distributed-mode)
- [Set up monitoring](../docs/GRAFANA_SETUP.md)
- [Configure priority queues](../docs/SCHEDULER_GUIDE.md)
- [Deploy to production](../deployment/README.md)

## Additional Resources

- [Architecture Overview](./ARCHITECTURE_DIAGRAM.md)
- [API Documentation](./API.md)
- [Configuration Guide](./CONFIGURATION.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)
