# Example Analysis Workflow

This directory contains example scenario configurations and analysis workflows.

## Example Files

### 1. `basic_power_test.json`
**Purpose**: Quick baseline and scaling test

**Use case**: Understanding how power scales with stream count

**Run time**: ~15 minutes

```bash
python3 run_tests.py batch --file docs/examples/basic_power_test.json
python3 analyze_results.py
```

**What you'll learn**:
- Idle system power consumption
- Power per stream
- Efficiency at different bitrates
- Optimal configuration for your hardware

### 2. `abr_ladder_comparison.json`
**Purpose**: Compare multi-resolution transcoding configurations

**Use case**: Optimizing adaptive bitrate (ABR) streaming setups

**Run time**: ~10 minutes

```bash
python3 run_tests.py batch --file docs/examples/abr_ladder_comparison.json
python3 analyze_results.py
```

**What you'll learn**:
- Power cost of multi-resolution transcoding
- Efficiency of different ladder configurations
- Bitrate vs resolution trade-offs
- Best ABR setup for energy efficiency

## Custom Workflow Examples

### Example 1: Find Optimal Bitrate for Single 1080p Stream

```bash
# Create custom scenario file
cat > /tmp/bitrate_sweep.json << 'EOF'
{
  "scenarios": [
    {"type": "single", "name": "1080p @ 2Mbps", "bitrate": "2000k", "resolution": "1920x1080", "fps": 30, "duration": 120},
    {"type": "single", "name": "1080p @ 4Mbps", "bitrate": "4000k", "resolution": "1920x1080", "fps": 30, "duration": 120},
    {"type": "single", "name": "1080p @ 6Mbps", "bitrate": "6000k", "resolution": "1920x1080", "fps": 30, "duration": 120},
    {"type": "single", "name": "1080p @ 8Mbps", "bitrate": "8000k", "resolution": "1920x1080", "fps": 30, "duration": 120}
  ]
}
EOF

# Run tests
python3 run_tests.py batch --file /tmp/bitrate_sweep.json

# Analyze
python3 analyze_results.py

# Look for highest efficiency_score in the rankings
```

### Example 2: Capacity Planning - How Many Streams Can I Run?

```bash
# Run stream count sweep
cat > /tmp/capacity_test.json << 'EOF'
{
  "scenarios": [
    {"type": "multi", "name": "2 Streams", "count": 2, "bitrate": "2500k", "resolution": "1920x1080", "fps": 30, "duration": 120},
    {"type": "multi", "name": "4 Streams", "count": 4, "bitrate": "2500k", "resolution": "1920x1080", "fps": 30, "duration": 120},
    {"type": "multi", "name": "8 Streams", "count": 8, "bitrate": "2500k", "resolution": "1920x1080", "fps": 30, "duration": 120},
    {"type": "multi", "name": "12 Streams", "count": 12, "bitrate": "2500k", "resolution": "1920x1080", "fps": 30, "duration": 120}
  ]
}
EOF

python3 run_tests.py batch --file /tmp/capacity_test.json
python3 analyze_results.py

# Use the trained model to predict beyond tested range
python3 analyze_results.py --predict 1,2,4,8,12,16,20,24

# Look at Model Quality Metrics section for R² score
# If R² > 0.9, predictions are reliable
# If R² < 0.7, collect more data points
```

### Example 3: Weekly Performance Regression Testing

```bash
# Setup
mkdir -p archives/weekly_tests
cd archives/weekly_tests

# Run standard benchmark
python3 ../../run_tests.py batch --file ../../docs/examples/basic_power_test.json

# Analyze with timestamp
python3 ../../analyze_results.py --export-csv weekly_$(date +%Y%m%d).csv

# Compare with previous week
# Look for:
# - Power increase (thermal paste degradation, dust buildup)
# - Model quality decrease (inconsistent measurements)
# - Efficiency changes (OS updates, background processes)
```

### Example 4: A/B Testing: H.264 vs H.265

```bash
# Note: Requires FFmpeg with libx265 support
cat > /tmp/codec_comparison.json << 'EOF'
{
  "scenarios": [
    {
      "type": "single",
      "name": "H.264 1080p @ 5Mbps",
      "bitrate": "5000k",
      "resolution": "1920x1080",
      "fps": 30,
      "duration": 180,
      "codec": "libx264"
    },
    {
      "type": "single",
      "name": "H.265 1080p @ 5Mbps",
      "bitrate": "5000k",
      "resolution": "1920x1080",
      "fps": 30,
      "duration": 180,
      "codec": "libx265"
    }
  ]
}
EOF

# Note: run_tests.py may need modification to support codec selection
# This is a future enhancement
```

## Analysis Tips

### Reading the Output

1. **Energy Efficiency Rankings**
   - Higher scores = more video per watt
   - Compare scenarios with same resolution/fps
   - Top-ranked = best for production

2. **Model Quality Metrics**
   - R² > 0.9: Excellent, predictions reliable
   - R² 0.7-0.9: Good, use predictions cautiously
   - R² < 0.7: Poor, collect more data

3. **Cross-Validation**
   - CV RMSE similar to RMSE: Model generalizes well
   - CV RMSE >> RMSE: Overfitting, need more data

### When to Re-test

- **Hardware changes**: New CPU, RAM, thermal paste
- **Software updates**: OS patches, FFmpeg updates
- **Configuration changes**: Power limits, CPU governor
- **Quarterly**: Normal maintenance and verification

## Common Pitfalls

1. **Insufficient cooldown**: Back-to-back tests without cooling
   - **Fix**: Increase cooldown to 30+ seconds
   - **Symptom**: Power increases over time

2. **Background processes**: Other apps consuming CPU
   - **Fix**: Run on dedicated test system
   - **Symptom**: High variance in measurements

3. **Thermal throttling**: CPU overheating under load
   - **Fix**: Improve cooling, reduce load
   - **Symptom**: Power decreases at high stream counts

4. **RAPL counter rollover**: Long tests on high-power systems
   - **Fix**: Keep tests under 5 minutes
   - **Symptom**: Negative energy values

## Next Steps

- Read [Power Prediction Model Documentation](../power-prediction-model.md)
- Review [README.md](../../README.md) for full feature set
- Check [CONTRIBUTING.md](../../CONTRIBUTING.md) for adding examples
