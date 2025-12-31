# ✅ Setup Complete: How to Access Your Grafana Dashboards

## Quick Summary

All exporters are configured and ready to collect metrics. VictoriaMetrics is set up to scrape them every second. Grafana dashboards have been streamlined and documented.

---

## 🚀 Getting Started (3 Steps)

### Step 1: Start the Monitoring Stack

Choose your deployment mode:

**Option A: Docker Compose (Development/Local Testing)**
```bash
cd /path/to/ffmpeg-rtmp
make up-build
```

Wait 30-60 seconds for all services to become healthy.

**Option B: Production (Distributed Mode)**
```bash
# On master node
make vm-up-build
```

### Step 2: Verify All Exporters Are Running

```bash
# Check container status
docker compose ps

# All these should show "Up" or "healthy":
# - victoriametrics (port 8428)
# - grafana (port 3000)
# - cpu-exporter-go (port 9500)
# - gpu-exporter-go (port 9505)
# - ffmpeg-exporter (port 9506)
# - docker-stats-exporter (port 9501)
# - node-exporter (port 9100)
# - cadvisor (port 8080)
# - nginx-exporter (port 9728)
# - results-exporter (port 9502)
# - qoe-exporter (port 9503)
# - cost-exporter (port 9504)
# - exporter-health-checker (port 9600)
```

Quick health check:
```bash
# VictoriaMetrics
curl http://localhost:8428/health
# Expected: {"status":"ok"}

# Check targets being scraped
curl http://localhost:8428/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
# All should show "health": "up"
```

### Step 3: Access Grafana

1. Open browser: **http://localhost:3000**
2. Login: 
   - Username: `admin`
   - Password: `admin`
3. (Optional) Change password or click "Skip"

---

## 📊 Your Dashboards (7 Total)

### 🌟 Start Here: System Overview
**URL:** http://localhost:3000/d/system-overview

**What you'll see:**
- CPU & GPU power consumption (live)
- System resource usage (CPU, Memory, GPU %)
- Network traffic
- All exporter health status

**Purpose:** Primary dashboard for overall health check

---

### 🎬 Transcoding Performance
**URL:** http://localhost:3000/d/transcoding-performance

**What you'll see:**
- FFmpeg FPS and bitrate (real-time)
- Encoding speed and dropped frames
- Encoder load and latency
- RTMP server stats

**Purpose:** Monitor encoding quality and throughput

**When to use:** While running transcoding tests

---

### 🔧 Hardware Details
**URL:** http://localhost:3000/d/hardware-details

**What you'll see:**
- Detailed CPU RAPL power zones
- GPU temperature, utilization, memory
- GPU clock speeds
- Container resource usage

**Purpose:** Deep dive into hardware metrics

**When to use:** Diagnosing performance or power issues

---

### ⚡ Energy Efficiency Analysis
**URL:** http://localhost:3000/d/energy-efficiency-dashboard

**What you'll see:**
- Energy efficiency leaderboard
- Pixels delivered per joule
- CPU vs GPU scaling analysis

**Purpose:** Find optimal configurations

**When to use:** After running batch tests

---

### 🔌 Power Monitoring
**URL:** http://localhost:3000/d/power-monitoring

**What you'll see:**
- Real-time CPU power (RAPL)
- Docker container overhead
- Power consumption trends

**Purpose:** Track power usage over time

---

### 🎯 QoE (Quality of Experience)
**URL:** http://localhost:3000/d/qoe-dashboard

**What you'll see:**
- VMAF and PSNR quality scores
- Quality per watt efficiency
- QoE-weighted metrics

**Purpose:** Analyze video quality vs efficiency

---

### 💰 Cost & ROI Analysis
**URL:** http://localhost:3000/d/cost-roi-dashboard

**What you'll see:**
- Total cost (compute + energy)
- Cost per pixel delivered
- Cost per viewer watch hour
- ROI metrics

**Purpose:** Business analysis and budgeting

---

## 🧪 Test the Setup

### Quick Verification

1. **Open System Overview dashboard**
   ```
   http://localhost:3000/d/system-overview
   ```

2. **Scroll to bottom → Check "Exporter Status" table**
   - All 12 exporters should show **green "UP"**
   - If any show red "DOWN", see troubleshooting below

3. **Look at power consumption graphs**
   - Should show current CPU/GPU power draw
   - Values will be low if idle (expected)

### Run a Test to See Live Data

```bash
# Terminal 1: Start a simple transcoding test
python3 scripts/run_tests.py single --name "test1" --bitrate 2000k --duration 60

# Browser: Open these dashboards in separate tabs
# Tab 1: System Overview - watch power consumption rise
# Tab 2: Transcoding Performance - see FPS, bitrate, encoding stats
# Tab 3: Hardware Details - monitor detailed hardware metrics
```

You should see:
- **System Overview:** CPU power increasing, CPU usage % going up
- **Transcoding Performance:** FPS updating, bitrate matching target, frames counting up
- **Hardware Details:** CPU/GPU metrics updating in real-time

---

## 🔍 Troubleshooting

### No Data in Dashboards

**Symptoms:** Panels show "No data"

**Solutions:**
1. Check VictoriaMetrics:
   ```bash
   curl http://localhost:8428/health
   ```
2. Check exporters are running:
   ```bash
   docker compose ps
   ```
3. Restart if needed:
   ```bash
   docker compose restart victoriametrics grafana
   ```

### Some Exporters Show "DOWN"

**Check which exporter:**
```bash
# In System Overview, note which exporter is down
# Then check its logs:
docker compose logs <exporter-name>

# Example:
docker compose logs cpu-exporter-go
docker compose logs ffmpeg-exporter
```

**Common issues:**
- **cpu-exporter-go:** Requires Linux with RAPL support
- **gpu-exporter-go:** Requires NVIDIA GPU and nvidia-docker
- **ffmpeg-exporter:** Needs FFmpeg process running

**Restart exporter:**
```bash
docker compose restart <exporter-name>
```

### GPU Exporters Not Working

**For nvidia profile services:**
```bash
docker compose --profile nvidia up -d
```

**Verify GPU access:**
```bash
nvidia-smi
# Should show GPU info
```

### Dashboard Loads Slowly

**Solutions:**
1. Reduce time range (top-right): Use "Last 15m" instead of "Last 1h"
2. Increase refresh interval: Change from "5s" to "30s"
3. Check VictoriaMetrics resources:
   ```bash
   docker stats victoriametrics
   ```

---

## 📚 Full Documentation

For complete guides:

1. **Dashboard Guide:** `master/monitoring/grafana/provisioning/dashboards/README.md`
   - All 7 dashboards explained
   - Exporter reference table
   - Panel descriptions

2. **Complete Walkthrough:** `master/monitoring/grafana/GRAFANA_WALKTHROUGH.md`
   - 16KB comprehensive guide
   - Step-by-step instructions
   - Common tasks and examples
   - Troubleshooting section

3. **VictoriaMetrics Config:** `master/monitoring/victoriametrics.yml`
   - All 12 scrape jobs
   - Target endpoints
   - Labels and configuration

4. **Docker Compose:** `docker-compose.yml`
   - All service definitions
   - Port mappings
   - Health checks

---

## 🎯 Next Steps

Now that everything is verified:

1. ✅ **All exporters are UP** (confirmed in System Overview)
2. ✅ **Dashboards are accessible** (can navigate to all 7)
3. ✅ **Data is flowing** (see real-time metrics)

**What's next:**

### For Development/Testing:
```bash
# Run batch tests to populate data
python3 scripts/run_tests.py batch --file batch_stress_matrix.json

# Then explore:
# - Energy Efficiency Analysis - find optimal settings
# - Cost & ROI Analysis - calculate costs
# - QoE Dashboard - analyze quality metrics
```

### For Production:
```bash
# Deploy distributed mode
make build-master build-agent

# Start master with monitoring
./bin/master --port 8080 &
make vm-up-build

# Register agents
./bin/agent --register --master http://MASTER_IP:8080
```

### For Optimization:
1. Use **Energy Efficiency Analysis** to compare configurations
2. Check **Cost & ROI** dashboard for budget planning
3. Monitor **QoE Dashboard** to ensure quality standards

---

## 📊 Exporter Reference Card

Quick reference for direct metrics access:

| Exporter | Port | Health Check | Metrics Endpoint |
|----------|------|--------------|------------------|
| VictoriaMetrics | 8428 | `curl http://localhost:8428/health` | `http://localhost:8428/metrics` |
| Grafana | 3000 | Browser: `http://localhost:3000` | N/A |
| CPU (RAPL) | 9500 | `curl http://localhost:9500/health` | `http://localhost:9500/metrics` |
| GPU (NVML) | 9505 | `curl http://localhost:9505/health` | `http://localhost:9505/metrics` |
| FFmpeg Stats | 9506 | `curl http://localhost:9506/health` | `http://localhost:9506/metrics` |
| Docker Stats | 9501 | `curl http://localhost:9501/health` | `http://localhost:9501/metrics` |
| Node Exporter | 9100 | `curl http://localhost:9100/` | `http://localhost:9100/metrics` |
| cAdvisor | 8080 | Browser: `http://localhost:8080` | `http://localhost:8080/metrics` |
| Nginx RTMP | 9728 | `curl http://localhost:9728/` | `http://localhost:9728/metrics` |
| Results | 9502 | `curl http://localhost:9502/health` | `http://localhost:9502/metrics` |
| QoE | 9503 | `curl http://localhost:9503/health` | `http://localhost:9503/metrics` |
| Cost | 9504 | `curl http://localhost:9504/health` | `http://localhost:9504/metrics` |
| Health Checker | 9600 | `curl http://localhost:9600/health` | `http://localhost:9600/metrics` |

---

## 🆘 Need Help?

- **Full Walkthrough:** See `master/monitoring/grafana/GRAFANA_WALKTHROUGH.md`
- **Dashboard Details:** See `master/monitoring/grafana/provisioning/dashboards/README.md`
- **Project Docs:** See main `README.md`
- **Issues:** https://github.com/psantana5/ffmpeg-rtmp/issues

---

**Enjoy your monitoring stack! 🎉**

All 12 exporters are configured, VictoriaMetrics is collecting every second, and Grafana has 7 streamlined dashboards ready to visualize your data.
