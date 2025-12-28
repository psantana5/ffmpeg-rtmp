# Power Monitoring (FFmpeg + Nginx-RTMP) – Energy, Performance, Observability

This project is a self-contained **streaming test + power monitoring stack** with **energy-aware transcoding recommendations**.

You can:

- Run **reproducible FFmpeg streaming scenarios** (single, multi-stream, mixed bitrates, batches).
- Collect **system power** via Intel RAPL and expose it to Prometheus.
- Collect **container overhead** (Docker engine and container CPU usage).
- Collect **host and container metrics** (node_exporter + cAdvisor).
- Optionally collect **NVIDIA GPU power/utilization** via DCGM exporter.
- Automatically generate **baseline vs test comparisons** and visualize them in Grafana.
- Trigger **alerting** for power thresholds via Prometheus rules + Alertmanager.
- **Get energy efficiency scores and recommendations** for optimal transcoding configurations.

---

## Architecture

### Components

- **Nginx RTMP** (`nginx-rtmp`)
  - RTMP ingest endpoint for FFmpeg.
  - Exposes a lightweight HTTP health endpoint.

- **Nginx RTMP exporter** (`nginx-exporter`)
  - Scraped by Prometheus.

- **RAPL exporter** (`rapl-exporter`)
  - Reads Intel RAPL counters from `/sys/class/powercap`.
  - Exposes:
    - `rapl_power_watts` (gauge)
    - `rapl_energy_joules_total` (counter since exporter start)

- **Docker stats exporter** (`docker-stats-exporter`)
  - Exposes Docker engine and container CPU/memory percentage.

- **node_exporter** (`node-exporter`)
  - Host CPU, memory, network, disk IO metrics.

- **cAdvisor** (`cadvisor`)
  - Container metrics (e.g. `container_memory_usage_bytes`).

- **results-exporter** (`results-exporter`)
  - Reads the latest `test_results/test_results_*.json`.
  - Queries Prometheus **per scenario time-window** and emits "scenario summary" metrics.
  - Enables Grafana dashboards to show **baseline vs test diffs** automatically.

- **Energy Advisor** (`advisor/`)
  - Scores transcoding configurations by energy efficiency.
  - Ranks scenarios by throughput-per-watt metric.
  - Recommends optimal pipeline for the hardware.
  - **Power Prediction Model**: Machine learning-based prediction of power consumption for untested workload sizes.
  - Extensible for future quality metrics (VMAF/PSNR).
  - See [Power Prediction Documentation](docs/power-prediction-model.md) for details.

- **Prometheus** (`prometheus`)
  - Scrapes all exporters.
  - Loads alert rules from `prometheus-alerts.yml`.
  - Sends alerts to Alertmanager.

- **Alertmanager** (`alertmanager`)
  - Receives alerts from Prometheus.
  - Default receiver is configured (extend to Slack/Email/etc.).

- **Grafana** (`grafana`)
  - Provisioned with datasources and dashboards.

---

## Requirements

### Required

- Docker + Docker Compose
- Python 3 (for `run_tests.py` and `analyze_results.py`)
- FFmpeg installed on the host (used by the test runner)

### Power monitoring (Intel RAPL)

- Intel CPU with RAPL enabled and available at `/sys/class/powercap`.
- The `rapl-exporter` runs privileged and mounts `/sys/class/powercap` read-only.

### Optional: NVIDIA GPU

- NVIDIA GPU + `nvidia-container-toolkit` installed.
- Start stack with:
  - `docker compose --profile nvidia up -d` or `make nvidia-up`

---

## Quick start

### 1) Start the stack

- `make up-build`

Or manually:

- `docker compose up -d --build`

### 2) Open UIs

- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9090
- Alertmanager: http://localhost:9093

### 3) Check Prometheus targets

- `make targets`

---

## Running tests

The main entrypoint is:

- `python3 run_tests.py`

This keeps backward compatibility (runs defaults if no subcommand is provided), but also provides a structured CLI.

### Single scenario

Example:

```bash
python3 run_tests.py --output-dir ./test_results single \
  --name "2Mbps_720p" \
  --bitrate 2000k --resolution 1280x720 --fps 30 \
  --duration 120 --stabilization 10 --cooldown 10
```

### Single scenario with baseline

This is the recommended flow if you want automatic baseline-vs-test dashboards:

```bash
python3 run_tests.py --output-dir ./test_results single \
  --with-baseline --baseline-duration 60 \
  --name "2Mbps" --bitrate 2000k --duration 120
```

### Multi-stream stress test

Same bitrate for all streams:

```bash
python3 run_tests.py --output-dir ./test_results multi \
  --with-baseline --baseline-duration 60 \
  --count 4 --bitrate 2500k --duration 120
```

Mixed bitrates:

```bash
python3 run_tests.py --output-dir ./test_results multi \
  --count 3 --bitrates "1000k,2500k,5M" --duration 120
```

---

## Output Ladders (Multi-Resolution Support)

The system supports **output ladders** for scenarios where you transcode to multiple resolutions simultaneously (e.g., 1080p, 720p, 480p). This is critical for adaptive bitrate streaming where clients dynamically select the best resolution for their connection.

### Defining Output Ladders

Add an `outputs` array to your scenario in the batch JSON:

```json
{
  "type": "single",
  "name": "Multi-resolution ladder test 1080p+720p+480p @ 2500k",
  "bitrate": "2500k",
  "resolution": "1920x1080",
  "fps": 30,
  "stabilization": 10,
  "duration": 120,
  "cooldown": 15,
  "outputs": [
    {"resolution": "1920x1080", "fps": 30},
    {"resolution": "1280x720", "fps": 30},
    {"resolution": "854x480", "fps": 30}
  ]
}
```

### Energy Efficiency Scoring with Output Ladders

When output ladders are present, the system uses a **pixel-based scoring algorithm**:

```
efficiency_score = total_pixels_delivered / total_energy_joules
```

Where:
- `total_pixels_delivered` = sum of (width × height × fps × duration) for each output
- `total_energy_joules` = measured energy consumption during the test

### Ladder-Aware Comparison

Scenarios are automatically **grouped by output ladder** for fair comparison:
- Only scenarios with identical output ladders are ranked against each other
- Prevents comparing single-resolution scenarios against multi-resolution ones
- Each ladder group gets its own ranking and best configuration recommendation

### Example: Comparing Codecs Across Ladders

```json
{
  "scenarios": [
    {
      "name": "H.264 - 1080p+720p+480p @ 2500k",
      "bitrate": "2500k",
      "outputs": [
        {"resolution": "1920x1080", "fps": 30},
        {"resolution": "1280x720", "fps": 30},
        {"resolution": "854x480", "fps": 30}
      ]
    },
    {
      "name": "H.264 - 1080p+720p+480p @ 5000k",
      "bitrate": "5000k",
      "outputs": [
        {"resolution": "1920x1080", "fps": 30},
        {"resolution": "1280x720", "fps": 30},
        {"resolution": "854x480", "fps": 30}
      ]
    }
  ]
}
```

The analysis will:
1. Group these scenarios together (same ladder: `1920x1080@30,1280x720@30,854x480@30`)
2. Rank them by pixels-per-joule efficiency
3. Recommend the most energy-efficient bitrate for this specific ladder

This enables answering: **"For a given set of output resolutions, what encoding settings save the most energy?"**

---

## Batch runs (stress matrix)

A ready-to-run batch template is included:

- `batch_stress_matrix.json`

Run it:

```bash
python3 run_tests.py --output-dir ./test_results batch --file batch_stress_matrix.json
```

Or via Make:

- `make test-batch`

---

## Analysis and reports

### Console + CSV with Energy Efficiency Recommendations

Analyze the latest run:

- `python3 analyze_results.py`

This command:
- Prints a comprehensive summary of all scenarios
- **Computes energy efficiency scores** for each configuration
- **Ranks scenarios** by efficiency (Mbps per watt)
- **Recommends the optimal configuration** for your hardware
- **Predicts power consumption for untested workload sizes** using machine learning
- Exports results to CSV (including efficiency scores, ranks, and predictions)

The analysis report now includes:

1. **Traditional power metrics**: Mean power, energy consumption, Docker overhead
2. **Energy efficiency rankings**: Shows which configurations deliver the most throughput per watt
3. **Recommendation**: Identifies the best configuration for energy-efficient transcoding
4. **Power scalability predictions**: ML-based predictions for 1, 2, 4, 8, 12 concurrent streams
5. **Measured vs Predicted comparison**: Validates model accuracy on training data

Example output:
```
ENERGY EFFICIENCY RANKINGS
─────────────────────────────────────────────────────────────────────
Rank   Scenario                             Efficiency          Power        Bitrate     
─────────────────────────────────────────────────────────────────────
1      4 streams @ 2500k                      0.0667 Mbps/W     150.00 W    2500k       
2      5 Mbps Stream                          0.0625 Mbps/W      80.00 W    5M          
3      2.5 Mbps Stream                        0.0417 Mbps/W      60.00 W    2500k       

RECOMMENDATION
─────────────────────────────────────────────────────────────────────
Most energy-efficient configuration: 4 streams @ 2500k
  Efficiency Score: 0.0667 Mbps/W
  Mean Power: 150.00 W
  Bitrate: 2500k

POWER SCALABILITY PREDICTIONS
══════════════════════════════════════════════════════════════════════
Model Type: LINEAR
Training Samples: 4
Stream Range: 1 - 8 streams

Predicted Power Consumption:
──────────────────────────────────────────────────────────────────────
   1 streams:    45.23 W
   2 streams:    78.45 W
   4 streams:   145.12 W
   8 streams:   278.67 W
  12 streams:   412.23 W

MEASURED vs PREDICTED COMPARISON
──────────────────────────────────────────────────────────────────────
Streams    Measured (W)    Predicted (W)   Diff (W)    
──────────────────────────────────────────────────────────────────────
1          45.00           45.23           +0.23
2          80.00           78.45           -1.55
4          150.00          145.12          -4.88
8          280.00          278.67          -1.33
```

For detailed information about the power prediction model, see [docs/power-prediction-model.md](docs/power-prediction-model.md).

### Python API

You can also use the advisor module programmatically:

```python
from advisor import TranscodingRecommender

# After analyzing scenarios
recommender = TranscodingRecommender()
ranked = recommender.analyze_and_rank(scenarios)

# Get best configuration
best = recommender.get_best_configuration(scenarios)
print(f"Best: {best['name']} - {best['efficiency_score']:.4f} Mbps/W")

# Get top 5
top_5 = recommender.get_top_n(scenarios, n=5)

# Get comprehensive summary
summary = recommender.get_recommendation_summary(scenarios)
```

### Baseline vs test dashboards (Grafana)

This project provisions two main dashboards:

- **Power Monitoring Dashboard** (`power-monitoring.json`)
  - Host power, energy, CPU/memory/network/disk, container memory, GPU power (if enabled).

- **Baseline vs Test** (`baseline-vs-test.json`)
  - Uses `results-exporter` metrics:
    - `results_scenario_delta_power_watts`
    - `results_scenario_delta_energy_wh`
    - `results_scenario_power_pct_increase`

In Grafana:

- Open **Baseline vs Test**
- Select a `run_id` (derived from the `test_results_*.json` filename)

---

## Energy-Aware Transcoding Advisor

### Overview

The advisor module (`advisor/`) transforms raw power measurements into actionable recommendations. It answers the question:

**"Which FFmpeg configuration delivers the most video quality per watt on this machine?"**

### Current Features (v0.2)

- **Energy Efficiency Scoring**: Multiple scoring algorithms
  - `throughput_per_watt`: Computes `throughput / power` for each scenario (default for single-resolution)
  - `pixels_per_joule`: Computes `total_pixels / energy_joules` (automatically used for multi-resolution ladders)
- **Output Ladder Support**: Groups and ranks scenarios by output resolution combinations
  - Throughput = bitrate (Mbps) × number of streams
  - Total pixels = sum of (width × height × fps × duration) across all outputs
  - Power = CPU watts + GPU watts (if available)
- **Automatic Ranking**: Sorts all configurations by efficiency
- **Ladder-Aware Comparison**: Only compares scenarios with identical output ladders
- **Best Configuration Selection**: Identifies optimal pipeline per ladder and overall
- **CSV Export**: Saves efficiency scores, ladder information, and pixel metrics alongside traditional metrics
- **Production-Grade**: Handles missing data, edge cases, uses measured metrics only

### Design Principles

1. **Measured metrics only**: No synthetic or estimated power values
2. **Pluggable scoring**: Easy to extend with new algorithms
3. **Hardware-agnostic**: Works on single developer machine or cloud infrastructure
4. **Future-ready**: Placeholder hooks for VMAF, multi-objective scoring, cost analysis

### Future Enhancements

The advisor is designed to be extended with:

- **Video quality integration** (VMAF/PSNR): Quality-adjusted efficiency scores
- **Multi-objective optimization**: Balance quality, energy, and cost
- **Hardware normalization**: Compare efficiency across different CPU/GPU models
- **Cost-aware scoring**: Include cloud pricing and TCO calculations
- **CLI interface**: Dedicated command for recommendations

### Module Structure

```
advisor/
├── __init__.py          # Public API
├── scoring.py           # Energy efficiency scoring algorithms
└── recommender.py       # Ranking and recommendation logic
```

---

## Alerting

### Prometheus alert rules

Alert rules are defined in:

- `prometheus-alerts.yml`

Current rules include:

- CPU package power thresholds (`rapl_power_watts`)
- GPU power thresholds (NVIDIA DCGM metric `DCGM_FI_DEV_POWER_USAGE`)

### Alertmanager

Alertmanager config is:

- `alertmanager/alertmanager.yml`

Extend receivers to Slack/email/webhook as needed.

---

## Development

### Python dependencies

- Runtime:
  - `pip install -r requirements.txt`
- Dev:
  - `pip install -r requirements-dev.txt`

### Lint / format / tests

- `make lint` - Run ruff linter
- `make format` - Auto-format code with ruff
- `make test` - Run pytest test suite (includes advisor module tests)

The test suite includes comprehensive tests for:
- Energy efficiency scoring algorithms
- Recommendation logic
- Integration with analysis pipeline

### Pre-commit

- Install hooks:
  - `pre-commit install`
- Run on all files:
  - `make pre-commit`

---

## Operational commands

Recommended: use the Makefile.

- `make up-build`
- `make down`
- `make ps`
- `make logs SERVICE=prometheus`
- `make prom-reload`

The legacy helper script `setup.sh` exists and performs basic checks and starts the stack, but the Makefile is the recommended interface.

---

## Troubleshooting

### Prometheus target is DOWN

- Check container status: `make ps`
- Check logs: `make logs SERVICE=<service>`
- Verify the target URL from Prometheus UI: http://localhost:9090/targets

### RAPL exporter has no zones

- Ensure Intel RAPL exists on the host:
  - `/sys/class/powercap/intel-rapl:0/energy_uj`
- Ensure container has permission (it runs privileged and mounts `/sys/class/powercap`).

### NVIDIA GPU metrics are DOWN

- Start with NVIDIA profile:
  - `make nvidia-up-build`
- Ensure `nvidia-container-toolkit` is installed.

### Results-exporter shows no data in Grafana

The results-exporter reads test results from the `test_results` directory and exposes them as Prometheus metrics.

**Issue**: New test result files added after container startup are not visible

**Solution**: The fix has been implemented to ensure:
- The `test_results` directory is automatically created when starting the stack with `make up` or `make up-build`
- The results-exporter container creates the directory if it doesn't exist on startup
- Files added after container startup are automatically detected on the next metrics scrape

**Verification**:
1. Check that the directory exists: `ls -la test_results/`
2. Check results-exporter logs: `make logs SERVICE=results-exporter`
3. Look for messages like:
   - `Results exporter initialized with results_dir=/results`
   - `Directory exists: True`
   - `New results file detected: test_results_YYYYMMDD_HHMMSS.json`

**Manual fix** (if needed):
- Ensure the directory exists: `mkdir -p test_results`
- Restart the results-exporter: `docker compose restart results-exporter`

---

## Project layout

- `docker-compose.yml` – stack definition
- `prometheus.yml` – scrape jobs, alertmanager integration
- `prometheus-alerts.yml` – alert rules
- `grafana/provisioning/` – dashboards and datasource provisioning
- `run_tests.py` – test runner / scenario CLI
- `analyze_results.py` – analysis, CSV export, and energy efficiency recommendations
- `advisor/` – energy-aware transcoding advisor module
  - `scoring.py` – efficiency scoring algorithms
  - `recommender.py` – ranking and recommendation logic
- `results-exporter/` – Prometheus exporter producing baseline-vs-test summary metrics
- `rapl-exporter/` – Intel RAPL exporter
- `docker-stats-exporter/` – Docker overhead exporter
- `test_results/` – output directory for test runs
- `tests/` – pytest test suite for advisor module

---

## Next improvements

- Integrate VMAF/PSNR quality metrics into energy efficiency scoring
- Add multi-objective optimization (quality vs energy vs cost)
- Hardware capability profiling and normalization
- Dedicated CLI command for energy recommendations
- Add a smaller "quick" batch for CI.
- Add Parquet/JSON exports for analysis workflows.
- Add statistical tests (t-test) across scenario groups.
- Add AMD GPU support (ROCm exporter).
