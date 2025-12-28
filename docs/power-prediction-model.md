# Energy-Aware Scalability Prediction Model

## Overview

The Power Prediction Model is a machine learning-based system that predicts power consumption for FFmpeg transcoding workloads based on the number of concurrent streams. This enables capacity planning, cost estimation, and energy-aware scaling decisions without requiring exhaustive testing.

**Key Features:**
- Automatic model selection (linear vs polynomial regression)
- Robust scenario name parsing
- Handles missing data gracefully
- Provides prediction confidence through R² scores
- Exports predictions to CSV for further analysis

---

## Mathematical Model

### Linear Regression (< 6 unique stream counts)

Used when training data contains fewer than 6 unique stream count values. Provides a simple, stable model suitable for small datasets.

**Formula:**
```
Power(streams) = β₀ + β₁ × streams
```

**Parameters:**
- `β₀` (intercept): Baseline power consumption representing idle/overhead power
- `β₁` (coefficient): Incremental power per additional stream (watts per stream)
- `streams`: Number of concurrent transcoding streams

**Example Interpretation:**
If β₀ = 40W and β₁ = 15W/stream, then:
- 0 streams: 40W (baseline/idle)
- 4 streams: 40 + (15 × 4) = 100W
- 8 streams: 40 + (15 × 8) = 160W

**Assumptions:**
- Linear scaling: Each additional stream adds constant power
- No thermal throttling or frequency scaling effects
- Consistent hardware behavior across workload range

---

### Polynomial Regression (≥ 6 unique stream counts)

Used when training data contains 6 or more unique stream count values. Captures non-linear effects in power consumption.

**Formula:**
```
Power(streams) = β₀ + β₁ × streams + β₂ × streams²
```

**Parameters:**
- `β₀` (intercept): Baseline power consumption
- `β₁` (linear coefficient): Linear component of power scaling
- `β₂` (quadratic coefficient): Non-linear scaling effects
- `streams`: Number of concurrent transcoding streams

**What the Quadratic Term Captures:**
1. **Thermal Throttling**: At high loads, CPUs may reduce frequency to manage heat
2. **Cache Contention**: More streams compete for L3 cache, reducing efficiency
3. **Memory Bandwidth Saturation**: DRAM bandwidth becomes bottleneck
4. **CPU Frequency Scaling**: Turbo boost behavior changes with core utilization
5. **Power Delivery Limits**: VRM (Voltage Regulator Module) constraints

**Example Interpretation:**
If β₀ = 35W, β₁ = 18W/stream, β₂ = -0.5W/stream²:
- 2 streams: 35 + (18 × 2) + (-0.5 × 4) = 69W
- 4 streams: 35 + (18 × 4) + (-0.5 × 16) = 99W
- 8 streams: 35 + (18 × 8) + (-0.5 × 64) = 147W (reduced efficiency)

The negative β₂ indicates diminishing returns: each additional stream adds less power than predicted by linear model.

---

## Data Requirements

### Input Data Structure

The model expects scenario data from `ResultsAnalyzer` with the following structure:

```python
scenarios = [
    {
        'name': '2 Streams @ 2500k',           # String with stream count
        'power': {
            'mean_watts': 80.0,                # Mean power during test
            'median_watts': 79.5,              # Not used by predictor
            'min_watts': 75.0,                 # Not used by predictor
            'max_watts': 85.0,                 # Not used by predictor
            'total_energy_joules': 4800.0      # Not used by predictor
        },
        'bitrate': '2500k',                    # Not used by predictor
        'resolution': '1280x720',              # Not used by predictor
        'fps': 30,                             # Not used by predictor
        'duration': 60.0                       # Not used by predictor
    },
    # ... more scenarios
]
```

**Required Fields:**
- `name`: String containing stream count information
- `power.mean_watts`: Float representing average power consumption

**Optional Fields:**
All other fields are ignored by the predictor but used by other analysis components.

---

### Stream Count Inference

The model automatically extracts stream counts from scenario names using pattern matching.

**Supported Patterns:**

| Pattern | Example | Extracted Count |
|---------|---------|----------------|
| `N stream(s)` | `"4 Streams @ 2500k"` | 4 |
| `N-stream` | `"8-stream test"` | 8 |
| `single stream` | `"Single Stream @ 1080p"` | 1 |
| Leading number | `"6 concurrent streams"` | 6 |
| Case insensitive | `"12 STREAMS Test"` | 12 |

**Non-Matchable Patterns:**
- `"Baseline (Idle)"` → `None` (no stream count)
- `"Multi Stream Test"` → `None` (ambiguous count)
- `"High Quality Encode"` → `None` (no stream count)

Scenarios without extractable stream counts are automatically filtered out during training.

---

### Data Quality Guidelines

**Minimum Requirements:**
- 1 valid data point (model will train but predictions will be poor)
- At least `mean_watts` power measurement for each scenario

**Recommended for Linear Model:**
- 4+ data points with different stream counts
- Even distribution across stream count range
- Example: [1, 2, 4, 8] streams

**Recommended for Polynomial Model:**
- 7+ data points with different stream counts
- Wide range of stream counts
- Example: [1, 2, 3, 4, 6, 8, 12] streams

**Data Collection Best Practices:**
1. Run each test for 60+ seconds to get stable power readings
2. Allow 10-15 second stabilization before measurement
3. Maintain consistent hardware configuration across tests
4. Keep ambient temperature stable
5. Use same codec, preset, and quality settings
6. Measure RAPL (Running Average Power Limit) counters for accuracy

---

## Model Training Algorithm

### Training Process

```
1. Data Extraction
   ├─ Parse scenario names to infer stream counts
   ├─ Extract mean_watts from power measurements
   └─ Filter scenarios missing either value

2. Feature Engineering
   ├─ X (features): Stream counts as numpy array [n_samples, 1]
   ├─ y (target): Power measurements as numpy array [n_samples]
   └─ Count unique stream values

3. Model Selection
   ├─ If unique_streams < 6:
   │   └─ Use Linear Regression
   └─ If unique_streams ≥ 6:
       └─ Use Polynomial Regression (degree=2)

4. Feature Transformation (Polynomial only)
   ├─ Input: [streams]
   ├─ Transform: [1, streams, streams²]
   └─ Example: [4] → [1, 4, 16]

5. Model Fitting
   ├─ Algorithm: Ordinary Least Squares (OLS)
   ├─ Objective: Minimize Σ(y_true - y_pred)²
   └─ Solver: sklearn LinearRegression

6. Model Validation
   ├─ Calculate R² score
   ├─ R² = 1 - (SS_res / SS_tot)
   │   Where:
   │   SS_res = Σ(y_true - y_pred)²  # Residual sum of squares
   │   SS_tot = Σ(y_true - y_mean)²  # Total sum of squares
   └─ Log R² for quality assessment
```

### R² Score Interpretation

| R² Value | Interpretation | Action |
|----------|---------------|--------|
| 0.95 - 1.00 | Excellent fit | High confidence predictions |
| 0.85 - 0.95 | Good fit | Reliable predictions |
| 0.70 - 0.85 | Moderate fit | Use with caution |
| 0.50 - 0.70 | Poor fit | Consider more data |
| < 0.50 | Very poor fit | Model not reliable |
| Negative | Model worse than mean | Do not use predictions |

---

## Prediction Methodology

### Prediction Algorithm

```
1. Input Validation
   └─ Check if model is trained (return None if not)

2. Feature Preparation
   ├─ Create feature array: X = [[streams]]
   └─ For polynomial: Transform to [1, streams, streams²]

3. Model Prediction
   ├─ Linear: Power = β₀ + β₁ × streams
   └─ Polynomial: Power = β₀ + β₁ × streams + β₂ × streams²

4. Post-Processing
   ├─ Clamp to non-negative: max(0, prediction)
   └─ Return as float (watts)
```

### Interpolation vs Extrapolation

**Interpolation (within training range):** Generally reliable

```
Training data: [2, 4, 8] streams
Prediction:    6 streams ✓ (between 4 and 8)
Confidence:    High
```

**Extrapolation (outside training range):** Use with caution

```
Training data: [2, 4, 8] streams
Prediction:    16 streams ⚠ (beyond 8)
Confidence:    Moderate (within 2x range)

Prediction:    64 streams ✗ (far beyond 8)
Confidence:    Low (> 2x range, avoid)
```

**Extrapolation Risks:**
- Linear model assumes constant scaling (may diverge from reality)
- Polynomial model can diverge rapidly outside training range
- Real systems may have thermal limits not captured in model
- CPU throttling behavior may change at extreme loads
- Power supply limits may cap maximum power

---

## Usage Examples

### Basic Training and Prediction

```python
from advisor import PowerPredictor

# Create predictor
predictor = PowerPredictor()

# Load scenarios (from ResultsAnalyzer)
scenarios = [
    {'name': '2 Streams @ 2500k', 'power': {'mean_watts': 80.0}},
    {'name': '4 Streams @ 2500k', 'power': {'mean_watts': 150.0}},
    {'name': '8 Streams @ 1080p', 'power': {'mean_watts': 280.0}},
]

# Train model
success = predictor.fit(scenarios)
if success:
    print("Model trained successfully!")
else:
    print("Failed to train (no valid data)")

# Make predictions
power_6 = predictor.predict(6)
print(f"Predicted power for 6 streams: {power_6:.2f} W")

power_12 = predictor.predict(12)
print(f"Predicted power for 12 streams: {power_12:.2f} W")
```

### Checking Model Information

```python
# Get model metadata
info = predictor.get_model_info()

print(f"Trained: {info['trained']}")
print(f"Model Type: {info['model_type']}")
print(f"Training Samples: {info['n_samples']}")
print(f"Stream Range: {info['stream_range']}")

# Example output:
# Trained: True
# Model Type: linear
# Training Samples: 3
# Stream Range: (2, 8)
```

### Integration with analyze_results.py

The model is automatically integrated when running analysis:

```bash
# Run analysis (includes power predictions)
python3 analyze_results.py test_results/test_results_20231215_143022.json

# Output includes:
# 1. Standard analysis report
# 2. Power scalability predictions section
# 3. Measured vs predicted comparison table
# 4. CSV export with predicted_mean_power_w column
```

---

## Output Formats

### Console Output

```
==================================================================================================
POWER SCALABILITY PREDICTIONS
==================================================================================================

Model Type: LINEAR
Training Samples: 4
Stream Range: 1 - 8 streams

Predicted Power Consumption:
──────────────────────────────────────────────────────────────────────────────────────────────────
   1 streams:    45.23 W
   2 streams:    78.45 W
   4 streams:   145.12 W
   8 streams:   278.67 W
  12 streams:   412.23 W

──────────────────────────────────────────────────────────────────────────────────────────────────
MEASURED vs PREDICTED COMPARISON
──────────────────────────────────────────────────────────────────────────────────────────────────
(Shows model fit quality on training data)
Streams    Measured (W)    Predicted (W)   Diff (W)    
──────────────────────────────────────────────────────────────────────────────────────────────────
1          45.00           45.23           +0.23
2          80.00           78.45           -1.55
4          150.00          145.12          -4.88
8          280.00          278.67          -1.33
──────────────────────────────────────────────────────────────────────────────────────────────────
```

### CSV Export

The `predicted_mean_power_w` column is added to the analysis CSV:

```csv
name,bitrate,resolution,fps,duration,mean_power_w,predicted_mean_power_w,...
"2 Streams @ 2500k",2500k,1280x720,30,60.0,80.0,78.45,...
"4 Streams @ 2500k",2500k,1280x720,30,60.0,150.0,145.12,...
"8 Streams @ 1080p",5000k,1920x1080,30,60.0,280.0,278.67,...
```

**Column Meaning:**
- `mean_power_w`: Actual measured power from Prometheus/RAPL
- `predicted_mean_power_w`: Model prediction based on stream count

---

## Limitations and Caveats

### Model Assumptions

1. **Consistent Hardware**: Same CPU, RAM, cooling across all measurements
2. **Consistent Configuration**: Same FFmpeg preset, codec, quality settings
3. **Stream Count Primary Factor**: Assumes power scales mainly with stream count
4. **Stable Environment**: Constant ambient temperature, no thermal throttling

### Not Accounted For

❌ **Different Codecs**: H.264 vs H.265 vs AV1 have different power profiles
❌ **Different Resolutions**: 720p vs 1080p vs 4K per stream
❌ **Different Bitrates**: 2500k vs 5000k per stream
❌ **Different Presets**: ultrafast vs medium vs slow
❌ **Ambient Temperature**: Heat affects CPU frequency and power
❌ **Power Management**: Governor settings (performance vs powersave)
❌ **Background Load**: Other processes competing for CPU
❌ **Turbo Boost State**: Enabled vs disabled
❌ **NUMA Effects**: Multi-socket systems with non-uniform memory access

### When Model May Be Inaccurate

⚠️ **Small Datasets**: < 3 training points
⚠️ **Extrapolation**: Predicting > 2x max training stream count
⚠️ **Heterogeneous Data**: Mixed codecs, resolutions, or settings
⚠️ **Thermal Throttling**: Training data includes throttled measurements
⚠️ **Inconsistent Measurements**: Wide variance in power readings
⚠️ **Low R² Score**: < 0.70 indicates poor model fit

---

## Use Cases

### 1. Capacity Planning

**Scenario:** Determine how many concurrent streams a server can handle within power budget.

```python
predictor = PowerPredictor()
predictor.fit(benchmark_scenarios)

# Power budget: 300W
max_power = 300.0
for streams in range(1, 20):
    predicted = predictor.predict(streams)
    if predicted > max_power:
        print(f"Max streams within {max_power}W: {streams - 1}")
        break
```

### 2. Cost Estimation

**Scenario:** Estimate monthly energy costs for different workload sizes.

```python
# Predict power for target workload
streams = 10
power_watts = predictor.predict(streams)

# Calculate monthly energy
hours_per_month = 730
kwh_per_month = (power_watts * hours_per_month) / 1000

# Calculate cost (assuming $0.12/kWh)
cost_per_month = kwh_per_month * 0.12

print(f"{streams} streams: {kwh_per_month:.2f} kWh/month = ${cost_per_month:.2f}")
```

### 3. Thermal Management

**Scenario:** Identify safe operating limits before load testing.

```python
# Check predicted power at different scales
for streams in [4, 8, 12, 16]:
    power = predictor.predict(streams)
    print(f"{streams} streams → {power:.0f}W")
    
    if power > 250:  # Server cooling limit
        print(f"  ⚠️  Exceeds thermal capacity")
```

### 4. Infrastructure Sizing

**Scenario:** Determine PDU (Power Distribution Unit) requirements for data center.

```python
# Calculate total rack power for 10 servers
servers = 10
streams_per_server = 8

power_per_server = predictor.predict(streams_per_server)
total_rack_power = power_per_server * servers

print(f"Total rack power: {total_rack_power:.0f}W")
print(f"Required PDU capacity: {total_rack_power * 1.2:.0f}W (20% headroom)")
```

---

## Model Validation

### Assessing Prediction Quality

1. **Check R² Score**: Should be > 0.70 for reliable predictions
   ```python
   # Logged automatically during training
   # INFO:root:PowerPredictor trained on 5 data points, R² = 0.9234
   ```

2. **Review Comparison Table**: Differences should be small
   ```
   Streams    Measured (W)    Predicted (W)   Diff (W)
   2          80.00           78.45           -1.55    ✓ Good
   4          150.00          145.12          -4.88    ✓ Good
   8          280.00          320.50          +40.50   ✗ Poor
   ```

3. **Cross-Validation**: Hold out some data points
   ```python
   # Train on subset
   train_scenarios = scenarios[:-2]
   predictor.fit(train_scenarios)
   
   # Test on held-out data
   test_scenarios = scenarios[-2:]
   for scenario in test_scenarios:
       streams = predictor._infer_stream_count(scenario['name'])
       predicted = predictor.predict(streams)
       actual = scenario['power']['mean_watts']
       error = abs(predicted - actual) / actual * 100
       print(f"{scenario['name']}: {error:.1f}% error")
   ```

### Improving Model Accuracy

**Collect More Data:**
- Add scenarios with different stream counts
- Fill gaps in stream count range
- Add replicate measurements for averaging

**Ensure Data Quality:**
- Verify stable power readings (low stddev)
- Check for thermal throttling during tests
- Confirm consistent test duration (60+ seconds)
- Validate RAPL measurements are accurate

**Consider Polynomial Model:**
- Collect ≥ 6 unique stream counts
- Model will automatically switch to polynomial
- Better captures non-linear scaling effects

**Standardize Test Conditions:**
- Same FFmpeg preset across all tests
- Same resolution and bitrate per stream
- Same ambient temperature
- Same power management settings

---

## Troubleshooting

### Model Won't Train

**Issue:** `predictor.fit()` returns `False`

**Causes:**
1. No scenarios with valid power data
2. No scenarios with parseable stream counts
3. All scenarios filtered out

**Solutions:**
```python
# Debug: Check what data was extracted
predictor = PowerPredictor()
for scenario in scenarios:
    streams = predictor._infer_stream_count(scenario['name'])
    power = scenario.get('power', {}).get('mean_watts')
    print(f"{scenario['name']}: streams={streams}, power={power}")
```

### Poor Predictions

**Issue:** Large differences between measured and predicted

**Causes:**
1. Low R² score (< 0.70)
2. Non-linear effects in data but using linear model
3. Inconsistent measurements in training data
4. Extrapolating far beyond training range

**Solutions:**
- Collect more training data (aim for 6+ unique stream counts)
- Check for outliers in training data
- Review test conditions for consistency
- Avoid predictions > 2x max training stream count

### Negative Predictions

**Issue:** Model predicts negative power

**This should not happen** - predictions are clamped to non-negative values. If you see this, it's a bug.

---

## Technical Implementation Details

### Dependencies

```python
numpy>=1.20.0          # Array operations, linear algebra
scikit-learn>=1.3.0    # Machine learning (LinearRegression, PolynomialFeatures)
```

### Key Classes and Methods

**PowerPredictor:**
- `__init__()`: Initialize empty model
- `fit(scenarios)`: Train on scenario data
- `predict(streams)`: Predict power for N streams
- `get_model_info()`: Get model metadata
- `_infer_stream_count(name)`: Parse stream count from name

**sklearn Components:**
- `LinearRegression`: OLS regression model
- `PolynomialFeatures(degree=2)`: Feature transformation for quadratic terms

### Code Location

- Model implementation: `advisor/modeling.py`
- Integration: `analyze_results.py`
- Tests: `tests/test_modeling.py`
- Documentation: `docs/power-prediction-model.md` (this file)

---

## Future Enhancements

### Potential Improvements

1. **Multi-Variable Models**: Incorporate resolution, bitrate, codec
2. **Time-Series Predictions**: Account for thermal buildup over time
3. **Confidence Intervals**: Provide prediction uncertainty ranges
4. **GPU Power Modeling**: Extend to NVIDIA/AMD GPU transcoding
5. **Ensemble Models**: Combine multiple models for robustness
6. **Automated Hyperparameter Tuning**: Optimize polynomial degree
7. **Feature Selection**: Identify most predictive variables
8. **Cross-Platform Validation**: Test on different CPU architectures

### Contributing

To improve the model:
1. Collect diverse training data (different workloads, hardware)
2. Document any prediction errors or limitations discovered
3. Suggest additional features or variables to incorporate
4. Share R² scores and model performance metrics

---

## References

- **RAPL (Running Average Power Limit)**: Intel's power measurement interface
- **Ordinary Least Squares (OLS)**: Statistical method for linear regression
- **scikit-learn Documentation**: https://scikit-learn.org/stable/modules/linear_model.html
- **Polynomial Regression**: https://en.wikipedia.org/wiki/Polynomial_regression
- **R² Score**: https://en.wikipedia.org/wiki/Coefficient_of_determination

---

## License

This component follows the same license as the main ffmpeg-rtmp project.

---

## Contact

For questions or issues related to the power prediction model:
1. Open an issue on GitHub repository
2. Include your R² score and training data characteristics
3. Provide example predictions showing unexpected behavior
