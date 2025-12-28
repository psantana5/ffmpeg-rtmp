#!/usr/bin/env python3
"""
Cost Metrics Prometheus Exporter

Exports cost analysis metrics as Prometheus metrics for Grafana visualization.

Metrics exported:
- cost_total: Total cost per scenario (energy + compute)
- cost_energy: Energy cost per scenario
- cost_compute: Compute cost per scenario (CPU + GPU)
- cost_per_pixel: Cost per pixel delivered
- cost_per_watch_hour: Cost per viewer watch hour

Usage:
    python3 cost_exporter.py --port 9503 --energy-cost 0.12 --cpu-cost 0.50
"""

import argparse
import json
import logging
import time
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Dict, List

from advisor.cost import CostModel

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class CostMetricsExporter:
    """
    Exports cost metrics as Prometheus metrics.
    
    Tracks cost analysis for transcoding scenarios.
    """
    
    def __init__(
        self,
        results_dir: Path,
        energy_cost_per_kwh: float = 0.0,
        cpu_cost_per_hour: float = 0.0,
        gpu_cost_per_hour: float = 0.0,
        currency: str = 'USD'
    ):
        """
        Initialize cost metrics exporter.
        
        Args:
            results_dir: Directory containing test results
            energy_cost_per_kwh: Energy cost ($/kWh or €/kWh)
            cpu_cost_per_hour: CPU/instance cost ($/h or €/h)
            gpu_cost_per_hour: GPU cost ($/h or €/h)
            currency: Currency code
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
    
    def generate_prometheus_metrics(self) -> str:
        """
        Generate Prometheus metrics in text format.
        
        Returns:
            Prometheus metrics text
        """
        # Check cache
        current_time = time.time()
        if (current_time - self.last_update) < self.cache_ttl and self.metrics_cache:
            return self.metrics_cache.get('output', '')
        
        # Load scenarios
        scenarios = self.load_latest_results()
        
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
            
            # Build labels
            labels = f'scenario="{safe_name}",currency="{currency}"'
            
            # Compute cost metrics
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
        currency=args.currency
    )
    MetricsHandler.exporter = exporter
    
    # Start HTTP server
    server = HTTPServer(('0.0.0.0', args.port), MetricsHandler)
    
    logger.info(f"Cost Metrics Exporter started on port {args.port}")
    logger.info(f"Pricing: {args.energy_cost} {args.currency}/kWh")
    logger.info(f"         {args.cpu_cost} {args.currency}/h (CPU)")
    logger.info(f"         {args.gpu_cost} {args.currency}/h (GPU)")
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
