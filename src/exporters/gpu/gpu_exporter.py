#!/usr/bin/env python3
"""
GPU Power Monitoring Exporter for Prometheus

Exports NVIDIA GPU power consumption metrics using nvidia-smi.
Metrics include power draw, temperature, utilization, and memory usage.

Requirements:
- NVIDIA GPU with driver installed
- nvidia-smi command available
"""

import logging
import subprocess
import time
import xml.etree.ElementTree as ET
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Dict, List, Optional

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)


class NvidiaSMIReader:
    """Reads GPU metrics from nvidia-smi."""

    def __init__(self):
        """Initialize NVIDIA SMI reader."""
        self.available = self._check_availability()
        if not self.available:
            logger.warning("nvidia-smi not available - GPU monitoring disabled")

    def _check_availability(self) -> bool:
        """Check if nvidia-smi is available."""
        try:
            result = subprocess.run(
                ['nvidia-smi', '-L'],
                capture_output=True,
                timeout=5
            )
            return result.returncode == 0
        except (subprocess.SubprocessError, FileNotFoundError):
            return False

    def get_gpu_metrics(self) -> List[Dict]:
        """
        Query GPU metrics using nvidia-smi.

        Returns:
            List of dicts with GPU metrics per GPU
        """
        if not self.available:
            return []

        try:
            # Query nvidia-smi with XML output for structured data
            result = subprocess.run(
                [
                    'nvidia-smi',
                    '-q',
                    '-x'  # XML format
                ],
                capture_output=True,
                text=True,
                timeout=10
            )

            if result.returncode != 0:
                logger.error(f"nvidia-smi query failed: {result.stderr}")
                return []

            # Parse XML output
            root = ET.fromstring(result.stdout)
            gpus = []

            for gpu_elem in root.findall('gpu'):
                gpu_id = gpu_elem.get('id', '0')

                # Extract metrics with safe defaults
                def get_text(path: str, default: str = '0') -> str:
                    elem = gpu_elem.find(path)
                    return elem.text if elem is not None and elem.text else default

                def get_float(path: str, default: float = 0.0) -> float:
                    text = get_text(path)
                    try:
                        # Remove units like 'W', 'MiB', 'C', '%'
                        text = text.split()[0] if ' ' in text else text
                        return float(text)
                    except (ValueError, IndexError):
                        return default

                gpu_data = {
                    'id': gpu_id,
                    'name': get_text('product_name', 'Unknown'),
                    'uuid': get_text('uuid', 'unknown'),
                    # Power metrics
                    'power_draw_watts': get_float('gpu_power_readings/power_draw'),
                    'power_limit_watts': get_float('gpu_power_readings/power_limit'),
                    # Temperature
                    'temperature_celsius': get_float('temperature/gpu_temp'),
                    # Utilization
                    'utilization_gpu_percent': get_float('utilization/gpu_util'),
                    'utilization_memory_percent': get_float('utilization/memory_util'),
                    'utilization_encoder_percent': get_float('utilization/encoder_util'),
                    'utilization_decoder_percent': get_float('utilization/decoder_util'),
                    # Memory
                    'memory_used_mb': get_float('fb_memory_usage/used'),
                    'memory_total_mb': get_float('fb_memory_usage/total'),
                    # Clock speeds
                    'clocks_graphics_mhz': get_float('clocks/graphics_clock'),
                    'clocks_sm_mhz': get_float('clocks/sm_clock'),
                    'clocks_memory_mhz': get_float('clocks/mem_clock'),
                }

                gpus.append(gpu_data)

            return gpus

        except (subprocess.SubprocessError, ET.ParseError) as e:
            logger.error(f"Error querying GPU metrics: {e}")
            return []


class GPUMetricsExporter:
    """Exports GPU metrics as Prometheus metrics."""

    def __init__(self):
        """Initialize GPU metrics exporter."""
        self.nvidia_smi = NvidiaSMIReader()
        self.metrics_cache = {}
        self.last_update = 0
        self.cache_ttl = 5  # Cache for 5 seconds

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

        output = []

        # Add alive metric for health check
        output.append("# HELP gpu_exporter_alive GPU exporter health check (always 1)")
        output.append("# TYPE gpu_exporter_alive gauge")
        output.append("gpu_exporter_alive 1")

        # Get GPU metrics
        gpus = self.nvidia_smi.get_gpu_metrics()

        if not gpus:
            # No GPUs available - return minimal metrics
            output.append("# HELP gpu_count Number of GPUs detected")
            output.append("# TYPE gpu_count gauge")
            output.append("gpu_count 0")
            result = '\n'.join(output) + '\n'
            self.metrics_cache['output'] = result
            self.last_update = current_time
            return result

        # GPU count
        output.append("# HELP gpu_count Number of GPUs detected")
        output.append("# TYPE gpu_count gauge")
        output.append(f"gpu_count {len(gpus)}")

        # Define all metrics
        output.append("# HELP gpu_power_draw_watts GPU power draw in watts")
        output.append("# TYPE gpu_power_draw_watts gauge")
        output.append("# HELP gpu_power_limit_watts GPU power limit in watts")
        output.append("# TYPE gpu_power_limit_watts gauge")
        output.append("# HELP gpu_temperature_celsius GPU temperature in Celsius")
        output.append("# TYPE gpu_temperature_celsius gauge")
        output.append("# HELP gpu_utilization_percent GPU utilization percentage")
        output.append("# TYPE gpu_utilization_percent gauge")
        output.append("# HELP gpu_memory_utilization_percent GPU memory utilization percentage")
        output.append("# TYPE gpu_memory_utilization_percent gauge")
        output.append("# HELP gpu_encoder_utilization_percent GPU encoder utilization percentage")
        output.append("# TYPE gpu_encoder_utilization_percent gauge")
        output.append("# HELP gpu_decoder_utilization_percent GPU decoder utilization percentage")
        output.append("# TYPE gpu_decoder_utilization_percent gauge")
        output.append("# HELP gpu_memory_used_bytes GPU memory used in bytes")
        output.append("# TYPE gpu_memory_used_bytes gauge")
        output.append("# HELP gpu_memory_total_bytes GPU total memory in bytes")
        output.append("# TYPE gpu_memory_total_bytes gauge")

        # Export metrics for each GPU
        for gpu in gpus:
            gpu_id = gpu['id']
            gpu_name = gpu['name']
            gpu_uuid = gpu['uuid']

            labels = f'gpu_id="{gpu_id}",gpu_name="{gpu_name}",gpu_uuid="{gpu_uuid}"'

            output.append(f"gpu_power_draw_watts{{{labels}}} {gpu['power_draw_watts']:.2f}")
            output.append(f"gpu_power_limit_watts{{{labels}}} {gpu['power_limit_watts']:.2f}")
            output.append(f"gpu_temperature_celsius{{{labels}}} {gpu['temperature_celsius']:.1f}")
            output.append(f"gpu_utilization_percent{{{labels}}} {gpu['utilization_gpu_percent']:.1f}")
            output.append(f"gpu_memory_utilization_percent{{{labels}}} {gpu['utilization_memory_percent']:.1f}")
            output.append(f"gpu_encoder_utilization_percent{{{labels}}} {gpu['utilization_encoder_percent']:.1f}")
            output.append(f"gpu_decoder_utilization_percent{{{labels}}} {gpu['utilization_decoder_percent']:.1f}")
            output.append(f"gpu_memory_used_bytes{{{labels}}} {gpu['memory_used_mb'] * 1024 * 1024:.0f}")
            output.append(f"gpu_memory_total_bytes{{{labels}}} {gpu['memory_total_mb'] * 1024 * 1024:.0f}")

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
    import argparse

    parser = argparse.ArgumentParser(description='GPU Power Metrics Prometheus Exporter')
    parser.add_argument('--port', type=int, default=9505, help='Port to listen on (default: 9505)')

    args = parser.parse_args()

    # Create exporter
    exporter = GPUMetricsExporter()
    MetricsHandler.exporter = exporter

    # Start HTTP server
    server = HTTPServer(('0.0.0.0', args.port), MetricsHandler)

    logger.info(f"GPU Metrics Exporter started on port {args.port}")
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
