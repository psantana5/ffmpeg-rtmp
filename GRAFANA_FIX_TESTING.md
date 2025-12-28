# Grafana Dashboard Data Issues - Fix Testing Guide

## Issues Fixed

### 1. Merge Conflict in results_exporter.py
**Problem**: Lines 641-763 had unresolved merge conflict markers that prevented proper metric export.

**Solution**: Merged both metric definition approaches:
- Export statistical confidence intervals (mean ± 2×stdev) 
- Export ML-based predictions when predictor is trained
- Fallback to statistical confidence intervals when ML predictions unavailable

**Affected Metrics**:
- `results_scenario_power_stdev` - Standard deviation of power measurements
- `results_scenario_predicted_power_watts` - ML model predictions
- `results_scenario_predicted_energy_joules` - ML model energy predictions
- `results_scenario_prediction_confidence_low` - Lower confidence bound
- `results_scenario_prediction_confidence_high` - Upper confidence bound

### 2. Datasource UID Mismatch
**Problem**: Dashboards referenced `uid: "prometheus"` but datasource was configured with `uid: DS_PROMETHEUS`, causing "cannot find datasource" errors.

**Solution**: Updated `grafana/provisioning/datasources/prometheus.yml` to use `uid: prometheus` and added `httpMethod: POST` for better reliability.

### 3. Missing Auto-Refresh Configuration
**Problem**: Future Load Predictions and Efficiency Forecasting dashboards didn't have auto-refresh, causing stale data.

**Solution**: Added 30-second auto-refresh to both dashboards and changed variable refresh from "on_dashboard_load" (1) to "on_time_range_change" (2).

## Testing Instructions

### 1. Start the Stack
```bash
docker compose up -d
```

### 2. Verify Results Exporter
Check that results exporter is running and serving metrics:
```bash
curl http://localhost:9502/health
curl http://localhost:9502/metrics | head -50
```

Expected: Should see `results_exporter_up 1` and metric definitions.

### 3. Check Prometheus
Verify Prometheus can scrape the results exporter:
```bash
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.job=="results-exporter")'
```

Expected: Target should show as "up" with `health: "up"`.

### 4. Access Grafana
1. Open http://localhost:3000
2. Login with admin/admin
3. Navigate to Dashboards

### 5. Test Future Load Predictions Dashboard
1. Open "Future Load Predictions" dashboard
2. Verify datasource is connected (no error message)
3. Check that Run ID variable loads available run IDs
4. Select a run ID
5. Verify all panels load data:
   - Measured vs Predicted Power Consumption
   - Prediction Confidence Intervals
   - Prediction Accuracy
   - Confidence Interval Width
   - Power Distribution Across Scenarios
   - Prediction Results Summary

**Note**: If no data is available, run a test to generate results:
```bash
make test
```

### 6. Test Efficiency Forecasting Dashboard
1. Open "Efficiency Forecasting" dashboard
2. Verify datasource is connected
3. Check Run ID variable loads
4. Select a run ID
5. Verify all panels load data:
   - Energy Efficiency Scores by Scenario
   - Efficiency Rankings (table)
   - Top 5 Most Efficient Configurations (Power)
   - Efficiency Score Distribution

### 7. Verify Auto-Refresh
1. Keep a dashboard open
2. Wait 30 seconds
3. Observe the dashboard automatically refreshes
4. Check the refresh indicator in the top-right corner shows "30s"

### 8. Test with New Data
1. Generate new test results:
   ```bash
   make test
   ```
2. Wait 30 seconds (or click the refresh button)
3. Verify new Run ID appears in dropdown
4. Verify data loads automatically without manual query execution

## Expected Dashboard Refresh Rates
- Power Monitoring: 5s
- Baseline vs Test: 10s
- Future Load Predictions: 30s
- Efficiency Forecasting: 30s
- Energy Efficiency Dashboard: 30s

## Troubleshooting

### Dashboard Shows "No data"
1. Check results exporter logs: `docker compose logs results-exporter`
2. Verify test results exist: `ls -la test_results/`
3. Check Prometheus has scraped metrics: http://localhost:9090/targets
4. Try manually refreshing the panel (click refresh icon on panel)

### "Cannot find datasource" Error
1. Check datasource configuration: http://localhost:3000/connections/datasources
2. Verify Prometheus datasource UID is "prometheus"
3. Test datasource connection

### Panels Load Initially But Stop Updating
1. Check dashboard refresh setting (top-right corner)
2. Verify variable refresh is set to "On time range change"
3. Check browser console for errors

### ML Predictions Not Showing
This is expected if:
- Not enough scenarios (need at least 3)
- ML predictor dependencies not available
- First run after starting the stack

In these cases, the exporter will use statistical confidence intervals instead.

## Files Modified
1. `results-exporter/results_exporter.py` - Fixed merge conflict, added fallback logic
2. `grafana/provisioning/datasources/prometheus.yml` - Fixed UID, added httpMethod
3. `grafana/provisioning/dashboards/future-load-predictions.json` - Added auto-refresh
4. `grafana/provisioning/dashboards/efficiency-forecasting.json` - Added auto-refresh
