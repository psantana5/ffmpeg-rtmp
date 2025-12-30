# Test Scripts

This directory contains all the utility scripts for running tests, analyzing results, and managing the project.

## Main Scripts

### `recommend_test.py`

**Purpose**: Hardware-aware benchmark recommendation tool

**Usage**:

```bash
# Get recommended test configuration for your hardware
python3 scripts/recommend_test.py
```

**Features**:
- Auto-detects CPU cores/threads, GPU, RAM, and system type
- Recommends optimal encoder (NVENC for GPU, x264/x265 for CPU)
- Suggests appropriate resolution and FPS based on hardware
- Adjusts test duration for laptops vs servers
- Outputs ready-to-run command with optimal settings

**Example Output**:
```
CPU: AMD Ryzen 9 5950X 16-Core Processor
Threads: 32
GPU: NVIDIA RTX 3080 (Driver: 525.147)
System Type: DESKTOP

✅ Recommended Command:
python3 scripts/run_tests.py single \
  --name recommended_test \
  --encoder h264_nvenc \
  --preset medium \
  --bitrate 20000k \
  --resolution 3840x2160 \
  --fps 60 \
  --duration 300
```

---

### `run_tests.py`

**Purpose**: Main test runner for FFmpeg streaming scenarios

**Usage**:

```bash
# Single stream test with default encoder (h264)
python3 scripts/run_tests.py single \
  --name "2Mbps_720p" \
  --bitrate 2000k \
  --resolution 1280x720 \
  --duration 120

# Single stream test with NVENC GPU encoder
python3 scripts/run_tests.py single \
  --name "4K_NVENC" \
  --encoder h264_nvenc \
  --preset medium \
  --bitrate 15000k \
  --resolution 3840x2160 \
  --fps 30 \
  --duration 180

# Single stream test with H.265 CPU encoder
python3 scripts/run_tests.py single \
  --name "1440p_H265" \
  --encoder h265 \
  --preset slow \
  --bitrate 8000k \
  --resolution 2560x1440 \
  --duration 240

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
- **NEW**: Support for multiple encoders (h264, h264_nvenc, h265, hevc_nvenc)
- **NEW**: Configurable encoder presets (ultrafast, veryfast, fast, medium, slow, slower)
- Baseline comparison support
- Configurable bitrate, resolution, FPS, duration
- Automatic results collection and JSON export
- **NEW**: Encoder and preset metadata exported to Grafana dashboards

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
      "name": "1080p @ 5M (H.264 CPU)",
      "bitrate": "5M",
      "resolution": "1920x1080",
      "duration": 120,
      "encoder": "h264",
      "preset": "fast"
    },
    {
      "type": "single",
      "name": "1080p @ 5M (H.264 NVENC)",
      "bitrate": "5M",
      "resolution": "1920x1080",
      "duration": 120,
      "encoder": "h264_nvenc",
      "preset": "medium"
    },
    {
      "type": "single",
      "name": "4K @ 15M (H.265)",
      "bitrate": "15M",
      "resolution": "3840x2160",
      "fps": 30,
      "duration": 180,
      "encoder": "h265",
      "preset": "slow"
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

**Note**: The `encoder` and `preset` fields are optional and default to `h264` and `veryfast` respectively.

## Encoder and Preset Options

### Supported Encoders

- **h264** (libx264) - CPU-based H.264 encoder, good quality, widely compatible
- **h264_nvenc** - NVIDIA GPU hardware H.264 encoder, fast, lower CPU usage
- **h265** (libx265) - CPU-based H.265/HEVC encoder, better compression than H.264
- **hevc_nvenc** - NVIDIA GPU hardware H.265/HEVC encoder

### Supported Presets

Presets control the speed/quality tradeoff:

- **ultrafast** - Fastest encoding, lowest quality
- **veryfast** - Very fast, lower quality (default)
- **fast** - Fast, good quality
- **medium** - Balanced speed/quality
- **slow** - Slower, better quality
- **slower** - Very slow, best quality

**Recommendation**: Use `fast` or `medium` for benchmarks, `slow` for production encoding.

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

To use GPU encoding in tests, specify an NVENC encoder:

```bash
# H.264 with NVENC
python3 scripts/run_tests.py single \
  --name "GPU_H264" \
  --encoder h264_nvenc \
  --preset medium \
  --bitrate 8000k \
  --resolution 1920x1080 \
  --duration 180

# H.265 with NVENC
python3 scripts/run_tests.py single \
  --name "GPU_H265" \
  --encoder hevc_nvenc \
  --preset medium \
  --bitrate 6000k \
  --resolution 1920x1080 \
  --duration 180
```

**Tip**: Use `scripts/recommend_test.py` to automatically detect GPU and generate the optimal command.

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
