# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- GitHub Actions CI workflows for linting, testing, Docker builds, and PromQL validation
- Dependabot configuration for automated dependency updates
- Issue and pull request templates
- Security policy (SECURITY.md)
- Contributing guidelines (CONTRIBUTING.md)
- Pre-commit hooks configuration
- Project badges in README
- Enhanced .gitignore with comprehensive patterns
- Coverage configuration (.coveragerc)
- Enhanced pyproject.toml with project metadata

### Changed
- Improved README with workflow status badges

### Fixed

### Removed

## [0.1.0] - Initial Release

### Added
- Core power monitoring stack with Prometheus and Grafana
- RAPL exporter for Intel CPU power monitoring
- Docker stats exporter for container overhead analysis
- QoE exporter for video quality metrics
- Cost exporter for energy cost calculations
- Results exporter for test analysis
- Health checker for exporter monitoring
- ML-based energy advisor with efficiency scoring
- Test runner with single, multi-stream, and batch modes
- Grafana dashboards for power monitoring, cost analysis, and efficiency forecasting
- Prometheus alerting rules
- Comprehensive documentation
- Python test suite with pytest

[Unreleased]: https://github.com/psantana5/ffmpeg-rtmp/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/psantana5/ffmpeg-rtmp/releases/tag/v0.1.0
