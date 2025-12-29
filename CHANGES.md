# Changes Summary

## 1. Fixed Exporter Port Assignments

### Problem
The default ports in the Python exporter scripts were swapped:
- `qoe_exporter.py` had default port 9504 (should be 9503)
- `cost_exporter.py` had default port 9503 (should be 9504)

### Solution
- Fixed `qoe_exporter.py` to use port 9503 by default
- Fixed `cost_exporter.py` to use port 9504 by default
- Updated docstrings to reflect correct ports

### Testing
```bash
# QoE metrics are now on port 9503
curl localhost:9503/metrics

# Cost metrics are now on port 9504
curl localhost:9504/metrics
```

## 2. Stockholm Pricing & Multi-Currency Support

### Changes
- **Default pricing**: Set to Stockholm, Sweden rates
  - Electricity: 2.0 SEK/kWh
  - CPU: 5.0 SEK/h
- **Currency support**: SEK, USD, and EUR
- **Grafana dashboards**: Added currency dropdown selector

### Load-Aware Cost Calculations
The cost exporter uses **load-aware pricing** that calculates costs based on actual resource usage:
- **CPU costs**: Based on actual CPU core usage over time (from Prometheus metrics)
- **Energy costs**: Based on actual power consumption in watts (from RAPL metrics)

For testing, test results must include:
- `cpu_usage_cores`: Array of CPU measurements (cores used at each interval)
- `power_watts`: Array of power measurements (watts at each interval)  
- `step_seconds`: Measurement interval (e.g., 5 seconds)

Example cost output:
```
# 2.5 Mbps stream costs ~0.11 SEK/min (~6.44 SEK/hour)
cost_total_load_aware{scenario="2.5_Mbps_Stream",currency="SEK",...} 0.10731556
cost_energy_load_aware{scenario="2.5_Mbps_Stream",currency="SEK",...} 0.00176000
cost_compute_load_aware{scenario="2.5_Mbps_Stream",currency="SEK",...} 0.10555556
```

### Configuration
You can now configure pricing using environment variables:

```bash
# Create a .env file (see .env.example)
cp .env.example .env

# Edit for your region:
# Stockholm (default)
CURRENCY=SEK
ENERGY_COST_PER_KWH=2.0
CPU_COST_PER_HOUR=5.0

# United States
CURRENCY=USD
ENERGY_COST_PER_KWH=0.12
CPU_COST_PER_HOUR=0.50

# European Union
CURRENCY=EUR
ENERGY_COST_PER_KWH=0.25
CPU_COST_PER_HOUR=0.75
```

### Grafana Dashboard
The cost dashboards now include a "Currency" dropdown in the top-left corner:
- SEK (Swedish Krona)
- USD (US Dollar)
- EUR (Euro)

## 3. Fixed nginx-rtmp Healthcheck

### Problem
The nginx-rtmp service was always showing as unhealthy because the healthcheck was looking for `/hls/stream.m3u8`, which only exists when a stream is actively broadcasting.

### Solution
Changed the healthcheck to use the proper `/health` endpoint:
```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost/health"]
```

### Result
The nginx-rtmp service will now correctly report as healthy when running, even without an active stream.

## Testing the Changes

### 1. Test Port Assignments
```bash
# Start the services
make up-build

# Check QoE metrics (port 9503)
curl localhost:9503/metrics

# Check cost metrics (port 9504)
curl localhost:9504/metrics

# Verify nginx-rtmp health
docker ps  # Should show nginx-rtmp as "healthy"
```

### 2. Test Currency Support
```bash
# View current pricing
curl localhost:9504/metrics | grep "# HELP"
# Should show "Total cost (SEK)"

# Change currency
export CURRENCY=USD
export ENERGY_COST_PER_KWH=0.12
export CPU_COST_PER_HOUR=0.50
make restart

# Check updated currency
curl localhost:9504/metrics | grep "# HELP"
# Should now show "Total cost (USD)"
```

### 3. Verify nginx-rtmp Healthcheck
```bash
# Check service health
docker ps | grep nginx-rtmp
# Should show "(healthy)" in the status

# Manually test health endpoint
curl localhost:8080/health
# Should return "healthy"
```

## Troubleshooting

### Cost Metrics Show All Zeros

**Problem**: Cost metrics display `0` for all scenarios.

**Cause**: The cost exporter requires load-aware data (CPU usage and power measurements) to calculate costs. If test results don't include these measurements, metrics default to `0`.

**Solution**: Ensure your test results include:
- `cpu_usage_cores`: Array of CPU measurements
- `power_watts`: Array of power measurements
- `step_seconds`: Measurement interval

Example test result format:
```json
{
  "scenarios": [{
    "name": "2.5_Mbps_Stream",
    "streams": 1,
    "bitrate": "2500k",
    "encoder_type": "x264",
    "duration": 60,
    "cpu_usage_cores": [1.25, 1.28, 1.26, ...],
    "power_watts": [52.1, 52.8, 53.4, ...],
    "step_seconds": 5
  }]
}
```

In production, these metrics are automatically collected from Prometheus when the cost exporter runs with `--prometheus-url` flag.

## Files Modified
- `src/exporters/qoe/qoe_exporter.py` - Fixed default port to 9503
- `src/exporters/cost/cost_exporter.py` - Fixed default port to 9504
- `docker-compose.yml` - Updated pricing defaults and fixed nginx healthcheck
- `grafana/provisioning/dashboards/cost-dashboard.json` - Added currency selector
- `grafana/provisioning/dashboards/cost-dashboard-load-aware.json` - Added currency selector
- `.env.example` - Created with pricing configurations

## Backward Compatibility
All changes are backward compatible. The docker-compose.yml uses environment variables with sensible defaults, so existing deployments will continue to work without modification.
