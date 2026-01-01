# ML Predictions System

## Overview

The ML Predictions System provides real-time quality of experience (QoE) and cost predictions for video transcoding operations using machine learning models. The system integrates Random Forest and Gradient Boosting models to predict VMAF, PSNR, transcoding cost, and CO2 emissions based on encoding parameters.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      ML Prediction Pipeline                      │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐  │
│  │   Rust ML    │─────▶│  Go Exporter │─────▶│  Prometheus  │  │
│  │   Library    │ FFI  │    (cgo)     │      │   Metrics    │  │
│  └──────────────┘      └──────────────┘      └──────────────┘  │
│         │                      │                       │         │
│         │                      │                       │         │
│         ▼                      ▼                       ▼         │
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐  │
│  │ Model Bundle │      │   HTTP API   │      │   Grafana    │  │
│  │  (JSON)      │      │  /metrics    │      │  Dashboard   │  │
│  └──────────────┘      │  /health     │      └──────────────┘  │
│                        │  /reload     │                         │
│                        └──────────────┘                         │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              VictoriaMetrics Data Export                 │   │
│  │           (Historical Data for Retraining)              │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### 1. Rust ML Library (`ml_rust`)

The core prediction engine implemented in Rust for performance and safety.

**Models:**
- **Random Forest Regression**: Predicts QoE metrics (VMAF, PSNR)
- **Gradient Boosting Regression**: Predicts cost (USD) and CO2 emissions (kg)

**Key Structures:**

```rust
pub struct PredictionFeatures {
    pub bitrate_kbps: f32,
    pub resolution_width: u32,
    pub resolution_height: u32,
    pub frame_rate: f32,
    pub frame_drop: f32,
    pub motion_intensity: f32,
}

pub struct PredictionResult {
    pub predicted_vmaf: f32,
    pub predicted_psnr: f32,
    pub predicted_cost_usd: f32,
    pub predicted_co2_kg: f32,
    pub confidence: f32,
    pub recommended_bitrate_kbps: u32,
}
```

**API Functions:**
- `load_model(path)` - Load model from disk
- `predict(features, model)` - Make prediction
- `save_model(model, path)` - Save model to disk
- `retrain(features, targets)` - Retrain model with new data

### 2. ML Prometheus Exporter

A Go-based exporter that bridges the Rust ML library with Prometheus using CGO.

**Endpoints:**
- `GET /metrics` - Prometheus metrics
- `GET /health` - Health check
- `POST /reload` - Reload model from disk

**Exposed Metrics:**

| Metric Name | Type | Description |
|------------|------|-------------|
| `qoe_predicted_vmaf` | Gauge | Predicted VMAF score (0-100) |
| `qoe_predicted_psnr` | Gauge | Predicted PSNR score (dB) |
| `cost_predicted_usd` | Gauge | Predicted transcoding cost (USD) |
| `cost_predicted_co2_kg` | Gauge | Predicted CO2 emissions (kg) |
| `prediction_confidence` | Gauge | Model confidence (0-1) |
| `recommended_bitrate_kbps` | Gauge | Recommended bitrate (kbps) |
| `ml_model_version` | Gauge | Current model version |
| `ml_model_last_update_timestamp` | Gauge | Last model update time |
| `ml_exporter_up` | Gauge | Exporter health status |

**Labels:**
- `bitrate` - Input bitrate configuration
- `resolution` - Video resolution (e.g., "1920x1080")
- `fps` - Frame rate

### 3. Grafana Dashboard

The ML Predictions dashboard (`ml-predictions.json`) provides visualization and alerting.

**Panels:**
1. **Predicted VMAF Over Time** - Time series of VMAF predictions
2. **Predicted PSNR Over Time** - Time series of PSNR predictions
3. **Predicted Cost (USD)** - Cost predictions with thresholds
4. **Predicted CO2 Emissions** - Environmental impact
5. **Prediction Confidence** - Model confidence levels
6. **Recommended Bitrate** - Optimal bitrate suggestions
7. **Model Version** - Current model version
8. **Last Model Update** - Time since last update
9. **Training Drift Indicator** - Detects model drift
10. **ML Exporter Status** - System health
11. **Prediction Input Features** - Table of input parameters
12. **Historical vs Predicted QoE** - Comparison with actual values

**Configured Alerts:**

| Alert | Condition | Description |
|-------|-----------|-------------|
| Low VMAF | `qoe_predicted_vmaf < 80` | Quality below acceptable threshold |
| High Cost | `cost_predicted_usd > threshold` | Cost exceeds budget |
| Low Confidence | `prediction_confidence < 0.7` | Model uncertainty too high |

## Deployment

### Quick Start

1. **Train the model with synthetic data:**
```bash
make train-model
```

2. **Deploy the ML exporter:**
```bash
make deploy-ml
```

3. **Access the dashboard:**
- Open Grafana at `http://localhost:3000`
- Navigate to "ML Predictions Dashboard"

### Docker Compose

The ML exporter is automatically included in `docker-compose.yml`:

```yaml
ml-predictions-exporter:
  build:
    context: .
    dockerfile: master/exporters/ml_predictions/Dockerfile
  ports:
    - "9505:9505"
  volumes:
    - ./ml_models:/app/ml_models:ro
  environment:
    - ML_EXPORTER_PORT=9505
    - MODEL_PATH=/app/ml_models/model.json
```

Start the stack:
```bash
docker compose up -d
```

### Manual Build

Build Rust library:
```bash
cd ml_rust
cargo build --release
```

Build Go exporter:
```bash
cd master/exporters/ml_predictions
CGO_ENABLED=1 go build -o ml_exporter
```

Run exporter:
```bash
./ml_exporter --port 9505 --model-path /path/to/model.json
```

## Model Training and Lifecycle

### Data Collection

Export training data from VictoriaMetrics:

```bash
python3 scripts/export_training_data.py \
  --vm-url http://localhost:8428 \
  --start 7d \
  --end now \
  --output ./ml_models/training_data.csv
```

Or generate synthetic data for bootstrapping:

```bash
python3 scripts/export_training_data.py --synthetic --output ./ml_models/training_data.csv
```

### Model Retraining

The model learns from historical QoE measurements and encoding parameters:

**Training Features:**
- Bitrate (kbps)
- Resolution (width × height)
- Frame rate (fps)
- Frame drop rate (0-1)
- Motion intensity (0-1)

**Training Targets:**
- VMAF score (0-100)
- PSNR score (dB)
- Transcoding cost (USD)
- CO2 emissions (kg)

**Acceptance Criteria:**
- R² score ≥ 0.85 for VMAF predictions
- R² score ≥ 0.85 for PSNR predictions

### Model Versioning

Models are stored in `ml_models/` with version metadata:

```json
{
  "version": "1.0.0",
  "trained_at": "2026-01-01T12:00:00Z",
  "metrics": {
    "vmaf_r2": 0.92,
    "psnr_r2": 0.89
  }
}
```

### Reload Model

To reload the model without restarting:

```bash
curl -X POST http://localhost:9505/reload
```

Or through the Makefile:
```bash
make update-dashboards
```

## Prediction Features

### Input Features

| Feature | Type | Range | Description |
|---------|------|-------|-------------|
| `bitrate_kbps` | float32 | 100-50000 | Target bitrate in kbps |
| `resolution_width` | uint32 | 640-7680 | Video width in pixels |
| `resolution_height` | uint32 | 360-4320 | Video height in pixels |
| `frame_rate` | float32 | 15-120 | Frame rate in fps |
| `frame_drop` | float32 | 0.0-1.0 | Frame drop rate |
| `motion_intensity` | float32 | 0.0-1.0 | Motion complexity |

### Output Predictions

| Prediction | Type | Range | Description |
|-----------|------|-------|-------------|
| `predicted_vmaf` | float32 | 0-100 | Video quality (VMAF) |
| `predicted_psnr` | float32 | 0-50 | Signal quality (dB) |
| `predicted_cost_usd` | float32 | 0+ | Transcoding cost |
| `predicted_co2_kg` | float32 | 0+ | CO2 emissions |
| `confidence` | float32 | 0-1 | Prediction confidence |
| `recommended_bitrate_kbps` | uint32 | 100-50000 | Optimal bitrate |

### Confidence Calculation

Confidence is computed based on:
- Input feature quality (e.g., low frame drop increases confidence)
- Predicted VMAF value (high VMAF increases confidence)
- Model variance (ensemble agreement)

**Confidence Thresholds:**
- ≥ 0.85: High confidence
- 0.70-0.85: Medium confidence
- < 0.70: Low confidence (alert triggered)

## Monitoring and Alerts

### Grafana Alerts

Configure alert notifications in Grafana:

1. Navigate to "ML Predictions Dashboard"
2. Edit panel with alert
3. Configure notification channel
4. Set severity and message

### Health Checks

Check exporter health:
```bash
curl http://localhost:9505/health
```

Expected response:
```json
{
  "status": "ok",
  "model_loaded": true,
  "timestamp": 1735747200
}
```

### Logs

View exporter logs:
```bash
docker compose logs -f ml-predictions-exporter
```

## Use Cases

### 1. Bitrate Optimization

The system recommends optimal bitrate based on resolution and quality targets:

```
Input: 1920x1080 @ 30fps
Predicted VMAF: 85
Recommended Bitrate: 2800 kbps
```

### 2. Cost Prediction

Estimate transcoding costs before execution:

```
Input: 4K @ 60fps, 15 Mbps
Predicted Cost: $0.55 USD
Predicted CO2: 0.11 kg
```

### 3. Quality Assurance

Alert when predicted quality falls below thresholds:

```
Alert: Predicted VMAF (75) < 80
Action: Increase bitrate or adjust encoding parameters
```

### 4. A/B Testing

Compare different encoding configurations:

```
Config A: 1080p @ 30fps, 2.5 Mbps → VMAF: 85, Cost: $0.12
Config B: 1080p @ 60fps, 5.0 Mbps → VMAF: 92, Cost: $0.22
```

## Testing

### Unit Tests

Run Rust tests:
```bash
cd ml_rust
cargo test
```

Expected output:
```
running 11 tests
test tests::test_vmaf_prediction_accuracy ... ok
test tests::test_psnr_prediction_accuracy ... ok
...
test result: ok. 11 passed
```

### Integration Testing

Test the exporter:

1. Start the exporter:
```bash
docker compose up -d ml-predictions-exporter
```

2. Query metrics:
```bash
curl http://localhost:9505/metrics | grep qoe_predicted
```

3. Verify predictions appear in Grafana dashboard

### Validation

Validate predictions against actual QoE measurements:

```bash
python3 scripts/validate_predictions.py \
  --predictions http://localhost:9505/metrics \
  --actuals ./test_results/test_results_*.json
```

## Performance

### Latency

- Model load time: < 100ms
- Prediction time: < 1ms per sample
- HTTP endpoint response: < 10ms

### Throughput

- Predictions per second: > 10,000
- Concurrent requests: 100+

### Resource Usage

- Memory: ~50 MB (model + exporter)
- CPU: < 5% (idle), < 20% (under load)

## Troubleshooting

### Model Not Loading

**Problem:** Exporter reports "model not loaded"

**Solution:**
1. Check model file exists: `ls -la ml_models/model.json`
2. Verify file permissions
3. Generate default model: `make train-model`

### Low Prediction Accuracy

**Problem:** R² score < 0.85

**Solution:**
1. Collect more training data
2. Export data from VictoriaMetrics: `make train-model`
3. Retrain with actual measurements

### CGO Build Errors

**Problem:** `cannot find -lffmpeg_ml`

**Solution:**
1. Build Rust library first: `cd ml_rust && cargo build --release`
2. Verify library path in Dockerfile
3. Check `CGO_LDFLAGS` environment variable

### Dashboard Not Showing Data

**Problem:** Empty panels in Grafana showing "No data"

**Common Causes:**
1. **Wrong datasource UID** - Dashboard panels referencing incorrect datasource
2. **Missing scrape configuration** - VictoriaMetrics not configured to scrape ML exporter
3. **Datasource mismatch** - Dashboard UID doesn't match provisioned datasource

**Solution:**

**Step 1: Verify Exporter is Running**
```bash
# Check exporter health
curl http://localhost:9505/health

# Verify metrics are being exposed
curl http://localhost:9505/metrics | grep qoe_predicted
```

**Step 2: Check VictoriaMetrics Scrape Configuration**

Ensure `master/monitoring/victoriametrics.yml` includes the ML exporter:

```yaml
- job_name: 'ml-predictions-exporter'
  static_configs:
    - targets: ['ml-predictions-exporter:9505']
      labels:
        service: 'ml-predictions'
        exporter: 'go-rust'
```

After adding, restart VictoriaMetrics:
```bash
docker compose restart victoriametrics
```

**Step 3: Verify Datasource UID in Dashboard**

The dashboard panels must use the correct datasource UID. Check `master/monitoring/grafana/provisioning/datasources/prometheus.yml` for the datasource UID:

```yaml
datasources:
  - name: VictoriaMetrics
    type: prometheus
    uid: DS_VICTORIAMETRICS  # This is the UID to use
    isDefault: true
```

Update all panel datasources in `ml-predictions.json` to match:
```json
{
  "datasource": {
    "type": "prometheus",
    "uid": "DS_VICTORIAMETRICS"
  }
}
```

**Step 4: Verify Metrics in VictoriaMetrics**
```bash
# Check if metrics are being scraped
curl -s 'http://localhost:8428/api/v1/query?query=ml_exporter_up' | jq

# Check for prediction metrics
curl -s 'http://localhost:8428/api/v1/query?query=qoe_predicted_vmaf' | jq
```

**Step 5: Set Dashboard Refresh Rate**

Configure automatic refresh in the dashboard JSON:
```json
{
  "refresh": "5s"
}
```

**Step 6: Restart Grafana**
```bash
docker compose restart grafana
```

**Verification:**
1. Open Grafana: `http://localhost:3000`
2. Navigate to "ML Predictions Dashboard"
3. Dashboard should refresh every 5 seconds
4. All panels should display prediction data

## Future Enhancements

- [ ] Quantile regression for prediction intervals
- [ ] Online learning for continuous model updates
- [ ] Multi-model ensembles for improved accuracy
- [ ] GPU acceleration for inference
- [ ] AutoML for hyperparameter tuning
- [ ] Drift detection and automatic retraining
- [ ] Per-codec specialized models
- [ ] Scene complexity analysis
- [ ] Real-time streaming metrics integration

## References

- [VMAF Documentation](https://github.com/Netflix/vmaf)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [Grafana Dashboard Guide](https://grafana.com/docs/grafana/latest/dashboards/)
- [Rust FFI Guide](https://doc.rust-lang.org/nomicon/ffi.html)

## Support

For issues or questions:
- GitHub Issues: `psantana5/ffmpeg-rtmp`
- Documentation: `docs/ML_PREDICTIONS.md`
- Dashboard: ML Predictions Dashboard in Grafana
