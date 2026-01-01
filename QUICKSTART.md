# FFmpeg-RTMP Distributed System - Quick Start Guide

## Overview

This guide will help you quickly start the distributed transcoding system with optimal configuration.

## Prerequisites

- Go 1.21+ installed
- FFmpeg installed
- Linux/macOS environment
- Ports 8080, 9090, 9091 available

## Quick Start

### 1. Build Binaries (if needed)

```bash
make build-distributed
```

This builds:
- `bin/master` - Master node coordinator
- `bin/agent` - Worker agent
- `bin/ffrtmp` - CLI tool

### 2. Start the System

Use the automated startup script:

```bash
./scripts/start-distributed.sh
```

This script:
- ✓ Checks and builds missing binaries
- ✓ Generates TLS certificates
- ✓ Starts master node with optimal settings
- ✓ Starts worker agent
- ✓ Verifies node registration
- ✓ Displays system status

### 3. Verify System Status

```bash
# List registered nodes
./bin/ffrtmp nodes list

# Check system health
curl -k https://localhost:8080/health
```

## Using the CLI

### Submit Jobs

```bash
# Submit a test job
./bin/ffrtmp jobs submit --scenario test-720p --duration 30

# Submit with custom parameters
./bin/ffrtmp jobs submit --scenario 4K60-h264 --bitrate 10M --duration 60
```

### Monitor Jobs

```bash
# List all jobs
./bin/ffrtmp jobs status

# Get specific job status (by sequence number)
./bin/ffrtmp jobs status 1

# Follow job progress
./bin/ffrtmp jobs status 1 --follow

# Get job logs
./bin/ffrtmp jobs logs 1
```

### Job Control

```bash
# Cancel a job
./bin/ffrtmp jobs cancel 1

# Pause a running job
./bin/ffrtmp jobs pause 1

# Resume a paused job
./bin/ffrtmp jobs resume 1

# Retry a failed job
./bin/ffrtmp jobs retry 1
```

## Configuration

### Environment Variables

```bash
# Master API key (auto-generated if not set)
export MASTER_API_KEY="your-secret-key"

# Master configuration
export MASTER_PORT="8080"
export DB_PATH="master.db"
export MAX_RETRIES="3"
export ENABLE_METRICS="true"
export METRICS_PORT="9090"

# Agent configuration
export HEARTBEAT_INTERVAL="30s"
export POLL_INTERVAL="10s"
export AGENT_METRICS_PORT="9091"
```

### Default Ports

- **8080** - Master API
- **9090** - Master metrics
- **9091** - Agent metrics

## System Architecture

```
┌─────────────────┐
│  Master Node    │
│  Port: 8080     │ ← Job submission, coordination
│  Metrics: 9090  │
└────────┬────────┘
         │
         ├── Job Queue
         │   ├── Live queue (highest priority)
         │   ├── Default queue
         │   └── Batch queue
         │
         ↓
┌─────────────────┐
│  Worker Agent   │
│  Polls: 10s     │ ← Job execution
│  Heartbeat: 30s │
│  Metrics: 9091  │
└─────────────────┘
```

## Monitoring

### Prometheus Metrics

Master metrics:
```bash
curl -s http://localhost:9090/metrics
```

Agent metrics:
```bash
curl -s http://localhost:9091/metrics
```

### Logs

View real-time logs:
```bash
# Both services
tail -f logs/master.log logs/agent.log

# Master only
tail -f logs/master.log

# Agent only
tail -f logs/agent.log
```

## Stopping the System

```bash
# Stop all components
pkill -f 'bin/(master|agent)'

# Or stop individually
pkill -f 'bin/master'
pkill -f 'bin/agent'
```

## Troubleshooting

### Master won't start

1. Check if port 8080 is in use:
   ```bash
   lsof -i :8080
   ```

2. Check logs:
   ```bash
   cat logs/master.log
   ```

3. Verify certificates exist:
   ```bash
   ls -la certs/master.*
   ```

### Agent won't register

1. Verify master is running:
   ```bash
   curl -k https://localhost:8080/health
   ```

2. Check agent logs:
   ```bash
   tail -20 logs/agent.log
   ```

3. Verify CA certificate:
   ```bash
   ls -la certs/agent.* certs/master.crt
   ```

### Jobs stuck in pending

1. Check node availability:
   ```bash
   ./bin/ffrtmp nodes list
   ```

2. Check agent logs for errors:
   ```bash
   grep -i error logs/agent.log
   ```

3. Verify job status:
   ```bash
   ./bin/ffrtmp jobs status <job-number>
   ```

### Can't query job logs

The system supports both UUID and sequence number lookups:

```bash
# By sequence number (recommended)
./bin/ffrtmp jobs logs 1

# By UUID (if you have it)
./bin/ffrtmp jobs logs 3dc22174-7bbf-47ca-b158-841e6323fc4a
```

## Advanced Usage

### Custom Scenarios

Edit the scenario configuration in the database or submit custom parameters:

```bash
./bin/ffrtmp jobs submit \
  --scenario custom \
  --duration 120 \
  --bitrate 8M \
  --engine ffmpeg \
  --queue live \
  --priority high
```

### Multiple Workers

Start additional agents on different machines:

```bash
# On worker machine
./bin/agent \
  --master "https://master-host:8080" \
  --ca /path/to/master.crt \
  --cert /path/to/agent.crt \
  --key /path/to/agent.key \
  --register
```

### High Availability

Configure multiple master nodes with shared database (PostgreSQL recommended for production).

## Development Mode

For development with less strict security:

```bash
# Start without TLS verification (not for production!)
./bin/agent \
  --master "https://localhost:8080" \
  --insecure-skip-verify \
  --register
```

## Next Steps

- Review [INTEGRATION_SUMMARY.md](INTEGRATION_SUMMARY.md) for architecture details
- Check [INTEGRATION_QUICKREF.md](INTEGRATION_QUICKREF.md) for CLI reference
- See [README.md](README.md) for full documentation

## Support

- Check logs in `logs/` directory
- Review scenarios in the database
- Monitor metrics endpoints
- File issues on GitHub
