#!/usr/bin/env python3
"""
Cost Metrics Prometheus Exporter

Exports cost analysis metrics as Prometheus metrics for Grafana visualization.

Load-aware metrics use actual CPU usage and power measurements from Prometheus
with advanced numerical integration (trapezoidal rule) for accurate cost calculation.

Mathematical Approach:
    - CPU Cost: Integrates CPU usage over time using trapezoidal rule
      Cost = (∫ cpu(t) dt) × price_per_core_second
    
    - Energy Cost: Integrates power consumption using trapezoidal rule
      Energy = ∫ power(t) dt (Joules)
      Cost = Energy × price_per_joule
    
    - Accuracy: O(h²) convergence vs O(h) for rectangular approximation

Metrics exported:
- cost_total_load_aware: Total cost (load-aware, scales with actual usage)
- cost_energy_load_aware: Energy cost (load-aware, scales with actual power)
- cost_compute_load_aware: Compute cost (load-aware, scales with actual CPU)
- cost_per_pixel: Cost efficiency metric ($/megapixel delivered)
- cost_per_watch_hour: Cost per viewer watch hour ($/viewer-hour)

All metrics include labels: scenario, streams, bitrate, encoder, currency, service

Usage:
    # With Prometheus for load-aware metrics
    python3 cost_exporter.py --port 9504 --prometheus-url http://prometheus:9090 \
        --energy-cost 0.12 --cpu-cost 0.50
"""

import argparse
import json
import logging
import re
import time
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Dict, List, Optional
from urllib.parse import urlencode
from urllib.request import Request, urlopen

from advisor.cost import CostModel

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class PrometheusClient:
    """Simple Prometheus client for querying metrics."""
    
    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip('/')
    
    def query_range(
        self, query: str, start: float, end: float, step: str = '5s'
    ) -> Optional[Dict]:
        """
        Query Prometheus for a range of values.
        
        Args:
            query: PromQL query string
            start: Start timestamp (unix seconds)
            end: End timestamp (unix seconds)
            step: Query resolution (e.g., '5s', '15s')
        
        Returns:
            Query response dict or None on error
        """
        params = {
            'query': query,
            'start': int(start),
            'end': int(end),
            'step': step
        }
        url = f"{self.base_url}/api/v1/query_range?{urlencode(params)}"
        req = Request(url, headers={'Accept': 'application/json'})
        
        # Log the exact query being executed
        logger.debug(f"Executing PromQL query: {query}")
        logger.debug(f"Query time range: start={int(start)}, end={int(end)}, step={step}")
        
        try:
            with urlopen(req, timeout=30) as resp:
                result = json.load(resp)
                # Debug logging: Log query status
                status = result.get('status', 'unknown')
                logger.debug(f"Query status: {status}")
                return result
        except Exception as e:
            logger.error(f"Prometheus query failed: {e}")
            return None
    
    def extract_values(self, query_response: Optional[Dict]) -> List[float]:
        """
        Extract values from Prometheus query response.
        
        Args:
            query_response: Response from query_range
        
        Returns:
            List of values
        """
        if not query_response:
            logger.debug("Query returned no response (likely a connection/network issue)")
            return []
        
        data = query_response.get('data', {})
        results = data.get('result', [])
        values = []
        
        for result in results:
            for ts, val in result.get('values', []):
                try:
                    values.append(float(val))
                except (ValueError, TypeError):
                    continue
        
        # Debug logging: Log number of samples returned
        num_series = len(results)
        num_samples = len(values)
        logger.debug(f"Extracted {num_samples} samples from {num_series} time series")
        
        return values


def _extract_stream_count(scenario: Dict) -> int:
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


class CostMetricsExporter:
    """
    Exports cost metrics as Prometheus metrics.
    
    Exports load-aware metrics that query Prometheus for real CPU/power metrics.
    """
    
    def __init__(
        self,
        results_dir: Path,
        energy_cost_per_kwh: float = 0.0,
        cpu_cost_per_hour: float = 0.0,
        gpu_cost_per_hour: float = 0.0,
        currency: str = 'USD',
        prometheus_url: Optional[str] = None,
        use_load_aware: bool = True
    ):
        """
        Initialize cost metrics exporter.
        
        Args:
            results_dir: Directory containing test results
            energy_cost_per_kwh: Energy cost ($/kWh or €/kWh)
            cpu_cost_per_hour: CPU/instance cost ($/h or €/h)
            gpu_cost_per_hour: GPU cost ($/h or €/h)
            currency: Currency code
            prometheus_url: Prometheus URL for load-aware mode
            use_load_aware: Use load-aware calculations if Prometheus available
        """
        self.results_dir = results_dir
        self.cost_model = CostModel(
            energy_cost_per_kwh=energy_cost_per_kwh,
            cpu_cost_per_hour=cpu_cost_per_hour,
            gpu_cost_per_hour=gpu_cost_per_hour,
            currency=currency
        )
        self.metrics_cache = {}
        self.last_update = 0
        self.cache_ttl = 60  # Cache for 60 seconds
        
        # Prometheus integration
        self.prometheus_client = None
        self.use_load_aware = use_load_aware
        if prometheus_url and use_load_aware:
            try:
                self.prometheus_client = PrometheusClient(prometheus_url)
                logger.info(f"Load-aware mode enabled (Prometheus: {prometheus_url})")
            except Exception as e:
                logger.warning(f"Failed to initialize Prometheus client: {e}")
                logger.info("Falling back to legacy mode")
        else:
            logger.info("Load-aware mode requires Prometheus URL")
    
    def load_latest_results(self) -> List[Dict]:
        """Load scenarios from most recent test results file."""
        json_files = sorted(self.results_dir.glob('test_results_*.json'), reverse=True)
        
        if not json_files:
            logger.warning(f"No test results found in {self.results_dir}")
            return []
        
        try:
            with open(json_files[0]) as f:
                data = json.load(f)
            
            scenarios = data.get('scenarios', [])
            logger.debug(f"Loaded {len(scenarios)} scenarios from {json_files[0].name}")
            return scenarios
        
        except Exception as e:
            logger.error(f"Error loading results: {e}")
            return []
    
    def enrich_scenario_with_prometheus(self, scenario: Dict) -> Dict:
        """
        Enrich scenario with real-time metrics from Prometheus.
        
        Queries Prometheus for:
        - CPU usage (container_cpu_usage_seconds_total rate)
        - Power consumption (rapl_power_watts)
        
        Args:
            scenario: Scenario dict with start_time, end_time, duration
        
        Returns:
            Enriched scenario with cpu_usage_cores, power_watts, step_seconds
        """
        if not self.prometheus_client:
            return scenario
        
        scenario_name = scenario.get('name', 'unknown')
        start_time = scenario.get('start_time')
        end_time = scenario.get('end_time')
        
        if not start_time or not end_time:
            logger.warning(f"Scenario '{scenario_name}': Missing timestamps")
            return scenario
        
        step_seconds = 5  # 5 second resolution
        
        logger.debug(f"Enriching scenario '{scenario_name}' with Prometheus data")
        
        try:
            # Query CPU usage
            # Use rate() to get CPU cores per second, aggregated across all containers
            # Exclude POD containers and aggregate by container name
            # This gives us the instantaneous CPU usage in cores at each sample point
            cpu_query = (
                'sum(rate(container_cpu_usage_seconds_total{name!~".*POD.*",name!=""}[30s]))'
            )
            logger.debug(f"Scenario '{scenario_name}': Querying CPU usage")
            cpu_response = self.prometheus_client.query_range(
                cpu_query, start_time, end_time, f'{step_seconds}s'
            )
            cpu_values = self.prometheus_client.extract_values(cpu_response)
            
            # Query power consumption
            # Sum all RAPL zones for total system power
            # RAPL provides instantaneous power measurements in watts
            power_query = 'sum(rapl_power_watts)'
            logger.debug(f"Scenario '{scenario_name}': Querying power consumption")
            power_response = self.prometheus_client.query_range(
                power_query, start_time, end_time, f'{step_seconds}s'
            )
            power_values = self.prometheus_client.extract_values(power_response)
            
            # Add to scenario
            if cpu_values:
                scenario['cpu_usage_cores'] = cpu_values
                scenario['step_seconds'] = step_seconds
                logger.debug(
                    f"Scenario '{scenario_name}': "
                    f"Enriched with {len(cpu_values)} CPU measurements"
                )
            else:
                logger.warning(
                    f"Scenario '{scenario_name}': "
                    f"No CPU usage data returned from Prometheus"
                )
            
            if power_values:
                scenario['power_watts'] = power_values
                logger.debug(
                    f"Scenario '{scenario_name}': "
                    f"Enriched with {len(power_values)} power measurements"
                )
            else:
                logger.warning(
                    f"Scenario '{scenario_name}': "
                    f"No power data returned from Prometheus"
                )
        
        except Exception as e:
            logger.error(
                f"Failed to enrich scenario '{scenario_name}': {e}"
            )
        
        return scenario
    
    def generate_prometheus_metrics(self) -> str:
        """
        Generate Prometheus metrics in text format.
        
        Exports only load-aware metrics:
        - cost_total_load_aware, cost_energy_load_aware, cost_compute_load_aware
        - cost_exporter_alive (health check metric)
        
        Returns:
            Prometheus metrics text
        """
        # Check cache
        current_time = time.time()
        if (current_time - self.last_update) < self.cache_ttl and self.metrics_cache:
            return self.metrics_cache.get('output', '')
        
        logger.debug("Generating Prometheus metrics (cache miss or expired)")
        
        # Load scenarios
        scenarios = self.load_latest_results()
        logger.info(f"Loaded {len(scenarios)} scenarios from results directory")
        
        # Enrich with Prometheus metrics if available
        if self.prometheus_client and self.use_load_aware:
            logger.debug("Enriching scenarios with Prometheus metrics")
            scenarios = [
                self.enrich_scenario_with_prometheus(s) for s in scenarios
            ]
        
        output = []
        
        # Add alive metric for health check
        output.append("# HELP cost_exporter_alive Cost exporter health check (always 1)")
        output.append("# TYPE cost_exporter_alive gauge")
        output.append("cost_exporter_alive 1")
        logger.debug("Set cost_exporter_alive=1")
        
        # Metrics definitions - Load-aware metrics only
        currency = self.cost_model.currency
        output.append(f"# HELP cost_total_load_aware Total cost ({currency}) - load-aware")
        output.append("# TYPE cost_total_load_aware gauge")
        output.append(f"# HELP cost_energy_load_aware Energy cost ({currency}) - load-aware")
        output.append("# TYPE cost_energy_load_aware gauge")
        output.append(f"# HELP cost_compute_load_aware Compute cost ({currency}) - load-aware")
        output.append("# TYPE cost_compute_load_aware gauge")
        output.append(f"# HELP cost_per_pixel Cost per pixel ({currency}/pixel) - load-aware")
        output.append("# TYPE cost_per_pixel gauge")
        output.append(
            f"# HELP cost_per_watch_hour "
            f"Cost per viewer watch hour ({currency}/hour) - load-aware"
        )
        output.append("# TYPE cost_per_watch_hour gauge")
        
        # Track metrics emission statistics
        metrics_emitted = 0
        scenarios_with_data = 0
        scenarios_without_data = 0
        
        # Export metrics for each scenario
        for scenario in scenarios:
            scenario_name = scenario.get('name', 'unknown')
            
            # Sanitize name for Prometheus labels
            safe_name = scenario_name.replace(' ', '_').replace('"', '')
            
            # Extract additional labels - use helper function for streams
            streams = _extract_stream_count(scenario)
            bitrate = scenario.get('bitrate', '')
            encoder = scenario.get('encoder_type', 'unknown')
            
            # Skip metrics without required labels (bitrate)
            if not bitrate:
                logger.debug(
                    f"Skipping scenario '{scenario_name}': "
                    f"missing bitrate label"
                )
                continue
            
            # Build labels with service label
            labels = (
                f'scenario="{safe_name}",'
                f'currency="{currency}",'
                f'streams="{streams}",'
                f'bitrate="{bitrate}",'
                f'encoder="{encoder}",'
                f'service="cost-analysis"'
            )
            
            # Compute and export load-aware costs if data is available
            has_load_aware_data = (
                'cpu_usage_cores' in scenario and 
                'power_watts' in scenario
            )
            
            if has_load_aware_data:
                scenarios_with_data += 1
                logger.debug(f"Scenario '{scenario_name}': Computing load-aware costs")
                
                load_aware_total_cost = self.cost_model.compute_total_cost_load_aware(scenario)
                load_aware_energy_cost = self.cost_model.compute_energy_cost_load_aware(scenario)
                load_aware_compute_cost = self.cost_model.compute_compute_cost_load_aware(scenario)
                cost_per_pixel = self.cost_model.compute_cost_per_pixel_load_aware(scenario)
                cost_per_watch_hour = (
                    self.cost_model.compute_cost_per_watch_hour_load_aware(scenario)
                )
                
                # Export load-aware metrics
                if load_aware_total_cost is not None:
                    output.append(
                        f"cost_total_load_aware{{{labels}}} {load_aware_total_cost:.8f}"
                    )
                    logger.debug(
                        f"Set cost_total_load_aware{{{safe_name}}}={load_aware_total_cost:.8f}"
                    )
                    metrics_emitted += 1
                if load_aware_energy_cost is not None:
                    output.append(
                        f"cost_energy_load_aware{{{labels}}} {load_aware_energy_cost:.8f}"
                    )
                    logger.debug(
                        f"Set cost_energy_load_aware{{{safe_name}}}={load_aware_energy_cost:.8f}"
                    )
                    metrics_emitted += 1
                if load_aware_compute_cost is not None:
                    output.append(
                        f"cost_compute_load_aware{{{labels}}} {load_aware_compute_cost:.8f}"
                    )
                    logger.debug(
                        f"Set cost_compute_load_aware{{{safe_name}}}={load_aware_compute_cost:.8f}"
                    )
                    metrics_emitted += 1
                if cost_per_pixel is not None:
                    output.append(
                        f"cost_per_pixel{{{labels}}} {cost_per_pixel:.12f}"
                    )
                    logger.debug(
                        f"Set cost_per_pixel{{{safe_name}}}={cost_per_pixel:.12f}"
                    )
                    metrics_emitted += 1
                if cost_per_watch_hour is not None:
                    output.append(
                        f"cost_per_watch_hour{{{labels}}} {cost_per_watch_hour:.8f}"
                    )
                    logger.debug(
                        f"Set cost_per_watch_hour{{{safe_name}}}={cost_per_watch_hour:.8f}"
                    )
                    metrics_emitted += 1
            else:
                scenarios_without_data += 1
                logger.debug(
                    f"Scenario '{scenario_name}': No load-aware data available, "
                    f"emitting metrics with value 0"
                )
                
                # Emit metrics with value 0 instead of skipping
                output.append(f"cost_total_load_aware{{{labels}}} 0")
                logger.debug(f"Set cost_total_load_aware{{{safe_name}}}=0 (no data)")
                metrics_emitted += 1
                
                output.append(f"cost_energy_load_aware{{{labels}}} 0")
                logger.debug(f"Set cost_energy_load_aware{{{safe_name}}}=0 (no data)")
                metrics_emitted += 1
                
                output.append(f"cost_compute_load_aware{{{labels}}} 0")
                logger.debug(f"Set cost_compute_load_aware{{{safe_name}}}=0 (no data)")
                metrics_emitted += 1
                
                output.append(f"cost_per_pixel{{{labels}}} 0")
                logger.debug(f"Set cost_per_pixel{{{safe_name}}}=0 (no data)")
                metrics_emitted += 1
                
                output.append(f"cost_per_watch_hour{{{labels}}} 0")
                logger.debug(f"Set cost_per_watch_hour{{{safe_name}}}=0 (no data)")
                metrics_emitted += 1
        
        logger.info(
            f"Metrics generation complete: {metrics_emitted} metrics emitted, "
            f"{scenarios_with_data} scenarios with data, "
            f"{scenarios_without_data} scenarios without data"
        )
        
        result = '\n'.join(output) + '\n'
        
        # Update cache
        self.metrics_cache['output'] = result
        self.last_update = current_time
        
        return result


class MetricsHandler(BaseHTTPRequestHandler):
    """HTTP request handler for Prometheus metrics."""
    
    exporter = None
    
    def do_GET(self):
        """Handle GET requests."""
        if self.path == '/metrics':
            try:
                metrics = self.exporter.generate_prometheus_metrics()
                
                self.send_response(200)
                self.send_header('Content-Type', 'text/plain; version=0.0.4')
                self.end_headers()
                self.wfile.write(metrics.encode())
            
            except Exception as e:
                logger.error(f"Error generating metrics: {e}")
                self.send_response(500)
                self.end_headers()
                self.wfile.write(b'Internal Server Error\n')
        
        elif self.path == '/health':
            self.send_response(200)
            self.send_header('Content-Type', 'text/plain')
            self.end_headers()
            self.wfile.write(b'OK\n')
        
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b'Not Found\n')
    
    def log_message(self, format, *args):
        """Suppress default logging."""
        pass


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='Cost Metrics Prometheus Exporter'
    )
    parser.add_argument(
        '--port',
        type=int,
        default=9503,
        help='Port to listen on (default: 9503)'
    )
    parser.add_argument(
        '--results-dir',
        type=Path,
        default=Path('./test_results'),
        help='Directory containing test results'
    )
    parser.add_argument(
        '--energy-cost',
        type=float,
        default=0.0,
        help='Energy cost per kWh (default: 0.0)'
    )
    parser.add_argument(
        '--cpu-cost',
        type=float,
        default=0.0,
        help='CPU/instance cost per hour (default: 0.0)'
    )
    parser.add_argument(
        '--gpu-cost',
        type=float,
        default=0.0,
        help='GPU cost per hour (default: 0.0)'
    )
    parser.add_argument(
        '--currency',
        type=str,
        default='USD',
        help='Currency code (default: USD)'
    )
    parser.add_argument(
        '--prometheus-url',
        type=str,
        default=None,
        help='Prometheus URL for load-aware mode (e.g., http://prometheus:9090)'
    )
    parser.add_argument(
        '--disable-load-aware',
        action='store_true',
        help='Disable load-aware calculations (use legacy duration-based)'
    )
    
    args = parser.parse_args()
    
    # Validate results directory
    if not args.results_dir.exists():
        logger.error(f"Results directory not found: {args.results_dir}")
        return 1
    
    # Create exporter
    exporter = CostMetricsExporter(
        results_dir=args.results_dir,
        energy_cost_per_kwh=args.energy_cost,
        cpu_cost_per_hour=args.cpu_cost,
        gpu_cost_per_hour=args.gpu_cost,
        currency=args.currency,
        prometheus_url=args.prometheus_url,
        use_load_aware=not args.disable_load_aware
    )
    MetricsHandler.exporter = exporter
    
    # Start HTTP server
    server = HTTPServer(('0.0.0.0', args.port), MetricsHandler)
    
    logger.info(f"Cost Metrics Exporter started on port {args.port}")
    logger.info(f"Pricing: {args.energy_cost} {args.currency}/kWh")
    logger.info(f"         {args.cpu_cost} {args.currency}/h (CPU)")
    logger.info(f"         {args.gpu_cost} {args.currency}/h (GPU)")
    if args.prometheus_url:
        logger.info(f"Prometheus: {args.prometheus_url} (load-aware mode)")
    logger.info(f"Metrics endpoint: http://localhost:{args.port}/metrics")
    logger.info(f"Health endpoint: http://localhost:{args.port}/health")
    
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Shutting down...")
        server.shutdown()
        return 0


if __name__ == '__main__':
    import sys
    sys.exit(main())
