# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added - 2026-01-07

**Auto-Discovery Phase 1: Visibility and Statistics**
- Self-process filtering: watch daemon no longer discovers its own child processes
- Enhanced statistics tracking with thread-safe Stats struct
  - Total scans, discoveries, attachments counters
  - Last scan duration and timestamp tracking
  - Active attachments gauge
- Performance monitoring with scan duration logging
- Detailed scan summaries: "Scan complete: new=X tracked=Y duration=Zms"
- Comprehensive test suite: `scripts/test_discovery_comprehensive.sh` (6 tests)
- Documentation: Phase 1 test summary and results analysis

**Auto-Discovery Phase 2: Intelligence and Filtering**
- Rich process metadata collection (5 new fields):
  - UserID and Username (extracted from file ownership and /etc/passwd)
  - ParentPID (from /proc/[pid]/stat field 4)
  - WorkingDir (from /proc/[pid]/cwd symlink)
  - ProcessAge (calculated from start time)
- Advanced filtering system with 6 filter types:
  - User-based filtering (whitelist/blacklist by username or UID)
  - Parent PID filtering (allow/block based on parent process)
  - Runtime-based filtering (min/max process age constraints)
  - Working directory filtering (allow/block by path)
  - Per-command filter overrides
  - Composable filters (all must pass for discovery)
- YAML configuration file support:
  - `--watch-config` flag for declarative policy management
  - Per-command resource limit overrides
  - Per-command filter rule overrides
  - Duration parsing (10s, 1m, 24h formats)
  - Validation at load time with helpful error messages
- Example configuration file: `examples/watch-config.yaml`
- Test suite for metadata extraction: `scripts/test_phase2_metadata.sh` (5 tests)
- Comprehensive documentation:
  - `docs/AUTO_DISCOVERY_PHASE2_SUMMARY.md` - Complete Phase 2 guide
  - `docs/AUTO_DISCOVERY_ENHANCEMENTS.md` - Enhancement roadmap
  - Updated `docs/AUTO_ATTACH.md` with all new features

### Changed - 2026-01-07
- Scanner now filters out watch daemon's own PID and children
- Process struct enhanced with metadata fields (backwards compatible)
- Watch daemon logging now includes scan statistics
- AUTO_ATTACH.md documentation significantly expanded with Phase 1 and 2 details

### Performance - 2026-01-07
- Scan performance: Sub-25ms for 6 processes (Phase 1 + Phase 2 combined)
- Metadata extraction adds ~5ms overhead per 6 processes
- Memory footprint: +64 bytes per Process struct (192 bytes total)
- Statistics tracking: Negligible overhead with RWMutex

### Security - 2026-01-07
- User-based filtering enables multi-tenant security
- Directory filtering prevents discovery in sensitive paths
- UID blacklisting (e.g., block root processes)
- Declarative policies reduce ad-hoc privilege escalation

**Auto-Discovery Phase 3: Reliability Features (Complete)**

#### Phase 3.1: State Persistence
- JSON-based state files with atomic writes (write to temp, then rename)
- Periodic flushing with configurable interval (default: 30s)
- Stale PID cleanup on startup (verifies process existence)
- Statistics preservation across restarts (total scans, discoveries, attachments)
- Per-process state tracking (PID, job ID, command, discovery/attachment timestamps)
- CLI flags: `--enable-state`, `--state-path`, `--state-flush-interval`
- Optional fsync for durability (disabled by default for performance)

#### Phase 3.2: Error Handling and Classification
- Error classification system with 5 types: Transient, Permanent, RateLimit, Resource, Unknown
- `DiscoveryError` struct with operation context, PID, timestamp, and retryable flag
- Pattern-based `ErrorClassifier` for intelligent error categorization
- `ErrorMetrics` tracking errors by type and consecutive failures
- Exponential backoff strategy (initial: 1s, max: 5min, multiplier: 2.0)

#### Phase 3.3: Retry Queue System
- Automatic retry mechanism for failed attachments with exponential backoff
- Configurable maximum retry attempts (default: 3)
- Background retry worker checking every 5 seconds for ready items
- Dead letter handling for items exceeding max attempts
- Per-item tracking: attempt count, last/next attempt time, error history
- CLI flags: `--enable-retry`, `--max-retry-attempts`

#### Phase 3.4: Health Check System
- Three health states: Healthy, Degraded, Unhealthy
- Separate tracking for scan health and attachment health
- Automatic status updates based on configurable thresholds:
  - Max 5 consecutive scan failures before unhealthy
  - Max 10 consecutive attachment failures before degraded
  - Max 2 minutes since last successful scan before unhealthy
- Detailed health reports with metrics (consecutive failures, timestamps, status duration)
- Automatic console logging when health degrades
- `GetHealthStatus()` and `GetHealthReport()` API methods

#### Integration and Testing
- Integrated error handling into `scanAndAttach()` and `attachToProcess()`
- Retry worker automatically started when enabled
- Health status logged on scan/attachment failures
- Comprehensive test suite: `scripts/test_phase3_reliability.sh` (6 tests)
- All reliability features validated and passing

### Performance - Phase 3 (2026-01-07)
- State persistence: Minimal overhead with periodic flushing
- Error classification: Sub-millisecond pattern matching
- Retry queue: Background worker with 5-second intervals
- Health checks: Lock-free read operations for status queries
- Typical state file size: 1-2KB for 3-4 processes

### Security - 2026-01-07
- User-based filtering enables multi-tenant security
- Directory filtering prevents discovery in sensitive paths
- UID blacklisting (e.g., block root processes)
- Declarative policies reduce ad-hoc privilege escalation

## [Previous Releases]

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
