# Grafana Dashboards Quick Reference

## 6 Dashboards - Clear Purposes

### 1. Production Monitoring 
**When:** Daily operations, on-call monitoring  
**URL:** http://localhost:3000/d/ffmpeg-rtmp-prod/  
**Panels:** SLA compliance, job success, bandwidth, resources, exporter health

### 2. Job Scheduler & Queue Management
**When:** Queue issues, job distribution analysis  
**URL:** http://localhost:3000/d/job-scheduler/  
**Panels:** Job states, queue depth, priority, worker assignments

### 3. Worker Node Monitoring
**When:** Worker performance issues, GPU problems  
**URL:** http://localhost:3000/d/worker-monitoring/  
**Panels:** CPU, GPU, memory, temperature, power, heartbeat

### 4. Quality & Performance Metrics
**When:** Quality analysis, scenario comparison  
**URL:** http://localhost:3000/d/quality-metrics/  
**Panels:** VMAF, PSNR, SSIM, frame stats, scenario results

### 5. Cost Analysis
**When:** Financial reporting, cost optimization  
**URL:** http://localhost:3000/d/cost-analysis/  
**Panels:** Cost per scenario, energy vs compute breakdown

### 6. ML Predictions
**When:** Model performance verification  
**URL:** http://localhost:3000/d/ml-predictions/  
**Panels:** Predicted VMAF/PSNR/cost, confidence, training drift

## Deployment

```bash
# Start system (dashboards auto-load)
./full-stack-deploy.sh

# Access Grafana
http://localhost:3000  # admin/admin
```

## Updates

```bash
# Edit dashboard
vim master/monitoring/grafana/provisioning/dashboards/production-monitoring.json

# Reload
docker restart grafana
```

See [README.md](README.md) for full documentation.
