#!/usr/bin/env python3
"""
List Exporters - Display Prometheus exporters and their status.

Shows all configured Prometheus scrape targets with health status.
"""

import argparse
import json
import logging
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from scripts.utils.prometheus_client import PrometheusClient

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description='List Prometheus exporters and their status',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # List all exporters
  python3 list_exporters.py
  
  # JSON output
  python3 list_exporters.py --json
  
  # Use different Prometheus URL
  python3 list_exporters.py --prometheus http://prometheus:9090
        """
    )
    
    parser.add_argument(
        '--prometheus',
        default='http://localhost:9090',
        help='Prometheus server URL (default: http://localhost:9090)'
    )
    
    parser.add_argument(
        '--json',
        action='store_true',
        help='Output in JSON format'
    )
    
    parser.add_argument(
        '--verbose', '-v',
        action='store_true',
        help='Enable verbose logging'
    )
    
    args = parser.parse_args()
    
    if args.verbose:
        logging.getLogger().setLevel(logging.DEBUG)
    
    # Initialize client
    client = PrometheusClient(args.prometheus)
    
    # Check health
    if not client.health_check():
        logger.error(f"Prometheus server not reachable at {args.prometheus}")
        sys.exit(1)
    
    # Get targets
    targets = client.get_targets()
    
    if targets is None:
        logger.error("Failed to retrieve targets")
        sys.exit(1)
    
    if args.json:
        print(json.dumps({'targets': targets}, indent=2))
        sys.exit(0)
    
    # Human-readable output
    print(f"\n{'='*70}")
    print(f"PROMETHEUS EXPORTERS")
    print(f"{'='*70}")
    print(f"Server: {args.prometheus}")
    print(f"Total Targets: {len(targets)}")
    
    # Group by job
    jobs = {}
    for target in targets:
        job = target.get('labels', {}).get('job', 'unknown')
        if job not in jobs:
            jobs[job] = []
        jobs[job].append(target)
    
    for job_name in sorted(jobs.keys()):
        job_targets = jobs[job_name]
        print(f"\n{'-'*70}")
        print(f"Job: {job_name} ({len(job_targets)} target(s))")
        print(f"{'-'*70}")
        
        for target in job_targets:
            url = target.get('scrapeUrl', 'N/A')
            health = target.get('health', 'unknown')
            last_error = target.get('lastError', '')
            
            status_symbol = '✓' if health == 'up' else '✗'
            print(f"  {status_symbol} {url:50s} [{health.upper()}]")
            
            if last_error:
                print(f"     Error: {last_error}")
    
    print(f"\n{'='*70}\n")


if __name__ == '__main__':
    main()
