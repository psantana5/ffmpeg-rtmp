# Quality-Aware Energy Efficiency Guide

This document describes the QoE (Quality of Experience) aware transcoding optimization features added to the energy-aware transcoding pipeline.

## Overview

The enhanced pipeline now supports:

1. **Video Quality Metrics** - VMAF and PSNR integration for quality assessment
2. **QoE-Aware Scoring** - Quality-per-watt and QoE efficiency scoring algorithms
3. **Cost Modeling** - TCO analysis with cloud pricing support
4. **ML Model Retraining** - Automated model versioning and updates
5. **Prometheus Exporters** - Real-time metrics for Grafana dashboards

## 1. Video Quality Metrics (VMAF & PSNR)

### VMAF (Video Multimethod Assessment Fusion)

VMAF is a perceptual video quality metric developed by Netflix that correlates with human perception.

**Score Range:** 0-100 (higher is better)
- 0-20: Poor quality
- 20-40: Fair quality
- 40-60: Good quality
- 60-80: Very good quality
- 80-100: Excellent quality

**Usage:**

```python
from advisor.quality import compute_vmaf

# Compute VMAF score
vmaf_score = compute_vmaf('reference.mp4', 'transcoded.mp4')
print(f"VMAF Score: {vmaf_score:.2f}")
```

**Requirements:**
- FFmpeg compiled with libvmaf support
- Both reference and transcoded video files

### PSNR (Peak Signal-to-Noise Ratio)

PSNR is a traditional objective quality metric based on pixel-level differences.

**Score Range:** Measured in dB (higher is better)
- < 20 dB: Very poor quality
- 20-25 dB: Poor quality
- 25-30 dB: Fair quality
- 30-35 dB: Good quality
- 35-40 dB: Very good quality
- > 40 dB: Excellent quality

**Usage:**

```python
from advisor.quality import compute_psnr

# Compute PSNR score
psnr_score = compute_psnr('reference.mp4', 'transcoded.mp4')
print(f"PSNR: {psnr_score:.2f} dB")
```

**Requirements:**
- FFmpeg (standard build, no special compilation needed)

## 2. QoE-Aware Scoring Algorithms

### Available Algorithms

The `EnergyEfficiencyScorer` now supports multiple algorithms:

#### 1. `throughput_per_watt` (Legacy)
Simple throughput-based efficiency:
```
score = bitrate_mbps / mean_watts
```

#### 2. `pixels_per_joule` (Ladder-aware)
Accounts for multi-resolution output:
```
score = total_pixels_delivered / total_energy_joules
```

#### 3. `quality_per_watt` (NEW)
Quality per unit power:
```
score = VMAF / mean_watts
```

#### 4. `qoe_efficiency_score` (NEW)
Comprehensive QoE efficiency:
```
score = total_pixels * (VMAF/100) / energy_joules
```

#### 5. `auto` (NEW, Default)
Automatically selects best algorithm:
- If VMAF available → `qoe_efficiency_score`
- Elif multiple outputs → `pixels_per_joule`
- Else → `throughput_per_watt`

### Usage Example

```python
from advisor.scoring import EnergyEfficiencyScorer

# Auto-select algorithm (recommended)
scorer = EnergyEfficiencyScorer(algorithm='auto')

scenario = {
    'name': 'Test Scenario',
    'vmaf_score': 85.0,
    'power': {'mean_watts': 100.0, 'total_energy_joules': 6000.0},
    'duration': 60,
    'resolution': '1920x1080',
    'fps': 30
}

score = scorer.compute_score(scenario)
print(f"Efficiency Score: {score}")

# Or use specific algorithm
scorer_qpw = EnergyEfficiencyScorer(algorithm='quality_per_watt')
qpw_score = scorer_qpw.compute_score(scenario)
print(f"Quality per Watt: {qpw_score:.4f} VMAF/W")
```

## 3. Cost Modeling

### Overview

The cost modeling module enables TCO (Total Cost of Ownership) analysis for transcoding workloads.

### Supported Pricing Models

- **Energy cost**: €/kWh or $/kWh
- **CPU cost**: €/hour or $/hour (instance pricing)
- **GPU cost**: €/hour or $/hour

### Available Metrics

1. **Total Cost**: Energy + Compute costs
2. **Energy Cost**: Based on power consumption
3. **Compute Cost**: Instance/GPU time costs
4. **Cost per Pixel**: Cost efficiency metric
5. **Cost per Watch Hour**: Viewer-centric metric

### Usage Example

```python
from advisor.cost import CostModel

# Initialize with pricing
model = CostModel(
    energy_cost_per_kwh=0.12,  # $0.12/kWh
    cpu_cost_per_hour=0.50,     # $0.50/hour
    gpu_cost_per_hour=1.00,     # $1.00/hour
    currency='USD'
)

scenario = {
    'name': 'Production Transcode',
    'power': {'mean_watts': 150.0},
    'gpu_power': {'mean_watts': 50.0},
    'duration': 3600,  # 1 hour
    'resolution': '1920x1080',
    'fps': 30
}

# Calculate costs
total_cost = model.compute_total_cost(scenario)
energy_cost = model.compute_energy_cost(scenario)
cost_per_pixel = model.compute_cost_per_pixel(scenario)
cost_per_watch_hour = model.compute_cost_per_watch_hour(scenario, viewers=100)

print(f"Total Cost: ${total_cost:.4f}")
print(f"Energy Cost: ${energy_cost:.4f}")
print(f"Cost per Pixel: ${cost_per_pixel:.2e}")
print(f"Cost per Watch Hour (100 viewers): ${cost_per_watch_hour:.4f}")
```

## 4. ML Model Retraining

### Overview

Automated model retraining from test results with versioning and hardware-specific models.

### Retraining Models

```bash
# Using make command
make retrain-models

# Or directly
python3 retrain_models.py --results-dir ./test_results --models-dir ./models

# With custom hardware ID
python3 retrain_models.py --hardware-id my_server_01
```

### Model Storage Structure

```
models/
├── <hardware_id>/
│   ├── power_predictor_20231228_120000.pkl
│   ├── power_predictor_latest.pkl (symlink)
│   ├── multivariate_predictor_20231228_120000.pkl
│   ├── multivariate_predictor_latest.pkl (symlink)
│   └── metadata.json
└── README.md
```

### Metadata Format

```json
{
  "hardware_id": "intel_i7_9700k_linux",
  "timestamp": "2023-12-28T12:00:00",
  "platform": {
    "system": "Linux",
    "processor": "Intel(R) Core(TM) i7-9700K",
    "machine": "x86_64",
    "python_version": "3.11.0"
  },
  "models": {
    "power_predictor": {
      "path": "power_predictor_latest.pkl",
      "type": "PowerPredictor"
    },
    "multivariate_predictor": {
      "path": "multivariate_predictor_latest.pkl",
      "type": "MultivariatePredictor"
    }
  }
}
```

## 5. Prometheus Exporters

### QoE Metrics Exporter

Exports video quality and QoE efficiency metrics.

**Start the exporter:**
```bash
python3 qoe_exporter.py --port 9502 --results-dir ./test_results
```

**Exported Metrics:**
- `qoe_vmaf_score` - VMAF quality score (0-100)
- `qoe_psnr_score` - PSNR quality score (dB)
- `qoe_quality_per_watt` - Quality per watt efficiency
- `qoe_efficiency_score` - QoE efficiency score
- `qoe_computation_duration_seconds` - Metric computation time

**Prometheus Configuration:**
```yaml
scrape_configs:
  - job_name: 'qoe-metrics'
    static_configs:
      - targets: ['localhost:9502']
```

### Cost Metrics Exporter

Exports cost analysis metrics.

**Start the exporter:**
```bash
python3 cost_exporter.py \
  --port 9503 \
  --results-dir ./test_results \
  --energy-cost 0.12 \
  --cpu-cost 0.50 \
  --gpu-cost 1.00 \
  --currency USD
```

**Exported Metrics:**
- `cost_total` - Total cost (energy + compute)
- `cost_energy` - Energy cost
- `cost_compute` - Compute cost (CPU + GPU)
- `cost_per_pixel` - Cost per pixel delivered
- `cost_per_watch_hour` - Cost per viewer watch hour

**Prometheus Configuration:**
```yaml
scrape_configs:
  - job_name: 'cost-metrics'
    static_configs:
      - targets: ['localhost:9503']
```

## 6. Integration with Grafana

### Dashboard Panels

The following panels can be created in Grafana:

#### 1. Quality Metrics Panel
```promql
# VMAF scores over time
qoe_vmaf_score

# PSNR scores over time
qoe_psnr_score
```

#### 2. QoE Efficiency Panel
```promql
# Quality per watt
qoe_quality_per_watt

# QoE efficiency score
qoe_efficiency_score
```

#### 3. Cost Analysis Panel
```promql
# Total cost comparison
cost_total

# Cost breakdown
cost_energy
cost_compute
```

#### 4. Cost Efficiency Panel
```promql
# Cost per pixel
cost_per_pixel

# Cost per watch hour
cost_per_watch_hour
```

## 7. Complete Workflow Example

### Step 1: Run Tests with Quality Assessment

```bash
# Run standard test suite
make test-suite

# Or custom test
make test-single NAME="QoE_Test" BITRATE=2500k DURATION=300
```

### Step 2: Compute Quality Metrics

```python
from advisor.quality import compute_vmaf, compute_psnr

# Compute quality metrics for test output
vmaf = compute_vmaf('reference.mp4', 'output.mp4')
psnr = compute_psnr('reference.mp4', 'output.mp4')

# Add to scenario
scenario['vmaf_score'] = vmaf
scenario['psnr_score'] = psnr
```

### Step 3: Analyze with QoE Scoring

```python
from advisor.scoring import EnergyEfficiencyScorer

scorer = EnergyEfficiencyScorer(algorithm='auto')
score = scorer.compute_score(scenario)

print(f"Efficiency Score: {score}")
```

### Step 4: Perform Cost Analysis

```python
from advisor.cost import CostModel

cost_model = CostModel(energy_cost_per_kwh=0.12)
total_cost = cost_model.compute_total_cost(scenario)
cost_per_pixel = cost_model.compute_cost_per_pixel(scenario)

print(f"Total Cost: ${total_cost:.4f}")
print(f"Cost per Pixel: ${cost_per_pixel:.2e}")
```

### Step 5: Retrain ML Models

```bash
make retrain-models
```

### Step 6: Start Exporters

```bash
# Terminal 1: QoE exporter
python3 qoe_exporter.py --port 9502

# Terminal 2: Cost exporter
python3 cost_exporter.py --port 9503 --energy-cost 0.12
```

### Step 7: Visualize in Grafana

Navigate to Grafana at `http://localhost:3000` and:
1. Add the new Prometheus data sources (ports 9502, 9503)
2. Create or import dashboards for QoE and cost metrics
3. Monitor quality, efficiency, and costs in real-time

## 8. Best Practices

### Quality Metrics
- Always use same reference video for consistent comparisons
- VMAF is more accurate but slower than PSNR
- Consider using PSNR for quick checks, VMAF for final validation

### Scoring Algorithms
- Use `auto` mode for general use (recommended)
- Use `qoe_efficiency_score` when quality data is available
- Use `pixels_per_joule` for ladder transcoding scenarios

### Cost Modeling
- Update pricing regularly to reflect current cloud costs
- Consider regional pricing differences
- Account for both energy and compute costs

### ML Models
- Retrain models after collecting new test results
- Use hardware-specific models for accurate predictions
- Monitor model R² scores to assess quality

## 9. Troubleshooting

### VMAF Not Available
```
Error: libvmaf not found
```
**Solution:** Install FFmpeg with libvmaf support:
```bash
# Ubuntu/Debian
sudo apt-get install ffmpeg libvmaf-dev

# Or compile from source with --enable-libvmaf
```

### No Quality Metrics in Results
**Solution:** Quality metrics must be computed separately and added to scenario metadata.

### Exporter Returns Empty Metrics
**Solution:** Ensure test results exist in the specified directory and contain valid scenarios.

## 10. API Reference

See the module docstrings for detailed API documentation:

- `advisor.quality.vmaf_integration`
- `advisor.quality.psnr`
- `advisor.scoring.EnergyEfficiencyScorer`
- `advisor.cost.CostModel`
- `advisor.modeling.PowerPredictor`
- `advisor.modeling.MultivariatePredictor`

## 11. Contributing

When adding new metrics or algorithms:

1. Add corresponding tests in `tests/`
2. Update this documentation
3. Add Prometheus exporter metrics if applicable
4. Ensure backward compatibility
5. Run full test suite: `make test`
