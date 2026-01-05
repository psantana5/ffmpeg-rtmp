# SLA Tracking Guide

This document describes the Service Level Agreement (SLA) tracking and monitoring capabilities in the FFmpeg-RTMP distributed transcoding system.

## Overview

SLA tracking automatically monitors job execution performance against defined service level targets. The system tracks:

- **Job completion time**: Whether jobs finish within expected duration
- **Success rate**: Percentage of jobs completed successfully vs failed
- **SLA compliance**: Percentage of jobs meeting SLA targets
- **Trend analysis**: Historical SLA performance over time

## Default SLA Targets

The system uses the following default SLA targets:

| Metric | Default Target | Description |
|--------|---------------|-------------|
| **Max Duration** | 600 seconds (10 minutes) | Jobs must complete within this time |
| **Max Failure Rate** | 5% | No more than 5% of jobs should fail |
| **Min Compliance Rate** | 95% | At least 95% of jobs should meet SLA |

## Prometheus Metrics

### Job Completion Metrics

#### `ffrtmp_worker_jobs_completed_total`
**Type**: Counter  
**Description**: Total number of jobs completed successfully on this worker  
**Labels**: `node_id`

Example:
```promql
ffrtmp_worker_jobs_completed_total{node_id="worker-1:9091"} 1523
```

#### `ffrtmp_worker_jobs_failed_total`
**Type**: Counter  
**Description**: Total number of jobs that failed on this worker  
**Labels**: `node_id`

Example:
```promql
ffrtmp_worker_jobs_failed_total{node_id="worker-1:9091"} 47
```

### SLA Compliance Metrics

#### `ffrtmp_worker_jobs_sla_compliant_total`
**Type**: Counter  
**Description**: Total number of jobs completed within SLA duration target  
**Labels**: `node_id`

Example:
```promql
ffrtmp_worker_jobs_sla_compliant_total{node_id="worker-1:9091"} 1450
```

#### `ffrtmp_worker_jobs_sla_violation_total`
**Type**: Counter  
**Description**: Total number of jobs that exceeded SLA duration target  
**Labels**: `node_id`

Example:
```promql
ffrtmp_worker_jobs_sla_violation_total{node_id="worker-1:9091"} 73
```

#### `ffrtmp_worker_sla_compliance_rate`
**Type**: Gauge  
**Description**: Current SLA compliance rate as percentage (0-100)  
**Calculation**: `(jobs_sla_compliant / (jobs_sla_compliant + jobs_sla_violation)) * 100`  
**Labels**: `node_id`

Example:
```promql
ffrtmp_worker_sla_compliance_rate{node_id="worker-1:9091"} 95.21
```

## Job Result Metrics

SLA information is included in every job result:

```json
{
  "job_id": "job-abc123",
  "status": "completed",
  "metrics": {
    "duration": 543.2,
    "sla_target_seconds": 600,
    "sla_compliant": true,
    "scenario": "1080p-h264",
    "engine": "ffmpeg"
  }
}
```

### Metric Fields

- **`sla_target_seconds`**: The SLA duration target that was applied (default: 600)
- **`sla_compliant`**: Boolean indicating if job met SLA (`true` if duration ≤ target)

## Prometheus Queries

### Basic SLA Queries

**Cluster-wide SLA compliance rate:**
```promql
# Average compliance across all workers
avg(ffrtmp_worker_sla_compliance_rate)
```

**Total jobs across cluster:**
```promql
sum(ffrtmp_worker_jobs_completed_total) + sum(ffrtmp_worker_jobs_failed_total)
```

**Total compliant jobs:**
```promql
sum(ffrtmp_worker_jobs_sla_compliant_total)
```

**Total SLA violations:**
```promql
sum(ffrtmp_worker_jobs_sla_violation_total)
```

### Success Rate

**Job success rate (% of jobs that didn't fail):**
```promql
(
  sum(ffrtmp_worker_jobs_completed_total)
) / (
  sum(ffrtmp_worker_jobs_completed_total) + sum(ffrtmp_worker_jobs_failed_total)
) * 100
```

**Job failure rate (% of jobs that failed):**
```promql
(
  sum(ffrtmp_worker_jobs_failed_total)
) / (
  sum(ffrtmp_worker_jobs_completed_total) + sum(ffrtmp_worker_jobs_failed_total)
) * 100
```

**Per-worker success rate:**
```promql
(
  ffrtmp_worker_jobs_completed_total
) / (
  ffrtmp_worker_jobs_completed_total + ffrtmp_worker_jobs_failed_total
) * 100
```

### Rate Calculations

**Job completion rate (jobs/sec) over last 5 minutes:**
```promql
rate(ffrtmp_worker_jobs_completed_total[5m])
```

**Job failure rate (failures/sec):**
```promql
rate(ffrtmp_worker_jobs_failed_total[5m])
```

**SLA violation rate (violations/sec):**
```promql
rate(ffrtmp_worker_jobs_sla_violation_total[5m])
```

**Jobs per hour:**
```promql
rate(ffrtmp_worker_jobs_completed_total[1h]) * 3600
```

### Trend Analysis

**SLA compliance trend over 24 hours:**
```promql
avg_over_time(ffrtmp_worker_sla_compliance_rate[24h])
```

**SLA compliance change (current vs 1 hour ago):**
```promql
ffrtmp_worker_sla_compliance_rate - ffrtmp_worker_sla_compliance_rate offset 1h
```

**Jobs completed in last hour:**
```promql
increase(ffrtmp_worker_jobs_completed_total[1h])
```

**SLA violations in last 24 hours:**
```promql
increase(ffrtmp_worker_jobs_sla_violation_total[24h])
```

### Worker Comparison

**Worker with highest SLA compliance:**
```promql
topk(1, ffrtmp_worker_sla_compliance_rate)
```

**Worker with most SLA violations:**
```promql
topk(1, ffrtmp_worker_jobs_sla_violation_total)
```

**Workers below 95% SLA compliance:**
```promql
ffrtmp_worker_sla_compliance_rate < 95
```

### Advanced Queries

**Percentage of workers meeting 95% SLA:**
```promql
(
  count(ffrtmp_worker_sla_compliance_rate >= 95)
) / (
  count(ffrtmp_worker_sla_compliance_rate)
) * 100
```

**Cluster SLA health score (0-100):**
```promql
avg(ffrtmp_worker_sla_compliance_rate) * 
(
  sum(ffrtmp_worker_jobs_completed_total) / 
  (sum(ffrtmp_worker_jobs_completed_total) + sum(ffrtmp_worker_jobs_failed_total))
)
```

## Grafana Dashboards

### SLA Overview Panel

Single stat panel showing cluster-wide SLA compliance:

```yaml
- title: "SLA Compliance Rate"
  type: singlestat
  targets:
    - expr: avg(ffrtmp_worker_sla_compliance_rate)
      legendFormat: "SLA Compliance"
  thresholds:
    - value: 0
      color: red
    - value: 90
      color: yellow
    - value: 95
      color: green
  unit: "percent"
```

### SLA Trend Graph

Time series showing SLA compliance over time:

```yaml
- title: "SLA Compliance Trend"
  type: graph
  targets:
    - expr: ffrtmp_worker_sla_compliance_rate
      legendFormat: "{{node_id}}"
    - expr: avg(ffrtmp_worker_sla_compliance_rate)
      legendFormat: "Cluster Average"
  yAxis:
    min: 0
    max: 100
    format: "percent"
  thresholds:
    - value: 95
      colorMode: critical
      line: true
```

### Job Success vs Failure

Stacked graph showing job outcomes:

```yaml
- title: "Job Completion Status"
  type: graph
  targets:
    - expr: rate(ffrtmp_worker_jobs_completed_total[5m]) * 60
      legendFormat: "Completed ({{node_id}})"
    - expr: rate(ffrtmp_worker_jobs_failed_total[5m]) * 60
      legendFormat: "Failed ({{node_id}})"
  stack: true
  yAxis:
    format: "jobs/min"
```

### SLA Violations Heatmap

Table showing violations per worker:

```yaml
- title: "SLA Violations by Worker"
  type: table
  targets:
    - expr: |
        sum by (node_id) (
          increase(ffrtmp_worker_jobs_sla_violation_total[24h])
        )
      format: table
      instant: true
  columns:
    - text: "Worker"
      value: "node_id"
    - text: "Violations (24h)"
      value: "Value"
```

### Success Rate Gauge

Gauge showing overall success rate:

```yaml
- title: "Job Success Rate"
  type: gauge
  targets:
    - expr: |
        (
          sum(ffrtmp_worker_jobs_completed_total)
        ) / (
          sum(ffrtmp_worker_jobs_completed_total) + 
          sum(ffrtmp_worker_jobs_failed_total)
        ) * 100
  thresholds:
    - value: 0
      color: red
    - value: 90
      color: yellow
    - value: 95
      color: green
  min: 0
  max: 100
  unit: "percent"
```

## Alerting Rules

### Critical SLA Alerts

**SLA Compliance Below Target:**
```yaml
- alert: SLAComplianceBelowTarget
  expr: ffrtmp_worker_sla_compliance_rate < 95
  for: 15m
  labels:
    severity: critical
  annotations:
    summary: "Worker {{$labels.node_id}} SLA compliance below 95%"
    description: "SLA compliance is {{$value | humanize}}%, below the 95% target for over 15 minutes"
    runbook_url: "https://docs.example.com/runbooks/sla-compliance"
    dashboard_url: "https://grafana.example.com/d/sla-dashboard"
```

**High Failure Rate:**
```yaml
- alert: HighJobFailureRate
  expr: |
    (
      rate(ffrtmp_worker_jobs_failed_total[5m])
    ) / (
      rate(ffrtmp_worker_jobs_completed_total[5m]) + 
      rate(ffrtmp_worker_jobs_failed_total[5m])
    ) > 0.10
  for: 10m
  labels:
    severity: critical
  annotations:
    summary: "Worker {{$labels.node_id}} failure rate above 10%"
    description: "Job failure rate is {{$value | humanizePercentage}}, exceeding 10% threshold"
```

### Warning Alerts

**SLA Violations Increasing:**
```yaml
- alert: SLAViolationsIncreasing
  expr: |
    increase(ffrtmp_worker_jobs_sla_violation_total[1h]) > 50
  for: 30m
  labels:
    severity: warning
  annotations:
    summary: "Increasing SLA violations on {{$labels.node_id}}"
    description: "{{$value}} SLA violations in the last hour, indicating performance degradation"
```

**Low Job Throughput:**
```yaml
- alert: LowJobThroughput
  expr: rate(ffrtmp_worker_jobs_completed_total[10m]) < 0.1
  for: 20m
  labels:
    severity: warning
  annotations:
    summary: "Low job completion rate on {{$labels.node_id}}"
    description: "Only {{$value | humanize}} jobs/sec being completed, below expected throughput"
```

### Info Alerts

**SLA Target Missed:**
```yaml
- alert: SLATargetMissed
  expr: increase(ffrtmp_worker_jobs_sla_violation_total[1h]) > 10
  for: 1h
  labels:
    severity: info
  annotations:
    summary: "{{$labels.node_id}} missed SLA on {{$value}} jobs in past hour"
    description: "Consider investigating if this is a pattern or isolated incident"
```

## Configuring SLA Targets

### Default Targets

The system uses default SLA targets defined in code:

```go
// Default: 600 seconds (10 minutes)
MaxDurationSeconds: 600

// Default: 5% failure rate
MaxFailureRate: 0.05
```

### Per-Scenario Targets (Future Feature)

In future releases, you'll be able to specify SLA targets per scenario:

```json
{
  "scenario": "4k-h265",
  "parameters": {
    "bitrate": "25M",
    "duration": 300
  },
  "sla_target": {
    "max_duration_seconds": 1800,
    "max_failure_rate": 0.02
  }
}
```

### Recommended Targets by Workload

| Workload | Max Duration | Reasoning |
|----------|-------------|-----------|
| **720p Fast** | 300s (5 min) | Low complexity, fast encoding |
| **1080p Standard** | 600s (10 min) | Default, general-purpose |
| **4K High Quality** | 1800s (30 min) | High complexity, intensive |
| **Long-form (> 30 min)** | 3600s (1 hour) | Extended content processing |

## Monitoring Best Practices

### 1. Set Realistic Targets

Base SLA targets on actual performance data:

```promql
# P95 duration over last 7 days
histogram_quantile(0.95, 
  rate(ffrtmp_job_duration_seconds_bucket[7d])
)
```

Use P95 + 20% buffer as SLA target.

### 2. Monitor Trends, Not Just Current State

Track SLA compliance over time:

```promql
avg_over_time(ffrtmp_worker_sla_compliance_rate[7d])
```

Alert on **degrading trends**, not just current violations.

### 3. Segment by Scenario

Different scenarios have different performance characteristics:

```promql
# (This query requires master metrics with scenario labels)
avg by (scenario) (job_duration_seconds)
```

Consider per-scenario SLA targets.

### 4. Correlate with Resource Metrics

When SLA violations occur, check:

```promql
# High CPU?
ffrtmp_worker_cpu_usage > 90

# Low memory?
ffrtmp_worker_memory_bytes / ffrtmp_worker_memory_total > 0.9

# High queue depth?
ffrtmp_master_queue_depth > 100
```

### 5. Balance SLA vs Cost

Tighter SLAs require more resources:

- **95% @ 10 min**: Standard capacity
- **99% @ 5 min**: 1.5x-2x capacity
- **99.9% @ 2 min**: 3x-4x capacity

Choose targets based on business requirements.

## Troubleshooting

### SLA Compliance Below Target

**Symptom**: `ffrtmp_worker_sla_compliance_rate < 95`

**Check:**
1. **Job duration distribution:**
   ```promql
   histogram_quantile(0.95, rate(job_duration_seconds_bucket[1h]))
   ```

2. **Worker capacity:**
   ```promql
   ffrtmp_worker_active_jobs / max_concurrent_jobs
   ```

3. **Resource bottlenecks:**
   ```promql
   ffrtmp_worker_cpu_usage > 90 or ffrtmp_worker_memory_bytes > threshold
   ```

**Solutions:**
- Increase worker count
- Reduce `max_concurrent_jobs` per worker
- Use hardware encoding (NVENC/QSV)
- Optimize encoder presets

### High Failure Rate

**Symptom**: `rate(ffrtmp_worker_jobs_failed_total[5m]) / rate(jobs_total[5m]) > 0.05`

**Check:**
1. **Error patterns in logs:**
   ```bash
   grep "JOB FAILED" /var/log/ffmpeg-rtmp/worker.log | tail -50
   ```

2. **Common failure reasons:**
   ```promql
   # (Requires error_reason label)
   topk(5, count by (error_reason) (job_failures))
   ```

**Solutions:**
- Fix input validation issues
- Increase resource limits (memory, disk)
- Update FFmpeg version
- Add error handling for edge cases

### Inconsistent SLA Performance

**Symptom**: SLA compliance varies significantly between workers

**Check:**
```promql
# Standard deviation across workers
stddev(ffrtmp_worker_sla_compliance_rate)
```

**Causes:**
- Hardware differences (some workers faster/slower)
- Uneven job distribution
- Network latency differences
- Resource contention

**Solutions:**
- Standardize hardware
- Implement weighted job distribution
- Use priority scheduling

## Integration Examples

### Python SLA Monitor

```python
import requests
import time

PROMETHEUS_URL = "http://localhost:9090"
SLA_TARGET = 95.0

def check_sla_compliance():
    query = "avg(ffrtmp_worker_sla_compliance_rate)"
    response = requests.get(
        f"{PROMETHEUS_URL}/api/v1/query",
        params={"query": query}
    )
    data = response.json()
    
    if data["status"] == "success" and data["data"]["result"]:
        compliance = float(data["data"]["result"][0]["value"][1])
        return compliance
    return None

def monitor_sla(interval=60):
    """Monitor SLA compliance and alert if below target"""
    while True:
        compliance = check_sla_compliance()
        
        if compliance is not None:
            print(f"Current SLA Compliance: {compliance:.2f}%")
            
            if compliance < SLA_TARGET:
                print(f"⚠️  ALERT: SLA compliance {compliance:.2f}% below target {SLA_TARGET}%")
                # Send notification (email, Slack, PagerDuty)
                send_alert(compliance)
            else:
                print(f"✅ SLA compliance within target")
        
        time.sleep(interval)

def send_alert(compliance):
    """Send alert notification"""
    # Implement your alerting logic here
    pass

if __name__ == "__main__":
    monitor_sla(interval=60)
```

### Bash SLA Check Script

```bash
#!/bin/bash
# Check SLA compliance before deploying new jobs

PROMETHEUS_URL="http://localhost:9090"
SLA_THRESHOLD=95

# Get current SLA compliance
QUERY="avg(ffrtmp_worker_sla_compliance_rate)"
RESPONSE=$(curl -s "${PROMETHEUS_URL}/api/v1/query?query=${QUERY}")
COMPLIANCE=$(echo "$RESPONSE" | jq -r '.data.result[0].value[1]')

echo "Current SLA Compliance: ${COMPLIANCE}%"

if (( $(echo "$COMPLIANCE < $SLA_THRESHOLD" | bc -l) )); then
    echo "❌ SLA compliance below ${SLA_THRESHOLD}%"
    echo "Recommendation: Wait for system to recover before submitting more jobs"
    exit 1
else
    echo "✅ SLA compliance healthy"
    echo "Safe to proceed with job submission"
    exit 0
fi
```

### Go SLA Checker

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type PrometheusResponse struct {
    Status string `json:"status"`
    Data   struct {
        Result []struct {
            Value []interface{} `json:"value"`
        } `json:"result"`
    } `json:"data"`
}

func getSLACompliance(prometheusURL string) (float64, error) {
    query := "avg(ffrtmp_worker_sla_compliance_rate)"
    url := fmt.Sprintf("%s/api/v1/query?query=%s", prometheusURL, query)
    
    resp, err := http.Get(url)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    
    var result PrometheusResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return 0, err
    }
    
    if len(result.Data.Result) == 0 {
        return 0, fmt.Errorf("no data")
    }
    
    compliance, ok := result.Data.Result[0].Value[1].(string)
    if !ok {
        return 0, fmt.Errorf("invalid format")
    }
    
    var value float64
    fmt.Sscanf(compliance, "%f", &value)
    return value, nil
}

func main() {
    prometheusURL := "http://localhost:9090"
    slaTarget := 95.0
    
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        compliance, err := getSLACompliance(prometheusURL)
        if err != nil {
            fmt.Printf("Error: %v\n", err)
            continue
        }
        
        fmt.Printf("SLA Compliance: %.2f%%\n", compliance)
        
        if compliance < slaTarget {
            fmt.Printf("⚠️  Below target (%.2f%%)\\n", slaTarget)
        } else {
            fmt.Println("✅ Meeting target")
        }
    }
}
```

## Related Documentation

- [Resource Limits Guide](RESOURCE_LIMITS.md) - Configure timeouts to meet SLA targets
- [Bandwidth Metrics](BANDWIDTH_METRICS.md) - Track I/O performance
- [Alerting Guide](ALERTING.md) - Set up SLA violation alerts
- [Production Operations](PRODUCTION_OPERATIONS.md) - Capacity planning for SLA compliance

## Support

For issues with SLA tracking:

1. Verify Prometheus metrics endpoint: `http://worker:9091/metrics`
2. Check for `ffrtmp_worker_sla_compliance_rate` in output
3. Ensure jobs are completing (both success and failure update metrics)
4. Review worker logs for SLA status in job completion messages
5. Consult GitHub issues for known problems
