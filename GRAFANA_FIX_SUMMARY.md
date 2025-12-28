# Grafana Dashboard Data Issues - Fix Summary

## Problem Statement
The Grafana dashboards (Future Load Predictions and Efficiency Forecasting) were experiencing intermittent "No data" issues. Users reported:
- Dashboards showing "No data" intermittently
- "Cannot find the datasource" errors
- Need to manually run queries in each panel for data to appear
- Data becoming stale and not updating automatically

## Root Causes Identified

### 1. Merge Conflict in results_exporter.py
**Issue**: Lines 641-763 contained unresolved merge conflict markers (`<<<<<<< HEAD`, `=======`, `>>>>>>>`)
- Prevented proper metric export
- Caused incomplete or inconsistent metrics in Prometheus
- Led to dashboards showing "No data" because required metrics weren't available

### 2. Datasource UID Mismatch
**Issue**: Dashboard JSON files referenced datasource with `uid: "prometheus"` but the datasource configuration used `uid: DS_PROMETHEUS`
- Caused "Cannot find the datasource" errors
- Required manual datasource selection in each panel
- Broke after Grafana restarts or dashboard reloads

### 3. Missing Auto-Refresh Configuration
**Issue**: Future Load Predictions and Efficiency Forecasting dashboards lacked auto-refresh settings
- Data became stale without manual refresh
- Variable queries didn't update when new test results were added
- Users had to manually refresh each panel

## Solutions Implemented

### 1. Resolved Merge Conflict (results_exporter.py)
**Changes**:
- Merged both metric definition approaches intelligently
- Export statistical confidence intervals (mean ± 2×stdev) in all cases
- Export ML-based predictions when ML predictor is trained
- Fallback to statistical confidence intervals when ML unavailable
- Export both predicted_power_watts AND predicted_energy_joules consistently

**Metrics Now Properly Exported**:
```python
# Statistical metrics (always available)
results_scenario_power_stdev
results_scenario_prediction_confidence_low
results_scenario_prediction_confidence_high

# Prediction metrics (ML when available, fallback to measured)
results_scenario_predicted_power_watts
results_scenario_predicted_energy_joules
```

**Code Impact**: 29 lines changed in results_exporter.py

### 2. Fixed Datasource UID (prometheus.yml)
**Changes**:
- Updated datasource UID from `DS_PROMETHEUS` to `prometheus`
- Added `httpMethod: POST` for improved query reliability
- Ensures dashboards can always find the datasource

**Before**:
```yaml
uid: DS_PROMETHEUS
```

**After**:
```yaml
uid: prometheus
httpMethod: POST
```

**Code Impact**: 3 lines changed in prometheus.yml

### 3. Added Auto-Refresh (Dashboard JSON Files)
**Changes**:
- Added `"refresh": "30s"` to dashboard-level configuration
- Changed variable refresh from `1` (on_dashboard_load) to `2` (on_time_range_change)
- Ensures data updates automatically every 30 seconds

**Dashboard Refresh Rates**:
- Power Monitoring: 5s (already configured)
- Baseline vs Test: 10s (already configured)
- Future Load Predictions: **30s (NEW)**
- Efficiency Forecasting: **30s (NEW)**
- Energy Efficiency Dashboard: 30s (already configured)

**Code Impact**: 2 lines changed per dashboard (4 total)

## Files Modified

1. **results-exporter/results_exporter.py**
   - Resolved merge conflict
   - Added intelligent fallback logic
   - Ensured consistent metric export

2. **grafana/provisioning/datasources/prometheus.yml**
   - Fixed datasource UID
   - Added httpMethod configuration

3. **grafana/provisioning/dashboards/future-load-predictions.json**
   - Added 30s auto-refresh
   - Updated variable refresh mode

4. **grafana/provisioning/dashboards/efficiency-forecasting.json**
   - Added 30s auto-refresh
   - Updated variable refresh mode

5. **GRAFANA_FIX_TESTING.md** (NEW)
   - Comprehensive testing guide
   - Troubleshooting steps
   - Expected behaviors

## Testing & Validation

### Automated Validation Performed
✅ Python syntax validation (py_compile)
✅ JSON syntax validation (all dashboard files)
✅ Docker Compose configuration validation
✅ Metric definitions verified in source code
✅ No remaining merge conflict markers
✅ CodeQL security scan (0 vulnerabilities)

### Manual Testing Required
See `GRAFANA_FIX_TESTING.md` for comprehensive testing procedures.

Key test scenarios:
1. Verify datasource connection
2. Check Run ID variable loads
3. Confirm all panels display data
4. Observe auto-refresh behavior
5. Test with new test results

## Expected Outcomes

After applying these fixes:

1. **Dashboards load data automatically**
   - No manual query execution needed
   - All panels populate on first load

2. **Datasource always available**
   - No "Cannot find the datasource" errors
   - Dashboards work after Grafana restarts

3. **Data stays current**
   - Auto-refresh every 30 seconds
   - New test results appear automatically
   - No stale data issues

4. **Metrics always available**
   - Statistical confidence intervals always exported
   - ML predictions when available
   - Consistent fallback behavior

## Rollback Plan

If issues occur, revert these commits:
```bash
git revert 32f7036 b0182bd 9cd5cc6
```

Or restore specific files from before changes:
```bash
git checkout 20af02c -- results-exporter/results_exporter.py
git checkout 20af02c -- grafana/provisioning/datasources/prometheus.yml
git checkout 20af02c -- grafana/provisioning/dashboards/future-load-predictions.json
git checkout 20af02c -- grafana/provisioning/dashboards/efficiency-forecasting.json
```

## Security Considerations

- CodeQL security scan passed with 0 alerts
- No secrets or credentials added
- No new security vulnerabilities introduced
- HTTP method changed to POST for Prometheus queries (best practice)

## Performance Impact

**Minimal performance impact**:
- Auto-refresh set to 30s (conservative interval)
- Metric export unchanged in complexity
- No additional database queries
- Dashboard load time unchanged

## Future Improvements

Potential enhancements (not in scope):
1. Make auto-refresh interval configurable via environment variable
2. Extract metric export logic to helper methods (reduce duplication)
3. Add health checks for ML predictor availability
4. Implement exponential backoff for failed queries
5. Add dashboard-level alerts for missing data

## Notes for User

### About Merge Conflicts in Your Local Repository
The user reported extensive merge conflicts in their local staging directory (`~/Doc/proj/ffmpeg-rtmp`). These conflicts are NOT present in the GitHub repository and appear to be a local merge state.

**Recommendation**: The user should:
1. Stash or commit local changes
2. Pull the fixed branch: `git checkout copilot/debug-grafana-data-issues`
3. Review and merge into their working branch
4. Resolve any local conflicts using the patterns from this fix

### ML Predictor Dependency
The ML-based predictions require the `advisor` module with `MultivariatePredictor`. If not available:
- Statistical confidence intervals are used instead
- Measured values exported as "predicted" values
- All dashboard queries still work correctly

This is intentional fallback behavior and not an error.

## Related Documentation

- `GRAFANA_FIX_TESTING.md` - Complete testing guide
- Dashboard configurations in `grafana/provisioning/dashboards/`
- Results exporter implementation in `results-exporter/results_exporter.py`

## Support

If issues persist after applying these fixes:
1. Check Docker logs: `docker compose logs results-exporter grafana prometheus`
2. Verify test results exist: `ls -la test_results/`
3. Test datasource manually in Grafana UI
4. Review Prometheus targets: http://localhost:9090/targets
5. Consult `GRAFANA_FIX_TESTING.md` troubleshooting section
