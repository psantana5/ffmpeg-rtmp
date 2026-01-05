# Worker Configuration Recommender

## Overview

The Worker Configuration Recommender is available in two forms:

1. **CLI Command** (Recommended): `ffrtmp config recommend` - Built into the ffrtmp CLI tool
2. **Bash Script** (Legacy): `scripts/recommend_config.sh` - Standalone shell script

Both analyze system hardware and deployment context to generate optimal configuration parameters for the FFmpeg-RTMP worker agent.

## Purpose

Manual configuration of worker parameters can be error-prone and sub-optimal. These tools automate the process by:

1. Detecting available hardware (CPU, RAM, GPU)
2. Analyzing deployment environment characteristics
3. Calculating optimal concurrency settings
4. Generating complete, ready-to-use configuration

## Usage

### CLI Command (Recommended)

```bash
# Interactive text output with hardware details
ffrtmp config recommend

# Production environment with JSON output
ffrtmp config recommend --environment production --output json

# Generate bash exports for sourcing
ffrtmp config recommend --environment production --output bash

# YAML output for config files
ffrtmp config recommend --environment production --output yaml
```

**Output formats:**
- `text` (default) - Human-readable output with example command
- `json` - Machine-readable JSON with full hardware info
- `yaml` - YAML format for configuration files
- `bash` - Shell export statements for sourcing

### Bash Script (Legacy)

```bash
# Basic usage (production environment, bash output)
./scripts/recommend_config.sh

# Specify environment
./scripts/recommend_config.sh --environment development
./scripts/recommend_config.sh --environment staging
./scripts/recommend_config.sh --environment production

# Output formats
./scripts/recommend_config.sh --output bash
./scripts/recommend_config.sh --output json
./scripts/recommend_config.sh --output yaml
```

## Hardware Detection

The tool automatically detects:

- **CPU Cores**: Number of available processing cores
- **CPU Model**: Processor identification
- **RAM**: Total system memory
- **GPU**: Presence and type (NVIDIA, AMD, Intel)
- **Node Type**: Classification (laptop, desktop, server, HPC)
- **Container**: Whether running in Docker/container
- **Master Location**: Whether master is on localhost

## Configuration Parameters

### Concurrent Jobs

Recommended values based on hardware:

| Node Type | CPU Cores | GPU | Development | Staging/Production |
|-----------|-----------|-----|-------------|-------------------|
| Laptop    | 4-8       | No  | 1           | 2-3               |
| Laptop    | 4-8       | Yes | 2           | 4                 |
| Desktop   | 8-16      | No  | 2           | 3-4               |
| Desktop   | 8-16      | Yes | 3           | 6                 |
| Server    | 16-32     | No  | 2           | 4-8               |
| Server    | 16-32     | Yes | 6           | 12                |
| HPC       | 32+       | No  | 4           | 8-16              |
| HPC       | 32+       | Yes | 12          | 24                |

### Polling Intervals

| Environment  | Poll Interval | Heartbeat Interval |
|--------------|---------------|-------------------|
| Development  | 5s            | 30s               |
| Staging      | 5s            | 30s               |
| Production   | 3s            | 15s               |

### TLS Configuration

| Environment  | TLS | Skip Verify | mTLS |
|--------------|-----|-------------|------|
| Development  | Yes | Yes         | No   |
| Staging      | Yes | No          | Yes  |
| Production   | Yes | No          | Yes  |

### Input Generation

| Environment  | Auto-Generate Input |
|--------------|-------------------|
| Development  | Yes (for testing) |
| Staging      | Yes (for testing) |
| Production   | No (use real files) |

## Output Examples

### Bash Script Output

```bash
#!/bin/bash
#
# FFmpeg-RTMP Worker Configuration
# Generated: 2026-01-05 11:30:00
# Environment: production
# Node Type: desktop (14 cores, 16GB RAM)
# GPU: none
#

# Recommended configuration
MAX_CONCURRENT_JOBS=4
POLL_INTERVAL="3s"
HEARTBEAT_INTERVAL="15s"
METRICS_PORT=9091
MASTER_URL="https://master.example.com:8080"

# Start worker command
./bin/agent \
  --master https://master.example.com:8080 \
  --register \
  --max-concurrent-jobs 4 \
  --poll-interval 3s \
  --heartbeat-interval 15s \
  --metrics-port 9091 \
  --generate-input=false \
  --cert /path/to/worker.crt \
  --key /path/to/worker.key \
  --ca /path/to/ca.crt \
  --api-key ${FFMPEG_RTMP_API_KEY}
```

### JSON Output

```json
{
  "environment": "production",
  "hardware": {
    "cpu_cores": 14,
    "cpu_model": "Intel(R) Core(TM) Ultra 5 235U",
    "ram_gb": 16,
    "gpu": "none",
    "has_gpu": false,
    "node_type": "desktop",
    "in_container": false
  },
  "recommendations": {
    "max_concurrent_jobs": 4,
    "poll_interval": "3s",
    "heartbeat_interval": "15s",
    "metrics_port": 9091,
    "master_url": "https://master.example.com:8080",
    "use_tls": true,
    "insecure_skip_verify": false,
    "use_mtls": true,
    "generate_input": false,
    "allow_master_as_worker": false
  }
}
```

### YAML Output

```yaml
environment: production

hardware:
  cpu_cores: 14
  cpu_model: "Intel(R) Core(TM) Ultra 5 235U"
  ram_gb: 16
  gpu: "none"
  has_gpu: false
  node_type: desktop
  in_container: false

recommendations:
  max_concurrent_jobs: 4
  poll_interval: 3s
  heartbeat_interval: 15s
  metrics_port: 9091
  master_url: https://master.example.com:8080
  use_tls: true
  insecure_skip_verify: false
  use_mtls: true
  generate_input: false
  allow_master_as_worker: false
```

## Integration Examples

### CI/CD Pipeline

```yaml
# .github/workflows/deploy-worker.yml
- name: Generate Worker Configuration
  run: |
    ./scripts/recommend_config.sh \
      --environment production \
      --output json > worker-config.json
    
- name: Deploy Worker
  run: |
    MAX_JOBS=$(jq -r '.recommendations.max_concurrent_jobs' worker-config.json)
    ./bin/agent --max-concurrent-jobs $MAX_JOBS --register
```

### Ansible Playbook

```yaml
- name: Generate worker configuration
  command: ./scripts/recommend_config.sh --environment production --output json
  register: worker_config

- name: Parse configuration
  set_fact:
    config: "{{ worker_config.stdout | from_json }}"

- name: Start worker with recommended settings
  systemd:
    name: ffrtmp-worker
    state: started
  environment:
    MAX_CONCURRENT_JOBS: "{{ config.recommendations.max_concurrent_jobs }}"
    POLL_INTERVAL: "{{ config.recommendations.poll_interval }}"
```

### Docker Compose

```yaml
services:
  worker:
    image: ffmpeg-rtmp-worker:latest
    command: >
      /bin/sh -c "
      /app/scripts/recommend_config.sh --environment production > /tmp/config.sh &&
      source /tmp/config.sh &&
      exec ./bin/agent --max-concurrent-jobs $MAX_CONCURRENT_JOBS
      "
```

## Customization

### Override Recommendations

Edit the generated bash script to customize:

```bash
# Generate base configuration
./scripts/recommend_config.sh > worker-config.sh

# Edit to override specific values
sed -i 's/MAX_CONCURRENT_JOBS=4/MAX_CONCURRENT_JOBS=6/' worker-config.sh

# Execute
bash worker-config.sh
```

### Add Custom Logic

```bash
#!/bin/bash
# my-worker-start.sh

# Generate recommendations
eval $(./scripts/recommend_config.sh | grep "^MAX_CONCURRENT")

# Apply custom business logic
if [ "$HOSTNAME" = "gpu-node-01" ]; then
  MAX_CONCURRENT_JOBS=8
fi

# Start worker
./bin/agent --max-concurrent-jobs $MAX_CONCURRENT_JOBS --register
```

## Decision Logic

### Concurrent Jobs Algorithm

```
IF has_gpu THEN
  base = cores * 0.75
ELSE
  base = cores * 0.25
END IF

IF environment = "development" THEN
  concurrent_jobs = base / 2
ELSE
  concurrent_jobs = base
END IF

concurrent_jobs = min(concurrent_jobs, max_safe_limit_by_node_type)
```

### Node Type Classification

- **Laptop**: cores <= 8
- **Desktop**: cores <= 16
- **Server**: cores <= 32
- **HPC**: cores > 32

### Safety Limits

Maximum concurrent jobs regardless of cores:

- Laptop: 4 (development), 4 (production)
- Desktop: 3 (development), 6 (production)
- Server: 6 (development), 12 (production)
- HPC: 12 (development), 24 (production)

## Best Practices

### 1. Review Before Using

Always review generated configuration before production deployment:

```bash
./scripts/recommend_config.sh > review.sh
less review.sh  # Review carefully
bash review.sh  # Execute if approved
```

### 2. Test in Staging

Test recommended configuration in staging first:

```bash
./scripts/recommend_config.sh --environment staging > staging-config.sh
# Deploy to staging, monitor, adjust if needed
```

### 3. Monitor Performance

After deploying recommended configuration:

```bash
# Watch CPU usage
top

# Monitor active jobs
watch -n 2 'curl -s http://localhost:9091/metrics | grep active_jobs'

# Check for throttling
dmesg | grep -i thermal
```

### 4. Adjust for Workload

Recommendations are general-purpose. Adjust based on actual workload:

- **CPU-intensive jobs**: Reduce concurrent jobs
- **I/O-bound jobs**: Can increase concurrent jobs
- **Mixed workload**: Use recommended values

### 5. Version Control

Store configuration for each environment:

```bash
./scripts/recommend_config.sh --environment development > configs/dev-worker.sh
./scripts/recommend_config.sh --environment staging > configs/staging-worker.sh
./scripts/recommend_config.sh --environment production > configs/prod-worker.sh

git add configs/
git commit -m "Add worker configurations for all environments"
```

## Troubleshooting

### Tool Fails to Detect Hardware

If hardware detection fails:

```bash
# Manually specify values
CPU_CORES=8 ./scripts/recommend_config.sh

# Or use fallback values
./scripts/recommend_config.sh 2>/dev/null || echo "MAX_CONCURRENT_JOBS=2"
```

### Generated Configuration Doesn't Work

Check prerequisites:

```bash
# Verify binary exists
test -f ./bin/agent || echo "Worker binary not found"

# Check master connectivity
curl -k https://master.example.com:8080/health

# Test with minimal config first
./bin/agent --master https://localhost:8080 --max-concurrent-jobs 1
```

### Performance Issues

If recommended settings cause issues:

```bash
# Reduce concurrent jobs
MAX_CONCURRENT_JOBS=$((MAX_CONCURRENT_JOBS - 1))

# Increase intervals
POLL_INTERVAL="10s"
HEARTBEAT_INTERVAL="60s"

# Restart with adjusted values
./bin/agent --max-concurrent-jobs $MAX_CONCURRENT_JOBS \
  --poll-interval $POLL_INTERVAL \
  --heartbeat-interval $HEARTBEAT_INTERVAL
```

## Limitations

1. **Hardware Detection**: May not detect all GPU types accurately
2. **Network Topology**: Cannot detect network constraints
3. **Workload Characteristics**: Assumes general-purpose transcoding
4. **Shared Resources**: Doesn't account for other processes on system
5. **Thermal Constraints**: Cannot detect thermal throttling limits

## Future Enhancements

Planned improvements:

- [ ] Machine learning-based recommendations from historical data
- [ ] Network bandwidth detection and consideration
- [ ] Thermal monitoring and adaptive throttling
- [ ] Workload-specific tuning (live streaming vs batch processing)
- [ ] Cost optimization mode for cloud deployments
- [ ] Interactive mode with guided questions
- [ ] Validation of generated configuration
- [ ] Automatic performance benchmarking

## Related Documentation

- [Worker Agent README](../worker/README.md)
- [Performance Tuning Guide](./PERFORMANCE_TUNING.md)
- [Deployment Guide](./DEPLOYMENT.md)
- [Monitoring Guide](./MONITORING.md)

## Support

For issues with the configuration tool:

1. Check logs: `./scripts/recommend_config.sh 2>&1 | tee config.log`
2. Verify hardware detection: `nproc`, `free -h`, `lspci | grep VGA`
3. Open GitHub issue with system details
