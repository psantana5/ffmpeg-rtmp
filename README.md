# Power Monitoring (FFmpeg + Nginx-RTMP) – Energy, Performance, Observability

This project is a self-contained **streaming test + power monitoring stack**.

You can:

- Run **reproducible FFmpeg streaming scenarios** (single, multi-stream, mixed bitrates, batches).
- Collect **system power** via Intel RAPL and expose it to Prometheus.
- Collect **container overhead** (Docker engine and container CPU usage).
- Collect **host and container metrics** (node_exporter + cAdvisor).
- Optionally collect **NVIDIA GPU power/utilization** via DCGM exporter.
- Automatically generate **baseline vs test comparisons** and visualize them in Grafana.
- Trigger **alerting** for power thresholds via Prometheus rules + Alertmanager.

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

### Console + CSV

Analyze the latest run:

- `python3 analyze_results.py`

This prints a summary and exports a CSV next to the results file.

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

- `make lint`
- `make format`
- `make test`

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

---

## Project layout

- `docker-compose.yml` – stack definition
- `prometheus.yml` – scrape jobs, alertmanager integration
- `prometheus-alerts.yml` – alert rules
- `grafana/provisioning/` – dashboards and datasource provisioning
- `run_tests.py` – test runner / scenario CLI
- `analyze_results.py` – analysis and CSV export
- `results-exporter/` – Prometheus exporter producing baseline-vs-test summary metrics
- `rapl-exporter/` – Intel RAPL exporter
- `docker-stats-exporter/` – Docker overhead exporter
- `test_results/` – output directory for test runs

---

## Next improvements

- Add a smaller "quick" batch for CI.
- Add Parquet/JSON exports for analysis workflows.
- Add statistical tests (t-test) across scenario groups.
- Add AMD GPU support (ROCm exporter).
