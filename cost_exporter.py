#!/usr/bin/env python3
"""
Cost Metrics Prometheus Exporter

Exports cost analysis metrics as Prometheus metrics for Grafana visualization.

Supports two modes:
1. Legacy mode: Load costs from test_results JSON files (duration-based)
2. Load-aware mode (recommended): Query Prometheus for real-time CPU/power metrics

Metrics exported:
- cost_total: Total cost per scenario (energy + compute)
- cost_energy: Energy cost per scenario
- cost_compute: Compute cost per scenario (CPU + GPU)
- cost_per_pixel: Cost per pixel delivered
- cost_per_watch_hour: Cost per viewer watch hour

Usage:
    # Legacy mode (from test results)
    python3 cost_exporter.py --port 9504 --energy-cost 0.12 --cpu-cost 0.50
    
    # Load-aware mode (query Prometheus)
    python3 cost_exporter.py --port 9504 --prometheus-url http://prometheus:9090 \
        --energy-cost 0.12 --cpu-cost 0.50
"""

import argparse
import json
import logging
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
        
        try:
            with urlopen(req, timeout=30) as resp:
                return json.load(resp)
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
        
        return values


class CostMetricsExporter:
    """
    Exports cost metrics as Prometheus metrics.
    
    Supports two modes:
    1. Legacy: Read from test results JSON (duration-based costs)
    2. Load-aware: Query Prometheus for real CPU/power metrics
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
            logger.info("Using legacy mode (duration-based costs)")
    
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
        
        start_time = scenario.get('start_time')
        end_time = scenario.get('end_time')
        
        if not start_time or not end_time:
            logger.warning(f"Scenario '{scenario.get('name')}': Missing timestamps")
            return scenario
        
        step_seconds = 5  # 5 second resolution
        
        try:
            # Query CPU usage
            # Use rate() to get cores per second, then average over step interval
            cpu_query = 'rate(container_cpu_usage_seconds_total{name!~".*POD.*"}[30s])'
            cpu_response = self.prometheus_client.query_range(
                cpu_query, start_time, end_time, f'{step_seconds}s'
            )
            cpu_values = self.prometheus_client.extract_values(cpu_response)
            
            # Query power consumption
            # Sum all RAPL zones for total system power
            power_query = 'sum(rapl_power_watts)'
            power_response = self.prometheus_client.query_range(
                power_query, start_time, end_time, f'{step_seconds}s'
            )
            power_values = self.prometheus_client.extract_values(power_response)
            
            # Add to scenario
            if cpu_values:
                scenario['cpu_usage_cores'] = cpu_values
                scenario['step_seconds'] = step_seconds
                logger.debug(
                    f"Scenario '{scenario.get('name')}': "
                    f"Enriched with {len(cpu_values)} CPU measurements"
                )
            
            if power_values:
                scenario['power_watts'] = power_values
                logger.debug(
                    f"Scenario '{scenario.get('name')}': "
                    f"Enriched with {len(power_values)} power measurements"
                )
        
        except Exception as e:
            logger.error(
                f"Failed to enrich scenario '{scenario.get('name')}': {e}"
            )
        
        return scenario
    
    def generate_prometheus_metrics(self) -> str:
        """
        Generate Prometheus metrics in text format.
        
        Uses load-aware calculations if Prometheus data is available,
        otherwise falls back to legacy duration-based calculations.
        
        Returns:
            Prometheus metrics text
        """
        # Check cache
        current_time = time.time()
        if (current_time - self.last_update) < self.cache_ttl and self.metrics_cache:
            return self.metrics_cache.get('output', '')
        
        # Load scenarios
        scenarios = self.load_latest_results()
        
        # Enrich with Prometheus metrics if available
        if self.prometheus_client and self.use_load_aware:
            scenarios = [
                self.enrich_scenario_with_prometheus(s) for s in scenarios
            ]
        
        output = []
        
        # Metrics definitions
        currency = self.cost_model.currency
        output.append(f"# HELP cost_total Total cost ({currency})")
        output.append("# TYPE cost_total gauge")
        output.append(f"# HELP cost_energy Energy cost ({currency})")
        output.append("# TYPE cost_energy gauge")
        output.append(f"# HELP cost_compute Compute cost ({currency})")
        output.append("# TYPE cost_compute gauge")
        output.append(f"# HELP cost_per_pixel Cost per pixel ({currency})")
        output.append("# TYPE cost_per_pixel gauge")
        output.append(f"# HELP cost_per_watch_hour Cost per watch hour ({currency})")
        output.append("# TYPE cost_per_watch_hour gauge")
        
        # Export metrics for each scenario
        for scenario in scenarios:
            scenario_name = scenario.get('name', 'unknown')
            
            # Sanitize name for Prometheus labels
            safe_name = scenario_name.replace(' ', '_').replace('"', '')
            
            # Extract additional labels
            streams = scenario.get('streams', 1)
            bitrate = scenario.get('bitrate', '')
            encoder = scenario.get('encoder_type', 'unknown')
            
            # Build labels
            labels = (
                f'scenario="{safe_name}",'
                f'currency="{currency}",'
                f'streams="{streams}",'
                f'bitrate="{bitrate}",'
                f'encoder="{encoder}"'
            )
            
            # Decide which cost calculation method to use
            use_load_aware = (
                self.use_load_aware and 
                'cpu_usage_cores' in scenario and 
                'power_watts' in scenario
            )
            
            if use_load_aware:
                # Load-aware calculations
                total_cost = self.cost_model.compute_total_cost_load_aware(scenario)
                energy_cost = self.cost_model.compute_energy_cost_load_aware(scenario)
                compute_cost = self.cost_model.compute_compute_cost_load_aware(scenario)
                cost_per_pixel = self.cost_model.compute_cost_per_pixel_load_aware(scenario)
                
                # For watch hour, try to get viewer count from scenario
                viewers = scenario.get('viewers', 1)  # Default to 1 if not specified
                cost_per_watch_hour = self.cost_model.compute_cost_per_watch_hour_load_aware(
                    scenario, viewers=viewers
                )
            else:
                # Legacy calculations (duration-based)
                total_cost = self.cost_model.compute_total_cost(scenario)
                energy_cost = self.cost_model.compute_energy_cost(scenario)
                compute_cost = self.cost_model.compute_compute_cost(scenario)
                cost_per_pixel = self.cost_model.compute_cost_per_pixel(scenario)
                cost_per_watch_hour = self.cost_model.compute_cost_per_watch_hour(
                    scenario, viewers=1
                )
            
            # Export total cost
            if total_cost is not None:
                output.append(f"cost_total{{{labels}}} {total_cost:.8f}")
            
            # Export energy cost
            if energy_cost is not None:
                output.append(f"cost_energy{{{labels}}} {energy_cost:.8f}")
            
            # Export compute cost
            if compute_cost is not None:
                output.append(f"cost_compute{{{labels}}} {compute_cost:.8f}")
            
            # Export cost per pixel
            if cost_per_pixel is not None:
                output.append(f"cost_per_pixel{{{labels}}} {cost_per_pixel:.4e}")
            
            # Export cost per watch hour
            if cost_per_watch_hour is not None:
                output.append(
                    f"cost_per_watch_hour{{{labels}}} {cost_per_watch_hour:.8f}"
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
