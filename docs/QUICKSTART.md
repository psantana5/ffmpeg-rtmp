# Quick Start Guide

This guide will walk you through getting started with the FFmpeg RTMP Power Monitoring system.

## Prerequisites Check

Before starting, ensure you have:

```bash
# Check Docker
docker --version
# Output: Docker version 24.0.x or higher

# Check Docker Compose
docker compose version
# Output: Docker Compose version v2.x.x or higher

# Check Python
python3 --version
# Output: Python 3.11.x or higher

# Check FFmpeg
ffmpeg -version
# Output: ffmpeg version 6.x or higher

# Check RAPL availability (Intel CPUs only)
ls /sys/class/powercap/intel-rapl:0/
# Should show energy_uj, max_energy_range_uj, etc.
```

## Step 1: Clone and Setup

```bash
# Clone the repository
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# Install Python dependencies
pip install -r requirements.txt
pip install -r requirements-dev.txt

# Install pre-commit hooks (optional, for contributors)
pre-commit install
```

## Step 2: Start the Monitoring Stack

```bash
# Start all services (Prometheus, Grafana, exporters, RTMP server)
make up-build

# Wait ~30 seconds for all services to start
sleep 30

# Verify all services are running
make ps

# Check Prometheus targets (all should be UP)
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[].health'
```

## Step 3: Run Your First Test

### Option A: Single Stream Test (Quick)

```bash
# Run a simple 60-second test
python3 run_tests.py --output-dir ./test_results single \
  --name "Quick Test 1080p @ 2500k" \
  --bitrate 2500k \
  --resolution 1920x1080 \
  --fps 30 \
  --duration 60 \
  --stabilization 5 \
  --cooldown 5
```

### Option B: Multi-Stream Stress Test

```bash
# Test with 4 concurrent streams
python3 run_tests.py --output-dir ./test_results multi \
  --count 4 \
  --bitrate 2500k \
  --resolution 1920x1080 \
  --fps 30 \
  --duration 120 \
  --stabilization 10 \
  --cooldown 10
```

### Option C: Batch Test Matrix

```bash
# Run predefined batch of scenarios
make test-batch

# This executes batch_stress_matrix.json with multiple configurations
```

## Step 4: Analyze Results

```bash
# Analyze the most recent test
python3 analyze_results.py

# Or analyze a specific test
python3 analyze_results.py test_results/test_results_20231215_143022.json

# Predict power for custom stream counts
python3 analyze_results.py --predict 1,2,4,8,16

# Export only (no console output)
python3 analyze_results.py --quiet --export-csv my_results.csv
```

The analysis will output:
1. **Detailed scenario metrics**: Power, energy, Docker overhead
2. **Energy efficiency rankings**: Best configurations for your hardware
3. **Power predictions**: ML model predictions for untested workloads
4. **CSV export**: All data exported to `test_results_YYYYMMDD_HHMMSS_analysis.csv`
5. **Model metadata**: Quality metrics saved to `model_metadata.json`

## Step 5: View Grafana Dashboards

```bash
# Open Grafana in your browser
open http://localhost:3000
# Default credentials: admin/admin

# Navigate to dashboards:
# 1. Power Monitoring Dashboard - Real-time power, CPU, memory, network
# 2. Energy Efficiency Dashboard - Scenario comparisons, efficiency scores
# 3. Baseline vs Test - Before/after comparisons
```

## Step 6: Explore Advanced Features

### Custom Test Scenarios

Create `my_scenarios.json`:

```json
{
  "scenarios": [
    {
      "type": "single",
      "name": "H.264 Baseline 720p",
      "bitrate": "2500k",
      "resolution": "1280x720",
      "fps": 30,
      "duration": 120,
      "stabilization": 10,
      "cooldown": 10
    },
    {
      "type": "multi",
      "name": "4 Streams Mixed Bitrates",
      "count": 4,
      "bitrates": ["1000k", "2500k", "5000k", "2500k"],
      "resolution": "1920x1080",
      "fps": 30,
      "duration": 180,
      "stabilization": 15,
      "cooldown": 15
    }
  ]
}
```

Run it:
```bash
python3 run_tests.py --output-dir ./test_results batch --file my_scenarios.json
```

### Output Ladder Testing

Test multi-resolution transcoding (ABR streaming):

```json
{
  "type": "single",
  "name": "ABR Ladder 1080p+720p+480p",
  "bitrate": "5000k",
  "resolution": "1920x1080",
  "fps": 30,
  "duration": 120,
  "outputs": [
    {"resolution": "1920x1080", "fps": 30},
    {"resolution": "1280x720", "fps": 30},
    {"resolution": "854x480", "fps": 30}
  ]
}
```

### GPU Power Monitoring (NVIDIA)

```bash
# Start with NVIDIA profile
make nvidia-up-build

# GPU metrics are automatically collected and included in analysis
```

## Common Workflows

### Daily Development: Quick Power Check

```bash
# Quick 30-second power baseline
python3 run_tests.py single --name "baseline" --bitrate 0k --duration 30

# Quick 60-second load test
python3 run_tests.py single --name "load" --bitrate 2500k --duration 60

# Analyze and compare
python3 analyze_results.py
```

### Weekly: Comprehensive Benchmarking

```bash
# Run full stress matrix (30-60 minutes)
make test-batch

# Analyze with detailed output
python3 analyze_results.py

# Archive results
cp test_results/test_results_*.json archives/weekly_$(date +%Y%m%d).json
cp test_results/*.csv archives/
```

### Production: Automated Monitoring

```bash
# Schedule periodic tests with cron
0 */4 * * * cd /path/to/ffmpeg-rtmp && make test-single && python3 analyze_results.py --quiet

# Monitor model metadata for drift
cat test_results/model_metadata.json | jq '.model_info.r2_score'

# Alert on poor model quality
if [ $(cat test_results/model_metadata.json | jq '.model_info.r2_score < 0.7') = "true" ]; then
  echo "Warning: Model quality degraded"
fi
```

## Troubleshooting

### Services Won't Start

```bash
# Check logs
make logs SERVICE=prometheus
make logs SERVICE=rapl-exporter

# Restart services
make down && make up-build
```

### No Power Data

```bash
# Verify RAPL access
ls -la /sys/class/powercap/intel-rapl:0/energy_uj

# Check rapl-exporter
curl http://localhost:9110/metrics | grep rapl_power_watts

# Verify Prometheus is scraping
curl http://localhost:9090/api/v1/query?query=rapl_power_watts
```

### FFmpeg Failures

```bash
# Check FFmpeg version
ffmpeg -version

# Test RTMP connectivity
ffmpeg -f lavfi -i testsrc=duration=10:size=1280x720:rate=30 \
  -f flv rtmp://localhost:1935/live/test

# Check nginx-rtmp health
curl http://localhost:8080/health
```

### Analysis Errors

```bash
# Check results file format
cat test_results/test_results_*.json | jq '.'

# Verify Prometheus connectivity
curl http://localhost:9090/-/healthy

# Run with debug logging
python3 -m pdb analyze_results.py
```

## Next Steps

- Read the [Full Documentation](../README.md)
- Explore [Power Prediction Model Details](../docs/power-prediction-model.md)
- Review [Contributing Guidelines](../CONTRIBUTING.md)
- Check [Issue Templates](../.github/ISSUE_TEMPLATE/) for reporting problems

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/psantana5/ffmpeg-rtmp/issues)
- **Discussions**: [GitHub Discussions](https://github.com/psantana5/ffmpeg-rtmp/discussions)
- **Documentation**: Browse `docs/` directory
