#!/usr/bin/env python3
"""
QoE Metrics Prometheus Exporter

Exports video quality metrics (VMAF, PSNR) and QoE-aware efficiency scores
as Prometheus metrics for Grafana visualization.

Metrics exported:
- qoe_vmaf_score: VMAF quality score (0-100)
- qoe_psnr_score: PSNR quality score (dB)
- qoe_quality_per_watt: Quality per watt efficiency
- qoe_efficiency_score: QoE efficiency score (quality-weighted pixels per joule)
- qoe_computation_duration_seconds: Time taken to compute quality metrics

Usage:
    python3 qoe_exporter.py --port 9502
"""

import argparse
import json
import logging
import time
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Dict, List

from advisor.scoring import EnergyEfficiencyScorer

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class QoEMetricsExporter:
    """
    Exports QoE metrics as Prometheus metrics.
    
    Tracks quality scores and efficiency metrics for transcoding scenarios.
    """
    
    def __init__(self, results_dir: Path):
        """
        Initialize QoE metrics exporter.
        
        Args:
            results_dir: Directory containing test results
        """
        self.results_dir = results_dir
        self.scorer = EnergyEfficiencyScorer(algorithm='auto')
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
    
    def compute_qoe_metrics(self, scenario: Dict) -> Dict:
        """
        Compute QoE metrics for a scenario.
        
        Args:
            scenario: Scenario dict
            
        Returns:
            Dict with QoE metrics
        """
        metrics = {
            'vmaf_score': scenario.get('vmaf_score'),
            'psnr_score': scenario.get('psnr_score'),
            'quality_per_watt': None,
            'qoe_efficiency_score': None,
            'computation_duration': 0.0
        }
        
        # Compute quality-per-watt if VMAF is available
        if metrics['vmaf_score'] is not None:
            scorer_qpw = EnergyEfficiencyScorer(algorithm='quality_per_watt')
            metrics['quality_per_watt'] = scorer_qpw.compute_score(scenario)
        
        # Compute QoE efficiency score
        if metrics['vmaf_score'] is not None:
            scorer_qoe = EnergyEfficiencyScorer(algorithm='qoe_efficiency_score')
            metrics['qoe_efficiency_score'] = scorer_qoe.compute_score(scenario)
        
        return metrics
    
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
        output.append("# HELP qoe_vmaf_score VMAF quality score (0-100)")
        output.append("# TYPE qoe_vmaf_score gauge")
        output.append("# HELP qoe_psnr_score PSNR quality score (dB)")
        output.append("# TYPE qoe_psnr_score gauge")
        output.append("# HELP qoe_quality_per_watt Quality per watt efficiency (VMAF/W)")
        output.append("# TYPE qoe_quality_per_watt gauge")
        output.append(
            "# HELP qoe_efficiency_score QoE efficiency score "
            "(quality-weighted pixels/joule)"
        )
        output.append("# TYPE qoe_efficiency_score gauge")
        output.append(
            "# HELP qoe_computation_duration_seconds Time to compute quality metrics"
        )
        output.append("# TYPE qoe_computation_duration_seconds gauge")
        
        # Export metrics for each scenario
        for scenario in scenarios:
            scenario_name = scenario.get('name', 'unknown')
            
            # Sanitize name for Prometheus labels
            safe_name = scenario_name.replace(' ', '_').replace('"', '')
            
            # Compute QoE metrics
            qoe_metrics = self.compute_qoe_metrics(scenario)
            
            # Build labels
            labels = f'scenario="{safe_name}"'
            
            # Export VMAF
            if qoe_metrics['vmaf_score'] is not None:
                output.append(
                    f"qoe_vmaf_score{{{labels}}} {qoe_metrics['vmaf_score']:.2f}"
                )
            
            # Export PSNR
            if qoe_metrics['psnr_score'] is not None:
                output.append(
                    f"qoe_psnr_score{{{labels}}} {qoe_metrics['psnr_score']:.2f}"
                )
            
            # Export quality per watt
            if qoe_metrics['quality_per_watt'] is not None:
                output.append(
                    f"qoe_quality_per_watt{{{labels}}} "
                    f"{qoe_metrics['quality_per_watt']:.6f}"
                )
            
            # Export QoE efficiency score
            if qoe_metrics['qoe_efficiency_score'] is not None:
                output.append(
                    f"qoe_efficiency_score{{{labels}}} "
                    f"{qoe_metrics['qoe_efficiency_score']:.4e}"
                )
            
            # Export computation duration
            output.append(
                f"qoe_computation_duration_seconds{{{labels}}} "
                f"{qoe_metrics['computation_duration']:.3f}"
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
        description='QoE Metrics Prometheus Exporter'
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
    
    args = parser.parse_args()
    
    # Validate results directory
    if not args.results_dir.exists():
        logger.error(f"Results directory not found: {args.results_dir}")
        return 1
    
    # Create exporter
    exporter = QoEMetricsExporter(results_dir=args.results_dir)
    MetricsHandler.exporter = exporter
    
    # Start HTTP server
    server = HTTPServer(('0.0.0.0', args.port), MetricsHandler)
    
    logger.info(f"QoE Metrics Exporter started on port {args.port}")
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
