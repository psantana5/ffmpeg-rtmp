# ML and Cost Optimization Migration Plan

## Overview

This document outlines the plan to migrate the ML and cost optimization components from Python to Go (exporters) and Rust (ML library). This migration aims to improve performance, reduce resource consumption, and provide better integration with the existing Go-based infrastructure.

## Current Architecture

### Python Components (Master Node)

**Exporters (master/exporters/):**
- `cost_exporter.py` (725 lines) - Prometheus exporter for cost metrics
  - Regional pricing integration
  - Load-aware cost calculations using trapezoidal integration
  - CO₂ emissions tracking
  - Monthly cost projections
  
- `qoe_exporter.py` (260 lines) - Quality of Experience metrics exporter
  - VMAF and PSNR quality scores
  - Quality per watt efficiency metrics
  - QoE efficiency scoring

- `results_exporter.py` (949 lines) - Results aggregation and ML predictions
  - Scenario metrics aggregation
  - Multivariate power prediction
  - Efficiency scoring
  - Baseline comparisons

**ML/Advisor Library (shared/advisor/):**
- `modeling.py` (1,146 lines) - Power prediction models
  - Linear regression (single-variable)
  - Polynomial regression (degree-2)
  - Multivariate prediction with confidence intervals
  - Bootstrap sampling for robust estimates
  
- `cost.py` (720 lines) - Cost modeling
  - Energy cost calculations
  - Compute cost (CPU/GPU)
  - Load-aware cost using trapezoidal integration
  - Cost per pixel and per watch hour

- `scoring.py` (656 lines) - Energy efficiency scoring
  - Pixels per joule
  - QoE efficiency (quality-weighted)
  - Throughput per watt
  - Quality per watt

- `recommender.py` (374 lines) - Configuration recommendations
  - Ranking by efficiency
  - Output ladder grouping
  - Top-N selection

- `regional_pricing.py` (365 lines) - Regional pricing and CO₂
  - Region-specific electricity pricing
  - Carbon intensity data
  - Dynamic pricing via Electricity Maps API

### Integration Points

1. **Exporters → VictoriaMetrics/Prometheus**
   - HTTP endpoints at :9502, :9503, :9504
   - Text-format Prometheus metrics
   - Health check endpoints

2. **Exporters → Test Results**
   - Read from `test_results/` directory
   - JSON format scenario data
   - File system watching for new results

3. **Grafana Dashboards**
   - `master/monitoring/grafana/provisioning/dashboards/`
   - Queries reference specific metric names
   - Must maintain metric compatibility

## Migration Strategy

### Phase 1: Rust ML Library

Create a standalone Rust library (`ml_rust/`) that implements:

#### Core ML Models
- **Linear Regression** - Ordinary least squares implementation
  ```rust
  pub struct LinearPredictor {
      coefficients: Vec<f64>,
      r2_score: f64,
  }
  impl LinearPredictor {
      pub fn fit(&mut self, x: &[f64], y: &[f64]) -> Result<()>
      pub fn predict(&self, x: f64) -> Result<f64>
      pub fn r2(&self) -> f64
  }
  ```

- **Polynomial Regression** - Using polynomial features
  ```rust
  pub struct PolynomialPredictor {
      degree: usize,
      coefficients: Vec<f64>,
      r2_score: f64,
  }
  ```

- **Multivariate Regression** - Multiple feature support
  ```rust
  pub struct MultivariatePredictor {
      feature_names: Vec<String>,
      coefficients: Vec<f64>,
      confidence_level: f64,
  }
  impl MultivariatePredictor {
      pub fn predict_with_confidence(&self, features: &[f64]) 
          -> Result<(f64, f64, f64)>  // (mean, ci_low, ci_high)
  }
  ```

#### Energy Efficiency Scoring
```rust
pub enum ScoringAlgorithm {
    PixelsPerJoule,
    QoEEfficiency,
    ThroughputPerWatt,
    QualityPerWatt,
}

pub struct EnergyEfficiencyScorer {
    algorithm: ScoringAlgorithm,
}
```

#### Cost Modeling
```rust
pub struct CostModel {
    energy_cost_per_kwh: f64,
    cpu_cost_per_hour: f64,
    gpu_cost_per_hour: f64,
    currency: String,
}
impl CostModel {
    pub fn compute_total_cost(&self, energy_j: f64, duration_h: f64, use_gpu: bool) -> f64
    pub fn compute_energy_cost_load_aware(&self, power_samples: &[f64], step_s: f64) -> f64
    pub fn compute_cost_per_pixel(&self, total_cost: f64, total_pixels: f64) -> Option<f64>
}
```

#### Regional Pricing
```rust
pub struct RegionalPricing {
    region: String,
    electricity_price: f64,
    carbon_intensity: f64,
}
impl RegionalPricing {
    pub fn new(region: String) -> Self
    pub fn compute_co2_emissions(&self, energy_kwh: f64) -> f64
}
```

#### C FFI for Go Integration
```rust
// Linear Predictor FFI
#[no_mangle]
pub extern "C" fn linear_predictor_new() -> *mut CLinearPredictor
#[no_mangle]
pub extern "C" fn linear_predictor_fit(
    predictor: *mut CLinearPredictor,
    streams: *const f64,
    power: *const f64,
    n: usize,
) -> i32
#[no_mangle]
pub extern "C" fn linear_predictor_predict(
    predictor: *const CLinearPredictor,
    streams: f64,
    result: *mut f64,
) -> i32
#[no_mangle]
pub extern "C" fn linear_predictor_free(predictor: *mut CLinearPredictor)

// Cost Model FFI
#[no_mangle]
pub extern "C" fn cost_model_new(...) -> *mut CCostModel
#[no_mangle]
pub extern "C" fn cost_model_compute_total_cost(...) -> f64
#[no_mangle]
pub extern "C" fn cost_model_free(model: *mut CCostModel)

// Regional Pricing FFI
#[no_mangle]
pub extern "C" fn regional_pricing_new(region: *const c_char) -> *mut CRegionalPricing
#[no_mangle]
pub extern "C" fn regional_pricing_compute_co2_emissions(...) -> f64
#[no_mangle]
pub extern "C" fn regional_pricing_free(pricing: *mut CRegionalPricing)
```

#### Dependencies
```toml
[dependencies]
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
ndarray = "0.16"  # For matrix operations if needed

[dev-dependencies]
approx = "0.5"  # For float comparisons in tests
```

#### Testing Strategy
- Unit tests for each ML model with known input/output
- Compare against Python implementation for validation
- Benchmark performance vs Python
- Test FFI layer from Go

### Phase 2: Go Exporters

Create Go implementations of the three Python exporters:

#### Cost Exporter (`master/exporters/cost_go/`)
```go
package main

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/psantana5/ffmpeg-rtmp/pkg/ml"  // FFI bindings to Rust
)

type CostExporter struct {
    costModel     *ml.CostModel
    regionalPrice *ml.RegionalPricing
    metricsCache  map[string]float64
    lastUpdate    time.Time
}

func (e *CostExporter) GenerateMetrics() string {
    // Load latest results from JSON
    // Call Rust ML library via FFI for calculations
    // Generate Prometheus metrics
}
```

Key features to implement:
- Load-aware cost calculations
- Regional pricing integration
- CO₂ emissions tracking
- Monthly projections
- Hourly cost metrics
- Prometheus metrics export

#### QoE Exporter (`master/exporters/qoe_go/`)
```go
type QoEExporter struct {
    scorer       *ml.EnergyEfficiencyScorer
    metricsCache map[string]float64
}

func (e *QoEExporter) ComputeQoEMetrics(scenario Scenario) QoEMetrics {
    // VMAF/PSNR scores
    // Quality per watt
    // QoE efficiency via Rust FFI
}
```

#### Results Exporter (`master/exporters/results_go/`)
```go
type ResultsExporter struct {
    linearPredictor *ml.LinearPredictor
    polyPredictor   *ml.PolynomialPredictor
    multiPredictor  *ml.MultivariatePredictor
    promClient      *PrometheusClient
}

func (e *ResultsExporter) BuildMetrics() string {
    // Load scenarios from JSON
    // Compute stats (power, energy, efficiency)
    // Train ML models if new data
    // Generate predictions
    // Export Prometheus metrics
}
```

Key features:
- JSON results parsing
- Prometheus query client (for enrichment)
- ML model training and prediction
- Baseline comparisons
- Efficiency scoring
- Output ladder support

### Phase 3: Integration and Testing

#### Metrics Compatibility Matrix

| Python Metric | Go Metric | Status |
|--------------|-----------|--------|
| `cost_total_load_aware` | `cost_total_load_aware` | ✓ Maintain |
| `cost_energy_load_aware` | `cost_energy_load_aware` | ✓ Maintain |
| `cost_compute_load_aware` | `cost_compute_load_aware` | ✓ Maintain |
| `cost_per_pixel` | `cost_per_pixel` | ✓ Maintain |
| `cost_usd_per_hour` | `cost_usd_per_hour` | ✓ Maintain |
| `co2_emissions_kg_per_hour` | `co2_emissions_kg_per_hour` | ✓ Maintain |
| `qoe_vmaf_score` | `qoe_vmaf_score` | ✓ Maintain |
| `qoe_quality_per_watt` | `qoe_quality_per_watt` | ✓ Maintain |
| `qoe_efficiency_score` | `qoe_efficiency_score` | ✓ Maintain |
| `results_scenario_*` | `results_scenario_*` | ✓ Maintain |
| `results_scenario_predicted_*` | `results_scenario_predicted_*` | ✓ Maintain |

#### Dockerfiles

**ml_rust/Dockerfile:**
```dockerfile
FROM rust:1.75 as builder
WORKDIR /build
COPY ml_rust/ .
RUN cargo build --release

FROM debian:bookworm-slim
COPY --from=builder /build/target/release/libffmpeg_ml.so /usr/local/lib/
RUN ldconfig
```

**master/exporters/cost_go/Dockerfile:**
```dockerfile
FROM golang:1.21 as builder
WORKDIR /build
COPY ml_rust/target/release/libffmpeg_ml.so /usr/local/lib/
COPY master/exporters/cost_go/ .
RUN go build -o cost_exporter

FROM debian:bookworm-slim
COPY --from=builder /build/cost_exporter /usr/local/bin/
COPY --from=builder /usr/local/lib/libffmpeg_ml.so /usr/local/lib/
RUN ldconfig
EXPOSE 9504
CMD ["/usr/local/bin/cost_exporter"]
```

#### docker-compose.yml Updates
```yaml
  cost-exporter-go:
    build:
      context: .
      dockerfile: master/exporters/cost_go/Dockerfile
    container_name: cost-exporter-go
    volumes:
      - ./test_results:/results:ro
      - ./pricing_config.json:/app/pricing_config.json:ro
    ports:
      - "9504:9504"
    environment:
      - COST_EXPORTER_PORT=9504
      - RESULTS_DIR=/results
      - PROMETHEUS_URL=http://victoriametrics:8428
      - REGION=us-east-1
```

#### Testing Plan

1. **Unit Tests**
   - Rust: `cargo test`
   - Go: `go test ./...`

2. **Integration Tests**
   - Run both Python and Go exporters side-by-side
   - Compare metric outputs
   - Validate ML predictions match

3. **Performance Benchmarks**
   - Memory usage comparison
   - CPU usage comparison
   - Scrape latency comparison
   - Prediction latency comparison

4. **Grafana Dashboard Testing**
   - Verify all panels work with Go exporters
   - Check for metric gaps or errors
   - Validate calculations in dashboards

### Phase 4: Deployment and Cleanup

#### Rollout Strategy

1. **Parallel Deployment** (Week 1-2)
   - Deploy Go exporters on separate ports (9514, 9515, 9516)
   - Keep Python exporters running
   - Monitor both sets of metrics
   - Validate accuracy

2. **Gradual Cutover** (Week 3)
   - Update Grafana dashboards to use Go metrics
   - Monitor for issues
   - Rollback capability maintained

3. **Full Cutover** (Week 4)
   - Switch ports (Go exporters → 9502, 9503, 9504)
   - Deprecate Python exporters
   - Update documentation

4. **Cleanup** (Week 5)
   - Remove Python exporter code
   - Remove Python dependencies
   - Update CI/CD pipelines
   - Archive Python implementation for reference

#### Documentation Updates

Files to update:
- `README.md` - Update architecture diagram
- `master/exporters/README.md` - Document new Go exporters
- `ml_rust/README.md` - Create Rust library documentation
- `shared/docs/INTERNAL_ARCHITECTURE.md` - Update exporter details
- `master/monitoring/grafana/GRAFANA_WALKTHROUGH.md` - Update metric sources

## Performance Targets

| Metric | Python | Go Target | Improvement |
|--------|--------|-----------|-------------|
| Memory Usage | ~100MB | <20MB | 5x reduction |
| CPU Usage (idle) | ~2% | <0.5% | 4x reduction |
| Scrape Latency | ~50ms | <10ms | 5x faster |
| ML Training Time | ~500ms | <100ms | 5x faster |
| ML Prediction Time | ~10ms | <2ms | 5x faster |

## Risk Mitigation

1. **Accuracy Validation**
   - Risk: ML predictions differ between Python and Rust
   - Mitigation: Extensive testing with known datasets, side-by-side comparison

2. **Metric Compatibility**
   - Risk: Grafana dashboards break
   - Mitigation: Maintain identical metric names/labels, parallel deployment

3. **Build Complexity**
   - Risk: Rust FFI and Go cgo increase build time
   - Mitigation: Docker layer caching, pre-built Rust libraries

4. **Performance Regression**
   - Risk: Go/Rust slower than expected
   - Mitigation: Benchmark early, optimize hot paths, profile

## Implementation Timeline

| Phase | Duration | Deliverables |
|-------|----------|--------------|
| Phase 1: Rust ML Library | 2 weeks | Working Rust library with FFI, tests passing |
| Phase 2: Go Exporters | 2 weeks | All three exporters in Go, feature-complete |
| Phase 3: Integration & Testing | 1 week | Side-by-side validation complete |
| Phase 4: Deployment | 1 week | Production cutover, cleanup |
| **Total** | **6 weeks** | Fully migrated system |

## Success Criteria

- [ ] All Rust unit tests passing
- [ ] All Go unit tests passing
- [ ] ML predictions match Python within 5% accuracy
- [ ] All Prometheus metrics maintained
- [ ] All Grafana dashboards functional
- [ ] Performance targets met
- [ ] Documentation updated
- [ ] Python code removed

## Appendix: File Mapping

### Python → Rust

| Python File | Rust Module | Lines | Complexity |
|------------|-------------|-------|------------|
| `modeling.py` | `ml_rust/src/models.rs` | 1146 → ~600 | High |
| `cost.py` | `ml_rust/src/cost.rs` | 720 → ~300 | Medium |
| `scoring.py` | `ml_rust/src/scoring.rs` | 656 → ~250 | Medium |
| `recommender.py` | `ml_rust/src/recommender.rs` | 374 → ~200 | Low |
| `regional_pricing.py` | `ml_rust/src/regional_pricing.rs` | 365 → ~200 | Low |
| - | `ml_rust/src/ffi.rs` | 0 → ~400 | Medium |

### Python → Go

| Python File | Go Package | Lines | Complexity |
|------------|------------|-------|------------|
| `cost_exporter.py` | `master/exporters/cost_go/` | 725 → ~500 | Medium |
| `qoe_exporter.py` | `master/exporters/qoe_go/` | 260 → ~200 | Low |
| `results_exporter.py` | `master/exporters/results_go/` | 949 → ~700 | High |

Total: ~3,500 Python lines → ~3,350 Rust/Go lines (similar complexity, better performance)
