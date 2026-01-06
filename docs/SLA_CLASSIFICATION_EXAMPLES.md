# SLA Classification Quick Reference

Quick examples for using job classification in the FFmpeg-RTMP system.

## Production Jobs (SLA-Worthy)

### Explicit Classification
```bash
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "1080p30-h264",
    "classification": "production",
    "parameters": {"duration": 300, "bitrate": "5M"}
  }'
```

### Automatic (Default Behavior)
```bash
# Any job without classification is treated as production by default
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "4K60-h265",
    "queue": "live",
    "parameters": {"duration": 600, "bitrate": "15M"}
  }'
```

## Test Jobs (NOT SLA-Worthy)

### Explicit Test Classification
```bash
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "1080p30-h264",
    "classification": "test",
    "parameters": {"duration": 5}
  }'
```

### Automatic Test Detection (Scenario Name)
```bash
# Jobs with "test" prefix are automatically excluded from SLA
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "test-encoder-validation",
    "parameters": {"duration": 10}
  }'
```

### Automatic Test Detection (Short Duration)
```bash
# Jobs with duration < 10s are automatically excluded from SLA
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "quick-check",
    "parameters": {"duration": 3}
  }'
```

## Benchmark Jobs (NOT SLA-Worthy)

```bash
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "benchmark-h264-vs-h265",
    "classification": "benchmark",
    "parameters": {"duration": 120}
  }'
```

## Debug Jobs (NOT SLA-Worthy)

```bash
curl -X POST https://master:8080/jobs \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "debug-pipeline-issue",
    "classification": "debug",
    "parameters": {"duration": 30}
  }'
```

## Check Job Classification

### In Job Result
```json
{
  "job_id": "job-abc123",
  "status": "completed",
  "metrics": {
    "duration": 285.4,
    "sla_target_seconds": 600,
    "sla_compliant": true,
    "sla_worthy": true,
    "sla_category": "production"
  }
}
```

### In Worker Logs
```
╔════════════════════════════════════════════════════════════════╗
║ ✅ JOB COMPLETED: job-abc123
║ Total Duration: 285.40 seconds
║ SLA Target: 600 seconds
║ SLA Status: ✅ SLA MET
║ Classification: production (SLA-worthy)
╚════════════════════════════════════════════════════════════════╝
```

## Query SLA Metrics

### Only Production Jobs
```promql
# Get SLA compliance (production jobs only)
avg(ffrtmp_worker_sla_compliance_rate)

# Get SLA violations (production jobs only)
sum(ffrtmp_worker_jobs_sla_violation_total)
```

### All Jobs (Including Tests)
```promql
# Total completed jobs (all types)
sum(ffrtmp_worker_jobs_completed_total)

# Total failed jobs (all types)
sum(ffrtmp_worker_jobs_failed_total)
```

## Classification Rules Summary

| Condition | SLA-Worthy? | Auto-Category |
|-----------|-------------|---------------|
| `classification="production"` | ✅ Yes | production |
| `classification="test"` | ❌ No | test |
| `classification="benchmark"` | ❌ No | benchmark |
| `classification="debug"` | ❌ No | debug |
| `scenario` starts with "test" | ❌ No | test |
| `scenario` starts with "debug" | ❌ No | debug |
| `scenario` starts with "benchmark" | ❌ No | benchmark |
| `duration` < 10 seconds | ❌ No | test |
| `queue="batch"` AND `priority="low"` | ❌ No | batch |
| No classification specified | ✅ Yes | production |

## Best Practices

1. **Always set explicit `classification`** for production workloads
2. **Use descriptive scenario names** (avoid "test-*" prefix for production)
3. **Set appropriate durations** (>= 10s for SLA tracking)
4. **Use correct queue/priority** for business requirements
5. **Monitor SLA metrics** to ensure accurate tracking

## See Also

- [SLA Classification Guide](SLA_CLASSIFICATION.md) - Complete documentation
- [SLA Tracking Guide](SLA_TRACKING.md) - Monitoring and metrics
- [API Documentation](../README.md) - Full API reference
