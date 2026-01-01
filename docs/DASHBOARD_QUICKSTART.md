# Quick Start: Viewing Cost & Energy Dashboard

## Prerequisites
- Docker and Docker Compose installed
- Repository cloned and in project root

## Step 1: Start the Stack

```bash
# Start VictoriaMetrics, Grafana, and the new Go cost exporter
docker compose up -d victoriametrics grafana cost-exporter-go

# Verify services are running
docker compose ps
```

Expected output:
```
NAME                  STATUS
victoriametrics       Up (healthy)
grafana              Up
cost-exporter-go     Up (healthy)
```

## Step 2: Verify Exporter is Working

```bash
# Check health
curl http://localhost:9514/health
# Expected: OK

# Check metrics
curl http://localhost:9514/metrics | head -20
# Expected: Prometheus-format metrics
```

## Step 3: Verify VictoriaMetrics is Scraping

```bash
# Check if VictoriaMetrics sees the exporter
curl -s 'http://localhost:8428/api/v1/query?query=cost_exporter_alive' | jq .

# Expected output:
# {
#   "status": "success",
#   "data": {
#     "resultType": "vector",
#     "result": [
#       {
#         "metric": {...},
#         "value": [timestamp, "1"]
#       }
#     ]
#   }
# }
```

## Step 4: Access Grafana

1. Open browser: `http://localhost:3000`
2. Login: 
   - Username: `admin`
   - Password: `admin`
3. Navigate to dashboard:
   - Click "Dashboards" (left sidebar)
   - Find "Cost & Energy Monitoring (Go + Rust)"
   - Or direct URL: `http://localhost:3000/d/cost-energy-monitoring`

## Step 5: Generate Test Data

To see metrics in the dashboard, you need test results:

```bash
# Option 1: Run a simple test
python3 scripts/run_tests.py single --name "test1" --bitrate 2000k --duration 60

# Option 2: Run batch tests
python3 scripts/run_tests.py batch --file batch_stress_matrix.json

# Wait 10 seconds for metrics to update
sleep 10
```

## What You Should See

### Dashboard Panels

1. **Total Cost by Scenario** - Line graph showing costs over time
2. **Cost Breakdown** - Stacked area showing energy vs compute costs
3. **CO₂ Emissions** - Line graph of carbon emissions
4. **Total Cost Gauge** - Single value with color threshold
5. **Total CO₂ Gauge** - Single value showing total emissions
6. **Cost Distribution** - Pie chart of cost per scenario
7. **Summary Table** - Detailed breakdown with all scenarios
8. **Exporter Status** - Health check (should show "UP" in green)

### Sample Metrics

If exporter is working but no test data:
- **Exporter Status**: ✅ UP (green)
- **Other panels**: Empty or "No data"

After running tests:
- All panels should populate with data
- Costs shown in USD
- CO₂ shown in kg
- Labels include scenario, region, bitrate, encoder

## Troubleshooting

### Dashboard shows "No data"

**Check exporter logs:**
```bash
docker logs cost-exporter-go
```

**Verify VictoriaMetrics can reach exporter:**
```bash
# From inside VictoriaMetrics container
docker exec victoriametrics wget -O- http://cost-exporter-go:9504/health
```

**Check VictoriaMetrics scrape targets:**
```bash
curl http://localhost:8428/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="cost-exporter-go")'
```

### Exporter Status shows "DOWN"

```bash
# Check if container is running
docker compose ps cost-exporter-go

# Restart if needed
docker compose restart cost-exporter-go

# Check health endpoint
curl http://localhost:9514/health
```

### No test results found

```bash
# Check results directory
ls -la test_results/

# Should see files like: test_results_20260101_*.json

# If empty, run a test
python3 scripts/run_tests.py single --name "quick-test" --duration 30
```

### Wrong datasource error

The dashboard uses VictoriaMetrics (UID: `DS_VICTORIAMETRICS`). 

**Verify datasource exists:**
1. Grafana → Configuration → Data Sources
2. Look for "VictoriaMetrics" (default)
3. Test connection should succeed

**Fix if missing:**
```bash
# Restart Grafana to reload provisioned datasources
docker compose restart grafana
```

## Testing the Complete Flow

End-to-end test:

```bash
# 1. Start services
docker compose up -d victoriametrics grafana cost-exporter-go

# 2. Wait for services to be ready
sleep 10

# 3. Check exporter health
curl http://localhost:9514/health

# 4. Run a test
python3 scripts/run_tests.py single --name "e2e-test" --bitrate 2000k --duration 30

# 5. Wait for metrics to be scraped
sleep 15

# 6. Query VictoriaMetrics
curl -s 'http://localhost:8428/api/v1/query?query=cost_total_load_aware' | jq .

# 7. Open Grafana dashboard
# http://localhost:3000/d/cost-energy-monitoring

# Should see data in all panels!
```

## Advanced: Side-by-Side Comparison

Run both Python and Go exporters to compare:

```bash
# Start both exporters
docker compose up -d cost-exporter cost-exporter-go

# Python metrics on port 9504
curl http://localhost:9504/metrics | grep cost_total

# Go metrics on port 9514  
curl http://localhost:9514/metrics | grep cost_total

# Both should have similar values for the same scenarios
```

## Configuration

### Change Region

Edit `docker-compose.yml`:
```yaml
  cost-exporter-go:
    environment:
      - REGION=eu-west-1  # Change this
```

Regional pricing:
- `us-east-1`: $0.13/kWh, 0.45 kg CO₂/kWh
- `us-west-2`: $0.10/kWh, 0.30 kg CO₂/kWh
- `eu-west-1`: $0.20/kWh, 0.28 kg CO₂/kWh
- `eu-north-1`: $0.08/kWh, 0.12 kg CO₂/kWh

### Change Costs

```yaml
  cost-exporter-go:
    environment:
      - ENERGY_COST=0.15  # Override regional pricing
      - CPU_COST=0.75     # Change compute cost
```

## Next Steps

Once the dashboard is working:

1. **Run more tests** to generate diverse data
2. **Compare scenarios** to find optimal settings
3. **Monitor costs** in real-time during encoding
4. **Set up alerts** for cost thresholds
5. **Export data** from summary table for reports

## Documentation

- **Dashboard Guide**: `COST_ENERGY_DASHBOARD.md`
- **Migration Plan**: `docs/ML_MIGRATION_PLAN.md`
- **Integration Complete**: `docs/INTEGRATION_COMPLETE.md`

## Support

If issues persist:
1. Check exporter logs: `docker logs cost-exporter-go`
2. Check VictoriaMetrics logs: `docker logs victoriametrics`
3. Check Grafana logs: `docker logs grafana`
4. Verify network: `docker network inspect ffmpeg-rtmp_streaming-net`
