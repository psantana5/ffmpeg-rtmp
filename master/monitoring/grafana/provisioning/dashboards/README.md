# Grafana Dashboards

This directory contains streamlined Grafana dashboards for monitoring FFmpeg transcoding performance, power consumption, and energy efficiency with all exporters integrated.

## Dashboard Organization (Updated December 2024)

The dashboard collection has been reorganized for clarity and efficiency. **Obsolete dashboards have been moved to the `archive/` directory.**

---

## Core Dashboards

### 1. System Overview  **START HERE**
**File:** `system-overview.json`  
**UID:** `system-overview`  
**Purpose:** Primary dashboard showing overall system health and resource utilization

**Use Cases:**
- Quick system health check
- Monitor all exporters at a glance
- Identify resource bottlenecks
- Track power consumption across CPU and GPU

**Key Panels:**
- CPU Power Consumption (RAPL) - from `cpu-exporter-go`
- GPU Power Consumption - from `gpu-exporter-go`
- CPU, Memory, GPU Usage Gauges
- Exporter Health Status
- Network Traffic
- Docker Container CPU Usage
- Exporter Status Table (all 12 exporters)

**Exporters Used:** cpu-exporter-go, gpu-exporter-go, node-exporter, cadvisor, exporter-health-checker

---

### 2. Transcoding Performance
**File:** `transcoding-performance.json`  
**UID:** `transcoding-performance`  
**Purpose:** Real-time FFmpeg encoding metrics and RTMP server stats

**Use Cases:**
- Monitor encoding quality and throughput
- Track dropped frames and latency
- Analyze encoding speed vs realtime
- Monitor RTMP stream health

**Key Panels:**
- FFmpeg Encoding FPS
- FFmpeg Output Bitrate (current + average)
- Encoding Speed (realtime multiplier)
- Dropped Frames Counter
- Total Frames Processed
- Encoder Load Percentage
- Encode Latency
- RTMP Server Stats (streams, viewers)
- Frame Processing Rate

**Exporters Used:** ffmpeg-exporter, nginx-exporter

---

### 3. Hardware Details
**File:** `hardware-details.json`  
**UID:** `hardware-details`  
**Purpose:** Deep dive into hardware metrics and container resource usage

**Use Cases:**
- Analyze detailed RAPL power zones
- Monitor GPU temperature and clock speeds
- Track GPU encoder/decoder utilization
- Monitor container-level resource usage

**Key Panels:**
- CPU RAPL Power Zones (detailed breakdown)
- GPU Temperature
- GPU Utilization Breakdown (core, memory, encoder, decoder)
- GPU Memory Usage
- GPU Clock Speeds (graphics, SM, memory)
- GPU Power Draw vs Limit
- Container Memory Usage (cAdvisor)
- Container Network Traffic (cAdvisor)

**Exporters Used:** cpu-exporter-go, gpu-exporter-go, cadvisor

---

### 4. Energy Efficiency Analysis
**File:** `energy-efficiency-dashboard.json`  
**UID:** `energy-efficiency-dashboard`  
**Purpose:** Advanced energy efficiency analysis and optimization

**Use Cases:**
- Find optimal transcoding configurations
- Compare energy efficiency across scenarios
- Identify CPU vs GPU tipping points
- Analyze stability and reliability

**Documentation:** See [ENERGY_EFFICIENCY_DASHBOARD.md](./ENERGY_EFFICIENCY_DASHBOARD.md)

**Key Panels:**
- Energy Efficiency Leaderboard
- Pixels Delivered per Joule
- Energy Wasted vs Optimal
- CPU vs GPU Scaling
- Efficiency Stability

**Exporters Used:** results-exporter, cpu-exporter-go, gpu-exporter-go

---

### 5. Power Monitoring
**File:** `power-monitoring.json`  
**Purpose:** Real-time power consumption monitoring with historical trends

**Use Cases:**
- Monitor CPU package power (RAPL)
- Track Docker container overhead
- Real-time power consumption during tests

**Exporters Used:** cpu-exporter-go, docker-stats-exporter

---

### 6. QoE (Quality of Experience) Dashboard
**File:** `qoe-dashboard.json`  
**UID:** `qoe-dashboard`  
**Purpose:** Video quality metrics and quality-per-watt analysis

**Use Cases:**
- Monitor VMAF and PSNR quality scores
- Analyze quality-weighted efficiency
- Track QoE-aware performance metrics

**Exporters Used:** qoe-exporter, results-exporter

---

### 7. Cost & ROI Analysis
**File:** `cost-roi-dashboard.json`  
**Purpose:** Load-aware cost analysis and ROI metrics

**Use Cases:**
- Track compute and energy costs
- Analyze cost per pixel delivered
- Monitor cost per viewer watch hour
- Calculate ROI for different configurations

**Exporters Used:** cost-exporter, results-exporter

---

## Exporter Reference

All dashboards pull data from these exporters configured in VictoriaMetrics:

| Exporter | Port | Metrics | Used In Dashboards |
|----------|------|---------|-------------------|
| **cpu-exporter-go** | 9500 | `rapl_power_watts`, `rapl_zones_discovered` | System Overview, Hardware Details, Power Monitoring |
| **gpu-exporter-go** | 9505 | `gpu_power_draw_watts`, `gpu_utilization_percent`, `gpu_memory_*`, `gpu_temperature_celsius`, `gpu_clocks_*` | System Overview, Hardware Details |
| **ffmpeg-exporter** | 9506 | `ffmpeg_fps`, `ffmpeg_bitrate_*`, `ffmpeg_dropped_frames_total`, `ffmpeg_frames_total`, `ffmpeg_encoder_load_percent`, `ffmpeg_encode_latency_ms`, `ffmpeg_speed` | Transcoding Performance |
| **docker-stats-exporter** | 9501 | `docker_*` | Power Monitoring |
| **node-exporter** | 9100 | `node_cpu_*`, `node_memory_*`, `node_network_*` | System Overview |
| **cadvisor** | 8080 | `container_cpu_*`, `container_memory_*`, `container_network_*` | System Overview, Hardware Details |
| **nginx-exporter** | 9728 | `nginx_rtmp_streams`, `nginx_rtmp_viewers_total` | Transcoding Performance |
| **results-exporter** | 9502 | `test_results_*` | Energy Efficiency, QoE |
| **qoe-exporter** | 9503 | `qoe_vmaf_score`, `qoe_psnr_score`, `qoe_quality_per_watt` | QoE Dashboard |
| **cost-exporter** | 9504 | `cost_total_*`, `cost_per_pixel`, `cost_per_watch_hour` | Cost & ROI |
| **exporter-health-checker** | 9600 | `exporter_health_*` | System Overview |
| **dcgm-exporter** (GPU) | 9400 | `DCGM_*` | Hardware Details (nvidia profile) |

---

## Quick Start Guide

### Step 1: Start the Stack

**For Docker Compose (Development):**
```bash
cd /path/to/ffmpeg-rtmp
make up-build
```

**For Production (Distributed Mode):**
```bash
# On master node
make vm-up-build
```

### Step 2: Access Grafana

Open your browser and navigate to:
```
http://localhost:3000
```

**Default Credentials:**
- Username: `admin`
- Password: `admin` (change on first login)

### Step 3: Navigate Dashboards

1. Click on **Dashboards** (四 icon) in the left sidebar
2. You'll see all 7 dashboards:
   - **System Overview** - Start here for overall health 
   - **Transcoding Performance** - FFmpeg and encoding metrics
   - **Hardware Details** - Deep hardware monitoring
   - **Energy Efficiency Analysis** - Optimization insights
   - **Power Monitoring** - Real-time power tracking
   - **QoE Dashboard** - Video quality metrics
   - **Cost & ROI Analysis** - Business metrics

### Step 4: Verify Data Collection

1. Open **System Overview** dashboard
2. Check the **Exporter Status** table at the bottom
3. All exporters should show **Status: UP** (green)
4. If any exporter shows **DOWN** (red), check:
   ```bash
   docker compose logs <exporter-name>
   make logs SERVICE=<exporter-name>
   ```

### Step 5: Run a Test to See Data

To populate dashboards with real data:

```bash
# Run a simple test
python3 scripts/run_tests.py single --name "test1" --bitrate 2000k --duration 60

# Watch data appear in dashboards (auto-refresh every 5s)
```

---

## Data Source Configuration

All dashboards use the **VictoriaMetrics** data source (Prometheus-compatible).

**Configuration:** `../datasources/prometheus.yml`
```yaml
apiVersion: 1
datasources:
  - name: VictoriaMetrics
    type: prometheus
    url: http://victoriametrics:8428
    isDefault: true
    jsonData:
      timeInterval: 1s
```

VictoriaMetrics scrapes all exporters every **1 second** for high-resolution metrics.

**Scrape Configuration:** `../victoriametrics.yml`

---

## Dashboard Provisioning

Dashboards are automatically loaded when Grafana starts (no manual import needed).

**Configuration:** `default.yml`
```yaml
apiVersion: 1
providers:
  - name: 'Default'
    orgId: 1
    folder: ''
    type: file
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
    access: proxy
    url: http://victoriametrics:8428
    isDefault: true
```

## Metrics Reference

### Results Exporter Metrics
Exported by `results-exporter` service (port 9502):

| Metric | Description |
|--------|-------------|
| `results_scenario_efficiency_score` | Energy efficiency (pixels/J or Mbps/W) |
| `results_scenario_mean_power_watts` | Mean CPU power (W) |
| `results_scenario_power_stdev` | Standard deviation of power measurements (W) |
| `results_scenario_prediction_confidence_high` | Upper bound of prediction confidence interval (W) |
| `results_scenario_prediction_confidence_low` | Lower bound of prediction confidence interval (W) |
| `results_scenario_total_energy_joules` | Total energy (J) |
| `results_scenario_total_energy_wh` | Total energy (Wh) |
| `results_scenario_total_pixels` | Total pixels delivered |
| `results_scenario_energy_wh_per_mbps` | Energy per Mbps (Wh/Mbps) |
| `results_scenario_energy_mj_per_frame` | Energy per frame (mJ/frame) |
| `results_scenario_delta_power_watts` | Power delta vs baseline (W) |
| `results_scenario_power_pct_increase` | Power increase vs baseline (%) |

**Labels:**
- `scenario` - Scenario name
- `bitrate` - Configured bitrate
- `resolution` - Primary resolution
- `fps` - Frames per second
- `streams` - Number of concurrent streams
- `output_ladder` - Output resolution ladder
- `encoder_type` - Encoder type (cpu/gpu)
- `run_id` - Test run identifier

#### Prediction Confidence Metrics

The prediction confidence metrics provide a statistical measure of the reliability of power predictions:

- **`prediction_confidence_high`**: Upper bound of the 95% confidence interval (mean + 2×stdev)
- **`prediction_confidence_low`**: Lower bound of the 95% confidence interval (mean - 2×stdev, minimum 0)
- **`power_stdev`**: Standard deviation of power measurements during the scenario

**Interpretation:**
- **Narrow interval** (small stdev): High confidence - stable, consistent power consumption
- **Wide interval** (large stdev): Low confidence - variable power consumption, less predictable

**Example Queries:**
```promql
# Confidence interval width (measure of prediction reliability)
results_scenario_prediction_confidence_high{run_id=~"$run_id"} - results_scenario_prediction_confidence_low{run_id=~"$run_id"}

# Scenarios with high confidence (stable power consumption)
results_scenario_power_stdev{run_id=~"$run_id"} < 5

# Efficiency with confidence bounds
results_scenario_efficiency_score{run_id=~"$run_id"}
```

### RAPL Power Metrics
Exported by `rapl-exporter` service (port 9500):

| Metric | Description |
|--------|-------------|
| `rapl_power_watts` | Instantaneous power (W) |
| `rapl_energy_joules_total` | Cumulative energy (J) |

**Labels:**
- `zone` - Power zone (e.g., "package-0", "core", "dram")

### Docker Metrics
Exported by `docker-stats-exporter` service (port 9501):

| Metric | Description |
|--------|-------------|
| `docker_container_cpu_percent` | CPU percentage per container |
| `docker_container_memory_percent` | Memory percentage per container |
| `docker_containers_total_cpu_percent` | Total CPU % across all containers |

## Quick Start Guide

### 1. Start the Stack
```bash
docker-compose up -d
```

### 2. Access Grafana
```bash
# Open in browser
xdg-open http://localhost:3000

# Or use curl to check status
curl -s http://localhost:3000/api/health | jq .
```

### 3. Run Tests
```bash
# Single stream test
python3 scripts/run_tests.py single --bitrate 2500k --resolution 1280x720 --fps 30

# Multiple streams test
python3 scripts/run_tests.py multi --count 4 --bitrate 2500k

# Batch tests
python3 scripts/run_tests.py batch --file batch_stress_matrix.json
```

### 4. View Results
- **During tests:** Use "Power Monitoring" dashboard
- **After tests:** Use "Energy Efficiency Analysis" dashboard
- **Compare baseline:** Use "Baseline vs Test" dashboard

## Customization

### Adding Custom Panels

1. Edit dashboard in Grafana UI
2. Export to JSON: Settings → JSON Model
3. Copy JSON to dashboard file
4. Restart Grafana to apply changes

### Modifying Queries

Edit PromQL expressions in panel queries:

```json
{
  "expr": "results_scenario_efficiency_score{output_ladder=\"1280x720@30\"}",
  "legendFormat": "{{scenario}}"
}
```

### Time Range Presets

Dashboards include these time ranges:
- Last 5 minutes
- Last 15 minutes
- Last 30 minutes
- Last 1 hour
- Last 3 hours
- Last 6 hours
- Last 12 hours
- Last 24 hours

## Troubleshooting

### Dashboard Shows "No Data"

**Check Prometheus:**
```bash
# Verify Prometheus is running
curl http://localhost:8428/-/healthy

# Check targets
curl http://localhost:8428/api/v1/targets | jq .
```

**Check Results Exporter:**
```bash
# Verify metrics are being exported
curl http://localhost:9502/metrics

# Check for test results files
ls -lh results/
```

**Check Grafana Logs:**
```bash
docker logs grafana
```

### Metrics Missing Labels

Verify results exporter is using the enhanced version:
```bash
curl -s http://localhost:9502/metrics | grep -E "(streams|output_ladder|encoder_type)"
```

Should show labels like:
```
results_scenario_mean_power_watts{...,streams="4",output_ladder="1280x720@30",encoder_type="cpu",...}
```

### Dashboard Not Auto-Loading

Check provisioning configuration:
```bash
# Verify provisioning directory is mounted
docker exec grafana ls -l /etc/grafana/provisioning/dashboards/

# Check provisioning logs
docker logs grafana | grep -i provision
```

### Slow Query Performance

Optimize PromQL queries:
- Use instant queries (`instant: true`) for tables
- Limit time ranges for range queries
- Add rate limiters if needed
- Consider increasing Prometheus retention

### Query Returns "Empty query result"

**Symptom:** Query shows "Empty query result" or "Result series: 0"

**Common Causes:**

1. **Non-existent metrics:** The metric name is not exported by any exporter
   - Verify metric exists: `curl http://localhost:9502/metrics | grep <metric_name>`
   - Check available metrics in the "Metrics Reference" section above
   - Note: Prediction confidence metrics (`results_scenario_prediction_confidence_high/low`) are now available

2. **No data in time range:** Metric exists but has no data points in selected time range
   - Adjust time range picker in Grafana
   - Verify tests have been run: `ls -lh test_results/`

3. **Label filter doesn't match:** Label selectors exclude all series
   - Check label values: `curl http://localhost:8428/api/v1/label/<label>/values`
   - Simplify query to test: remove label filters one by one

4. **Datasource not configured:** Dashboard cannot reach Prometheus
   - See "Datasource Issues" section below

### Datasource Issues

**Symptom:** "Cannot find datasource" or datasource errors

**Common Causes:**

1. **Dashboard file in wrong directory:**
   -  Dashboards belong in: `grafana/provisioning/dashboards/`
   -  **NOT** in: `grafana/provisioning/datasources/`
   - Only `prometheus.yml` should be in datasources directory

2. **Datasource variable not defined:**
   - Verify dashboard has `DS_PROMETHEUS` variable in `templating.list`
   - Check panels use `"uid": "${DS_PROMETHEUS}"`

3. **Prometheus not running:**
   - Check: `curl http://localhost:8428/-/healthy`
   - View logs: `docker logs prometheus`

## Best Practices

### 1. Dashboard Organization
- Use folders for different purposes (monitoring, analysis, debugging)
- Tag dashboards appropriately
- Use descriptive titles and descriptions

### 2. Query Optimization
- Use recording rules for expensive queries
- Limit cardinality of labels
- Use appropriate step intervals
- Cache results where possible

### 3. Alerting
- Set up alerts for critical thresholds
- Use notification channels (email, Slack, etc.)
- Test alerts before relying on them
- Document alert response procedures

### 4. Data Retention
- Balance retention time vs disk space
- Use downsampling for long-term data
- Archive important results externally
- Regular cleanup of old metrics

## Integration Examples

### Export Dashboard Snapshot
```bash
# Using Grafana API
curl -X POST http://admin:admin@localhost:3000/api/snapshots \
  -H "Content-Type: application/json" \
  -d '{
    "dashboard": {...},
    "name": "Energy Efficiency Snapshot",
    "expires": 3600
  }'
```

### Automated Reporting
```python
# Generate PDF report
import requests

url = "http://localhost:3000/render/d-solo/energy-efficiency-dashboard"
params = {
    "orgId": 1,
    "from": "now-1h",
    "to": "now",
    "panelId": 1,
    "width": 1000,
    "height": 500
}

response = requests.get(url, params=params, auth=("admin", "admin"))
with open("report.png", "wb") as f:
    f.write(response.content)
```

### Embedding Panels
```html
<!-- Embed panel in web page -->
<iframe
  src="http://localhost:3000/d-solo/energy-efficiency-dashboard?orgId=1&panelId=1"
  width="800"
  height="400"
  frameborder="0">
</iframe>
```

## Resources

### Documentation
- [Grafana Documentation](https://grafana.com/docs/)
- [PromQL Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Dashboard Best Practices](https://grafana.com/docs/grafana/latest/best-practices/)

### Community
- [Grafana Community](https://community.grafana.com/)
- [Prometheus Users](https://prometheus.io/community/)

### Related Files
- Dashboard provisioning: `default.yml`
- Data source config: `../datasources/prometheus.yml`
- Results exporter: `../../results-exporter/results_exporter.py`
- Test runner: `../../run_tests.py`

## Support

For issues or questions:
1. Check this README and dashboard-specific documentation
2. Review Grafana and Prometheus logs
3. Verify metrics are being exported correctly
4. Consult the troubleshooting sections
5. Open an issue in the repository

---

**Last Updated:** 2024-12-27
**Grafana Version:** 9.5.0+
**Prometheus Version:** 2.40+

---

## Troubleshooting

### No Data in Dashboards

**Problem:** Dashboard panels show "No data"

**Solutions:**
1. Check VictoriaMetrics is running and healthy:
   ```bash
   curl http://localhost:8428/health
   ```

2. Verify exporters are up:
   ```bash
   docker compose ps
   # All exporters should show "healthy" or "running"
   ```

3. Check if VictoriaMetrics is scraping:
   ```bash
   curl http://localhost:8428/api/v1/targets
   ```

4. View exporter metrics directly:
   ```bash
   curl http://localhost:9500/metrics  # CPU exporter
   curl http://localhost:9506/metrics  # FFmpeg exporter
   curl http://localhost:9505/metrics  # GPU exporter
   ```

### Exporter Shows DOWN

**Problem:** Exporter status shows red/DOWN in System Overview

**Solutions:**
1. Check exporter logs:
   ```bash
   docker compose logs <exporter-name>
   ```

2. Restart the exporter:
   ```bash
   docker compose restart <exporter-name>
   ```

3. For GPU exporters (requires NVIDIA GPU):
   ```bash
   # Start with nvidia profile
   docker compose --profile nvidia up -d
   ```

### Dashboard Not Loading

**Problem:** Dashboard doesn't appear in Grafana

**Solutions:**
1. Check Grafana logs:
   ```bash
   docker compose logs grafana
   ```

2. Verify dashboard file exists:
   ```bash
   ls -l master/monitoring/grafana/provisioning/dashboards/*.json
   ```

3. Restart Grafana to reload provisioning:
   ```bash
   docker compose restart grafana
   ```

### Slow Dashboard Performance

**Problem:** Dashboard takes long to load or is slow

**Solutions:**
1. Reduce time range (use last 15m instead of 1h)
2. Increase refresh interval (10s instead of 5s)
3. Check VictoriaMetrics resource usage:
   ```bash
   docker stats victoriametrics
   ```

---

## Archive

Obsolete dashboards have been moved to `archive/` directory:
- `baseline-vs-test.json` - Replaced by System Overview
- `benchmark-history.json` - History available in VictoriaMetrics directly
- `cost-dashboard-load-aware.json` - Duplicate, consolidated into cost-roi-dashboard
- `cost-dashboard.json` - Duplicate, consolidated into cost-roi-dashboard
- `efficiency-forecasting.json` - Advanced forecasting, limited practical use
- `future-load-predictions.json` - Advanced forecasting, limited practical use

These are kept for reference but not loaded by default.

---

## Dashboard Customization

All dashboards support:
- **Time Range Selection** - Top-right corner
- **Auto-refresh** - Set to 5s by default (configurable)
- **Panel Editing** - Click panel title → Edit
- **Variable Queries** - Can add template variables for filtering
- **Export** - Share → Export → Save JSON

**Note:** Changes made in UI are temporary unless you export and save the JSON.

---

## Metrics Retention

- **VictoriaMetrics Default:** 30 days
- **Resolution:** 1-second scrape interval
- **Downsampling:** None (full resolution retained)

To change retention period, edit `docker-compose.yml`:
```yaml
victoriametrics:
  command:
    - "--retentionPeriod=30d"  # Change to 60d, 90d, etc.
```

---

## Next Steps

1.  **Verify Setup:** Open System Overview and confirm all exporters are UP
2.  **Run Tests:** Execute transcoding tests to populate metrics
3.  **Explore:** Navigate through all 7 dashboards
4.  **Optimize:** Use Energy Efficiency dashboard to find optimal settings
5.  **Analyze:** Check Cost & ROI dashboard for business insights

For more information, see:
- [VictoriaMetrics Configuration](../victoriametrics.yml)
- [Docker Compose Stack](../../../../../docker-compose.yml)
- [Main Documentation](../../../../../README.md)
