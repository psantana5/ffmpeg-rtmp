# Grafana Dashboard Quick Start

## TL;DR

```bash
docker-compose up -d
open http://localhost:3000  # admin/admin
```

Dashboard auto-loads at:
**http://localhost:3000/d/ffmpeg-rtmp-prod/ffmpeg-rtmp-production-monitoring**

## That's It!

No manual import. No configuration. Just works.

## What You Get

11 panels monitoring:
- ✅ SLA compliance (target: 95%)
- ✅ Job success rate (target: 90%)
- ✅ Bandwidth usage per worker
- ✅ Active jobs count
- ✅ CPU/Memory usage
- ✅ Cancellation stats
- ✅ SLA violations table

## Verify Deployment

```bash
./scripts/deploy-grafana-dashboards.sh
```

## Update Dashboard

```bash
# Edit JSON
vim master/monitoring/grafana/provisioning/dashboards/production-monitoring.json

# Reload
docker restart grafana
```

## Troubleshooting

**No data?**
```bash
# Check worker metrics
curl http://localhost:9091/metrics

# Check VictoriaMetrics
curl http://localhost:8428/api/v1/query?query=up
```

**Dashboard missing?**
```bash
# Restart Grafana
docker restart grafana

# Check logs
docker logs grafana | grep -i dashboard
```

## Full Documentation

- **Quick Start**: [README.md](README.md)
- **Complete Guide**: [AUTOMATED_DEPLOYMENT.md](AUTOMATED_DEPLOYMENT.md)
- **Dashboard JSON**: [production-monitoring.json](../../master/monitoring/grafana/provisioning/dashboards/production-monitoring.json)

## Panel Highlights

1. **SLA Compliance** - Green ≥95%, Yellow ≥80%, Red <80%
2. **Success Rate** - Green ≥90%, Yellow ≥70%, Red <70%
3. **Bandwidth** - Real-time MB/s per worker
4. **Active Jobs** - Current job count
5. **CPU/Memory** - 0-100% per worker
6. **Cancellations** - Graceful (good) vs Forceful (bad)
7. **Violations** - Top 10 problem workers

## Pro Tips

- **Auto-refresh**: Dashboard refreshes every 10s
- **Time range**: Default 6h, adjustable in UI
- **Worker filter**: Click legend to show/hide workers
- **Zoom**: Click and drag on any graph
- **Export**: Share > Export for backup
