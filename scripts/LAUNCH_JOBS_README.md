# Job Launcher Script - Production Guide

## Overview

`launch_jobs.sh` is a production-grade script for submitting large batches of jobs to the ffmpeg-rtmp distributed transcoding system. It includes comprehensive error handling, progress monitoring, batch processing, and detailed reporting.

## Features

- ✅ **Batch Processing**: Submit jobs in configurable batches to avoid overwhelming the master
- ✅ **Progress Monitoring**: Real-time progress bar and detailed logging
- ✅ **Error Handling**: Graceful error recovery and detailed failure reporting
- ✅ **Health Checks**: Pre-flight verification of master server availability
- ✅ **Flexible Configuration**: Support for random or fixed job parameters
- ✅ **Dry Run Mode**: Test without actually submitting jobs
- ✅ **JSON Output**: Machine-readable results for automation
- ✅ **Performance Metrics**: Submission rate and timing statistics

## Quick Start

### Basic Usage

Submit 1000 jobs with default settings:

```bash
./scripts/launch_jobs.sh
```

### Common Examples

#### Submit 100 jobs for testing
```bash
./scripts/launch_jobs.sh --count 100
```

#### Submit to a specific master server
```bash
./scripts/launch_jobs.sh --master https://master.example.com:8080
```

#### Submit with specific scenario
```bash
./scripts/launch_jobs.sh --count 500 --scenario "4K60-h264" --priority high
```

#### High-volume submission with tuned batching
```bash
./scripts/launch_jobs.sh \
  --count 10000 \
  --batch-size 100 \
  --delay 50 \
  --output large_batch_results.json
```

#### Mixed workload (production-like)
```bash
./scripts/launch_jobs.sh \
  --count 1000 \
  --scenario random \
  --priority mixed \
  --queue mixed \
  --engine auto
```

#### Dry run to test configuration
```bash
./scripts/launch_jobs.sh --count 100 --dry-run --verbose
```

## Command-Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `--count N` | Number of jobs to submit | 1000 |
| `--master URL` | Master server URL | http://localhost:8080 |
| `--scenario NAME` | Job scenario (see list below) | random |
| `--batch-size N` | Jobs per batch | 50 |
| `--delay MS` | Milliseconds between batches | 100 |
| `--priority LEVEL` | Priority: high, medium, low, mixed | mixed |
| `--queue TYPE` | Queue: live, default, batch, mixed | mixed |
| `--engine ENGINE` | Engine: auto, ffmpeg, gstreamer | auto |
| `--output FILE` | Output file for results | job_launch_results.json |
| `--dry-run` | Test mode without submission | false |
| `--verbose` | Enable debug logging | false |
| `--help` | Show help message | - |

## Job Scenarios

The script supports the following predefined scenarios:

- `4K60-h264` - 4K resolution at 60fps using H.264
- `4K60-h265` - 4K resolution at 60fps using H.265
- `4K30-h264` - 4K resolution at 30fps using H.264
- `1080p60-h264` - Full HD at 60fps
- `1080p30-h264` - Full HD at 30fps
- `720p60-h264` - HD at 60fps
- `720p30-h264` - HD at 30fps
- `480p30-h264` - SD at 30fps
- `random` - Randomly select from all scenarios

## Parameter Randomization

When using `random` or `mixed` values, the script automatically varies parameters:

### Random Parameters per Job
- **Duration**: 30-300 seconds
- **Bitrate**: Appropriate for scenario resolution
  - 4K: 10-25 Mbps
  - 1080p: 4-10 Mbps
  - 720p: 2-5 Mbps
  - 480p: 1-3 Mbps
- **Priority**: Randomly distributed (when set to "mixed")
- **Queue**: Randomly distributed (when set to "mixed")

## Output Format

The script generates a JSON file with detailed results:

```json
[
  {
    "id": "job-uuid-1",
    "sequence_number": 1,
    "scenario": "4K60-h264",
    "confidence": "auto",
    "status": "queued",
    "queue": "default",
    "priority": "medium",
    "created_at": "2026-01-05T10:30:00Z"
  },
  {
    "id": "job-uuid-2",
    "sequence_number": 2,
    ...
  }
]
```

### Parsing Results

Extract job IDs:
```bash
jq -r '.[] | select(.id != null) | .id' job_launch_results.json
```

Count successful submissions:
```bash
jq '[.[] | select(.error == null)] | length' job_launch_results.json
```

Count failures:
```bash
jq '[.[] | select(.error != null)] | length' job_launch_results.json
```

## Performance Tuning

### For Maximum Throughput

```bash
./scripts/launch_jobs.sh \
  --count 10000 \
  --batch-size 200 \
  --delay 20
```

### For Stability (Large Jobs)

```bash
./scripts/launch_jobs.sh \
  --count 5000 \
  --batch-size 25 \
  --delay 200
```

### For Testing/Development

```bash
./scripts/launch_jobs.sh \
  --count 10 \
  --batch-size 1 \
  --delay 1000 \
  --verbose
```

## Monitoring During Execution

The script provides:

1. **Real-time Progress Bar**
   ```
   [INFO] Progress: [=========================                         ] 50% (500/1000)
   ```

2. **Batch Completion Logs** (in verbose mode)
   ```
   [DEBUG] Completed batch 10, sleeping 100ms...
   ```

3. **Error Notifications**
   ```
   [ERROR] Job #532 failed with HTTP 503: Service temporarily unavailable
   ```

## Integration Examples

### Automated Load Testing

```bash
#!/bin/bash
# Run daily load test at 2 AM
0 2 * * * cd /opt/ffmpeg-rtmp && ./scripts/launch_jobs.sh \
  --count 5000 \
  --output "/var/log/ffmpeg-rtmp/jobs_$(date +\%Y\%m\%d).json"
```

### CI/CD Pipeline Integration

```yaml
# GitHub Actions example
- name: Load Test with 100 Jobs
  run: |
    ./scripts/launch_jobs.sh \
      --count 100 \
      --master ${{ secrets.MASTER_URL }} \
      --output test_results/load_test.json
    
- name: Verify Job Submission
  run: |
    success_count=$(jq '[.[] | select(.error == null)] | length' test_results/load_test.json)
    if [ "$success_count" -lt 95 ]; then
      echo "Too many failures: only $success_count/100 succeeded"
      exit 1
    fi
```

### Benchmarking Script

```bash
#!/bin/bash
# Benchmark different batch sizes
for batch_size in 10 25 50 100 200; do
  echo "Testing batch size: $batch_size"
  ./scripts/launch_jobs.sh \
    --count 1000 \
    --batch-size "$batch_size" \
    --output "benchmark_${batch_size}.json"
  sleep 60  # Cool down between tests
done
```

## Troubleshooting

### Master Server Not Responding

```
[ERROR] Master server health check failed at http://localhost:8080/health
```

**Solution**: 
- Verify master is running: `docker compose ps master`
- Check master logs: `docker compose logs master`
- Verify URL is correct

### High Failure Rate

```
[WARN] Total failed: 450
```

**Solution**:
- Increase `--delay` to reduce load
- Decrease `--batch-size` for gentler submission
- Check master server resources (CPU, memory)
- Review error details in output JSON

### Script Exits Prematurely

**Solution**:
- Use `--verbose` to see detailed logs
- Check for network connectivity issues
- Verify script has execute permissions: `chmod +x scripts/launch_jobs.sh`

## Best Practices

1. **Start Small**: Test with `--count 10` before large batches
2. **Use Dry Run**: Verify configuration with `--dry-run` first
3. **Monitor Resources**: Watch master server CPU/memory during submission
4. **Tune Batching**: Adjust `--batch-size` and `--delay` based on your infrastructure
5. **Save Results**: Always specify `--output` for audit trails
6. **Health Check First**: Manually verify `curl $MASTER_URL/health` before large batches

## Advanced Usage

### Environment Variables

Set default master URL:
```bash
export MASTER_URL="https://production-master.example.com:8080"
./scripts/launch_jobs.sh --count 1000
```

### Parallel Execution

Submit to multiple masters simultaneously:
```bash
./scripts/launch_jobs.sh --master http://master1:8080 --count 500 --output master1.json &
./scripts/launch_jobs.sh --master http://master2:8080 --count 500 --output master2.json &
wait
```

### Custom Scenarios via Modification

Edit the `SCENARIOS` array in the script to add custom scenarios:

```bash
SCENARIOS=(
    "4K60-h264"
    "custom-8K-h265"     # Add your custom scenario
    "custom-HDR-av1"      # Another custom scenario
)
```

## Requirements

- **bash** 4.0+
- **curl** for HTTP requests
- **jq** (optional) for JSON parsing
- **Running master server** with accessible API

## Support

For issues or questions:
1. Check the main project README
2. Review master server logs
3. Open an issue on GitHub with:
   - Script output (with `--verbose`)
   - Master server version
   - Output JSON file

## Version History

- **v1.0.0** (2026-01-05): Initial production release
  - Batch processing
  - Progress monitoring
  - Error handling
  - JSON output
