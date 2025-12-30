# Shared Components

This directory contains components used by **both master and worker nodes** in the distributed system.

## Purpose

Shared components provide:
- **Common data models**: Ensure API compatibility between master and workers
- **Shared libraries**: Reusable Go packages for authentication, storage, etc.
- **Utilities and scripts**: Tools that can be run from any node
- **ML models**: Energy efficiency scoring models
- **Documentation**: Project-wide documentation

## Directory Structure

```
shared/
├── pkg/                       # Shared Go packages
│   ├── api/                   # HTTP API handlers and definitions
│   ├── models/                # Data models (Job, Node, Result, etc.)
│   ├── auth/                  # Authentication middleware
│   ├── agent/                 # Agent logic (hardware detection, execution)
│   ├── store/                 # Storage interfaces (SQLite, in-memory)
│   ├── tls/                   # TLS utilities for secure connections
│   ├── metrics/               # Prometheus metrics utilities
│   └── logging/               # Structured logging
├── scripts/                   # Utility scripts
│   ├── run_tests.py           # Test runner
│   ├── analyze_results.py     # Results analyzer
│   ├── run_benchmarks.sh      # Benchmark automation
│   └── retrain_models.py      # ML model retraining
├── advisor/                   # Energy efficiency advisor
│   └── quality/               # ML models for efficiency scoring
├── models/                    # Trained ML model files
│   └── x86_64_linux/          # Platform-specific models
└── docs/                      # Project documentation
    ├── architecture.md
    ├── distributed_architecture_v1.md
    ├── DEPLOYMENT_MODES.md
    └── ...
```

## Shared Go Packages

These packages are imported by both `cmd/master` and `cmd/agent`:

### `pkg/api/`
HTTP API request/response handlers used by master

**Example**:
```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/api"

router := api.NewRouter(store, jobQueue)
```

### `pkg/models/`
Core data structures for distributed system

**Key models**:
- `Node`: Represents a registered worker node
- `Job`: Transcoding job with parameters
- `Result`: Job execution results
- `HardwareCapabilities`: CPU/GPU/RAM specs

**Example**:
```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/models"

node := &models.Node{
    ID: uuid.New().String(),
    Address: "worker-01",
    Type: "desktop",
    CPUThreads: 8,
    HasGPU: false,
}
```

### `pkg/auth/`
Authentication middleware for API endpoints

**Features**:
- API key validation
- Bearer token support
- Environment variable configuration

**Example**:
```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/auth"

// On master
router.Use(auth.RequireAPIKey(apiKey))

// On agent
client := &http.Client{}
req.Header.Set("Authorization", "Bearer "+apiKey)
```

### `pkg/agent/`
Agent-specific logic used by workers

**Functions**:
- `DetectHardware()`: Auto-detect CPU, GPU, RAM
- `ExecuteJob()`: Run FFmpeg transcoding
- `AnalyzeResults()`: Calculate efficiency scores

**Example**:
```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/agent"

caps, err := agent.DetectHardware()
result, err := agent.ExecuteJob(job, caps)
```

### `pkg/store/`
Storage abstractions for master node

**Implementations**:
- `MemoryStore`: In-memory storage (development)
- `SQLiteStore`: SQLite persistence (production)

**Example**:
```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/store"

store, err := store.NewSQLiteStore("master.db")
store.SaveJob(job)
```

### `pkg/tls/`
TLS utilities for secure connections

**Functions**:
- `GenerateSelfSignedCert()`: Auto-generate TLS certificates
- `LoadTLSConfig()`: Load certificate files
- `NewTLSClient()`: Create TLS-enabled HTTP client

**Example**:
```go
import tlsutil "github.com/psantana5/ffmpeg-rtmp/pkg/tls"

// Generate cert on master
cert, key, err := tlsutil.GenerateSelfSignedCert()

// Load cert on agent
tlsConfig, err := tlsutil.LoadTLSConfig(certFile, keyFile, caFile)
```

### `pkg/metrics/`
Prometheus metrics registration and collection

**Example**:
```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/metrics"

metrics.JobsTotal.Inc()
metrics.JobDuration.Observe(duration.Seconds())
```

### `pkg/logging/`
Structured logging utilities

**Example**:
```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/logging"

log := logging.New("master")
log.Info("Server started", "port", 8080)
log.Error("Connection failed", "error", err)
```

## Shared Scripts

### `scripts/run_tests.py`
Run transcoding tests with various configurations

**Usage**:
```bash
# Single test
python3 scripts/run_tests.py single --name test1 --bitrate 2000k --duration 60

# Batch tests
python3 scripts/run_tests.py batch --file batch_stress_matrix.json
```

**Use cases**:
- Run from master node to test overall system
- Run from worker node for local testing
- Run from developer machine for feature testing

### `scripts/analyze_results.py`
Analyze test results and generate efficiency rankings

**Usage**:
```bash
python3 scripts/analyze_results.py
```

**Output**:
- CSV export of results
- Efficiency rankings
- Recommendations for optimal settings

### `scripts/run_benchmarks.sh`
Automated benchmark suite

**Usage**:
```bash
bash scripts/run_benchmarks.sh
```

**Runs**:
- Baseline tests
- Multi-bitrate tests
- Codec comparisons
- Resolution tests

### `scripts/retrain_models.py`
Retrain ML models from test results

**Usage**:
```bash
python3 scripts/retrain_models.py --results-dir ./test_results --models-dir ./models
```

## Energy Efficiency Advisor

### `advisor/`
ML-based energy efficiency scoring

**Components**:
- Feature extraction from results
- Random Forest models for scoring
- Recommendation engine

**Used by**:
- Workers: Score job results locally
- Master: Aggregate scores for visualization
- Scripts: Analyze and compare results

**Models**:
- `models/x86_64_linux/`: Pre-trained models for x86_64 Linux
- Training data: Historical test results

## Documentation

### `docs/`
All project documentation in one place

**Key documents**:
- `architecture.md`: System architecture overview
- `distributed_architecture_v1.md`: Distributed mode details
- `DEPLOYMENT_MODES.md`: Production vs development
- `PRODUCTION_FEATURES.md`: Production-ready features
- `getting-started.md`: Setup walkthrough
- `troubleshooting.md`: Common issues

**Why shared?**:
- Documentation applies to both master and workers
- Developers need full context regardless of component
- Centralized documentation is easier to maintain

## Using Shared Components

### In Master Binary

```go
// cmd/master/main.go
import (
    "github.com/psantana5/ffmpeg-rtmp/pkg/api"
    "github.com/psantana5/ffmpeg-rtmp/pkg/models"
    "github.com/psantana5/ffmpeg-rtmp/pkg/store"
    "github.com/psantana5/ffmpeg-rtmp/pkg/auth"
)

func main() {
    store := store.NewMemoryStore()
    router := api.NewRouter(store)
    router.Use(auth.RequireAPIKey(apiKey))
    // ...
}
```

### In Agent Binary

```go
// cmd/agent/main.go
import (
    "github.com/psantana5/ffmpeg-rtmp/pkg/agent"
    "github.com/psantana5/ffmpeg-rtmp/pkg/models"
    tlsutil "github.com/psantana5/ffmpeg-rtmp/pkg/tls"
)

func main() {
    caps, _ := agent.DetectHardware()
    node := &models.Node{ /* ... */ }
    tlsConfig, _ := tlsutil.LoadTLSConfig(cert, key, ca)
    // ...
}
```

### In Scripts

```python
# scripts/analyze_results.py
from shared.advisor.quality.efficiency import score_efficiency

result = load_result("test_results/test1.json")
score = score_efficiency(result)
```

## Version Compatibility

**Important**: Master and worker binaries must use **compatible versions** of shared packages.

**Best practices**:
1. **Tag releases**: Use semantic versioning for releases
2. **Test compatibility**: Run integration tests after updating shared code
3. **Document breaking changes**: Clearly mark API changes in CHANGELOG
4. **Gradual rollout**: Update master first, then workers

**Breaking change example**:
```go
// Old API (v1.0)
type Job struct {
    Scenario string
}

// New API (v2.0) - BREAKING
type Job struct {
    Scenario   string
    Parameters map[string]interface{} // NEW REQUIRED FIELD
}
```

**Solution**: Add field with backward compatibility
```go
// Better (v1.1) - Non-breaking
type Job struct {
    Scenario   string
    Parameters map[string]interface{} `json:"parameters,omitempty"` // Optional
}
```

## Testing Shared Code

```bash
# Test shared Go packages
cd shared/pkg
go test ./...

# Test shared Python scripts
cd shared/scripts
python3 -m pytest

# Test advisor models
cd shared/advisor
python3 -m pytest
```

## Related Documentation

- [FOLDER_ORGANIZATION.md](../FOLDER_ORGANIZATION.md) - Overall project structure
- [../master/README.md](../master/README.md) - Master node components
- [../worker/README.md](../worker/README.md) - Worker node components
- [docs/DEPLOYMENT_MODES.md](docs/DEPLOYMENT_MODES.md) - Deployment guide

## Contributing to Shared Code

When modifying shared components:

1. **Consider both master and worker**: Changes affect both
2. **Maintain backward compatibility**: Avoid breaking changes when possible
3. **Update tests**: Test both master and worker usage
4. **Document changes**: Update this README and relevant docs
5. **Version appropriately**: Bump version if API changes

## Support

For issues with shared components:
- Check if issue is master-specific or worker-specific first
- Include which component (master/worker) is using the shared code
- Provide version info: `git log -1 --oneline shared/pkg`
