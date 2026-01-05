# Grafana Dashboard Automated Deployment

Complete guide for automated Grafana dashboard provisioning and deployment.

## Overview

Dashboards are automatically loaded via Grafana's provisioning system when using docker-compose. No manual import required.

**Key Features:**
- ✅ Zero-touch deployment with docker-compose
- ✅ Automatic dashboard loading on Grafana startup
- ✅ Version controlled dashboard JSON files
- ✅ Instant updates when restarting Grafana
- ✅ No API keys or manual UI steps required

## Quick Start

### Deploy with Docker Compose

```bash
# Start entire stack (includes Grafana + dashboards)
docker-compose up -d

# Access Grafana
open http://localhost:3000
# User: admin / Pass: admin

# Production dashboard will be automatically loaded at:
# http://localhost:3000/d/ffmpeg-rtmp-prod/ffmpeg-rtmp-production-monitoring
```

### Verify Deployment

```bash
# Run automated deployment script
./scripts/deploy-grafana-dashboards.sh
```

Expected output:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Grafana Dashboard Automated Deployment
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Step 1: Checking prerequisites...

✓ Grafana container is running
✓ Provisioning directory mounted

Step 2: Dashboard inventory

Available dashboards:
  • production-monitoring (24K)
  • worker-monitoring (18K)
  • distributed-scheduler (22K)

Step 3: Validating dashboard JSON...
  ✓ production-monitoring.json
  ✓ worker-monitoring.json
  ✓ distributed-scheduler.json

  Valid: 3/3 dashboards

Step 4: Reloading Grafana...
Waiting for Grafana to start... ✓

Step 5: Verifying dashboards loaded...
✓ Dashboard verified: ffmpeg-rtmp-prod

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Deployment Complete!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Access Grafana:
  URL: http://localhost:3000
  User: admin
  Pass: admin

Production Dashboard:
  http://localhost:3000/d/ffmpeg-rtmp-prod/ffmpeg-rtmp-production-monitoring
```

## How It Works

### Provisioning Architecture

```
docker-compose.yml
    │
    └─> Grafana Container
         │
         ├─> /etc/grafana/provisioning/datasources/
         │    └─> prometheus.yml (VictoriaMetrics datasource)
         │
         └─> /etc/grafana/provisioning/dashboards/
              ├─> default.yml (dashboard provider config)
              └─> *.json (dashboard files)
                   ├─> production-monitoring.json ← NEW!
                   ├─> worker-monitoring.json
                   └─> distributed-scheduler.json
```

### Docker Compose Configuration

From `docker-compose.yml`:

```yaml
grafana:
  image: grafana/grafana:latest
  container_name: grafana
  user: root
  ports:
    - "3000:3000"
  environment:
    - GF_SECURITY_ADMIN_PASSWORD=admin
    - GF_USERS_ALLOW_SIGN_UP=false
  volumes:
    - grafana-data:/var/lib/grafana
    - ./master/monitoring/grafana/provisioning:/etc/grafana/provisioning:ro
```

The key line:
```yaml
- ./master/monitoring/grafana/provisioning:/etc/grafana/provisioning:ro
```

This mounts our local provisioning directory into Grafana, enabling automatic dashboard loading.

### Provisioning Config

**Datasource**: `master/monitoring/grafana/provisioning/datasources/prometheus.yml`

```yaml
apiVersion: 1
datasources:
  - name: VictoriaMetrics
    type: prometheus
    uid: DS_VICTORIAMETRICS
    access: proxy
    url: http://victoriametrics:8428
    isDefault: true
```

**Dashboard Provider**: `master/monitoring/grafana/provisioning/dashboards/default.yml`

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

## Production Monitoring Dashboard

### Dashboard Details

**File**: `master/monitoring/grafana/provisioning/dashboards/production-monitoring.json`

**UID**: `ffmpeg-rtmp-prod`

**URL**: `http://localhost:3000/d/ffmpeg-rtmp-prod/ffmpeg-rtmp-production-monitoring`

**Panels**: 11 comprehensive monitoring panels

### Panel Overview

1. **SLA Compliance Rate** (Gauge)
   - Target: 95% compliance
   - Shows: Current SLA compliance percentage

2. **Job Success Rate** (Gauge)
   - Target: 90% success
   - Shows: Ratio of completed vs failed jobs

3. **Bandwidth Usage** (Timeseries)
   - Shows: MB/s per worker
   - Tracks network throughput

4. **Active Jobs** (Stat)
   - Shows: Currently running jobs across all workers

5. **SLA Compliance Trend** (Timeseries)
   - Shows: SLA compliance over time
   - Helps identify degradation

6. **Job Completion Rates** (Timeseries)
   - Shows: Completed, Failed, Canceled jobs
   - Color-coded: Green/Red/Yellow

7. **CPU Usage by Worker** (Timeseries)
   - Shows: CPU % per worker
   - Max: 100%

8. **Memory Usage by Worker** (Timeseries)
   - Shows: Memory % per worker
   - Max: 100%

9. **Worker Bandwidth Utilization** (Timeseries)
   - Shows: Bandwidth usage as % of capacity
   - Helps with capacity planning

10. **Cancellation Stats** (Stat)
    - Shows: Graceful vs Forceful cancellations
    - Color-coded: Green (graceful), Red (forceful)

11. **Top 10 SLA Violations** (Table)
    - Shows: Workers with most SLA violations in 24h
    - Helps identify problem workers

### Dashboard Settings

- **Refresh**: 10 seconds (auto-refresh)
- **Time Range**: Last 6 hours (default)
- **Theme**: Dark
- **Tags**: production, ffmpeg-rtmp, phase1

## Adding New Dashboards

### Step 1: Create Dashboard JSON

```bash
# Create new dashboard file
vim master/monitoring/grafana/provisioning/dashboards/my-dashboard.json
```

### Step 2: Set Required Fields

Ensure these fields are set:

```json
{
  "id": null,                    // Must be null for provisioning
  "uid": "my-dashboard-uid",     // Unique identifier
  "title": "My Dashboard",       // Display name
  "tags": ["my-tag"],           // Optional tags
  "timezone": "",               // Use browser timezone
  "schemaVersion": 38,          // Current schema version
  "version": 0,                 // Will auto-increment
  "panels": [...]               // Your panels
}
```

### Step 3: Validate JSON

```bash
python3 -m json.tool my-dashboard.json > /dev/null && echo "Valid"
```

### Step 4: Reload Grafana

```bash
# Option 1: Restart container
docker restart grafana

# Option 2: Use deployment script
./scripts/deploy-grafana-dashboards.sh
```

### Step 5: Access Dashboard

```
http://localhost:3000/d/{uid}/{title}
```

## Updating Existing Dashboards

### Method 1: Edit JSON Directly (Recommended)

```bash
# 1. Edit dashboard file
vim master/monitoring/grafana/provisioning/dashboards/production-monitoring.json

# 2. Validate
python3 -m json.tool production-monitoring.json > /dev/null

# 3. Reload Grafana
docker restart grafana
```

### Method 2: Export from UI

If you prefer UI editing:

```bash
# 1. Make changes in Grafana UI
# 2. Export dashboard JSON (Share > Export > Save to file)
# 3. Copy to provisioning directory
cp ~/Downloads/dashboard.json master/monitoring/grafana/provisioning/dashboards/production-monitoring.json

# 4. Fix required fields
# - Set "id": null
# - Keep same "uid"
# - Set "version": 0

# 5. Reload
docker restart grafana
```

## Troubleshooting

### Dashboard Not Appearing

**Check 1: Container running?**
```bash
docker ps | grep grafana
```

**Check 2: Volume mounted?**
```bash
docker inspect grafana | grep -A 5 Mounts
```

**Check 3: JSON valid?**
```bash
python3 -m json.tool production-monitoring.json
```

**Check 4: Grafana logs**
```bash
docker logs grafana | grep -i dashboard
```

Expected output:
```
logger=provisioning.dashboard type=file name=Default
Provisioning dashboards from configuration
Successfully loaded dashboard from file /etc/grafana/provisioning/dashboards/production-monitoring.json
```

### Dashboard Shows "No Data"

**Check 1: Datasource connected?**
```bash
curl -s http://localhost:3000/api/datasources | jq '.[] | {name, url, type}'
```

**Check 2: VictoriaMetrics responding?**
```bash
curl http://localhost:8428/api/v1/query?query=up
```

**Check 3: Workers exporting metrics?**
```bash
curl http://localhost:9091/metrics | grep worker_
```

**Check 4: Prometheus queries correct?**
- Open dashboard in edit mode
- Check panel queries
- Test in VictoriaMetrics: http://localhost:8428/vmui

### Permission Denied Errors

**Fix**: Grafana container runs as root in docker-compose

```yaml
grafana:
  user: root  # Required for provisioning access
```

If you get permission errors:

```bash
# Fix provisioning directory permissions
chmod -R 755 master/monitoring/grafana/provisioning
```

### Dashboard Changes Not Appearing

**Cause**: Grafana caches dashboards

**Solution**:
```bash
# Hard restart
docker-compose down
docker-compose up -d

# Or clear Grafana database
docker volume rm ffmpeg-rtmp_grafana-data
docker-compose up -d
```

## Best Practices

### Dashboard Design

1. **Keep UIDs stable** - Don't change UID after deployment
2. **Use meaningful names** - Title should describe purpose
3. **Add tags** - Group related dashboards
4. **Version control** - Commit dashboard JSON to git
5. **Test queries** - Verify in VictoriaMetrics first

### Deployment Workflow

1. **Develop locally** - Use Grafana UI for quick iteration
2. **Export to JSON** - Export finished dashboard
3. **Clean JSON** - Remove IDs, set version to 0
4. **Validate** - Check JSON syntax
5. **Commit** - Add to git
6. **Deploy** - Restart Grafana container

### Panel Guidelines

- **Use templating** - Variables for worker_id, time ranges
- **Set thresholds** - Yellow/Red warnings for critical metrics
- **Add units** - Percent, MB/s, seconds, etc.
- **Meaningful legends** - Use {{worker_id}}, {{job_id}}
- **Appropriate visualizations** - Gauge for %, timeseries for trends

## API Alternative

If you prefer API-based deployment:

```bash
# Upload dashboard via API
curl -X POST http://admin:admin@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @production-monitoring.json
```

However, **provisioning is recommended** because:
- ✅ Automatic on startup
- ✅ Version controlled
- ✅ No API keys needed
- ✅ Consistent across environments

## Production Deployment

### Kubernetes

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards
data:
  production-monitoring.json: |
    <paste dashboard JSON>
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
spec:
  template:
    spec:
      containers:
      - name: grafana
        volumeMounts:
        - name: dashboards
          mountPath: /etc/grafana/provisioning/dashboards
      volumes:
      - name: dashboards
        configMap:
          name: grafana-dashboards
```

### Docker Swarm

```yaml
services:
  grafana:
    configs:
      - source: production-dashboard
        target: /etc/grafana/provisioning/dashboards/production-monitoring.json

configs:
  production-dashboard:
    file: ./master/monitoring/grafana/provisioning/dashboards/production-monitoring.json
```

## References

- **Grafana Provisioning Docs**: https://grafana.com/docs/grafana/latest/administration/provisioning/
- **Dashboard JSON Schema**: https://grafana.com/docs/grafana/latest/dashboards/json-model/
- **VictoriaMetrics Integration**: https://docs.victoriametrics.com/Grafana.html

## Support

For issues:
1. Check this guide's troubleshooting section
2. Review Grafana logs: `docker logs grafana`
3. Validate JSON: `python3 -m json.tool dashboard.json`
4. Run deployment script: `./scripts/deploy-grafana-dashboards.sh`
