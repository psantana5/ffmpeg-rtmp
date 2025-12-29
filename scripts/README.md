# Test Scripts

This directory contains all the utility scripts for running tests, analyzing results, and managing the project.

## Main Scripts

### `run_tests.py`

**Purpose**: Main test runner for FFmpeg streaming scenarios

**Usage**:

```bash
# Single stream test
python3 scripts/run_tests.py single \
  --name "2Mbps_720p" \
  --bitrate 2000k \
  --resolution 1280x720 \
  --duration 120

# Multi-stream test
python3 scripts/run_tests.py multi \
  --count 4 \
  --bitrate 2500k \
  --duration 120

# Batch test from JSON
python3 scripts/run_tests.py batch \
  --file batch_stress_matrix.json
```

**Features**:
- Single, multi-stream, and batch test modes
- Baseline comparison support
- Configurable bitrate, resolution, FPS, duration
- Automatic results collection and JSON export

[Full test runner documentation →](./run_tests.md)

---

### `analyze_results.py`

**Purpose**: Analyze test results and generate efficiency reports

**Usage**:

```bash
# Analyze latest results
python3 scripts/analyze_results.py

# Use advanced multivariate model
python3 scripts/analyze_results.py --multivariate

# Predict power for custom stream counts
python3 scripts/analyze_results.py --predict-future 1,2,4,8,16

# Analyze specific results file
python3 scripts/analyze_results.py test_results/test_results_20231215_143022.json
```

**Output**:
- Console report with efficiency rankings
- CSV export with all metrics
- Power consumption predictions
- Best configuration recommendations

---

### `retrain_models.py`

**Purpose**: Train/retrain ML models for power prediction

**Usage**:

```bash
# Retrain models with all results
python3 scripts/retrain_models.py \
  --results-dir ./test_results \
  --models-dir ./models

# Retrain for specific hardware
python3 scripts/retrain_models.py \
  --hardware-id intel_i7_9700k
```

**Features**:
- Trains univariate and multivariate models
- Cross-validation for model selection
- Hardware-specific model storage
- Bootstrapping for confidence intervals

---

### `generate_plots.py`

**Purpose**: Generate visualization plots from test results

**Usage**:

```bash
python3 scripts/generate_plots.py
```

**Output**:
- Power consumption over time
- Energy efficiency comparisons
- Cost analysis charts
- Exported to `plots/` directory

---

### `setup.sh`

**Purpose**: Initial setup script (legacy - prefer using Makefile)

**Usage**:

```bash
./scripts/setup.sh
```

**What it does**:
- Checks prerequisites (Docker, Python, FFmpeg)
- Creates necessary directories
- Copies exporter files
- Builds and starts the stack

**Note**: Most functionality is now available through the Makefile. Consider using `make up-build` instead.

---

## Quick Reference

### Common Test Scenarios

**Quick Test (1 minute)**:
```bash
python3 scripts/run_tests.py single --name quick --bitrate 1000k --duration 60
```

**Production-like (4 streams, 5 minutes)**:
```bash
python3 scripts/run_tests.py multi --count 4 --bitrate 2500k --duration 300
```

**Full Stress Matrix**:
```bash
python3 scripts/run_tests.py batch --file batch_stress_matrix.json
```

### Test with Baseline Comparison

Adding `--with-baseline` enables automatic baseline vs test comparison in Grafana:

```bash
python3 scripts/run_tests.py single \
  --with-baseline \
  --baseline-duration 60 \
  --name "test" \
  --bitrate 2000k \
  --duration 120
```

### Understanding Test Results

Test results are stored in `test_results/test_results_YYYYMMDD_HHMMSS.json`:

```json
{
  "run_id": "20231215_143022",
  "timestamp": "2023-12-15T14:30:22",
  "scenarios": [
    {
      "name": "2Mbps_720p",
      "type": "single",
      "bitrate": "2000k",
      "resolution": "1280x720",
      "duration": 120,
      "start_time": "2023-12-15T14:30:30",
      "end_time": "2023-12-15T14:32:30",
      "streams": 1
    }
  ]
}
```

### Batch Test Configuration

Create a JSON file with multiple scenarios:

```json
{
  "scenarios": [
    {
      "type": "single",
      "name": "1080p @ 5M",
      "bitrate": "5M",
      "resolution": "1920x1080",
      "duration": 120
    },
    {
      "type": "multi",
      "name": "4 streams @ 2.5M",
      "count": 4,
      "bitrate": "2500k",
      "duration": 120
    }
  ]
}
```

## Troubleshooting

### "FFmpeg not found"

Install FFmpeg:
- Ubuntu/Debian: `sudo apt-get install ffmpeg`
- macOS: `brew install ffmpeg`

### "Cannot connect to RTMP server"

Ensure the nginx-rtmp container is running:
```bash
docker ps | grep nginx-rtmp
make logs SERVICE=nginx-rtmp
```

### "Test results directory not found"

Create the directory:
```bash
mkdir -p test_results
```

Or use the Makefile which creates it automatically:
```bash
make up-build
```

## Advanced Usage

### Custom Output Ladders

Test multiple resolutions simultaneously (adaptive bitrate streaming):

```json
{
  "type": "single",
  "name": "Multi-resolution ladder",
  "bitrate": "5000k",
  "duration": 120,
  "outputs": [
    {"resolution": "1920x1080", "fps": 30},
    {"resolution": "1280x720", "fps": 30},
    {"resolution": "854x480", "fps": 30}
  ]
}
```

### GPU Testing

Start the stack with NVIDIA profile:

```bash
make nvidia-up-build
```

The test runner will automatically detect and use GPU encoding if available.

## Integration with CI/CD

Example GitHub Actions workflow:

```yaml
- name: Run streaming tests
  run: |
    make up-build
    python3 scripts/run_tests.py batch --file batch_stress_matrix.json
    python3 scripts/analyze_results.py
```

## Development

To add a new test type:

1. Edit `scripts/run_tests.py`
2. Add a new subcommand to the CLI parser
3. Implement the test logic in a new method
4. Update this README with usage examples

## Performance Tips

- Use shorter durations for development/testing (30-60s)
- Use longer durations for production analysis (5-10 minutes)
- Add stabilization time (10-30s) before tests to let system settle
- Add cooldown time (10-30s) between tests to let system cool down
