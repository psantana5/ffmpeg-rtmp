# Changelog

All notable changes to this project will be documented in this file.

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
- Users upgrading from 1.x should update Grafana dashboards to use VictoriaMetrics datasource
- Historical Prometheus data will need to be migrated or will be lost after upgrade
- Python RAPL exporter is no longer supported; use Go CPU exporter instead
- Update any external monitoring tools to point to VictoriaMetrics at port 8428

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
