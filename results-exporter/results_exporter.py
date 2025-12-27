#!/usr/bin/env python3

import json
import os
import re
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from statistics import mean
from urllib.parse import urlencode
from urllib.request import urlopen, Request


def _escape_label_value(value: str) -> str:
    return value.replace("\\", "\\\\").replace('"', '\\"')


def _parse_bitrate_to_mbps(bitrate: str) -> float | None:
    """Parse a bitrate string like '2500k', '5M', '1000000' to Mbps."""
    if not isinstance(bitrate, str):
        return None
    s = bitrate.strip().lower()
    try:
        if s.endswith('kbps'):
            val = float(s[:-4]) / 1000.0
        elif s.endswith('k'):
            val = float(s[:-1]) / 1000.0
        elif s.endswith('mbps'):
            val = float(s[:-4])
        elif s.endswith('m'):
            val = float(s[:-1])
        elif s.endswith('bps'):
            # raw bits per second
            val = float(s[:-3]) / 1_000_000.0
        else:
            # assume kbps if reasonably large, else Mbps if small integer
            num = float(s)
            if num > 1000:
                val = num / 1000.0
            else:
                val = num
        return val
    except Exception:
        return None


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
        """Extract labels for a scenario including derived metadata."""
        # Extract stream count from scenario name
        streams = self._extract_stream_count(scenario)
        
        # Determine output ladder identifier
        output_ladder = self._get_output_ladder_id(scenario)
        
        # Detect encoder type (cpu/gpu) from scenario metadata or name
        encoder_type = scenario.get("encoder_type", self._detect_encoder_type(scenario))
        
        return {
            "scenario": str(scenario.get("name", "")),
            "bitrate": str(scenario.get("bitrate", "")),
            "resolution": str(scenario.get("resolution", "")),
            "fps": str(scenario.get("fps", "")),
            "streams": str(streams),
            "output_ladder": output_ladder,
            "encoder_type": encoder_type,
        }
    
    def _extract_stream_count(self, scenario: dict) -> int:
        """Extract number of concurrent streams from scenario name."""
        name = scenario.get("name", "").lower()
        # Look for patterns like "2 streams", "4 Streams", etc.
        match = re.search(r'(\d+)\s+streams?', name)
        if match:
            return int(match.group(1))
        return 1
    
    def _get_output_ladder_id(self, scenario: dict) -> str:
        """Get output ladder identifier for grouping scenarios."""
        outputs = scenario.get("outputs")
        
        if outputs and isinstance(outputs, list) and len(outputs) > 0:
            # Multi-resolution ladder
            ladder_parts = []
            for output in outputs:
                resolution = output.get("resolution", "")
                fps = output.get("fps", "")
                if resolution and fps:
                    # Parse resolution for proper sorting
                    width, height = self._parse_resolution(resolution)
                    if width and height:
                        ladder_parts.append((width, height, fps, resolution))
            
            if ladder_parts:
                # Sort by width (descending), then height (descending)
                ladder_parts.sort(key=lambda x: (x[0], x[1]), reverse=True)
                # Build ladder string
                formatted = [f"{res}@{fps}" for _, _, fps, res in ladder_parts]
                return ",".join(formatted)
        
        # Single resolution
        resolution = scenario.get("resolution", "N/A")
        fps = scenario.get("fps", "N/A")
        if resolution != "N/A" and fps != "N/A":
            return f"{resolution}@{fps}"
        
        return "unknown"
    
    def _detect_encoder_type(self, scenario: dict) -> str:
        """Detect encoder type (cpu/gpu) from scenario name or metadata."""
        name = scenario.get("name", "").lower()
        
        # Check for explicit mentions
        if "gpu" in name or "nvenc" in name or "qsv" in name or "vaapi" in name:
            return "gpu"
        if "cpu" in name or "x264" in name or "libx264" in name:
            return "cpu"
        
        # Default to CPU for most scenarios
        return "cpu"
    
    def _compute_efficiency_score(self, scenario: dict, stats: dict) -> float | None:
        """
        Compute energy efficiency score.
        
        Formula: (throughput_mbps * streams) / mean_power_watts
        Returns pixels per joule if resolution data is available.
        """
        mean_watts = stats.get("mean_power_w", 0)
        total_energy_j = stats.get("total_energy_j", 0)
        duration = stats.get("duration_s", 0)
        
        if mean_watts <= 0 or total_energy_j <= 0 or duration <= 0:
            return None
        
        # Try to compute pixels per joule (preferred for output ladders)
        outputs = scenario.get("outputs")
        total_pixels = 0
        
        if outputs and isinstance(outputs, list):
            # Multi-resolution ladder
            for output in outputs:
                resolution = output.get("resolution", "")
                fps = output.get("fps", 0)
                width, height = self._parse_resolution(resolution)
                if width and height and fps:
                    total_pixels += width * height * fps * duration
        else:
            # Single resolution
            resolution = scenario.get("resolution", "")
            fps = scenario.get("fps", 0)
            width, height = self._parse_resolution(resolution)
            if width and height and fps:
                total_pixels = width * height * fps * duration
        
        if total_pixels > 0:
            # Pixels per joule
            return total_pixels / total_energy_j
        
        # Fallback: throughput per watt
        bitrate_mbps = _parse_bitrate_to_mbps(scenario.get("bitrate", ""))
        streams = self._extract_stream_count(scenario)
        throughput_mbps = bitrate_mbps * streams
        
        if throughput_mbps > 0:
            return throughput_mbps / mean_watts
        
        return None
    
    def _parse_resolution(self, resolution: str) -> tuple[int | None, int | None]:
        """Parse resolution string to (width, height)."""
        if not resolution or resolution == "N/A":
            return (None, None)
        
        try:
            parts = resolution.lower().split('x')
            if len(parts) == 2:
                width = int(parts[0].strip())
                height = int(parts[1].strip())
                return (width, height)
        except (ValueError, AttributeError):
            pass
        
        return (None, None)

    def _labels_str(self, labels: dict) -> str:
        parts = [f'{k}="{_escape_label_value(str(v))}"' for k, v in labels.items()]
        return "{" + ",".join(parts) + "}"

    def _compute_scenario_stats(self, start: float, end: float, scenario: dict) -> dict:
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
        container_data = self.client.query_range(container_cpu_query, start, end, step="5s")
        container_values = _extract_values(container_data)
        mean_container_cpu = mean(container_values) if container_values else 0.0

        total_energy_wh = (energy_j / 3600) if duration > 0 else 0.0

        # Docker overhead is estimated: docker_engine_cpu_percent × total power
        estimated_docker_overhead_w = 0.0
        docker_engine_query = "docker_engine_cpu_percent"
        docker_engine_data = self.client.query_range(docker_engine_query, start, end, step="5s")
        docker_engine_values = _extract_values(docker_engine_data)
        mean_docker_engine_cpu = mean(docker_engine_values) if docker_engine_values else 0.0
        if mean_power > 0:
            estimated_docker_overhead_w = (mean_docker_engine_cpu / 100.0) * mean_power

        # Normalized energy metrics
        energy_wh_per_min = None
        energy_wh_per_mbps = None
        energy_mj_per_frame = None
        if total_energy_wh and duration > 0:
            energy_wh_per_min = total_energy_wh * 60.0 / duration
        mbps = _parse_bitrate_to_mbps(scenario.get("bitrate", ""))
        if total_energy_wh and mbps and mbps > 0:
            energy_wh_per_mbps = total_energy_wh / mbps
        fps_val = scenario.get("fps")
        if isinstance(fps_val, (int, float)) and energy_j and duration > 0 and (fps_val * duration) > 0:
            per_frame_mj = (energy_j / (fps_val * duration)) * 1000.0
            energy_mj_per_frame = per_frame_mj

        return {
            "duration_s": duration,
            "mean_power_w": mean_power,
            "total_energy_j": energy_j,
            "total_energy_wh": total_energy_wh,
            "container_cpu_percent": mean_container_cpu,
            "measured_power_watts": mean_power,  # alias for clarity
            "estimated_docker_overhead_watts": estimated_docker_overhead_w,
            "energy_wh_per_min": energy_wh_per_min,
            "energy_wh_per_mbps": energy_wh_per_mbps,
            "energy_mj_per_frame": energy_mj_per_frame,
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
        output.append("# HELP results_scenario_measured_power_watts Measured CPU package power (W) from RAPL")
        output.append("# TYPE results_scenario_measured_power_watts gauge")
        output.append("# HELP results_scenario_estimated_docker_overhead_watts Estimated Docker overhead power (W)")
        output.append("# TYPE results_scenario_estimated_docker_overhead_watts gauge")
        output.append("# HELP results_scenario_total_energy_joules Total energy (J) during scenario (from rapl_energy_joules_total)")
        output.append("# TYPE results_scenario_total_energy_joules gauge")
        output.append("# HELP results_scenario_total_energy_wh Total energy (Wh) during scenario (derived from mean power)")
        output.append("# TYPE results_scenario_total_energy_wh gauge")
        output.append("# HELP results_scenario_container_cpu_percent Mean total container CPU percent during scenario")
        output.append("# TYPE results_scenario_container_cpu_percent gauge")
        output.append("# HELP results_scenario_energy_wh_per_min Energy per minute (Wh/min)")
        output.append("# TYPE results_scenario_energy_wh_per_min gauge")
        output.append("# HELP results_scenario_energy_wh_per_mbps Energy per Mbps (Wh/Mbps)")
        output.append("# TYPE results_scenario_energy_wh_per_mbps gauge")
        output.append("# HELP results_scenario_energy_mj_per_frame Energy per frame (mJ/frame)")
        output.append("# TYPE results_scenario_energy_mj_per_frame gauge")
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
        output.append("# HELP results_scenario_efficiency_score Energy efficiency score (pixels/J or Mbps/W)")
        output.append("# TYPE results_scenario_efficiency_score gauge")
        output.append("# HELP results_scenario_total_pixels Total pixels delivered across all outputs")
        output.append("# TYPE results_scenario_total_pixels gauge")

        baseline = self._find_baseline(scenarios)
        baseline_stats = None
        if baseline and baseline.get("start_time") and baseline.get("end_time"):
            baseline_stats = self._compute_scenario_stats(baseline["start_time"], baseline["end_time"], baseline)

        for scenario in scenarios:
            start = scenario.get("start_time")
            end = scenario.get("end_time")
            if not start or not end:
                continue

            stats = self._compute_scenario_stats(start, end, scenario)
            labels = {"run_id": run_id}
            labels.update(self._scenario_labels(scenario))
            lbl = self._labels_str(labels)

            output.append(f"results_scenario_duration_seconds{lbl} {stats['duration_s']:.3f}")
            output.append(f"results_scenario_mean_power_watts{lbl} {stats['mean_power_w']:.4f}")
            output.append(f"results_scenario_measured_power_watts{lbl} {stats['measured_power_watts']:.4f}")
            output.append(f"results_scenario_estimated_docker_overhead_watts{lbl} {stats['estimated_docker_overhead_watts']:.4f}")
            output.append(f"results_scenario_total_energy_joules{lbl} {stats['total_energy_j']:.4f}")
            output.append(f"results_scenario_total_energy_wh{lbl} {stats['total_energy_wh']:.6f}")
            output.append(
                f"results_scenario_container_cpu_percent{lbl} {stats['container_cpu_percent']:.4f}"
            )
            if stats["energy_wh_per_min"] is not None:
                output.append(f"results_scenario_energy_wh_per_min{lbl} {stats['energy_wh_per_min']:.6f}")
            if stats["energy_wh_per_mbps"] is not None:
                output.append(f"results_scenario_energy_wh_per_mbps{lbl} {stats['energy_wh_per_mbps']:.6f}")
            if stats["energy_mj_per_frame"] is not None:
                output.append(f"results_scenario_energy_mj_per_frame{lbl} {stats['energy_mj_per_frame']:.4f}")
            
            # Compute and export efficiency score
            efficiency_score = self._compute_efficiency_score(scenario, stats)
            if efficiency_score is not None:
                output.append(f"results_scenario_efficiency_score{lbl} {efficiency_score:.4e}")
            
            # Compute and export total pixels
            outputs = scenario.get("outputs")
            total_pixels = 0
            duration = stats.get("duration_s", 0)
            
            if outputs and isinstance(outputs, list):
                for output_item in outputs:
                    resolution = output_item.get("resolution", "")
                    fps = output_item.get("fps", 0)
                    width, height = self._parse_resolution(resolution)
                    if width and height and fps and duration:
                        total_pixels += width * height * fps * duration
            else:
                resolution = scenario.get("resolution", "")
                fps = scenario.get("fps", 0)
                width, height = self._parse_resolution(resolution)
                if width and height and fps and duration:
                    total_pixels = width * height * fps * duration
            
            if total_pixels > 0:
                output.append(f"results_scenario_total_pixels{lbl} {total_pixels:.0f}")

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
