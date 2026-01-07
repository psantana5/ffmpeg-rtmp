# Complete Grafana Walkthrough Guide

This guide walks you through accessing and using Grafana to visualize data from all the exporters in the FFmpeg RTMP monitoring stack.

## Table of Contents
1. [Initial Setup](#initial-setup)
2. [Accessing Grafana](#accessing-grafana)
3. [Dashboard Tour](#dashboard-tour)
4. [Understanding the Data](#understanding-the-data)
5. [Common Tasks](#common-tasks)
6. [Troubleshooting](#troubleshooting)

---

## Initial Setup

### Prerequisites

Make sure you have the monitoring stack running:

**Docker Compose (Development):**
```bash
cd /path/to/ffmpeg-rtmp
make up-build
```

**Distributed Mode (Production):**
```bash
# On master node
make vm-up-build
```

Wait about 30-60 seconds for all services to become healthy.

### Verify Services Are Running

```bash
# Check all containers are up
docker compose ps

# Expected output should show these services running:
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

### Verify VictoriaMetrics is Collecting Data

```bash
# Check VictoriaMetrics health
curl http://localhost:8428/health

# Expected output: {"status":"ok"}

# Check scrape targets
curl http://localhost:8428/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'

# All targets should show "health": "up"
```

---

## Accessing Grafana

### Step 1: Open Grafana in Your Browser

Navigate to:
```
http://localhost:3000
```

For remote servers, replace `localhost` with your server's IP address.

### Step 2: Login

On the login page:
- **Username:** `admin`
- **Password:** `admin`

On first login, you'll be prompted to change the password. You can:
- Set a new password
- Or click "Skip" to continue with default credentials

### Step 3: Grafana Home Page

After login, you'll see the Grafana home page with:
- Left sidebar with icons for navigation
- Main area showing "Welcome to Grafana"
- Recent dashboards (will be empty initially)

---

## Dashboard Tour

### Opening Dashboards

1. Click the **Dashboards** icon (四) in the left sidebar
2. You'll see a list of all available dashboards
3. Click any dashboard name to open it

### Recommended Exploration Order

#### 1. System Overview (Start Here) 

**Purpose:** Get an overall view of system health and all exporters.

**What You'll See:**
- **Top Row:** Two large time-series graphs
  - Left: CPU Power Consumption (RAPL) - shows watts consumed by CPU cores, DRAM, etc.
  - Right: GPU Power Consumption - shows watts consumed by GPU(s)
  
- **Middle Row:** Four gauges showing current status
  - CPU Usage %
  - Memory Usage %
  - GPU Utilization %
  - Exporter Health (UP/DOWN)

- **Third Row:** Two graphs
  - Left: Network Traffic (RX/TX bytes per second)
  - Right: Docker Container CPU Usage (each container's CPU %)

- **Bottom:** Table showing all exporter status
  - Green "UP" = exporter is healthy
  - Red "DOWN" = exporter has issues

**How to Interpret:**
- Power graphs show energy consumption in real-time
- Gauges turn yellow/red when resources are high
- Network spikes indicate data transfer activity
- Container CPU shows which containers are busy

**Auto-Refresh:** Dashboard updates every 5 seconds automatically.

#### 2. Transcoding Performance

**Purpose:** Monitor FFmpeg encoding metrics in real-time.

**What You'll See:**
- **Top Row:**
  - Left: FFmpeg Encoding FPS (frames per second)
  - Right: Output Bitrate (current + rolling average)

- **Middle Row:** Four key metrics
  - Encoding Speed (1.0 = realtime, >1.0 = faster than realtime)
  - Dropped Frames (should be low/zero)
  - Total Frames Processed
  - Encoder Load %

- **Third Row:**
  - Left: Encode Latency (milliseconds per frame)
  - Right: RTMP Server Stats (active streams, viewers)

- **Bottom:** Frame Processing Rate over time

**How to Interpret:**
- FPS should be stable (e.g., 30 or 60 depending on your source)
- Speed > 1.0 means encoding faster than playback speed
- Dropped frames indicate performance issues
- Higher encoder load = more CPU/GPU usage

**When to Use:** While running transcoding tests or streaming.

#### 3. Hardware Details

**Purpose:** Deep dive into hardware-level metrics.

**What You'll See:**
- **Row 1:**
  - Left: CPU RAPL Power Zones - detailed breakdown (package, cores, DRAM, GPU)
  - Right: GPU Temperature in Celsius

- **Row 2:**
  - Left: GPU Utilization Breakdown (core, memory, encoder, decoder)
  - Right: GPU Memory Usage (used vs total)

- **Row 3:**
  - Left: GPU Clock Speeds (graphics, SM, memory clocks in Hz)
  - Right: GPU Power Draw vs Limit

- **Row 4:**
  - Left: Container Memory Usage (per container)
  - Right: Container Network Traffic

**How to Interpret:**
- RAPL zones show where power is consumed (cores vs DRAM vs uncore)
- GPU temperature should stay below thermal throttle point (~83°C for most GPUs)
- Encoder/Decoder utilization shows if GPU video engines are used
- Clock speeds indicate GPU boost state

**When to Use:** Diagnosing power issues, thermal problems, or GPU utilization.

#### 4. Energy Efficiency Analysis

**Purpose:** Compare test scenarios for energy efficiency.

**What You'll See:**
- Energy Efficiency Leaderboard (sorted by efficiency score)
- Pixels Delivered per Joule
- Energy Wasted vs Optimal
- CPU vs GPU Scaling analysis
- Efficiency Stability metrics

**How to Interpret:**
- Higher "pixels per joule" = more energy efficient
- Compare different codec/bitrate/resolution combinations
- Identify sweet spots for your workload

**When to Use:** After running batch tests to optimize configurations.

#### 5. Power Monitoring

**Purpose:** Track power consumption with historical trends.

**What You'll See:**
- CPU Package Power (RAPL) time series
- Docker Container Overhead
- Power consumption baselines

**How to Interpret:**
- Baseline power = idle system consumption
- Spikes indicate transcoding activity
- Container overhead shows Docker's impact

**When to Use:** Analyzing power trends over time, capacity planning.

#### 6. QoE (Quality of Experience)

**Purpose:** Monitor video quality metrics.

**What You'll See:**
- VMAF scores (0-100, higher = better quality)
- PSNR scores (in dB, higher = better)
- Quality per Watt efficiency
- QoE-weighted efficiency scores

**How to Interpret:**
- VMAF > 95 = excellent quality
- VMAF 80-95 = good quality
- VMAF < 80 = quality degradation noticeable
- Quality per Watt shows efficiency with quality consideration

**When to Use:** After quality analysis runs, comparing codecs/settings.

#### 7. Cost & ROI Analysis

**Purpose:** Business metrics - cost tracking and ROI.

**What You'll See:**
- Total Cost (compute + energy)
- Cost per Pixel Delivered
- Cost per Viewer Watch Hour
- Load-aware cost calculations

**How to Interpret:**
- Costs calculated from actual CPU usage and power consumption
- Compare scenarios to find most cost-effective configuration
- ROI metrics for business decisions

**When to Use:** Capacity planning, budget analysis, comparing cloud vs on-prem.

---

## Understanding the Data

### Time Range Selection

Top-right corner of each dashboard:
- Default: "Last 15 minutes"
- Click to change: Last 5m, 15m, 30m, 1h, 6h, 24h, etc.
- Or set custom range: "From" and "To" dates

**Tip:** Use shorter ranges (5m-15m) for real-time monitoring, longer ranges (1h-24h) for trend analysis.

### Auto-Refresh

Next to time range:
- Default: "5s" (refreshes every 5 seconds)
- Click to change: Off, 5s, 10s, 30s, 1m, 5m, 15m, 30m, 1h, 2h, 1d
- Toggle on/off with the pause/play button

**Tip:** Use 5s for active monitoring, 30s-1m to reduce load, or Off when reviewing historical data.

### Panel Interactions

Each panel (graph, gauge, table) supports:
- **Hover:** Shows exact values at cursor position
- **Click & Drag:** Zoom into a time range
- **Double-Click:** Reset zoom
- **Click Legend:** Hide/show a series
- **Panel Menu (top-right of panel):**
  - View → See panel in full screen
  - Edit → Modify panel (requires permissions)
  - Share → Get link or export image
  - Inspect → View raw data and queries

### Colors and Thresholds

- **Green:** Normal/good (e.g., low CPU usage)
- **Yellow:** Warning (e.g., moderate resource usage)
- **Orange:** High (e.g., high resource usage)
- **Red:** Critical (e.g., >90% resource usage, exporter down)

---

## Common Tasks

### Task 1: Check if All Exporters Are Working

1. Open **System Overview** dashboard
2. Scroll to bottom to **Exporter Status** table
3. Look at the "Status" column:
   - All should be green "UP"
   - If any show red "DOWN", see troubleshooting section

### Task 2: Monitor a Transcoding Test in Real-Time

1. Start your transcoding test in terminal:
   ```bash
   python3 scripts/run_tests.py single --name "test1" --bitrate 2000k --duration 60
   ```

2. Open **Transcoding Performance** dashboard

3. Watch these panels update:
   - FFmpeg Encoding FPS (should match your target fps)
   - Output Bitrate (should match your target bitrate)
   - Encoding Speed (should be >1.0 for smooth processing)
   - Dropped Frames (should stay at 0)

4. Simultaneously open **System Overview** to see:
   - CPU/GPU power consumption increase
   - CPU/Memory usage increase
   - Container CPU usage spike

### Task 3: Compare Power Consumption Across Tests

1. Run multiple tests with different settings
2. Open **Power Monitoring** dashboard
3. Set time range to cover all tests (e.g., "Last 1 hour")
4. Identify power spikes for each test
5. Note the baseline power between tests

### Task 4: Find Most Efficient Configuration

1. Run batch tests:
   ```bash
   python3 scripts/run_tests.py batch --file batch_stress_matrix.json
   ```

2. Open **Energy Efficiency Analysis** dashboard

3. Look at:
   - Energy Efficiency Leaderboard (top configurations)
   - Pixels per Joule graph (higher = better)
   - Energy Wasted chart (lower = better)

4. Note the top 3 configurations for your use case

### Task 5: Calculate Cost for Production Deployment

1. Run representative test workload
2. Open **Cost & ROI Analysis** dashboard
3. Note:
   - Total Cost per test run
   - Cost per Pixel Delivered
   - Cost per Viewer Watch Hour
4. Extrapolate to production scale:
   - Cost per hour × 24 × 30 = monthly cost estimate
   - Cost per viewer × expected viewers = total cost

### Task 6: Diagnose Performance Issues

**Scenario: Encoding is slow or dropping frames**

1. Open **Transcoding Performance** dashboard
   - Check Encoding Speed (should be >1.0)
   - Check Dropped Frames (should be 0)
   - Check Encoder Load (if >90%, may be bottleneck)

2. Open **System Overview** dashboard
   - Check CPU Usage (if >90%, CPU bottleneck)
   - Check Memory Usage (if >90%, memory bottleneck)
   - Check GPU Utilization (if <50%, not using GPU efficiently)

3. Open **Hardware Details** dashboard
   - Check GPU Encoder Utilization (should be high if using GPU encoding)
   - Check GPU Temperature (if >80°C, may be thermal throttling)
   - Check CPU RAPL Power Zones (package power at limit?)

4. Based on findings:
   - High CPU, low GPU → Switch to GPU encoding
   - High GPU temp → Improve cooling or reduce GPU load
   - High memory → Reduce concurrent streams or resolution
   - High encoder load → Reduce bitrate or resolution

---

## Troubleshooting

### Problem: No Data in Dashboards

**Symptoms:** All panels show "No data"

**Checks:**
1. Verify VictoriaMetrics is running:
   ```bash
   curl http://localhost:8428/health
   ```
   Expected: `{"status":"ok"}`

2. Check if exporters are running:
   ```bash
   docker compose ps
   ```
   All should show "Up" status

3. Check VictoriaMetrics logs:
   ```bash
   docker compose logs victoriametrics | tail -50
   ```
   Look for scrape errors

**Solutions:**
- If VictoriaMetrics is down: `docker compose restart victoriametrics`
- If exporters are down: `docker compose restart <exporter-name>`
- Check firewall rules if using remote server

### Problem: Some Panels Show Data, Others Don't

**Symptoms:** Partial data availability

**Checks:**
1. Open **System Overview** → **Exporter Status** table
2. Identify which exporters show "DOWN"

3. Check specific exporter:
   ```bash
   docker compose logs <exporter-name>
   curl http://localhost:<exporter-port>/metrics
   ```

**Solutions:**
- Restart specific exporter: `docker compose restart <exporter-name>`
- For GPU exporters: Check if nvidia-docker is installed and GPU is accessible
- Check exporter-specific requirements (e.g., RAPL needs Linux kernel with RAPL support)

### Problem: GPU Metrics Not Available

**Symptoms:** GPU panels show "No data"

**Checks:**
1. Verify GPU is present:
   ```bash
   nvidia-smi
   ```

2. Check GPU exporters are running:
   ```bash
   docker compose ps gpu-exporter-go dcgm-exporter
   ```

**Solutions:**
- GPU exporters require `--profile nvidia`:
  ```bash
  docker compose --profile nvidia up -d
  ```
- Verify nvidia-docker runtime is installed
- Check GPU is not in exclusive mode

### Problem: Dashboard Loads Slowly

**Symptoms:** Dashboard takes >10 seconds to load

**Solutions:**
1. Reduce time range (e.g., from 1h to 15m)
2. Increase refresh interval (from 5s to 30s)
3. Check VictoriaMetrics resource usage:
   ```bash
   docker stats victoriametrics
   ```
   If CPU/memory is high, may need more resources

4. Consider reducing scrape interval in `victoriametrics.yml`:
   ```yaml
   scrape_interval: 5s  # Change from 1s to 5s
   ```

### Problem: Historical Data is Missing

**Symptoms:** Can't see data from yesterday/last week

**Checks:**
1. Check VictoriaMetrics retention setting:
   ```bash
   docker compose logs victoriametrics | grep retentionPeriod
   ```
   Default: 30 days

**Solutions:**
- Data older than retention period is automatically deleted
- To keep data longer, edit `docker-compose.yml`:
  ```yaml
  victoriametrics:
    command:
      - "--retentionPeriod=90d"  # Keep for 90 days
  ```
  Then restart: `docker compose up -d victoriametrics`

---

## Tips and Best Practices

### 1. Establish Baselines

Before running tests:
1. Start the stack and let it idle for 5 minutes
2. Open **Power Monitoring** dashboard
3. Note the baseline power consumption (idle state)
4. Use this to calculate delta (test power - baseline power)

### 2. Use Multiple Dashboards Simultaneously

Open dashboards in separate browser tabs:
- Tab 1: System Overview (overall health)
- Tab 2: Transcoding Performance (encoding metrics)
- Tab 3: Hardware Details (deep dive)

Toggle between tabs during tests to correlate metrics.

### 3. Take Screenshots for Reports

To capture dashboard state:
1. Click panel menu (top-right of panel)
2. Select "Share" → "Link to rendered image"
3. Or use browser screenshot tool
4. Useful for documentation and presentations

### 4. Export Data for Analysis

To export raw data:
1. Click panel menu
2. Select "Inspect" → "Data"
3. Click "Download CSV"
4. Import into Excel, Python, R for further analysis

### 5. Set Up Alerts (Advanced)

Grafana supports alerting on metric thresholds:
1. Edit a panel
2. Switch to "Alert" tab
3. Set conditions (e.g., CPU >90% for 5 minutes)
4. Configure notification channel (email, Slack, etc.)

**Note:** Requires Grafana Enterprise or basic SMTP setup.

---

## Next Steps

Now that you understand how to use Grafana:

1.  **Verified exporters are working** - System Overview shows all UP
2.  **Explored all dashboards** - Understand what each shows
3.  **Monitored a test** - Saw real-time data

**What's Next:**
- Run batch tests to populate Energy Efficiency and Cost dashboards
- Compare different codecs (H.264 vs H.265 vs AV1)
- Optimize settings based on Energy Efficiency Leaderboard
- Set up production deployment with optimized configuration

---

## Resources

- **VictoriaMetrics Query Language:** https://docs.victoriametrics.com/MetricsQL.html
- **Grafana Documentation:** https://grafana.com/docs/grafana/latest/
- **PromQL Tutorial:** https://prometheus.io/docs/prometheus/latest/querying/basics/
- **Project Documentation:** [../../../../../README.md](../../../../../README.md)
- **Exporter Details:** [../victoriametrics.yml](../victoriametrics.yml)

---

**Questions or Issues?**

Check the [troubleshooting guide](./README.md#troubleshooting) or project [GitHub issues](https://github.com/psantana5/ffmpeg-rtmp/issues).
