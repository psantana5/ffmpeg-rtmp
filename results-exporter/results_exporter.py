#!/usr/bin/env python3

import json
import os
import re
import sys
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from statistics import mean, stdev
from urllib.parse import urlencode
from urllib.request import urlopen, Request

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))

try:
    from advisor import MultivariatePredictor
    PREDICTOR_AVAILABLE = True
except ImportError:
    PREDICTOR_AVAILABLE = False


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

        # ML predictor for future predictions
        self.predictor = None
        self.predictor_trained = False

        # Log initialization
        print(f"Results exporter initialized with results_dir={self.results_dir}")
        print(f"Directory exists: {self.results_dir.exists()}")
        print(f"ML Predictor available: {PREDICTOR_AVAILABLE}")
        if self.results_dir.exists():
            files = list(self.results_dir.glob("test_results_*.json"))
            print(f"Found {len(files)} result file(s) at startup")

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
        """
        Extract number of concurrent streams from scenario name.
        
        Args:
            scenario: Scenario dictionary with 'name' field
            
        Returns:
            Number of streams (int, minimum 1)
            
        Examples:
            "2 streams @ 2500k" -> 2
            "4 Streams" -> 4
            "Single test" -> 1 (default)
        """
        name = scenario.get("name", "").lower()
        # Look for patterns like "2 streams", "4 Streams", etc.
        match = re.search(r'(\d+)\s+streams?', name)
        if match:
            return int(match.group(1))
        return 1

    def _get_output_ladder_id(self, scenario: dict) -> str:
        """
        Get output ladder identifier for grouping scenarios.
        
        Output ladder IDs enable fair comparison between scenarios with identical
        output configurations (e.g., comparing different encoders/bitrates for
        the same resolution ladder).
        
        Args:
            scenario: Scenario dictionary with optional 'outputs' field
            
        Returns:
            Ladder identifier string, formatted as comma-separated resolution@fps pairs,
            sorted by resolution (descending). Examples:
                - Single resolution: "1280x720@30"
                - Multi-resolution: "1920x1080@30,1280x720@30,854x480@30"
                - Unknown: "unknown"
                
        Scenarios with different outputs:
            - Different resolution order -> Same ladder ID (sorted consistently)
            - Different resolutions -> Different ladder IDs
            - Missing outputs field -> Uses single resolution/fps
        """
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
        """
        Detect encoder type (cpu/gpu) from scenario name or metadata.
        
        Uses heuristics to identify the encoder type based on common naming
        patterns in scenario names.
        
        Args:
            scenario: Scenario dictionary with 'name' field
            
        Returns:
            Encoder type string: "cpu" or "gpu"
            
        Detection Logic:
            - GPU indicators: "gpu", "nvenc", "qsv", "vaapi" in scenario name
            - CPU indicators: "cpu", "x264", "libx264" in scenario name
            - Default: "cpu" (most conservative assumption)
            
        Examples:
            {"name": "GPU transcode"} -> "gpu"
            {"name": "NVENC test"} -> "gpu"
            {"name": "x264 encoding"} -> "cpu"
            {"name": "2 streams @ 2500k"} -> "cpu" (default)
        """
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
        Compute energy efficiency score for a scenario.
        
        The efficiency score is primarily computed as pixels per joule, which
        provides a quality-aware efficiency metric. Falls back to throughput
        per watt (Mbps/W) if resolution data is unavailable.
        
        Args:
            scenario: Scenario dictionary containing:
                - resolution: str (e.g., "1280x720")
                - fps: int (e.g., 30)
                - bitrate: str (e.g., "2500k")
                - outputs: list (optional, for multi-resolution ladders)
            stats: Statistics dictionary containing:
                - mean_power_w: float (mean power in watts)
                - total_energy_j: float (total energy in joules)
                - duration_s: float (duration in seconds)
        
        Returns:
            Energy efficiency score (float) or None if insufficient data.
            - Primary: pixels per joule (higher is better)
            - Fallback: Mbps per watt (higher is better)
            
        Formula (pixels per joule):
            total_pixels = sum(width * height * fps * duration for each output)
            efficiency_score = total_pixels / total_energy_j
            
        Formula (throughput per watt - fallback):
            throughput_mbps = bitrate_mbps * streams
            efficiency_score = throughput_mbps / mean_power_w
            
        Examples:
            Single 720p stream for 100s at 5000J:
                pixels = 1280 * 720 * 30 * 100 = 2,764,800,000
                efficiency = 2,764,800,000 / 5000 = 552,960 pixels/J
                
            Multi-resolution ladder (1080p+720p) for 100s at 7500J:
                pixels = (1920*1080*30*100) + (1280*720*30*100) = 8,985,600,000
                efficiency = 8,985,600,000 / 7500 = 1,198,080 pixels/J
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
        """
        Parse resolution string to (width, height) tuple.
        
        Args:
            resolution: Resolution string in format "WIDTHxHEIGHT"
            
        Returns:
            Tuple of (width, height) as integers, or (None, None) if parsing fails
            
        Supported Formats:
            - "1920x1080" -> (1920, 1080)
            - "1280x720" -> (1280, 720)
            - "854x480" -> (854, 480)
            - "N/A" -> (None, None)
            - Invalid format -> (None, None)
            
        Examples:
            >>> exporter._parse_resolution("1920x1080")
            (1920, 1080)
            >>> exporter._parse_resolution("invalid")
            (None, None)
        """
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
        power_stdev = 0.0
        if duration > 0:
            window = f"{max(1, int(duration))}s"
            energy_query = f'sum(increase(rapl_energy_joules_total{{zone=~"package.*"}}[{window}]))'
            energy_data = self.client.query(energy_query, ts=end)
            energy_j = _extract_instant_value(energy_data)
            mean_power = (energy_j / duration) if duration > 0 else 0.0
            
            # Query power samples over time to calculate standard deviation
            # Note: stdev requires at least 2 values; if 0 or 1 values, power_stdev remains 0.0
            power_query = f'sum(rapl_power_watts{{zone=~"package.*"}})'
            power_data = self.client.query_range(power_query, start, end, step="5s")
            power_values = _extract_values(power_data)
            if power_values and len(power_values) > 1:
                power_stdev = stdev(power_values)

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
        
        # Calculate prediction confidence bounds using ~95% confidence interval
        # Using mean ± 2×stdev as an approximation (covers ~95.4% for normal distribution)
        # For a precise 95% CI, use mean ± 1.96×stdev, but 2×stdev is simpler and conservative
        prediction_confidence_high = mean_power + (2.0 * power_stdev) if mean_power > 0 else 0.0
        prediction_confidence_low = max(0.0, mean_power - (2.0 * power_stdev)) if mean_power > 0 else 0.0

        return {
            "duration_s": duration,
            "mean_power_w": mean_power,
            "power_stdev_w": power_stdev,
            "total_energy_j": energy_j,
            "total_energy_wh": total_energy_wh,
            "container_cpu_percent": mean_container_cpu,
            "measured_power_watts": mean_power,  # alias for clarity
            "estimated_docker_overhead_watts": estimated_docker_overhead_w,
            "energy_wh_per_min": energy_wh_per_min,
            "energy_wh_per_mbps": energy_wh_per_mbps,
            "energy_mj_per_frame": energy_mj_per_frame,
            "prediction_confidence_high": prediction_confidence_high,
            "prediction_confidence_low": prediction_confidence_low,
        }

    def _train_predictor(self, scenarios: list[dict]):
        """
        Train multivariate predictor on collected scenario data.
        
        This method trains ML models for power prediction on the available
        scenario data. The predictor can then be used to generate predictions
        for untested configurations.
        
        Args:
            scenarios: List of scenario dicts with computed stats
        """
        if not PREDICTOR_AVAILABLE:
            return

        # Only train if we have enough data
        if len(scenarios) < 3:
            return

        try:
            # Initialize predictor if needed
            if self.predictor is None:
                self.predictor = MultivariatePredictor(
                    models=['linear', 'poly2', 'rf'],  # Skip poly3 and gbm for speed
                    confidence_level=0.95,
                    n_bootstrap=50,  # Reduced for performance
                    cv_folds=min(3, len(scenarios))
                )

            # Train on mean_power_watts
            success = self.predictor.fit(scenarios, target='mean_power_watts')
            if success:
                self.predictor_trained = True
                print(f"ML predictor trained on {len(scenarios)} scenarios")
                info = self.predictor.get_model_info()
                print(f"  Best model: {info['best_model']} (R²={info['best_score']['r2']:.4f})")

        except Exception as e:
            print(f"Failed to train predictor: {e}")
            self.predictor_trained = False

    def _predict_for_scenario(self, scenario: dict, stats: dict) -> dict | None:
        """
        Generate predictions for a scenario using trained ML model.
        
        Args:
            scenario: Scenario dict
            stats: Computed stats for the scenario
            
        Returns:
            Dict with prediction results or None if predictor not available
        """
        if not self.predictor_trained or self.predictor is None:
            return None

        try:
            # Extract features for prediction
            features = self.predictor._extract_features(scenario)
            if features is None:
                return None

            # Make prediction with confidence intervals
            prediction = self.predictor.predict(features, return_confidence=True)

            # Predict energy (power * duration)
            if prediction['mean'] and stats.get('duration_s'):
                predicted_energy = prediction['mean'] * stats['duration_s']
            else:
                predicted_energy = None

            return {
                'power_watts': prediction.get('mean'),
                'energy_joules': predicted_energy,
                'ci_low': prediction.get('ci_low'),
                'ci_high': prediction.get('ci_high'),
            }

        except Exception:
            return None

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

        # Detect new results file and log it (skip initial empty cache)
        if run_id != self._cached_run_id and self._cached_run_id != "":
            print(f"New results file detected: {latest.name}")

        if (now - self._last_refresh) < self.cache_seconds and run_id == self._cached_run_id:
            return self._cached_metrics

        try:
            data = self._load_results(latest)
            scenarios = data.get("scenarios", [])
            if self._cached_run_id == "":
                # First time loading
                print(f"Loaded {len(scenarios)} scenarios from {latest.name}")
            else:
                # New file loaded
                print(f"Loaded {len(scenarios)} scenarios from {latest.name}")
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
<<<<<<< HEAD
        output.append("# HELP results_scenario_prediction_confidence_high Upper bound of prediction confidence interval (W)")
        output.append("# TYPE results_scenario_prediction_confidence_high gauge")
        output.append("# HELP results_scenario_prediction_confidence_low Lower bound of prediction confidence interval (W)")
        output.append("# TYPE results_scenario_prediction_confidence_low gauge")
        output.append("# HELP results_scenario_power_stdev Standard deviation of power measurements (W)")
        output.append("# TYPE results_scenario_power_stdev gauge")
=======
        output.append("# HELP results_scenario_predicted_power_watts Predicted power consumption (W) from ML model")
        output.append("# TYPE results_scenario_predicted_power_watts gauge")
        output.append("# HELP results_scenario_predicted_energy_joules Predicted total energy (J) from ML model")
        output.append("# TYPE results_scenario_predicted_energy_joules gauge")
        output.append("# HELP results_scenario_predicted_efficiency_score Predicted efficiency score from ML model")
        output.append("# TYPE results_scenario_predicted_efficiency_score gauge")
        output.append("# HELP results_scenario_prediction_confidence_low Lower bound of prediction confidence interval")
        output.append("# TYPE results_scenario_prediction_confidence_low gauge")
        output.append("# HELP results_scenario_prediction_confidence_high Upper bound of prediction confidence interval")
        output.append("# TYPE results_scenario_prediction_confidence_high gauge")
>>>>>>> a9959ea (Add Prometheus metrics and CLI for multivariate predictions)

        baseline = self._find_baseline(scenarios)
        baseline_stats = None
        if baseline and baseline.get("start_time") and baseline.get("end_time"):
            baseline_stats = self._compute_scenario_stats(baseline["start_time"], baseline["end_time"], baseline)

        # Collect scenarios with stats for predictor training
        scenarios_with_stats = []
        for scenario in scenarios:
            start = scenario.get("start_time")
            end = scenario.get("end_time")
            if not start or not end:
                continue
            stats = self._compute_scenario_stats(start, end, scenario)
            # Merge stats into scenario for predictor
            scenario_copy = dict(scenario)
            scenario_copy['power'] = {
                'mean_watts': stats['mean_power_w'],
                'total_energy_joules': stats['total_energy_j']
            }
            scenario_copy['container_usage'] = {
                'cpu_percent': stats['container_cpu_percent']
            }
            scenario_copy['docker_overhead'] = {
                'cpu_percent': 0.0  # Not directly available from stats
            }
            scenario_copy['duration'] = stats['duration_s']
            scenarios_with_stats.append((scenario_copy, stats))

        # Train predictor on scenarios with valid data
        if PREDICTOR_AVAILABLE and not self.predictor_trained:
            valid_scenarios = [s for s, _ in scenarios_with_stats if s.get('power', {}).get('mean_watts')]
            if len(valid_scenarios) >= 3:
                self._train_predictor(valid_scenarios)

        for scenario_copy, stats in scenarios_with_stats:
            # Use original scenario for labels
            scenario = next((s for s in scenarios if s.get('name') == scenario_copy.get('name')), scenario_copy)

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
<<<<<<< HEAD
            
<<<<<<< HEAD
            # Export prediction confidence metrics
            output.append(f"results_scenario_prediction_confidence_high{lbl} {stats['prediction_confidence_high']:.4f}")
            output.append(f"results_scenario_prediction_confidence_low{lbl} {stats['prediction_confidence_low']:.4f}")
            output.append(f"results_scenario_power_stdev{lbl} {stats['power_stdev_w']:.4f}")
=======
=======

>>>>>>> 15caa26 (Apply linting fixes to codebase)
            # Generate ML predictions if predictor is trained
            predictions = self._predict_for_scenario(scenario_copy, stats)
            if predictions:
                if predictions['power_watts'] is not None:
                    output.append(f"results_scenario_predicted_power_watts{lbl} {predictions['power_watts']:.4f}")
                if predictions['energy_joules'] is not None:
                    output.append(f"results_scenario_predicted_energy_joules{lbl} {predictions['energy_joules']:.4f}")
                if predictions['ci_low'] is not None:
                    output.append(f"results_scenario_prediction_confidence_low{lbl} {predictions['ci_low']:.4f}")
                if predictions['ci_high'] is not None:
                    output.append(f"results_scenario_prediction_confidence_high{lbl} {predictions['ci_high']:.4f}")
>>>>>>> a9959ea (Add Prometheus metrics and CLI for multivariate predictions)

            if baseline_stats and scenario.get('name') != (baseline.get('name') if baseline else None):
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
