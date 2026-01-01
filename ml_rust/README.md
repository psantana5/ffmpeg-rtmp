# Rust ML Library for FFmpeg Power Prediction

This directory will contain the Rust implementation of the ML and cost optimization library.

## Status: TODO

Implementation pending. See `../docs/ML_MIGRATION_PLAN.md` for details.

## Structure

```
ml_rust/
├── Cargo.toml          # Rust project configuration
├── src/
│   ├── lib.rs          # Library entry point
│   ├── models.rs       # ML models (linear, polynomial, multivariate)
│   ├── cost.rs         # Cost modeling
│   ├── scoring.rs      # Energy efficiency scoring
│   ├── recommender.rs  # Configuration recommendations
│   ├── regional_pricing.rs  # Regional pricing and CO₂
│   └── ffi.rs          # C FFI for Go integration
└── tests/
    └── integration_tests.rs

## Implementation Plan

1. Implement core regression models using ordinary least squares
2. Add cost modeling with trapezoidal integration
3. Implement efficiency scoring algorithms
4. Create FFI bindings for Go (cgo) integration
5. Write comprehensive unit and integration tests
6. Benchmark against Python implementation

## Dependencies

- `serde` - Serialization/deserialization
- `serde_json` - JSON support
- `ndarray` - Optional, for matrix operations

## Building

```bash
cargo build --release
```

## Testing

```bash
cargo test
cargo test --release  # With optimizations
```

## FFI Usage from Go

```go
// #cgo LDFLAGS: -L./ml_rust/target/release -lffmpeg_ml
// #include "ml_rust/src/ffi.h"
import "C"

// Create predictor
predictor := C.linear_predictor_new()
defer C.linear_predictor_free(predictor)

// Train
streams := []float64{1.0, 2.0, 3.0}
power := []float64{50.0, 75.0, 100.0}
C.linear_predictor_fit(predictor, &streams[0], &power[0], C.ulong(len(streams)))

// Predict
var result C.double
C.linear_predictor_predict(predictor, 4.0, &result)
```
