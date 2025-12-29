# Grafana Dashboard Organization

## Folder Structure

The dashboards are organized into **4 topic-based folders** for better organization and easier navigation:

### 1. Power & Energy Monitoring
**Focus**: Real-time power consumption and energy usage tracking

**Dashboards**:
- `power-monitoring.json` - Real-time power metrics from RAPL, node-exporter, and cAdvisor
- `baseline-vs-test.json` - Baseline comparison for idle vs. active workloads

**Key Metrics**:
- `rapl_power_watts{zone}` - Real-time CPU package power
- `node_energy_joules_total` - Cumulative energy consumption
- `rate(node_energy_joules_total[5m])` - Power consumption rate (Watts)

**Mathematical Expressions**:
```promql
# Average power consumption across all zones
avg(rapl_power_watts)

# Energy consumed in time window (Joules)
increase(node_energy_joules_total[5m])

# Power efficiency: Work done per Watt
sum(rate(results_scenario_mean_power_watts[5m])) / avg(rapl_power_watts)
```

---

### 2. Efficiency & QoE Analysis
**Focus**: Quality of Experience and energy efficiency metrics

**Dashboards**:
- `energy-efficiency-dashboard.json` - Comprehensive efficiency analysis
- `qoe-dashboard.json` - Video quality metrics (VMAF, PSNR) and QoE scores

**Key Metrics**:
- `qoe_vmaf_score{scenario}` - Video quality score (0-100)
- `qoe_psnr_score{scenario}` - Peak Signal-to-Noise Ratio (dB)
- `qoe_quality_per_watt{scenario}` - Quality per Watt efficiency (VMAF/W)
- `qoe_efficiency_score{scenario}` - Quality-weighted pixels per joule

**Mathematical Expressions**:
```promql
# Quality-to-Power Ratio (higher is better)
qoe_vmaf_score / results_scenario_mean_power_watts

# Efficiency percentile analysis (95th percentile)
histogram_quantile(0.95, 
  sum(rate(qoe_efficiency_score_bucket[5m])) by (le, scenario)
)

# Quality standard deviation (consistency metric)
stddev(qoe_vmaf_score) by (scenario)

# Normalized efficiency score (0-1 scale)
(qoe_efficiency_score - min(qoe_efficiency_score)) / 
(max(qoe_efficiency_score) - min(qoe_efficiency_score))
```

**Derived Metrics**:
- **Efficiency Ratio** = `(VMAF × Resolution) / (Power × Duration)`
- **Quality Consistency** = `1 - (stddev(VMAF) / avg(VMAF))`
- **Bits per Quality Point** = `Bitrate / VMAF`

---

### 3. Cost Analysis
**Focus**: Economic metrics with multi-currency support

**Dashboards**:
- `cost-dashboard.json` - Basic cost analysis
- `cost-dashboard-load-aware.json` - Advanced load-aware cost calculations

**Key Metrics**:
- `cost_total_load_aware{scenario,currency}` - Total cost (energy + compute)
- `cost_energy_load_aware{scenario,currency}` - Energy cost component
- `cost_compute_load_aware{scenario,currency}` - Compute cost component

**Mathematical Expressions**:
```promql
# Cost per stream (for multi-stream scenarios)
cost_total_load_aware / on(scenario) label_replace(
  results_scenario_stream_count, "scenario", "$1", "scenario", "(.*)"
)

# Cost efficiency: Quality per currency unit
qoe_vmaf_score / cost_total_load_aware

# Break-even analysis: Cost vs. theoretical max revenue
# Assuming $0.10 per quality point per hour
(qoe_vmaf_score * 0.10 * results_scenario_duration_seconds / 3600) - 
cost_total_load_aware

# Cost trend rate (SEK per hour)
rate(cost_total_load_aware[1h]) * 3600

# Cumulative cost over time window
sum_over_time(cost_total_load_aware[24h])
```

**Derived Metrics**:
- **Cost per Megapixel** = `Total Cost / (Resolution × FPS × Duration)`
- **ROI Metric** = `(Quality Score × Price Factor - Cost) / Cost`
- **Cost Variance** = `stddev(cost_total_load_aware) / avg(cost_total_load_aware)`

---

### 4. Predictions & Forecasting
**Focus**: ML-based predictions and trend forecasting

**Dashboards**:
- `future-load-predictions.json` - Multivariate load predictions
- `efficiency-forecasting.json` - Efficiency trend forecasts

**Key Metrics**:
- `results_scenario_predicted_power_watts` - ML-predicted power consumption
- `results_scenario_predicted_energy_joules` - ML-predicted energy usage

**Mathematical Expressions**:
```promql
# Prediction accuracy (Mean Absolute Percentage Error)
abs(results_scenario_mean_power_watts - results_scenario_predicted_power_watts) / 
results_scenario_mean_power_watts * 100

# Prediction confidence interval (95%)
results_scenario_predicted_power_watts + (1.96 * stddev(results_scenario_mean_power_watts))
results_scenario_predicted_power_watts - (1.96 * stddev(results_scenario_mean_power_watts))

# Linear regression trend (next hour)
predict_linear(results_scenario_mean_power_watts[30m], 3600)

# Exponential moving average (smooth predictions)
avg_over_time(results_scenario_predicted_power_watts[10m])

# Anomaly detection (3-sigma rule)
abs(results_scenario_mean_power_watts - 
    avg_over_time(results_scenario_mean_power_watts[1h])) > 
(3 * stddev_over_time(results_scenario_mean_power_watts[1h]))
```

**Derived Metrics**:
- **Forecast Error** = `|Actual - Predicted| / Actual × 100`
- **Trend Slope** = `(Value_t - Value_t-1) / ΔTime`
- **Seasonal Variance** = `stddev(hourly_avg) by (hour_of_day)`

---

## Common PromQL Patterns

### Aggregation Functions
```promql
# Time-series averages
avg(metric) by (label)
sum(metric) by (label)
min(metric) by (label)
max(metric) by (label)

# Statistical measures
stddev(metric) by (label)         # Standard deviation
quantile(0.5, metric) by (label)  # Median
quantile(0.95, metric) by (label) # 95th percentile
```

### Rate Calculations
```promql
# Per-second rate of increase
rate(counter[5m])

# Total increase in time window
increase(counter[1h])

# Instantaneous rate
irate(counter[5m])
```

### Mathematical Operations
```promql
# Ratios and percentages
(metric_a / metric_b) * 100

# Normalization (0-1 scale)
(metric - scalar(min(metric))) / 
(scalar(max(metric)) - scalar(min(metric)))

# Weighted averages
sum(metric * weight) / sum(weight)
```

### Time-based Aggregations
```promql
# Moving averages
avg_over_time(metric[10m])

# Sum over time window
sum_over_time(metric[1h])

# Standard deviation over time
stddev_over_time(metric[30m])
```

---

## Dashboard Design Principles

### 1. **Hierarchy of Information**
- **Top Row**: Key Performance Indicators (KPIs) - single stat panels
- **Middle Rows**: Time series trends - graph panels
- **Bottom Rows**: Detailed breakdowns - tables and heatmaps

### 2. **Color Coding**
- **Green**: Optimal/Efficient (high VMAF, low cost, low power)
- **Yellow**: Acceptable range
- **Orange**: Warning threshold
- **Red**: Critical/Inefficient

### 3. **Thresholds**
- **VMAF**: >95 (green), 85-95 (yellow), 70-85 (orange), <70 (red)
- **Efficiency**: >10 VMAF/W (green), 5-10 (yellow), 2-5 (orange), <2 (red)
- **Cost**: <0.01 SEK (green), 0.01-0.05 (yellow), 0.05-0.10 (orange), >0.10 (red)

### 4. **Time Ranges**
- **Default**: Last 1 hour
- **Quick Ranges**: 5m, 15m, 30m, 1h, 3h, 6h, 12h, 24h, 7d
- **Refresh**: 10s (for real-time monitoring)

---

## Label Conventions

All metrics follow consistent labeling:
- `scenario`: Test scenario name (e.g., "2.5_Mbps_Stream")
- `currency`: Currency code (SEK, USD, EUR)
- `bitrate`: Video bitrate (e.g., "2500k")
- `resolution`: Video resolution (e.g., "1280x720")
- `fps`: Frames per second (e.g., "30")
- `encoder`: Encoder type (e.g., "x264")
- `streams`: Number of concurrent streams

---

## Advanced Query Examples

### Multi-Scenario Comparison
```promql
# Compare efficiency across all scenarios
sort_desc(
  avg(qoe_efficiency_score) by (scenario)
)
```

### Cost-Quality Pareto Frontier
```promql
# Find scenarios on the optimal trade-off curve
# (High quality, low cost)
topk(5, qoe_vmaf_score / cost_total_load_aware)
```

### Capacity Planning
```promql
# Estimate maximum concurrent streams before hitting power limit
# Assuming 150W total power budget
floor(150 / avg(results_scenario_mean_power_watts))
```

### Energy Budget Alerting
```promql
# Alert if projected daily energy exceeds budget (10 kWh = 36 MJ)
predict_linear(
  sum(increase(node_energy_joules_total[1h]))[6h:1h], 
  86400
) > 36000000
```

---

## Performance Considerations

### Query Optimization
1. **Use recording rules** for frequently accessed complex queries
2. **Limit time ranges** to reduce query load
3. **Use `rate()` over `irate()`** for smoother graphs
4. **Aggregate before math operations** when possible

### Dashboard Best Practices
1. **Limit to 20-30 panels per dashboard**
2. **Use variables** for dynamic filtering
3. **Set appropriate scrape intervals** (10-30s for most metrics)
4. **Use instant queries** for single-stat panels

---

## Troubleshooting

### No Data in Dashboards
1. Check Prometheus targets: `http://localhost:9090/targets`
2. Verify exporters are running: `docker compose ps`
3. Test metrics endpoint: `curl localhost:9503/metrics`
4. Check time range (data may be outside selected window)

### Incorrect Values
1. Verify metric types (gauge vs. counter)
2. Check label matching in queries
3. Ensure units are consistent (Watts vs. milliwatts)
4. Validate rate() time ranges (should be at least 4x scrape interval)

### Performance Issues
1. Reduce query time range
2. Increase evaluation interval
3. Use recording rules for complex queries
4. Optimize aggregation queries

---

## References

- [PromQL Documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Best Practices](https://grafana.com/docs/grafana/latest/best-practices/)
- [VMAF Documentation](https://github.com/Netflix/vmaf)
- [Energy Measurement with RAPL](https://web.eece.maine.edu/~vweaver/projects/rapl/)

---

## Changelog

### v2.0 - 2025-12-29
- **Folder Organization**: Dashboards organized into 4 topic-based folders
- **Advanced Mathematics**: Added derived metrics, statistical analysis, forecasting
- **Multi-Currency**: Full support for SEK, USD, EUR with currency selector
- **Documentation**: Comprehensive PromQL formula explanations
- **Performance**: Optimized queries with proper aggregations
