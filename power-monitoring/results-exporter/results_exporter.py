#!/usr/bin/env python3

import json
import os
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from statistics import mean
from urllib.parse import urlencode
from urllib.request import urlopen, Request


def _escape_label_value(value: str) -> str:
    return value.replace("\\", "\\\\").replace('"', '\\"')


class PrometheusClient:
    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")

    def query(self, query: str, ts: float | None = None):
        params = {"query": query}
        if ts is not None:
            params["time"] = int(ts)
        url = f"{self.base_url}/api/v1/query?{urlencode(params)}"
        req = Request(url, headers={"Accept": "application/json"})
        try:
            with urlopen(req, timeout=30) as resp:
                return json.load(resp)
        except Exception:
            return None

    def query_range(self, query: str, start: float, end: float, step: str = "15s"):
        params = {
            "query": query,
            "start": int(start),
            "end": int(end),
            "step": step,
        }
        url = f"{self.base_url}/api/v1/query_range?{urlencode(params)}"
        req = Request(url, headers={"Accept": "application/json"})
        try:
            with urlopen(req, timeout=30) as resp:
                return json.load(resp)
        except Exception:
            return None


def _extract_values(query_range_response) -> list[float]:
    if not query_range_response:
        return []
    data = query_range_response.get("data", {})
    results = data.get("result", [])
    values: list[float] = []
    for result in results:
        for ts, val in result.get("values", []):
            try:
                values.append(float(val))
            except Exception:
                continue
    return values


def _extract_instant_value(query_response) -> float:
    if not query_response:
        return 0.0
    data = query_response.get("data", {})
    results = data.get("result", [])
    if not results:
        return 0.0
    value = results[0].get("value")
    if not value or len(value) < 2:
        return 0.0
    try:
        return float(value[1])
    except Exception:
        return 0.0


class ResultsExporter:
    def __init__(self):
        self.results_dir = Path(os.getenv("RESULTS_DIR", "/results"))
        self.prometheus_url = os.getenv("PROMETHEUS_URL", "http://prometheus:9090")
        self.cache_seconds = int(os.getenv("RESULTS_EXPORTER_CACHE_SECONDS", "15"))
        self.client = PrometheusClient(self.prometheus_url)

        self._last_refresh = 0.0
        self._cached_metrics = ""
        self._cached_run_id = ""

    def _latest_results_file(self) -> Path | None:
        if not self.results_dir.exists():
            return None
        files = sorted(self.results_dir.glob("test_results_*.json"), reverse=True)
        return files[0] if files else None

    def _load_results(self, path: Path) -> dict:
        with path.open() as f:
            return json.load(f)

    def _find_baseline(self, scenarios: list[dict]) -> dict | None:
        for s in scenarios:
            name = str(s.get("name", ""))
            if "baseline" in name.lower():
                return s
        return None

    def _scenario_labels(self, scenario: dict) -> dict:
        return {
            "scenario": str(scenario.get("name", "")),
            "bitrate": str(scenario.get("bitrate", "")),
            "resolution": str(scenario.get("resolution", "")),
            "fps": str(scenario.get("fps", "")),
        }

    def _labels_str(self, labels: dict) -> str:
        parts = [f'{k}="{_escape_label_value(str(v))}"' for k, v in labels.items()]
        return "{" + ",".join(parts) + "}"

    def _compute_scenario_stats(self, start: float, end: float) -> dict:
        duration = max(0.0, float(end) - float(start))

        energy_j = 0.0
        mean_power = 0.0
        if duration > 0:
            window = f"{max(1, int(duration))}s"
            energy_query = f'sum(increase(rapl_energy_joules_total{{zone=~"package.*"}}[{window}]))'
            energy_data = self.client.query(energy_query, ts=end)
            energy_j = _extract_instant_value(energy_data)
            mean_power = (energy_j / duration) if duration > 0 else 0.0

        container_cpu_query = "docker_containers_total_cpu_percent"
        container_data = self.client.query_range(container_cpu_query, start, end)
        container_values = _extract_values(container_data)
        mean_container_cpu = mean(container_values) if container_values else 0.0

        total_energy_wh = (energy_j / 3600) if duration > 0 else 0.0

        return {
            "duration_s": duration,
            "mean_power_w": mean_power,
            "total_energy_j": energy_j,
            "total_energy_wh": total_energy_wh,
            "container_cpu_percent": mean_container_cpu,
        }

    def build_metrics(self) -> str:
        now = time.time()
        latest = self._latest_results_file()
        if not latest:
            return (
                "# HELP results_exporter_up Results exporter is running\n"
                "# TYPE results_exporter_up gauge\n"
                "results_exporter_up 1\n"
            )

        run_id = latest.stem
        if (now - self._last_refresh) < self.cache_seconds and run_id == self._cached_run_id:
            return self._cached_metrics

        try:
            data = self._load_results(latest)
            scenarios = data.get("scenarios", [])
        except Exception:
            return (
                "# HELP results_exporter_up Results exporter is running\n"
                "# TYPE results_exporter_up gauge\n"
                "results_exporter_up 0\n"
            )

        output: list[str] = []
        output.append("# HELP results_exporter_up Results exporter is running")
        output.append("# TYPE results_exporter_up gauge")
        output.append("results_exporter_up 1")

        output.append("# HELP results_scenario_duration_seconds Scenario duration in seconds")
        output.append("# TYPE results_scenario_duration_seconds gauge")
        output.append("# HELP results_scenario_mean_power_watts Mean CPU package power (W) during scenario")
        output.append("# TYPE results_scenario_mean_power_watts gauge")
        output.append("# HELP results_scenario_total_energy_joules Total energy (J) during scenario (from rapl_energy_joules_total)")
        output.append("# TYPE results_scenario_total_energy_joules gauge")
        output.append("# HELP results_scenario_total_energy_wh Total energy (Wh) during scenario (derived from mean power)")
        output.append("# TYPE results_scenario_total_energy_wh gauge")
        output.append("# HELP results_scenario_container_cpu_percent Mean total container CPU percent during scenario")
        output.append("# TYPE results_scenario_container_cpu_percent gauge")
        output.append("# HELP results_scenario_net_power_watts Mean power above baseline (W)")
        output.append("# TYPE results_scenario_net_power_watts gauge")
        output.append("# HELP results_scenario_net_energy_wh Energy above baseline (Wh)")
        output.append("# TYPE results_scenario_net_energy_wh gauge")
        output.append("# HELP results_scenario_net_container_cpu_percent Container CPU percent above baseline")
        output.append("# TYPE results_scenario_net_container_cpu_percent gauge")
        output.append("# HELP results_scenario_delta_power_watts Mean power delta vs baseline (W)")
        output.append("# TYPE results_scenario_delta_power_watts gauge")
        output.append("# HELP results_scenario_delta_energy_wh Energy delta vs baseline (Wh)")
        output.append("# TYPE results_scenario_delta_energy_wh gauge")
        output.append("# HELP results_scenario_power_pct_increase Power percent increase vs baseline")
        output.append("# TYPE results_scenario_power_pct_increase gauge")

        baseline = self._find_baseline(scenarios)
        baseline_stats = None
        if baseline and baseline.get("start_time") and baseline.get("end_time"):
            baseline_stats = self._compute_scenario_stats(baseline["start_time"], baseline["end_time"])

        for scenario in scenarios:
            start = scenario.get("start_time")
            end = scenario.get("end_time")
            if not start or not end:
                continue

            stats = self._compute_scenario_stats(start, end)
            labels = {"run_id": run_id}
            labels.update(self._scenario_labels(scenario))
            lbl = self._labels_str(labels)

            output.append(f"results_scenario_duration_seconds{lbl} {stats['duration_s']:.3f}")
            output.append(f"results_scenario_mean_power_watts{lbl} {stats['mean_power_w']:.4f}")
            output.append(f"results_scenario_total_energy_joules{lbl} {stats['total_energy_j']:.4f}")
            output.append(f"results_scenario_total_energy_wh{lbl} {stats['total_energy_wh']:.6f}")
            output.append(
                f"results_scenario_container_cpu_percent{lbl} {stats['container_cpu_percent']:.4f}"
            )

            if baseline_stats and scenario is not baseline:
                d_power = stats["mean_power_w"] - baseline_stats["mean_power_w"]
                d_energy = stats["total_energy_wh"] - baseline_stats["total_energy_wh"]
                d_cpu = stats["container_cpu_percent"] - baseline_stats["container_cpu_percent"]
                pct = 0.0
                if baseline_stats["mean_power_w"] > 0:
                    pct = (d_power / baseline_stats["mean_power_w"]) * 100
                output.append(f"results_scenario_delta_power_watts{lbl} {d_power:.4f}")
                output.append(f"results_scenario_delta_energy_wh{lbl} {d_energy:.6f}")
                output.append(f"results_scenario_power_pct_increase{lbl} {pct:.4f}")
                output.append(f"results_scenario_net_power_watts{lbl} {d_power:.4f}")
                output.append(f"results_scenario_net_energy_wh{lbl} {d_energy:.6f}")
                output.append(f"results_scenario_net_container_cpu_percent{lbl} {d_cpu:.4f}")
            else:
                output.append(f"results_scenario_delta_power_watts{lbl} 0")
                output.append(f"results_scenario_delta_energy_wh{lbl} 0")
                output.append(f"results_scenario_power_pct_increase{lbl} 0")
                output.append(f"results_scenario_net_power_watts{lbl} 0")
                output.append(f"results_scenario_net_energy_wh{lbl} 0")
                output.append(f"results_scenario_net_container_cpu_percent{lbl} 0")

        metrics = "\n".join(output) + "\n"
        self._cached_metrics = metrics
        self._cached_run_id = run_id
        self._last_refresh = now
        return metrics


class Handler(BaseHTTPRequestHandler):
    exporter: ResultsExporter | None = None

    def do_GET(self):
        if self.path == "/metrics":
            payload = self.exporter.build_metrics()
            self.send_response(200)
            self.send_header("Content-type", "text/plain; charset=utf-8")
            self.end_headers()
            self.wfile.write(payload.encode("utf-8"))
            return

        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-type", "text/plain; charset=utf-8")
            self.end_headers()
            self.wfile.write(b"OK\n")
            return

        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        pass


def main():
    port = int(os.getenv("RESULTS_EXPORTER_PORT", "9502"))
    exporter = ResultsExporter()

    Handler.exporter = exporter
    server = HTTPServer(("0.0.0.0", port), Handler)
    print(f"Results exporter listening on :{port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
