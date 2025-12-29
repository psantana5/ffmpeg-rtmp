# RAPL Exporter

Monitors CPU power consumption using Intel RAPL (Running Average Power Limit) interface.

## Overview

The RAPL exporter reads energy consumption counters from the Linux kernel's powercap interface and exposes them as Prometheus metrics. It provides real-time power consumption data for CPU packages and their subzones (cores, GPU, DRAM, etc.).

## Requirements

- **Intel CPU** with RAPL support (Sandy Bridge or newer)
- **Linux kernel** with powercap interface
- **Read access** to `/sys/class/powercap/intel-rapl:*`

## Metrics

### `rapl_power_watts`

Current power consumption in watts.

**Type**: Gauge

**Labels**:
- `zone`: CPU package or subzone name (e.g., `package_0`, `package_0_core`, `package_0_dram`)

**Example**:
```
rapl_power_watts{zone="package_0"} 45.23
rapl_power_watts{zone="package_0_core"} 38.12
rapl_power_watts{zone="package_0_dram"} 7.11
```

## Endpoints

- **Metrics**: `http://localhost:9500/metrics`
- **Health**: `http://localhost:9500/health`

## Configuration

### Environment Variables

- `RAPL_EXPORTER_PORT`: Port to listen on (default: 9500)

### Docker Compose

```yaml
rapl-exporter:
  build: ./src/exporters/rapl
  privileged: true
  volumes:
    - /sys/class/powercap:/sys/class/powercap:ro
    - /sys/devices:/sys/devices:ro
  ports:
    - "9500:9500"
  environment:
    - RAPL_EXPORTER_PORT=9500
```

## How It Works

1. On startup, scans `/sys/class/powercap/intel-rapl:*` for available zones
2. Reads `energy_uj` (energy in microjoules) for each zone
3. Calculates power as: `power_watts = (energy_delta_uj / interval_seconds) / 1,000,000`
4. Exposes current power consumption on each Prometheus scrape

## Troubleshooting

### "No RAPL zones found"

**Cause**: RAPL interface not available

**Solutions**:
1. Check if running on Intel CPU: `cat /proc/cpuinfo | grep "model name"`
2. Check if RAPL is available: `ls /sys/class/powercap/`
3. Verify kernel module loaded: `lsmod | grep rapl`

### "Permission denied" reading energy_uj

**Cause**: Insufficient permissions

**Solutions**:
1. Run container with `privileged: true` (already configured)
2. Or grant read access: `sudo chmod -R a+r /sys/class/powercap/`

### Power readings seem incorrect

**Possible causes**:
- Thermal throttling: CPU reduces power under heavy load
- Turbo boost: Power spikes during burst workloads
- RAPL accuracy: ±5-10% typical error margin

**Verification**:
```bash
# Compare with system tools
sudo turbostat --show PkgWatt --interval 1
```

## Technical Details

### RAPL Zones

Typical zone hierarchy:
```
intel-rapl:0 (package-0)
├── intel-rapl:0:0 (core)
├── intel-rapl:0:1 (uncore/gpu)
└── intel-rapl:0:2 (dram)

intel-rapl:1 (package-1) # If multi-socket
├── ...
```

### Accuracy

- **Sampling rate**: Every Prometheus scrape (default: 5s)
- **Resolution**: Microjoules (μJ)
- **Accuracy**: ±5-10% typical, ±15% worst case
- **Latency**: <1ms to read counters

### Counter Wraparound

Energy counters are 32-bit and wrap around. The exporter handles wraparound by tracking the maximum range for each zone.

## Performance Impact

- **CPU**: Negligible (<0.1%)
- **Memory**: ~10-20 MB
- **Disk I/O**: None (reads from kernel memory)
- **Network**: ~500 bytes per scrape

## Related Documentation

- [Intel RAPL Interface](https://www.kernel.org/doc/html/latest/power/powercap/powercap.html)
- [Exporter Overview](../README.md)
- [Architecture](../../../docs/architecture.md)

## Examples

### Query Power Consumption

```promql
# Total system power
sum(rapl_power_watts)

# CPU core power only
rapl_power_watts{zone=~".*core"}

# Average power over 5 minutes
avg_over_time(rapl_power_watts[5m])

# Power increase during test
rapl_power_watts - rapl_power_watts offset 5m
```

### Alert on High Power

```yaml
- alert: HighCPUPower
  expr: rapl_power_watts{zone="package_0"} > 200
  for: 5m
  annotations:
    summary: "CPU power exceeds 200W for 5 minutes"
```
