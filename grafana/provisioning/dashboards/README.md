# Grafana Dashboards

This directory contains Grafana dashboards for monitoring FFmpeg transcoding performance, power consumption, and energy efficiency.

## Available Dashboards

### 1. Energy Efficiency Analysis
**File:** `energy-efficiency-dashboard.json`  
**UID:** `energy-efficiency-dashboard`  
**Purpose:** Advanced energy efficiency analysis and optimization

**Use Cases:**
- Find optimal transcoding configurations
- Compare energy efficiency across scenarios
- Identify CPU vs GPU tipping points
- Analyze stability and reliability
- Support decision-making for production deployments

**Documentation:** See [ENERGY_EFFICIENCY_DASHBOARD.md](./ENERGY_EFFICIENCY_DASHBOARD.md)  
**Visual Guide:** See [VISUAL_SUMMARY.md](./VISUAL_SUMMARY.md)

**Key Panels:**
- Energy Efficiency Leaderboard
- Pixels Delivered per Joule
- Energy Wasted vs Optimal
- CPU vs GPU Scaling
- Efficiency Stability
- Energy per Mbps Throughput
- Energy per Frame
- Power Overhead vs Baseline

---

### 2. Power Monitoring
**File:** `power-monitoring.json`  
**Purpose:** Real-time power consumption monitoring

**Use Cases:**
- Monitor CPU package power (RAPL)
- Track Docker container overhead
- Real-time power consumption during tests

**Key Panels:**
- CPU Package Power (RAPL)
- Docker Container Overhead

---

### 3. Baseline vs Test
**File:** `baseline-vs-test.json`  
**Purpose:** Compare baseline and test scenario metrics

**Use Cases:**
- Validate baseline measurements
- Compare test scenarios to idle state
- Identify power deltas

---

## Dashboard Access

Once Grafana is running, access dashboards at:

```
http://localhost:3000
```

**Default Credentials:**
- Username: `admin`
- Password: `admin` (change on first login)

## Dashboard Provisioning

Dashboards are automatically provisioned when Grafana starts.

**Configuration:** `default.yml`
```yaml
apiVersion: 1
providers:
  - name: 'Default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
```

## Data Sources

All dashboards use the provisioned Prometheus data source.

**Configuration:** `../datasources/prometheus.yml`
```yaml
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

## Metrics Reference

### Results Exporter Metrics
Exported by `results-exporter` service (port 9502):

| Metric | Description |
|--------|-------------|
| `results_scenario_efficiency_score` | Energy efficiency (pixels/J or Mbps/W) |
| `results_scenario_mean_power_watts` | Mean CPU power (W) |
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
| `docker_containers_total_cpu_percent` | Total container CPU % |
| `docker_engine_cpu_percent` | Docker engine CPU % |

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
python3 run_tests.py single --bitrate 2500k --resolution 1280x720 --fps 30

# Multiple streams test
python3 run_tests.py multi --count 4 --bitrate 2500k

# Batch tests
python3 run_tests.py batch --file batch_stress_matrix.json
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
curl http://localhost:9090/-/healthy

# Check targets
curl http://localhost:9090/api/v1/targets | jq .
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
