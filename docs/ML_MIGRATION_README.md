# ML and Cost Optimization Migration

This directory will contain the new implementation of ML and cost optimization components in Go (exporters) and Rust (ML library).

## Status: Planning Complete ✓

The migration plan has been fully documented in `docs/ML_MIGRATION_PLAN.md`.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Master Node (Go)                        │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐   │
│  │Cost Exporter │   │ QoE Exporter │   │Results Export│   │
│  │   (Go)       │   │    (Go)      │   │   er (Go)    │   │
│  └──────┬───────┘   └──────┬───────┘   └──────┬───────┘   │
│         │                   │                   │            │
│         └───────────────────┴───────────────────┘            │
│                             │                                 │
│                             ▼                                 │
│                  ┌──────────────────────┐                   │
│                  │   Rust ML Library    │                   │
│                  │   (via FFI/cgo)      │                   │
│                  │─────────────────────│                   │
│                  │ - Linear Regression  │                   │
│                  │ - Polynomial Reg.    │                   │
│                  │ - Multivariate ML    │                   │
│                  │ - Cost Modeling      │                   │
│                  │ - Efficiency Scoring │                   │
│                  │ - Regional Pricing   │                   │
│                  └──────────────────────┘                   │
│                                                               │
│                             │                                 │
│                             ▼                                 │
│                    VictoriaMetrics                           │
│                    (Prometheus Metrics)                       │
└─────────────────────────────────────────────────────────────┘
                             │
                             ▼
                        Grafana
                      (Dashboards)
```

## Migration Benefits

### Performance Improvements
- **5x faster** metric generation (<10ms vs ~50ms)
- **5x lower** memory usage (<20MB vs ~100MB)
- **4x lower** CPU usage in idle state
- **5x faster** ML training and predictions

### Operational Benefits
- **Better resource utilization** - Lower overhead on master node
- **Improved scalability** - Can handle more concurrent exporters
- **Native Go integration** - Consistent with existing infrastructure
- **Type safety** - Compile-time checks prevent runtime errors
- **Easier deployment** - Single binary, no Python runtime needed

### Development Benefits
- **Unified codebase** - All critical components in Go/Rust
- **Better tooling** - Go and Rust have excellent development tools
- **Performance profiling** - Built-in profilers for optimization
- **Memory safety** - Rust prevents common bugs

## Current Python Implementation

### Exporters (Master Node)
- **cost_exporter.py** (725 lines) - Cost metrics with regional pricing
- **qoe_exporter.py** (260 lines) - Quality of Experience metrics
- **results_exporter.py** (949 lines) - Results aggregation and ML predictions

### ML/Advisor Library (shared/advisor/)
- **modeling.py** (1,146 lines) - Power prediction models
- **cost.py** (720 lines) - Cost modeling
- **scoring.py** (656 lines) - Energy efficiency scoring
- **recommender.py** (374 lines) - Configuration recommendations
- **regional_pricing.py** (365 lines) - Regional pricing and CO₂

**Total:** ~4,200 lines of Python code

## Target Implementation

### Rust ML Library (ml_rust/)
- **models.rs** (~600 lines) - Linear, polynomial, multivariate regression
- **cost.rs** (~300 lines) - Cost calculations
- **scoring.rs** (~250 lines) - Efficiency algorithms
- **recommender.rs** (~200 lines) - Configuration ranking
- **regional_pricing.rs** (~200 lines) - Pricing and CO₂
- **ffi.rs** (~400 lines) - C FFI for Go integration

**Subtotal:** ~1,950 lines of Rust code

### Go Exporters (master/exporters/)
- **cost_go/** (~500 lines) - Cost exporter
- **qoe_go/** (~200 lines) - QoE exporter
- **results_go/** (~700 lines) - Results exporter

**Subtotal:** ~1,400 lines of Go code

**Total:** ~3,350 lines (comparable to Python, but more performant)

## Implementation Plan

See `docs/ML_MIGRATION_PLAN.md` for the complete implementation plan including:

- Detailed architecture design
- API specifications
- FFI layer design
- Integration testing strategy
- Deployment rollout plan
- Risk mitigation strategies
- 6-week timeline

## Next Steps

1. **Phase 1: Rust ML Library** (2 weeks)
   - Implement core regression models
   - Add cost modeling and scoring
   - Create FFI bindings for Go
   - Write comprehensive tests

2. **Phase 2: Go Exporters** (2 weeks)
   - Implement cost exporter
   - Implement QoE exporter
   - Implement results exporter
   - Integrate Rust library via cgo

3. **Phase 3: Integration & Testing** (1 week)
   - Side-by-side validation with Python
   - Grafana dashboard testing
   - Performance benchmarking

4. **Phase 4: Deployment** (1 week)
   - Parallel deployment
   - Gradual cutover
   - Cleanup Python code
   - Update documentation

## Dependencies

### Rust Dependencies
```toml
[dependencies]
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
ndarray = "0.16"  # Optional, for matrix operations

[dev-dependencies]
approx = "0.5"
```

### Go Dependencies
```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)
```

## Testing Strategy

### Unit Tests
- Rust: `cd ml_rust && cargo test`
- Go: `cd master/exporters/cost_go && go test ./...`

### Integration Tests
- Compare metric outputs between Python and Go
- Validate ML predictions match within 5% accuracy
- Test Grafana dashboard compatibility

### Performance Tests
- Benchmark memory usage
- Benchmark CPU usage
- Measure scrape latency
- Profile ML prediction time

## Success Criteria

- [ ] All Rust unit tests passing
- [ ] All Go unit tests passing
- [ ] ML predictions within 5% of Python accuracy
- [ ] All Prometheus metrics maintained
- [ ] All Grafana dashboards functional
- [ ] Performance targets met (5x improvement)
- [ ] Documentation complete
- [ ] Python code removed

## References

- **Architecture**: `docs/INTERNAL_ARCHITECTURE.md`
- **Migration Plan**: `docs/ML_MIGRATION_PLAN.md`
- **Current Exporters**: `master/exporters/README.md`
- **Current Advisor**: `shared/advisor/README.md`
