#!/usr/bin/env python3
"""
Exporter Health Check Script

Periodically checks all exporters to ensure they are:
1. Responding to requests
2. Returning metrics
3. Returning relevant/fresh data

This script can be run:
- From inside an existing container (e.g., prometheus)
- As a standalone script on the host
- As a scheduled cron job

Usage:
    # Single check
    python3 check_exporters_health.py
    
    # Continuous monitoring (every 60 seconds)
    python3 check_exporters_health.py --interval 60
    
    # Export results as Prometheus metrics
    python3 check_exporters_health.py --port 9600
"""

import argparse
import json
import logging
import re
import sys
import time
from dataclasses import dataclass
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Dict, List, Optional, Tuple
from urllib.request import Request, urlopen
from urllib.error import URLError, HTTPError

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


@dataclass
class ExporterConfig:
    """Configuration for an exporter."""
    name: str
    url: str
    job_name: str
    expected_metrics: List[str]
    data_freshness_seconds: Optional[int] = None


# Define all exporters to check
EXPORTERS = [
    ExporterConfig(
        name="nginx-rtmp-exporter",
        url="http://nginx-exporter:9728/metrics",
        job_name="nginx-rtmp",
        expected_metrics=["nginx_rtmp_connections", "nginx_rtmp_streams"],
        data_freshness_seconds=30
    ),
    ExporterConfig(
        name="rapl-exporter",
        url="http://rapl-exporter:9500/metrics",
        job_name="rapl-power",
        expected_metrics=["rapl_power_watts", "rapl_energy_joules_total"],
        data_freshness_seconds=10
    ),
    ExporterConfig(
        name="docker-stats-exporter",
        url="http://docker-stats-exporter:9501/metrics",
        job_name="docker-stats",
        expected_metrics=["docker_engine_cpu_percent", "docker_container_cpu_percent"],
        data_freshness_seconds=15
    ),
    ExporterConfig(
        name="node-exporter",
        url="http://node-exporter:9100/metrics",
        job_name="node-exporter",
        expected_metrics=["node_cpu_seconds_total", "node_memory_MemAvailable_bytes"],
        data_freshness_seconds=10
    ),
    ExporterConfig(
        name="cadvisor",
        url="http://cadvisor:8080/metrics",
        job_name="cadvisor",
        expected_metrics=["container_cpu_usage_seconds_total", "container_memory_usage_bytes"],
        data_freshness_seconds=15
    ),
    ExporterConfig(
        name="dcgm-exporter",
        url="http://dcgm-exporter:9400/metrics",
        job_name="dcgm",
        expected_metrics=["DCGM_FI_DEV_GPU_UTIL", "DCGM_FI_DEV_POWER_USAGE"],
        data_freshness_seconds=10
    ),
    ExporterConfig(
        name="results-exporter",
        url="http://results-exporter:9502/metrics",
        job_name="results-exporter",
        expected_metrics=["scenario_", "scenario_duration_seconds"],
        data_freshness_seconds=60
    ),
    ExporterConfig(
        name="qoe-exporter",
        url="http://qoe-exporter:9503/metrics",
        job_name="qoe-exporter",
        expected_metrics=["qoe_", "quality_"],
        data_freshness_seconds=60
    ),
    ExporterConfig(
        name="cost-exporter",
        url="http://cost-exporter:9504/metrics",
        job_name="cost-exporter",
        expected_metrics=["cost_exporter_alive", "cost_total_load_aware", "cost_energy_load_aware"],
        data_freshness_seconds=60
    ),
]


@dataclass
class HealthCheckResult:
    """Result of a health check."""
    exporter_name: str
    timestamp: datetime
    is_reachable: bool
    has_metrics: bool
    has_expected_metrics: bool
    has_data: bool
    metric_count: int
    sample_count: int
    error_message: Optional[str] = None
    missing_metrics: List[str] = None
    
    @property
    def is_healthy(self) -> bool:
        """Check if exporter is healthy."""
        return self.is_reachable and self.has_metrics and self.has_data


class ExporterHealthChecker:
    """Checks health of Prometheus exporters."""
    
    def __init__(self, exporters: List[ExporterConfig], timeout: int = 10, use_unicode: bool = True):
        """
        Initialize health checker.
        
        Args:
            exporters: List of exporters to check
            timeout: Request timeout in seconds
            use_unicode: Use Unicode characters in output (default: True)
        """
        self.exporters = exporters
        self.timeout = timeout
        self.use_unicode = use_unicode
        self.results: Dict[str, HealthCheckResult] = {}
    
    def fetch_metrics(self, url: str) -> Optional[str]:
        """
        Fetch metrics from an exporter.
        
        Args:
            url: Exporter metrics URL
        
        Returns:
            Metrics text or None on error
        """
        try:
            req = Request(url, headers={'Accept': 'text/plain'})
            with urlopen(req, timeout=self.timeout) as resp:
                return resp.read().decode('utf-8')
        except (URLError, HTTPError) as e:
            logger.debug(f"Failed to fetch {url}: {e}")
            return None
        except Exception as e:
            logger.error(f"Unexpected error fetching {url}: {e}")
            return None
    
    def parse_metrics(self, metrics_text: str) -> Tuple[int, int]:
        """
        Parse metrics to count metric names and samples.
        
        Args:
            metrics_text: Prometheus metrics text
        
        Returns:
            Tuple of (metric_count, sample_count)
        """
        if not metrics_text:
            return 0, 0
        
        metric_names = set()
        sample_count = 0
        
        for line in metrics_text.split('\n'):
            line = line.strip()
            
            # Skip comments and empty lines
            if not line or line.startswith('#'):
                continue
            
            # Extract metric name (before '{' or ' ')
            match = re.match(r'^([a-zA-Z_:][a-zA-Z0-9_:]*)', line)
            if match:
                metric_names.add(match.group(1))
                sample_count += 1
        
        return len(metric_names), sample_count
    
    def check_expected_metrics(
        self, metrics_text: str, expected_metrics: List[str]
    ) -> Tuple[bool, List[str]]:
        """
        Check if expected metrics are present.
        
        Args:
            metrics_text: Prometheus metrics text
            expected_metrics: List of expected metric names (or prefixes)
        
        Returns:
            Tuple of (all_found, missing_metrics)
        """
        if not metrics_text:
            return False, expected_metrics
        
        missing = []
        for expected in expected_metrics:
            # Support prefix matching for metric families
            if expected.endswith('_'):
                # Prefix match
                pattern = re.compile(f'^{re.escape(expected)}', re.MULTILINE)
                if not pattern.search(metrics_text):
                    missing.append(expected)
            else:
                # Exact match
                if f'{expected}{{' not in metrics_text and f'{expected} ' not in metrics_text:
                    missing.append(expected)
        
        return len(missing) == 0, missing
    
    def check_has_data(self, metrics_text: str) -> bool:
        """
        Check if metrics have actual data (non-zero samples).
        
        Args:
            metrics_text: Prometheus metrics text
        
        Returns:
            True if metrics contain data
        """
        if not metrics_text:
            return False
        
        # Look for non-comment lines with values
        for line in metrics_text.split('\n'):
            line = line.strip()
            if not line or line.startswith('#'):
                continue
            
            # If we find at least one metric line, we have data
            if ' ' in line:
                return True
        
        return False
    
    def check_exporter(self, config: ExporterConfig) -> HealthCheckResult:
        """
        Check health of a single exporter.
        
        Args:
            config: Exporter configuration
        
        Returns:
            Health check result
        """
        logger.debug(f"Checking {config.name}...")
        
        result = HealthCheckResult(
            exporter_name=config.name,
            timestamp=datetime.now(),
            is_reachable=False,
            has_metrics=False,
            has_expected_metrics=False,
            has_data=False,
            metric_count=0,
            sample_count=0
        )
        
        # Fetch metrics
        metrics_text = self.fetch_metrics(config.url)
        
        if metrics_text is None:
            result.error_message = f"Failed to reach {config.url}"
            status_icon = "X" if not self.use_unicode else "❌"
            logger.warning(f"{status_icon} {config.name}: Not reachable")
            return result
        
        result.is_reachable = True
        
        # Parse metrics
        metric_count, sample_count = self.parse_metrics(metrics_text)
        result.metric_count = metric_count
        result.sample_count = sample_count
        result.has_metrics = metric_count > 0
        
        if not result.has_metrics:
            result.error_message = "No metrics found"
            status_icon = "!" if not self.use_unicode else "⚠️"
            logger.warning(f"{status_icon} {config.name}: No metrics found")
            return result
        
        # Check for expected metrics
        has_expected, missing = self.check_expected_metrics(
            metrics_text, config.expected_metrics
        )
        result.has_expected_metrics = has_expected
        result.missing_metrics = missing
        
        if not has_expected:
            result.error_message = f"Missing metrics: {', '.join(missing)}"
            status_icon = "!" if not self.use_unicode else "⚠️"
            logger.warning(
                f"{status_icon} {config.name}: Missing expected metrics: {', '.join(missing)}"
            )
        
        # Check for data
        result.has_data = self.check_has_data(metrics_text)
        
        if not result.has_data:
            result.error_message = "No data in metrics"
            status_icon = "!" if not self.use_unicode else "⚠️"
            logger.warning(f"{status_icon} {config.name}: No data in metrics")
        
        # Log success
        if result.is_healthy:
            status_icon = "OK" if not self.use_unicode else "✓"
            logger.info(
                f"{status_icon} {config.name}: OK ({metric_count} metrics, {sample_count} samples)"
            )
        
        return result
    
    def check_all(self) -> Dict[str, HealthCheckResult]:
        """
        Check all exporters.
        
        Returns:
            Dictionary of exporter_name -> HealthCheckResult
        """
        logger.info("=" * 80)
        logger.info("Checking exporter health...")
        logger.info("=" * 80)
        
        results = {}
        for config in self.exporters:
            result = self.check_exporter(config)
            results[config.name] = result
        
        # Summary
        healthy_count = sum(1 for r in results.values() if r.is_healthy)
        total_count = len(results)
        
        logger.info("=" * 80)
        logger.info(f"Health check complete: {healthy_count}/{total_count} exporters healthy")
        logger.info("=" * 80)
        
        self.results = results
        return results
    
    def generate_prometheus_metrics(self) -> str:
        """
        Generate Prometheus metrics from health check results.
        
        Returns:
            Prometheus metrics text
        """
        output = []
        
        # Health status metrics
        output.append("# HELP exporter_health_status Health status of exporter (1=healthy, 0=unhealthy)")
        output.append("# TYPE exporter_health_status gauge")
        
        for name, result in self.results.items():
            status = 1 if result.is_healthy else 0
            output.append(f'exporter_health_status{{exporter="{name}"}} {status}')
        
        # Reachability metrics
        output.append("# HELP exporter_reachable Exporter is reachable (1=yes, 0=no)")
        output.append("# TYPE exporter_reachable gauge")
        
        for name, result in self.results.items():
            reachable = 1 if result.is_reachable else 0
            output.append(f'exporter_reachable{{exporter="{name}"}} {reachable}')
        
        # Metric count
        output.append("# HELP exporter_metric_count Number of unique metrics exposed")
        output.append("# TYPE exporter_metric_count gauge")
        
        for name, result in self.results.items():
            output.append(f'exporter_metric_count{{exporter="{name}"}} {result.metric_count}')
        
        # Sample count
        output.append("# HELP exporter_sample_count Number of metric samples exposed")
        output.append("# TYPE exporter_sample_count gauge")
        
        for name, result in self.results.items():
            output.append(f'exporter_sample_count{{exporter="{name}"}} {result.sample_count}')
        
        # Has data
        output.append("# HELP exporter_has_data Exporter has data (1=yes, 0=no)")
        output.append("# TYPE exporter_has_data gauge")
        
        for name, result in self.results.items():
            has_data = 1 if result.has_data else 0
            output.append(f'exporter_has_data{{exporter="{name}"}} {has_data}')
        
        return '\n'.join(output) + '\n'
    
    def print_summary(self):
        """Print a human-readable summary."""
        ok_icon = "OK" if not self.use_unicode else "✓ OK"
        fail_icon = "FAIL" if not self.use_unicode else "✗ FAIL"
        
        print()
        print("=" * 80)
        print("EXPORTER HEALTH SUMMARY")
        print("=" * 80)
        print(f"{'Exporter':<30} {'Status':<10} {'Metrics':<10} {'Samples':<10} {'Notes'}")
        print("-" * 80)
        
        for name, result in sorted(self.results.items()):
            status = ok_icon if result.is_healthy else fail_icon
            notes = result.error_message or ""
            
            print(
                f"{name:<30} {status:<10} {result.metric_count:<10} "
                f"{result.sample_count:<10} {notes}"
            )
        
        print("-" * 80)
        healthy = sum(1 for r in self.results.values() if r.is_healthy)
        total = len(self.results)
        print(f"Total: {healthy}/{total} healthy")
        print("=" * 80)


class HealthCheckMetricsHandler(BaseHTTPRequestHandler):
    """HTTP request handler for health check metrics."""
    
    checker = None
    
    def do_GET(self):
        """Handle GET requests."""
        if self.path == '/metrics':
            try:
                # Run health check
                self.checker.check_all()
                metrics = self.checker.generate_prometheus_metrics()
                
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
        description='Check health of Prometheus exporters'
    )
    parser.add_argument(
        '--interval',
        type=int,
        default=0,
        help='Check interval in seconds (0 for single check)'
    )
    parser.add_argument(
        '--port',
        type=int,
        default=0,
        help='Port to expose metrics (0 for no server)'
    )
    parser.add_argument(
        '--timeout',
        type=int,
        default=10,
        help='Request timeout in seconds (default: 10)'
    )
    parser.add_argument(
        '--debug',
        action='store_true',
        help='Enable debug logging'
    )
    parser.add_argument(
        '--no-unicode',
        action='store_true',
        help='Disable Unicode characters in output'
    )
    
    args = parser.parse_args()
    
    if args.debug:
        logger.setLevel(logging.DEBUG)
    
    # Create checker
    checker = ExporterHealthChecker(
        EXPORTERS, 
        timeout=args.timeout,
        use_unicode=not args.no_unicode
    )
    
    # Server mode
    if args.port > 0:
        HealthCheckMetricsHandler.checker = checker
        server = HTTPServer(('0.0.0.0', args.port), HealthCheckMetricsHandler)
        
        logger.info(f"Health Check Metrics Server started on port {args.port}")
        logger.info(f"Metrics endpoint: http://localhost:{args.port}/metrics")
        logger.info(f"Health endpoint: http://localhost:{args.port}/health")
        
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            logger.info("Shutting down...")
            server.shutdown()
            return 0
    
    # Single or continuous check mode
    if args.interval > 0:
        logger.info(f"Running continuous health checks (interval: {args.interval}s)")
        logger.info("Press Ctrl+C to stop")
        
        try:
            while True:
                checker.check_all()
                checker.print_summary()
                time.sleep(args.interval)
        except KeyboardInterrupt:
            logger.info("Stopping...")
            return 0
    else:
        # Single check
        checker.check_all()
        checker.print_summary()
        
        # Return non-zero if any exporter is unhealthy
        if all(r.is_healthy for r in checker.results.values()):
            return 0
        else:
            return 1


if __name__ == '__main__':
    sys.exit(main())
