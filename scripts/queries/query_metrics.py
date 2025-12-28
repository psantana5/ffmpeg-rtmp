#!/usr/bin/env python3
"""
Query Metrics - Query Prometheus metrics with time ranges.

Simple utility to query Prometheus metrics from command line.
"""

import argparse
import json
import logging
import statistics
import sys
import time
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
        description='Query Prometheus metrics',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Instant query
  python3 query_metrics.py --query 'rapl_power_watts'
  
  # Range query (last hour)
  python3 query_metrics.py --query 'rapl_power_watts' --range 1h
  
  # Range query with custom times
  python3 query_metrics.py --query 'rapl_power_watts' --start 1234567890 --end 1234567900
  
  # JSON output
  python3 query_metrics.py --query 'up' --json
        """
    )
    
    parser.add_argument(
        '--query',
        required=True,
        help='PromQL query string'
    )
    
    parser.add_argument(
        '--prometheus',
        default='http://localhost:9090',
        help='Prometheus server URL (default: http://localhost:9090)'
    )
    
    parser.add_argument(
        '--range',
        help='Query range (e.g., 1h, 30m, 24h)'
    )
    
    parser.add_argument(
        '--start',
        type=float,
        help='Start timestamp (unix time)'
    )
    
    parser.add_argument(
        '--end',
        type=float,
        help='End timestamp (unix time)'
    )
    
    parser.add_argument(
        '--step',
        default='15s',
        help='Query step for range queries (default: 15s)'
    )
    
    parser.add_argument(
        '--json',
        action='store_true',
        help='Output raw JSON'
    )
    
    parser.add_argument(
        '--stats',
        action='store_true',
        help='Show statistics (mean, min, max, etc.)'
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
    
    # Execute query
    if args.range or (args.start and args.end):
        # Range query
        if args.start and args.end:
            start = args.start
            end = args.end
        else:
            # Parse range
            end = time.time()
            range_str = args.range.lower()
            
            if range_str.endswith('h'):
                hours = int(range_str[:-1])
                start = end - (hours * 3600)
            elif range_str.endswith('m'):
                minutes = int(range_str[:-1])
                start = end - (minutes * 60)
            elif range_str.endswith('s'):
                seconds = int(range_str[:-1])
                start = end - seconds
            else:
                logger.error(f"Invalid range format: {args.range}")
                sys.exit(1)
        
        data = client.query_range(args.query, start, end, args.step)
    else:
        # Instant query
        data = client.query(args.query)
    
    if data is None:
        logger.error("Query failed")
        sys.exit(1)
    
    # Output
    if args.json:
        print(json.dumps(data, indent=2))
    else:
        # Human-readable output
        results = data.get('data', {}).get('result', [])
        
        print(f"\nQuery: {args.query}")
        print(f"Results: {len(results)}\n")
        
        for result in results:
            metric = result.get('metric', {})
            metric_str = json.dumps(metric)
            
            if 'value' in result:
                # Instant query
                timestamp, value = result['value']
                print(f"{metric_str}: {value}")
            
            elif 'values' in result:
                # Range query
                values = result['values']
                print(f"{metric_str}: {len(values)} samples")
                
                if args.stats:
                    value_floats = [float(v[1]) for v in values]
                    print(f"  Mean:   {statistics.mean(value_floats):.2f}")
                    print(f"  Median: {statistics.median(value_floats):.2f}")
                    print(f"  Min:    {min(value_floats):.2f}")
                    print(f"  Max:    {max(value_floats):.2f}")
                    if len(value_floats) > 1:
                        print(f"  Stdev:  {statistics.stdev(value_floats):.2f}")


if __name__ == '__main__':
    main()
