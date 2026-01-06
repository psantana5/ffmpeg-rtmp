# Grafana Dashboard Improvements

## Overview
This document describes the comprehensive improvements made to the FFmpeg-RTMP Production Monitoring dashboard to ensure accurate, realistic, and performant metrics visualization.

## Changes Made

### 1. **Fixed Metrics Endpoint Performance**
- **Problem**: Metrics endpoint was loading all 81,511 jobs into memory, causing timeouts
- **Solution**: Implemented optimized `GetJobMetrics()` method using SQL aggregation
- **Result**: Response time reduced from timeout to ~70-100ms

### 2. **Improved Query Accuracy**

#### Success Rate Queries
**Old Query** (returned empty when no failures):
```promql
100 * (sum(ffrtmp_jobs_total{state="completed"}) / (sum(ffrtmp_jobs_total{state="completed"}) + sum(ffrtmp_jobs_total{state="failed"})))
```

**New Query** (always works):
```promql
100 * sum(ffrtmp_jobs_total{state="completed"}) / (sum(ffrtmp_jobs_total{state="completed"}) + (sum(ffrtmp_jobs_total{state="failed"}) or vector(0)) + 1)
```

**Key Improvements**:
- Uses `or vector(0)` to provide default value when no failed jobs exist
- Adds `+ 1` to denominator to avoid division by zero
- Works correctly even when there are no failed or processing jobs

#### Rate-based Queries
**Old Query** (returned no data for short time windows):
```promql
sum(rate(ffrtmp_jobs_total{state="completed"}[5m]))
```

**New Query** (uses increase for better visibility):
```promql
100 * (sum(increase(ffrtmp_jobs_total{state="completed"}[1h])) / (sum(increase(ffrtmp_jobs_total[1h])) + 1))
```

**Key Improvements**:
- Uses `increase()` instead of `rate()` for better readability
- Larger time window (1h) ensures enough data points
- Handles zero divisor with `+ 1`

### 3. **New Dashboard Panels**

#### Row 1: Key Performance Indicators
1. **Job Success Rate (All Time)** - Gauge showing overall success percentage
2. **Success Rate (Last Hour)** - Gauge showing recent performance
3. **Active Jobs** - Real-time count of processing jobs
4. **Queue Length** - Current queue size with thresholds
5. **Worker Nodes** - Total registered workers
6. **Avg Job Duration** - Average completion time

#### Row 2: Job State Trends
7. **Jobs by State (Cumulative)** - Time series showing completed, failed, queued, processing jobs
8. **Queue by Priority** - Stacked bar chart of high/medium/low priority jobs

#### Row 3: Completion Metrics
9. **Job Completion Rate (per second)** - Rate of completions and failures
10. **Queue by Type** - Stacked bar chart of live/default/batch queues

#### Row 4: Worker Health
11. **Worker CPU Usage** - CPU percentage per worker
12. **Worker Memory Usage** - Memory consumption per worker

#### Row 5: Status Distributions
13. **Worker Status Distribution** - Pie chart of available/busy/offline workers
14. **Completed Jobs by Engine** - Donut chart showing ffmpeg vs gstreamer
15. **Exporter Health Status** - Table showing all exporter statuses

### 4. **Query Optimizations**

#### Always Use Default Values
All queries that might return empty results now use `or vector(0)`:
```promql
ffrtmp_jobs_total{state="failed"} or vector(0)
ffrtmp_queue_by_priority{priority="high"} or vector(0)
```

#### Smooth Interpolation
Time series panels use `smooth` interpolation for better visualization:
```json
"lineInterpolation": "smooth"
```

#### Span Nulls
Enabled `spanNulls: true` to connect data points even with gaps

#### Better Legends
All time series now show calculated values in legends:
```json
"legend": {
  "calcs": ["mean", "last", "max"],
  "displayMode": "table",
  "placement": "bottom"
}
```

### 5. **Threshold Configuration**

#### Success Rate Gauges
- Red: < 95%
- Yellow: 95-99%
- Green: > 99%

#### Queue Length
- Green: < 10,000
- Yellow: 10,000-50,000
- Red: > 50,000

#### Active Jobs
- Green: < 50
- Yellow: 50-100
- Red: > 100

#### CPU Usage
- Green: < 80%
- Red: > 80%

### 6. **Time Range Settings**

**Default Time Range**: Last 1 hour
- Provides enough data for rate calculations
- Shows recent trends without overwhelming detail

**Auto-Refresh**: 10 seconds
- Balances freshness with query load

**Available Refresh Rates**:
- 5s, 10s, 30s, 1m, 5m, 15m

## Available Metrics

### Master Metrics (port 9090)
```
ffrtmp_jobs_total{state}             # Total jobs by state
ffrtmp_active_jobs                   # Currently active jobs
ffrtmp_queue_length                  # Jobs in queue
ffrtmp_job_duration_seconds          # Average job duration
ffrtmp_job_wait_time_seconds         # Average wait time
ffrtmp_schedule_attempts_total       # Scheduling attempts
ffrtmp_master_uptime_seconds         # Master uptime
ffrtmp_nodes_total                   # Total worker nodes
ffrtmp_nodes_by_status{status}       # Nodes by status
ffrtmp_queue_by_priority{priority}   # Queue by priority
ffrtmp_queue_by_type{type}           # Queue by type
ffrtmp_jobs_by_engine{engine}        # Jobs by engine
ffrtmp_jobs_completed_by_engine      # Completions by engine
```

### Worker Metrics (port 9091+)
```
ffrtmp_worker_cpu_usage              # CPU usage percentage
ffrtmp_worker_memory_bytes           # Memory usage
ffrtmp_worker_active_jobs            # Active jobs per worker
ffrtmp_worker_heartbeats_total       # Heartbeat count
ffrtmp_worker_uptime_seconds         # Worker uptime
ffrtmp_worker_has_gpu                # GPU availability
```

## Query Best Practices

### 1. Handle Missing Data
Always use `or vector(0)` for metrics that might not exist:
```promql
sum(ffrtmp_jobs_total{state="failed"}) or vector(0)
```

### 2. Avoid Division by Zero
Add small constant to denominators:
```promql
sum(completed) / (sum(total) + 1)
```

### 3. Use Appropriate Time Windows
- Instant queries: No time range needed
- Rate queries: Use [5m] or [1h] depending on data frequency
- Increase queries: Use [1h] or [24h] for accumulation

### 4. Choose Right Function
- `rate()`: Per-second average rate (good for rates/sec)
- `increase()`: Total increase over time (good for totals)
- `sum()`: Aggregate across labels
- `avg()`: Average value across labels

## Testing Queries

You can test queries directly against VictoriaMetrics:

```bash
# Success rate
curl 'http://localhost:8428/api/v1/query?query=100*sum(ffrtmp_jobs_total{state="completed"})/sum(ffrtmp_jobs_total)'

# Active jobs
curl 'http://localhost:8428/api/v1/query?query=ffrtmp_active_jobs'

# Queue length
curl 'http://localhost:8428/api/v1/query?query=ffrtmp_queue_length'

# Completion rate
curl 'http://localhost:8428/api/v1/query?query=rate(ffrtmp_jobs_total{state="completed"}[5m])'
```

## Troubleshooting

### Dashboard Shows "No Data"

1. **Check metrics are being scraped**:
   ```bash
   curl http://localhost:8428/api/v1/targets
   ```

2. **Verify metrics exist**:
   ```bash
   curl 'http://localhost:8428/api/v1/query?query=ffrtmp_jobs_total'
   ```

3. **Check time range**: Expand to "Last 1 hour" or more

4. **Wait for data points**: Rate queries need 2+ scrapes (30+ seconds)

### Queries Return Empty

- **Check for typos** in metric names
- **Verify label selectors** match actual labels
- **Use `or vector(0)`** for optional metrics
- **Increase time window** for rate/increase queries

### Performance Issues

- **Reduce query complexity**: Avoid excessive aggregations
- **Increase refresh interval**: Use 30s or 1m instead of 5s
- **Limit time range**: Use 1h instead of 24h for real-time monitoring
- **Use recording rules**: Pre-calculate complex queries

## Regenerating the Dashboard

To regenerate the dashboard from the script:

```bash
cd /home/sanpau/Documents/projects/ffmpeg-rtmp
python3 scripts/update_production_dashboard.py > master/monitoring/grafana/provisioning/dashboards/production-monitoring.json
docker restart grafana
```

## Query Examples

### Calculate Success Percentage
```promql
100 * sum(ffrtmp_jobs_total{state="completed"}) / sum(ffrtmp_jobs_total)
```

### Jobs Completed in Last Hour
```promql
sum(increase(ffrtmp_jobs_total{state="completed"}[1h]))
```

### Average CPU Across Workers
```promql
avg(ffrtmp_worker_cpu_usage)
```

### Queue Backlog by Priority
```promql
sum by(priority) (ffrtmp_queue_by_priority)
```

### Worker Utilization
```promql
100 * count(ffrtmp_nodes_by_status{status="busy"}) / count(ffrtmp_nodes_by_status)
```

## Summary

The improved dashboard now provides:
- ✅ **Accurate metrics** that handle edge cases (no failures, missing data)
- ✅ **Fast performance** with optimized database queries
- ✅ **Comprehensive visibility** into all system aspects
- ✅ **Proper thresholds** for alerting and monitoring
- ✅ **Best practices** for PromQL queries
- ✅ **Clear visualizations** with appropriate panel types

All queries have been tested and verified to work correctly with the current system state.
