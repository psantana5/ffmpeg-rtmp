# Energy Efficiency Dashboard for FFmpeg Transcoding

## Overview

The **Energy Efficiency Dashboard** is an advanced Grafana dashboard designed to provide deep insights into the energy consumption and efficiency of FFmpeg transcoding workloads. It helps you make informed decisions about transcoding configurations by visualizing energy tradeoffs, optimal settings, and CPU vs GPU performance characteristics.

## Dashboard Panels

### 1. Energy Efficiency Leaderboard

**Type:** Table
**Purpose:** Ranks all transcoding scenarios by their energy efficiency score

**Columns:**
- `scenario` - Scenario name
- `output_ladder` - Output resolution ladder (e.g., "1920x1080@30,1280x720@30")
- `encoder` - Encoder type (cpu/gpu)
- `streams` - Number of concurrent streams
- `mean_power_watts` - Average power consumption (W)
- `total_energy_joules` - Total energy consumed (J)
- `efficiency_score` - Efficiency metric (pixels/J or Mbps/W)

**Interpretation:** Higher efficiency scores are better. The table is sorted descending by efficiency score, so the best configurations appear at the top.

**Use Case:** Quickly identify which transcoding configurations deliver the best energy efficiency for your workload.

---

### 2. Pixels Delivered per Joule

**Type:** Horizontal Bar Chart
**Purpose:** Shows total pixels delivered per joule of energy consumed

**Metric:** `results_scenario_efficiency_score` (already computed as pixels/J)

**Interpretation:**
- Higher bars = more efficient configurations
- Useful for comparing scenarios with different output resolutions
- Groups scenarios by output ladder for fair comparison

**Use Case:** Compare energy efficiency across different resolution configurations. Ideal for identifying the most efficient encoding ladder for your use case.

---

### 3. Energy Wasted vs Optimal

**Type:** Horizontal Bar Chart
**Purpose:** Shows extra energy consumed compared to the most efficient configuration within the same output ladder

**PromQL:**
```promql
results_scenario_total_energy_joules
- on(output_ladder) group_left()
min by(output_ladder) (results_scenario_total_energy_joules)
```

**Interpretation:**
- Green bars (≈0) = optimal or near-optimal configurations
- Yellow/Orange/Red bars = increasingly wasteful configurations
- Negative values impossible (means the scenario IS the optimal one)

**Use Case:** Identify how much energy you're wasting by using suboptimal configurations. Helps quantify the cost of convenience vs efficiency tradeoffs.

---

### 4. CPU vs GPU Scaling

**Type:** Time Series (Line/Scatter)
**Purpose:** Compares CPU and GPU encoder power consumption as stream count increases

**Metrics:**
- CPU: `avg by(streams, encoder_type) (results_scenario_mean_power_watts{encoder_type="cpu"})`
- GPU: `avg by(streams, encoder_type) (results_scenario_mean_power_watts{encoder_type="gpu"})`

**Interpretation:**
- Blue line = CPU encoder power scaling
- Green line = GPU encoder power scaling
- The "tipping point" is where lines cross (if they do)

**Use Case:** Determine at what concurrency level GPU transcoding becomes more energy-efficient than CPU transcoding. Essential for scaling decisions.

---

### 5. Efficiency Stability

**Type:** Horizontal Bar Chart
**Purpose:** Shows variability in efficiency scores over time (coefficient of variation)

**PromQL:**
```promql
stddev_over_time(results_scenario_efficiency_score[5m])
/ avg_over_time(results_scenario_efficiency_score[5m])
```

**Interpretation:**
- Green (low values) = stable, predictable configurations
- Yellow/Orange = moderate variance
- Red (high values) = unstable, noisy configurations
- Lower is better

**Use Case:** Identify configurations that behave consistently vs those that have unpredictable performance. Critical for production workloads requiring SLAs.

---

### 6. Energy per Mbps Throughput

**Type:** Time Series
**Purpose:** Shows energy efficiency in terms of Wh per Mbps of throughput

**Metric:** `results_scenario_energy_wh_per_mbps`

**Interpretation:**
- Lower is better (less energy per unit of throughput)
- Useful for bitrate-focused comparisons
- Shows efficiency trends over time

**Use Case:** Compare energy efficiency across different bitrate configurations. Helps answer "which bitrate is most energy-efficient?"

---

### 7. Energy per Frame

**Type:** Time Series
**Purpose:** Shows energy consumption per video frame (mJ/frame)

**Metric:** `results_scenario_energy_mj_per_frame`

**Interpretation:**
- Lower is better (less energy per frame)
- Useful for frame rate and resolution comparisons
- Independent of bitrate variations

**Use Case:** Evaluate energy cost of different resolution/fps combinations. Useful for determining optimal frame rates.

---

### 8. Power Overhead vs Baseline

**Type:** Table
**Purpose:** Shows power increase compared to idle baseline state

**Columns:**
- `scenario` - Scenario name
- `bitrate` - Configured bitrate
- `resolution` - Output resolution
- `fps` - Frames per second
- `mean_power_watts` - Average power (W)
- `delta_power_watts` - Power increase vs baseline (W)
- `power_increase_pct` - Percentage increase vs baseline (%)

**Interpretation:**
- Shows the actual overhead of transcoding workloads
- Helps understand incremental power cost of each scenario
- Sorted by power increase percentage

**Use Case:** Quantify the actual power overhead of transcoding. Useful for capacity planning and cost estimation.

---

## Metrics Reference

All metrics are exported by the `results-exporter` service (port 9502) and scraped by Prometheus.

### Core Metrics

| Metric | Type | Unit | Description |
|--------|------|------|-------------|
| `results_scenario_efficiency_score` | gauge | pixels/J or Mbps/W | Energy efficiency score |
| `results_scenario_mean_power_watts` | gauge | W | Mean CPU package power during scenario |
| `results_scenario_total_energy_joules` | gauge | J | Total energy consumed |
| `results_scenario_total_energy_wh` | gauge | Wh | Total energy in watt-hours |
| `results_scenario_total_pixels` | gauge | pixels | Total pixels delivered |
| `results_scenario_duration_seconds` | gauge | s | Scenario duration |
| `results_scenario_energy_wh_per_mbps` | gauge | Wh/Mbps | Energy per megabit throughput |
| `results_scenario_energy_mj_per_frame` | gauge | mJ/frame | Energy per video frame |
| `results_scenario_delta_power_watts` | gauge | W | Power delta vs baseline |
| `results_scenario_power_pct_increase` | gauge | % | Power increase vs baseline |

### Labels

All metrics include these labels:

| Label | Description | Example |
|-------|-------------|---------|
| `scenario` | Scenario name | "4 streams @ 2500k" |
| `bitrate` | Configured bitrate | "2500k" |
| `resolution` | Primary resolution | "1280x720" |
| `fps` | Frames per second | "30" |
| `streams` | Number of concurrent streams | "4" |
| `output_ladder` | Output resolution ladder | "1920x1080@30,1280x720@30" |
| `encoder_type` | Encoder type | "cpu" or "gpu" |
| `run_id` | Test run identifier | "test_results_20231201_120000" |

---

## Usage Examples

### Example 1: Finding the Most Efficient Configuration

1. Look at the **Energy Efficiency Leaderboard**
2. Sort by `efficiency_score` (descending)
3. The top entry is your most efficient configuration
4. Note the `output_ladder`, `streams`, and `encoder` values

### Example 2: Optimizing for a Specific Output Ladder

1. Filter the **Energy Wasted vs Optimal** panel by your desired output ladder
2. Configurations showing ~0 wasted energy are optimal
3. Compare those against the **Efficiency Stability** panel
4. Choose the most stable among the optimal configurations

### Example 3: Determining CPU vs GPU Crossover Point

1. View the **CPU vs GPU Scaling** panel
2. Identify where the GPU line drops below the CPU line
3. That stream count is your "tipping point"
4. Use GPU encoding for higher concurrency, CPU for lower

### Example 4: Capacity Planning

1. Use **Power Overhead vs Baseline** to see actual power draw
2. Multiply by expected concurrent jobs to estimate total power
3. Factor in cooling overhead (typically 1.5x for data centers)
4. Calculate energy cost: `power_watts * hours * $per_kwh / 1000`

---

## Interpretation Guide

### What is a "Good" Efficiency Score?

Efficiency scores are relative, not absolute. Compare within your own environment:

- **High efficiency:** Scores in top 25% of your scenarios
- **Medium efficiency:** Middle 50%
- **Low efficiency:** Bottom 25%

For pixels-per-joule scoring:
- 1080p@30fps single stream: ~500,000 - 1,000,000 pixels/J (typical)
- Multi-resolution ladder: Higher total (sum across all outputs)

For Mbps-per-watt scoring:
- CPU encoding: ~0.05 - 0.1 Mbps/W (typical)
- GPU encoding: ~0.1 - 0.3 Mbps/W (typical, hardware dependent)

### When to Use Which Panel

| Question | Panel to Use |
|----------|--------------|
| Which config is most efficient overall? | Energy Efficiency Leaderboard |
| How much energy am I wasting? | Energy Wasted vs Optimal |
| Should I use CPU or GPU encoding? | CPU vs GPU Scaling |
| Which bitrate is most energy-efficient? | Energy per Mbps Throughput |
| Which configs are most reliable? | Efficiency Stability |
| What's my actual power overhead? | Power Overhead vs Baseline |

---

## Advanced Tips

### 1. Ladder-Aware Comparisons

Always compare scenarios within the same `output_ladder` group. Comparing a single 720p output to a multi-resolution ladder (1080p+720p+480p) is meaningless.

**Filter by ladder:**
```promql
results_scenario_efficiency_score{output_ladder="1280x720@30"}
```

### 2. Time-Based Analysis

Use Grafana's time range selector to analyze:
- Long-term trends (7-day view)
- Scenario-specific behavior (narrow to test duration)
- Stability during different times of day

### 3. Combining Metrics

Create custom panels combining metrics:

**Energy efficiency per stream:**
```promql
results_scenario_efficiency_score / on(scenario) results_scenario_streams
```

**Cost per pixel (assuming $0.10/kWh):**
```promql
(results_scenario_total_energy_wh / 1000) * 0.10 / results_scenario_total_pixels
```

### 4. Alerting

Set up Prometheus alerts for:
- Efficiency drop below threshold
- Power consumption exceeding budget
- High coefficient of variation (unstable configs)

---

## Troubleshooting

### Panel Shows "No Data"

**Possible causes:**
1. `results-exporter` service not running
2. No test results available in `/results` directory
3. Time range doesn't include test execution
4. Prometheus not scraping results-exporter

**Check:**
```bash
# Verify results-exporter is running
curl http://localhost:9502/metrics

# Check Prometheus targets
curl http://localhost:9090/api/v1/targets | grep results-exporter
```

### Efficiency Scores Seem Wrong

**Check:**
1. Verify scenario duration is reasonable (not too short)
2. Ensure power metrics are being collected (RAPL available)
3. Check for baseline scenario in test results
4. Verify resolution/fps data is present in scenarios

### Labels Missing or Incorrect

**Check:**
1. Scenario names follow expected format ("N streams @ bitrate")
2. Output ladder format is correct in test scenarios
3. Encoder type detection logic matches your naming convention

---

## Integration with Existing Dashboards

This dashboard complements the existing dashboards:

- **power-monitoring.json:** Real-time power metrics during tests
- **baseline-vs-test.json:** Comparison of baseline vs test scenarios
- **energy-efficiency-dashboard.json:** Post-test analysis and decision support

**Workflow:**
1. Run tests → monitor with power-monitoring dashboard
2. Wait for completion → analyze with baseline-vs-test dashboard
3. Make decisions → use energy-efficiency-dashboard

---

## Future Enhancements

Planned improvements:

1. **Quality Metrics:** Integrate VMAF/PSNR for quality-adjusted efficiency
2. **GPU Metrics:** Add DCGM-based GPU power tracking
3. **Cost Estimation:** Integrate cloud pricing models
4. **Recommendations:** AI-driven configuration suggestions
5. **Comparative Views:** Side-by-side scenario comparison panels

---

## References

- [Grafana Documentation](https://grafana.com/docs/)
- [PromQL Documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Intel RAPL Documentation](https://www.kernel.org/doc/html/latest/power/powercap/powercap.html)

---

## Support

For issues or questions:
1. Check the Troubleshooting section above
2. Review Prometheus query syntax in panel definitions
3. Verify metrics are being exported correctly
4. Open an issue in the repository

---

**Last Updated:** 2024-12-27
**Dashboard Version:** 1.0
**Compatibility:** Grafana 9.5.0+, Prometheus 2.40+
