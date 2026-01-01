# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added - Dynamic Input Video Generation + Hardware-Aware Encoding

**Worker Automation & Hardware Detection:**
- Automatic hardware encoder detection (NVENC, QSV, VAAPI, software fallback)
- Dynamic test input video generation based on job parameters
- Hardware-accelerated input generation with automatic fallback to software encoding
- Automatic cleanup of generated input files with optional persistence
- New `--generate-input` flag to enable/disable automatic generation (default: enabled)
- Support for `PERSIST_INPUTS` environment variable for debugging

**Encoder Detection (`shared/pkg/agent/encoder_detector.go`):**
- Detects available FFmpeg encoders at runtime
- Prioritizes encoders: h264_nvenc → h264_qsv → h264_vaapi → libx264
- Detects hardware acceleration methods via `ffmpeg -hwaccels`
- Reports encoder capabilities to master node during registration
- Provides human-readable explanations for encoder selection

**Input Generation (`shared/pkg/agent/input_generator.go`):**
- Generates test videos with configurable resolution, framerate, duration
- Uses testsrc2 with noise filter for realistic content
- Hardware-accelerated generation using detected encoders
- Automatic fallback to libx264 if hardware encoder fails
- Safe cleanup with input_ prefix validation
- Job parameter support: resolution_width, resolution_height, frame_rate, duration_seconds

**Prometheus Metrics:**
- `ffrtmp_worker_input_generation_duration_seconds`: Time to generate input
- `ffrtmp_worker_input_file_size_bytes`: Size of generated input file
- `ffrtmp_worker_total_inputs_generated`: Counter of total inputs created

**Documentation:**
- Updated worker/README.md with hardware-aware features section
- Created worker/EXAMPLES.md with comprehensive usage examples
- Added troubleshooting guide for encoder detection and input generation
- Created IMPLEMENTATION_SUMMARY.md with detailed feature documentation

**Testing:**
- Full test coverage for encoder detection and input generation
- Tests validate automatic fallback behavior
- Tests verify safety of cleanup operations
- All tests passing with both hardware and software encoders

**Benefits:**
- Workers can operate standalone without pre-existing input files
- Hardware acceleration used automatically when available
- Graceful degradation to software encoding
- Realistic test content for ML model training
- Simplified worker deployment and configuration

### Changed

**Worker Lifecycle:**
- Added encoder detection step during startup
- Added input generation before job execution
- Added cleanup step after job completion
- Enhanced logging for encoder selection and input generation

**Hardware Detection:**
- Extended `DetectHardware()` to include encoder capabilities
- Node registration now includes detected GPU capabilities
- Worker reports available encoders to master

## [Previous Releases]

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2025-12-29

### Added
- **VictoriaMetrics** as primary time-series database (30-day retention, 10x storage efficiency)
- **Go FFmpeg Stats Exporter** for real-time encoding metrics (encoder load, dropped frames, bitrate, latency)
- **ARM64 support** in CI/CD pipeline (multi-arch Docker builds for linux/amd64 and linux/arm64)
- **Performance Benchmark Automation** with 4 workload scenarios (Laptop, Desktop, Single-GPU, Dual-GPU)
- **Benchmark History Dashboard** in Grafana for tracking performance over time
- `make run-benchmarks` command for automated benchmark execution
- `scripts/run_benchmarks.sh` for benchmark workflow automation

### Changed
- **BREAKING**: Switched from Prometheus to VictoriaMetrics as default datasource
- Updated all Grafana dashboards to use VictoriaMetrics queries
- VictoriaMetrics now has 1-second scrape granularity (vs 5-second for Prometheus)
- Retention period increased from 7 days to 30 days
- Go exporters now handle all CPU and GPU power monitoring

### Removed
- **DEPRECATED**: Python RAPL exporter (replaced by Go CPU exporter)
- Prometheus container and related configuration files
- Legacy Python power monitoring code
- `prometheus-data` Docker volume

### Fixed
- Health endpoint consistency across all Go exporters
- Container restart resiliency for all exporters
- Zero missing metrics under high load conditions

### Performance
- 70%+ CPU reduction in exporter overhead (Go vs Python)
- 1-second metric scrape granularity with minimal jitter
- 10x storage efficiency with VictoriaMetrics compression
- Zero missing metrics during high-load scenarios

### Migration Notes
- **IMPORTANT**: Run `docker compose down -v` before upgrading to remove old Prometheus and RAPL exporter containers
- Users upgrading from 1.x should update Grafana dashboards to use VictoriaMetrics datasource
- Historical Prometheus data will need to be migrated or will be lost after upgrade
- Python RAPL exporter is no longer supported; use Go CPU exporter instead
- Update any external monitoring tools to point to VictoriaMetrics at port 8428
- Cost and results exporters now connect to VictoriaMetrics instead of Prometheus

### Security
- All exporters run with minimal privileges (except hardware-access exporters)
- Static binary builds reduce attack surface
- Health endpoints do not expose sensitive information

## [1.0.0] - Previous Release

### Initial Release
- Python-based power monitoring stack
- Prometheus for metrics storage
- Grafana dashboards for visualization
- Basic CPU and GPU exporters
