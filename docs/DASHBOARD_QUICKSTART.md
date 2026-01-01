# Quick Start: Viewing Cost & Energy Dashboard

## Prerequisites
- Docker and Docker Compose installed
- Repository cloned and in project root
- Go 1.21+ installed (for building the CLI tool)

## Step 0: Build the CLI Tool

```bash
# Build the ffrtmp CLI for submitting jobs
go build -o bin/ffrtmp ./cmd/ffrtmp

# Optional: Add to PATH
export PATH=$PATH:$(pwd)/bin

# Or install it
go install ./cmd/ffrtmp
```

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

To see metrics in the dashboard, you need to submit transcoding jobs using the Go CLI tool:

```bash
# Option 1: Run a simple test
ffrtmp jobs submit --scenario "test1" --bitrate 2000k --duration 60

# Option 2: Submit multiple jobs for comparison
ffrtmp jobs submit --scenario "4K60-h264" --duration 120 --bitrate 10M
ffrtmp jobs submit --scenario "1080p60-h265" --duration 60 --bitrate 5M

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
ffrtmp jobs submit --scenario "quick-test" --duration 30
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
ffrtmp jobs submit --scenario "e2e-test" --bitrate 2000k --duration 30

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

## CLI Tool Reference

The `ffrtmp` CLI tool provides comprehensive job management capabilities:

```bash
# Submit jobs
ffrtmp jobs submit --scenario <name> [options]

# Check job status
ffrtmp jobs status <job-id>

# List compute nodes
ffrtmp nodes list

# Cancel jobs
ffrtmp jobs cancel <job-id>
```

**For complete CLI documentation**, see: `cmd/ffrtmp/README.md`

**Options:**
- `--scenario` - Test scenario name (e.g., "4K60-h264", "1080p60-h265")
- `--bitrate` - Target bitrate (e.g., "10M", "5M", "2000k")
- `--duration` - Duration in seconds
- `--confidence` - Confidence level: "auto", "high", "medium", "low"
- `--queue` - Queue type: "live", "default", "batch"
- `--priority` - Priority: "high", "medium", "low"

**Master URL configuration:**
```bash
# Option 1: Command-line flag
ffrtmp jobs submit --scenario test --master https://localhost:8080

# Option 2: Config file ~/.ffrtmp/config.yaml (recommended)
cat > ~/.ffrtmp/config.yaml << EOF
master_url: https://localhost:8080
api_key: your-api-key-here
EOF
ffrtmp jobs submit --scenario test  # No --master flag needed!
```

**Note:** Use HTTPS for production. The CLI supports self-signed certificates for localhost.

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
