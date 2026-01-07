# FFmpeg Auto-Attach Quick Reference

## Quick Start

```bash
# Start auto-discovery daemon
ffrtmp watch

# Run FFmpeg with resource limits
ffrtmp run --job-id job-001 --cpu-quota 200 --memory-limit 4096 -- ffmpeg -i input.mp4 output.mp4

# Attach to existing process
ffrtmp attach --pid 12345 --job-id job-001
```

## Commands

| Command | Purpose | Use Case |
|---------|---------|----------|
| `ffrtmp run` | Spawn new process with governance | Manual job submission |
| `ffrtmp attach` | Attach to existing process | External process governance |
| `ffrtmp watch` | Auto-discover and govern | Production edge nodes |

## Common Flags

### Resource Limits
- `--cpu-quota INT` - CPU percentage (100=1 core, 200=2 cores)
- `--cpu-weight INT` - CPU scheduling weight (1-10000)
- `--memory-limit INT` - Memory limit in MB
- `--nice INT` - Process priority (-20 to 19)

### Identification
- `--job-id STRING` - Job identifier
- `--sla-eligible` - Mark for SLA tracking
- `--pid INT` - Process ID (attach only)

### Watch Configuration
- `--scan-interval DURATION` - How often to scan (default: 10s)
- `--target STRING` - Command names to discover (default: ffmpeg, gst-launch-1.0)

## Examples

### Production Edge Node
```bash
# Start watch daemon with limits
ffrtmp watch \
  --scan-interval 5s \
  --cpu-quota 150 \
  --memory-limit 2048
```

### High-Priority Live Stream
```bash
# Run with elevated priority
ffrtmp run \
  --job-id live-stream \
  --sla-eligible \
  --cpu-quota 400 \
  --memory-limit 8192 \
  -- ffmpeg -i rtmp://source -c copy rtmp://destination
```

### Govern External Process
```bash
# Find process
ps aux | grep ffmpeg

# Attach with limits
ffrtmp attach \
  --pid 12345 \
  --job-id external-001 \
  --cpu-weight 75 \
  --nice 5
```

## Resource Limit Guidelines

### CPU Quota
- **Light workloads**: 50-100 (0.5-1 core)
- **Standard transcoding**: 100-200 (1-2 cores)
- **Heavy workloads**: 200-400 (2-4 cores)
- **Unlimited**: 0

### Memory Limit
- **480p transcoding**: 512-1024 MB
- **720p transcoding**: 1024-2048 MB
- **1080p transcoding**: 2048-4096 MB
- **4K transcoding**: 4096-8192 MB

### CPU Weight
- **Low priority**: 25-50
- **Normal priority**: 100 (default)
- **High priority**: 150-200
- **Critical**: 200+

### Nice Value
- **Low priority**: 10-19
- **Normal**: 0 (default)
- **High priority**: -5 to -1
- **Critical**: -10 to -6 (requires privileges)

## Output Formats

### Standard Output
```
Job: transcode-001
PID: 12345
Exit Code: 0
Duration: 5.23s
Platform SLA: true (completed_successfully)
```

### JSON Output
```bash
ffrtmp run --json --job-id job-001 -- echo test
```

```json
{
  "job_id": "job-001",
  "pid": 12345,
  "exit_code": 0,
  "duration": 5.23,
  "platform_sla": true,
  "platform_sla_reason": "completed_successfully"
}
```

## SLA Reasons

| Reason | Meaning |
|--------|---------|
| `completed_successfully` | Process exited 0 |
| `observed_to_completion` | Attached process completed |
| `detached_workload_continues` | Wrapper detached, process still running |
| `non_zero_exit` | Process failed |

## Troubleshooting

### Nice value fails
```
Warning: failed to set nice value: permission denied
```
**Solution**: Run with `sudo` or adjust limits in `/etc/security/limits.conf`

### Cgroup creation fails
```
WARNING: Failed to create cgroup: permission denied
```
**Solution**: Ensure user has write access to `/sys/fs/cgroup` or run with appropriate permissions

### Process not discovered
Check target commands match:
```bash
ffrtmp watch --target ffmpeg --target my-encoder
```

## Integration Examples

### Systemd Service
```ini
[Unit]
Description=FFmpeg Auto-Attach Daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ffrtmp watch --scan-interval 10s --cpu-quota 150
Restart=always
User=ffmpeg

[Install]
WantedBy=multi-user.target
```

### Docker Compose
```yaml
services:
  ffmpeg-watcher:
    image: ffmpeg-rtmp:latest
    command: ffrtmp watch --scan-interval 5s
    privileged: true
    volumes:
      - /sys/fs/cgroup:/sys/fs/cgroup:rw
      - /proc:/proc:ro
```

### Kubernetes DaemonSet
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ffmpeg-watcher
spec:
  template:
    spec:
      hostPID: true
      containers:
      - name: watcher
        image: ffmpeg-rtmp:latest
        command: ["ffrtmp", "watch"]
        securityContext:
          privileged: true
```

## Performance Tips

1. **Scan Interval**: Balance between responsiveness and CPU overhead
   - Fast detection: 3-5s
   - Normal: 10s (default)
   - Low overhead: 30-60s

2. **CPU Quota**: Leave headroom for system processes
   - Example: 8-core system → max 600-700% quota

3. **Memory Limits**: Set based on input resolution, not duration
   - HD streams: 2GB
   - 4K streams: 8GB

4. **Multiple Processes**: Use watch mode instead of manual attachment
   - Automatically handles process lifecycle
   - No manual tracking needed

## See Also

- Full documentation: `docs/AUTO_ATTACH.md`
- Worker integration: `worker/README.md`
- API reference: `docs/api/`
