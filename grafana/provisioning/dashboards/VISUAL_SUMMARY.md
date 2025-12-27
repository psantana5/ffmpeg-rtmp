# Energy Efficiency Dashboard - Visual Summary

## Dashboard Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Energy Efficiency Dashboard                              │
│                  FFmpeg Transcoding Analysis & Optimization                  │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. ENERGY EFFICIENCY LEADERBOARD                                    [Table] │
├─────────────────────────────────────────────────────────────────────────────┤
│ Scenario            │ Output Ladder    │ Encoder │ Streams │ Power │ Score  │
│ 4 streams @ 1000k   │ 1280x720@30      │ cpu     │ 4       │ 45W   │ 1.2M   │
│ 2 streams @ 2500k   │ 1280x720@30      │ cpu     │ 2       │ 38W   │ 1.1M   │
│ ...                 │ ...              │ ...     │ ...     │ ...   │ ...    │
└─────────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────┬──────────────────────────────────┐
│ 2. PIXELS PER JOULE           [Bar Chart]│ 3. ENERGY WASTED vs OPTIMAL [Bar]│
├──────────────────────────────────────────┼──────────────────────────────────┤
│  Scenario A  ████████████████████ 1.2M  │  Scenario A  █ 12J               │
│  Scenario B  ████████████████ 1.0M      │  Scenario B  ████ 45J            │
│  Scenario C  ████████████ 850K          │  Scenario C  █████████ 120J      │
│  Scenario D  ██████ 450K                │  Scenario D  ████████████ 200J   │
│                                          │                                  │
│  ▲ Higher bars = Better efficiency      │  ▲ Bars show excess energy used  │
└──────────────────────────────────────────┴──────────────────────────────────┘

┌──────────────────────────────────────────┬──────────────────────────────────┐
│ 4. CPU vs GPU SCALING         [Line Plot]│ 5. EFFICIENCY STABILITY      [Bar]│
├──────────────────────────────────────────┼──────────────────────────────────┤
│ Power (W)                                │  Scenario A  █ 0.02 (stable)     │
│   120 ┤                         ╭──○ GPU │  Scenario B  ███ 0.08            │
│   100 ┤                    ╭────╯        │  Scenario C  █████ 0.15 (noisy)  │
│    80 ┤               ╭────╯             │  Scenario D  ████ 0.12           │
│    60 ┤          ╭────╯                  │                                  │
│    40 ┤     ╭────╯ CPU                   │  ▲ Lower = More stable           │
│    20 ┤─────╯                            │                                  │
│     0 ┼──────┬──────┬──────┬──────       │                                  │
│       1      2      4      8  Streams    │                                  │
│                                          │                                  │
│  ○ Tipping point: 6 streams              │                                  │
└──────────────────────────────────────────┴──────────────────────────────────┘

┌──────────────────────────────────────────┬──────────────────────────────────┐
│ 6. ENERGY per MBPS       [Time Series]   │ 7. ENERGY per FRAME  [Time Series]│
├──────────────────────────────────────────┼──────────────────────────────────┤
│ Wh/Mbps                                  │ mJ/frame                         │
│  0.08 ┤                                  │   15 ┤                           │
│  0.06 ┤     ╭──────────────              │   12 ┤     ╭──────────           │
│  0.04 ┤─────╯          Scenario A        │    9 ┤─────╯      1080p         │
│  0.02 ┤     ─ ─ ─ ─ ─ ─ Scenario B       │    6 ┤    ─ ─ ─ ─  720p         │
│  0.00 ┼──────┬──────┬──────┬──────       │    3 ┤    · · · ·   480p         │
│       0m     5m     10m    15m    Time   │    0 ┼──────┬──────┬──────       │
│                                          │      0m     5m     10m    Time   │
│  ▼ Lower lines = Better efficiency       │  ▼ Lower = Less energy per frame │
└──────────────────────────────────────────┴──────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│ 8. POWER OVERHEAD vs BASELINE                                       [Table] │
├─────────────────────────────────────────────────────────────────────────────┤
│ Scenario            │ Bitrate │ Res.    │ FPS │ Power │ Delta │ Increase %  │
│ 8 streams @ 5000k   │ 5000k   │ 720p    │ 30  │ 125W  │ +105W │ +525%      │
│ 4 streams @ 2500k   │ 2500k   │ 720p    │ 30  │  65W  │  +45W │ +225%      │
│ 2 streams @ 1000k   │ 1000k   │ 720p    │ 30  │  35W  │  +15W │  +75%      │
│ Baseline (Idle)     │    0k   │   N/A   │  0  │  20W  │    0W │    0%      │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Key Metrics Explained

### 🎯 Efficiency Score
**What:** Pixels delivered per joule of energy consumed  
**Unit:** pixels/J (e.g., 1,200,000 pixels/J)  
**Better:** Higher values  
**Use:** Compare overall efficiency across scenarios

### ⚡ Power Consumption
**What:** Average electrical power during transcoding  
**Unit:** Watts (W)  
**Better:** Lower values (but consider throughput)  
**Use:** Capacity planning, cost estimation

### 🔋 Total Energy
**What:** Total energy consumed during scenario  
**Unit:** Joules (J) or Watt-hours (Wh)  
**Better:** Lower values (per unit of work)  
**Use:** Compare energy efficiency of different configs

### 📊 Wasted Energy
**What:** Extra energy vs optimal config (same ladder)  
**Unit:** Joules (J)  
**Better:** Zero (means you're using optimal config)  
**Use:** Quantify cost of suboptimal choices

### 🎚️ Stability (CV)
**What:** Coefficient of variation in efficiency  
**Unit:** Dimensionless ratio (0-1+)  
**Better:** Lower values (more stable)  
**Use:** Select reliable configs for production

## Color Coding

```
Performance Indicators:
  🟢 Green   - Optimal/Excellent (top quartile)
  🟡 Yellow  - Good (middle range)
  🟠 Orange  - Fair (below average)
  🔴 Red     - Poor (bottom quartile)

Stability Indicators:
  🟢 <0.05   - Very stable
  🟡 0.05-0.1 - Stable
  🟠 0.1-0.2  - Moderate variance
  🔴 >0.2     - High variance (avoid)

Energy Waste:
  🟢 0-50J   - Minimal waste
  🟡 50-200J - Moderate waste
  🟠 200-500J- Significant waste
  🔴 >500J   - Excessive waste
```

## Decision Workflows

### Workflow 1: Find Best Configuration
```
1. Open "Energy Efficiency Leaderboard"
   ↓
2. Filter by desired output_ladder
   ↓
3. Check top-ranked scenarios
   ↓
4. Verify stability in "Efficiency Stability"
   ↓
5. Select most stable among top performers
```

### Workflow 2: CPU vs GPU Decision
```
1. Open "CPU vs GPU Scaling"
   ↓
2. Identify crossover point (if exists)
   ↓
3. If streams < crossover: Use CPU
   If streams > crossover: Use GPU
   ↓
4. Validate with "Energy Wasted vs Optimal"
```

### Workflow 3: Optimize for Cost
```
1. Open "Power Overhead vs Baseline"
   ↓
2. Calculate: power_watts × hours × $per_kWh / 1000
   ↓
3. Compare costs across scenarios
   ↓
4. Balance cost vs efficiency_score
```

## Integration Points

```
┌──────────────────┐
│  Test Execution  │
│  (run_tests.py)  │
└────────┬─────────┘
         │
         ↓
┌──────────────────┐    Scrapes    ┌──────────────┐
│ Results Exporter │◄───────────────│  Prometheus  │
│  (port 9502)     │   every 5s     │  (port 9090) │
└────────┬─────────┘                └──────┬───────┘
         │                                  │
         │ Exports                          │ Queries
         ↓                                  ↓
┌──────────────────┐                ┌──────────────┐
│  Prometheus      │                │   Grafana    │
│  Time Series DB  │                │  (port 3000) │
└──────────────────┘                └──────────────┘
                                           │
                                           │ Visualizes
                                           ↓
                                    ┌──────────────┐
                                    │  Dashboard   │
                                    │  (This!)     │
                                    └──────────────┘
```

## Quick Reference Card

```
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃              ENERGY EFFICIENCY QUICK REFERENCE            ┃
┣━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┫
┃                                                           ┃
┃  METRIC               GOOD          FAIR         POOR    ┃
┃  ──────────────────────────────────────────────────────  ┃
┃  Efficiency Score     >1M pixels/J  500K-1M     <500K    ┃
┃  Power Overhead       <50W          50-100W     >100W    ┃
┃  Energy Waste         <50J          50-200J     >200J    ┃
┃  Stability (CV)       <0.05         0.05-0.15   >0.15    ┃
┃                                                           ┃
┃  ──────────────────────────────────────────────────────  ┃
┃                                                           ┃
┃  COMMON PATTERNS:                                         ┃
┃   • More streams = Higher power (non-linear scaling)      ┃
┃   • Higher bitrate = More energy per frame                ┃
┃   • Multi-res ladder = More total pixels, better score    ┃
┃   • GPU efficient at high concurrency (>4 streams)        ┃
┃   • CPU efficient at low concurrency (<4 streams)         ┃
┃                                                           ┃
┃  ──────────────────────────────────────────────────────  ┃
┃                                                           ┃
┃  OPTIMIZATION PRIORITIES:                                 ┃
┃   1. Maximize efficiency_score                            ┃
┃   2. Minimize energy_waste                                ┃
┃   3. Ensure stability (CV < 0.1)                          ┃
┃   4. Balance throughput vs power                          ┃
┃                                                           ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

## Troubleshooting Checklist

- [ ] Dashboard shows "No Data"
  - Check results-exporter running: `curl localhost:9502/metrics`
  - Verify Prometheus scraping: Check Targets page
  - Adjust time range to include test execution

- [ ] Metrics seem incorrect
  - Verify test scenarios completed successfully
  - Check RAPL power monitoring is active
  - Ensure baseline scenario ran before tests

- [ ] CPU vs GPU panel empty
  - Confirm encoder_type labels are being set
  - Check scenario names include encoder indicators
  - Verify both CPU and GPU tests were executed

- [ ] Efficiency scores missing
  - Ensure resolution/fps data present in scenarios
  - Check energy metrics are non-zero
  - Verify duration is greater than 0

## Notes for Operators

1. **Refresh Interval:** Dashboard auto-refreshes every 30s
2. **Data Retention:** Prometheus keeps 7 days by default
3. **Panel Customization:** All panels are editable in Grafana
4. **Export Options:** Dashboards can be exported as JSON/PDF
5. **Alerting:** Consider setting up alerts for efficiency drops

---

**Version:** 1.0  
**Created:** 2024-12-27  
**Dashboard UID:** energy-efficiency-dashboard
