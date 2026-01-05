# Grafana Dashboard - FFmpeg-RTMP Production Monitoring

**IMPORTANT: Dashboards are now automatically deployed!**  
See [AUTOMATED_DEPLOYMENT.md](AUTOMATED_DEPLOYMENT.md) for complete guide.

## Quick Start

```bash
# Start stack (dashboards auto-load)
docker-compose up -d

# Verify deployment
./scripts/deploy-grafana-dashboards.sh

# Access Grafana
open http://localhost:3000  # admin/admin

# Production Dashboard:
# http://localhost:3000/d/ffmpeg-rtmp-prod/ffmpeg-rtmp-production-monitoring
```

## Overview

This directory contains Grafana dashboards that are **automatically provisioned** when you start the system with docker-compose.

### Available Dashboards

1. **Production Monitoring** (`production-monitoring.json`) - NEW!
   - 11 comprehensive panels covering Phase 1 metrics
   - SLA compliance, job success rate, bandwidth, resources
   - Auto-loaded at: `/d/ffmpeg-rtmp-prod/`

2. **Worker Monitoring** (`worker-monitoring.json`)
   - Worker-specific resource tracking
   - CPU, memory, GPU utilization

3. **Distributed Scheduler** (`distributed-scheduler.json`)
   - Job queue and scheduling metrics
   - Worker assignments and load balancing

## No Manual Import Required!

❌ **OLD WAY** (manual import):
- Download JSON
- Open Grafana UI
- Import > Upload JSON file
- Configure datasource
- Save

✅ **NEW WAY** (automatic):
- `docker-compose up -d`
- Dashboards appear automatically
- Zero configuration needed

## How It Works

Grafana automatically loads dashboards from:
```
master/monitoring/grafana/provisioning/dashboards/*.json
```

This directory is mounted into the Grafana container via docker-compose:
```yaml
volumes:
  - ./master/monitoring/grafana/provisioning:/etc/grafana/provisioning:ro
```

See [AUTOMATED_DEPLOYMENT.md](AUTOMATED_DEPLOYMENT.md) for full details on:
- Architecture
- Adding new dashboards
- Updating existing dashboards
- Troubleshooting
- Production deployment

## Dashboard Files

Dashboard JSON files are stored at:
- `master/monitoring/grafana/provisioning/dashboards/production-monitoring.json`
- Old location: `docs/grafana/ffmpeg-rtmp-complete-dashboard.json` (deprecated)

## Migration from Old Dashboard

If you previously manually imported the old dashboard:

1. Delete old dashboard from Grafana UI
2. Restart Grafana: `docker restart grafana`
3. New production dashboard will auto-load
4. Access at: http://localhost:3000/d/ffmpeg-rtmp-prod/

## Datasource Configuration

VictoriaMetrics datasource is automatically configured:
- **Name**: VictoriaMetrics
- **UID**: DS_VICTORIAMETRICS
- **URL**: http://victoriametrics:8428
- **Default**: Yes

No manual datasource setup required.

## Documentation

- **[AUTOMATED_DEPLOYMENT.md](AUTOMATED_DEPLOYMENT.md)** - Complete deployment guide
- **[Production Monitoring Dashboard](../../master/monitoring/grafana/provisioning/dashboards/production-monitoring.json)** - Dashboard JSON

## Troubleshooting

**Dashboard not appearing?**
```bash
# Run automated deployment script
./scripts/deploy-grafana-dashboards.sh
```

**No data in panels?**
```bash
# Check workers are exporting metrics
curl http://localhost:9091/metrics

# Check VictoriaMetrics receiving data
curl http://localhost:8428/api/v1/query?query=up
```

**Need to update dashboard?**
```bash
# Edit JSON file
vim master/monitoring/grafana/provisioning/dashboards/production-monitoring.json

# Restart Grafana
docker restart grafana
```

For detailed troubleshooting, see [AUTOMATED_DEPLOYMENT.md](AUTOMATED_DEPLOYMENT.md#troubleshooting).

## Old Manual Import Instructions (Deprecated)

The old `ffmpeg-rtmp-complete-dashboard.json` in this directory is **deprecated**.  
Use the automatically provisioned dashboards instead.

If you need the old dashboard for reference, it's still available at:
`docs/grafana/ffmpeg-rtmp-complete-dashboard.json`
