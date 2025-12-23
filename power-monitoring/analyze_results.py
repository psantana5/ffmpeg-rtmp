#!/usr/bin/env python3
"""
Automated analysis of test results
Queries Prometheus and generates comprehensive reports
"""

import requests
import json
import statistics
import sys
import logging
from pathlib import Path
from typing import Dict, List, Optional
import csv

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class PrometheusClient:
    """Client for querying Prometheus API"""
    
    def __init__(self, base_url: str = 'http://localhost:9090'):
        self.base_url = base_url
        
    def query_range(self, query: str, start: float, end: float, step: str = '15s') -> Optional[Dict]:
        """Execute range query"""
        url = f"{self.base_url}/api/v1/query_range"
        params = {
            'query': query,
            'start': int(start),
            'end': int(end),
            'step': step
        }
        
        try:
            response = requests.get(url, params=params, timeout=30)
            response.raise_for_status()
            return response.json()
        except Exception as e:
            logger.error(f"Error querying Prometheus range: {e}")
            return None


class ResultsAnalyzer:
    """Analyzes test results and generates reports"""
    
    def __init__(self, results_file: Path, prometheus_url: str = 'http://localhost:9090'):
        self.results_file = results_file
        self.client = PrometheusClient(prometheus_url)
        
        with open(results_file) as f:
            self.data = json.load(f)
        
        self.scenarios = self.data.get('scenarios', [])
        logger.info(f"Loaded {len(self.scenarios)} scenarios from {results_file}")
    
    def get_metric_stats(self, data: Optional[Dict]) -> Optional[Dict]:
        """Calculate statistics from metric data"""
        if not data or 'data' not in data or 'result' not in data['data']:
            return None
        
        results = data['data']['result']
        if not results:
            return None
        
        values = []
        for result in results:
            if 'values' in result:
                values.extend([float(v[1]) for v in result['values']])
            elif 'value' in result:
                values.append(float(result['value'][1]))
        
        if not values:
            return None
        
        return {
            'mean': statistics.mean(values),
            'median': statistics.median(values),
            'stdev': statistics.stdev(values) if len(values) > 1 else 0,
            'min': min(values),
            'max': max(values),
            'samples': len(values)
        }
    
    def analyze_scenario(self, scenario: Dict) -> Dict:
        """Analyze a single scenario"""
        name = scenario['name']
        start = scenario['start_time']
        end = scenario['end_time']
        
        if not start or not end:
            logger.warning(f"Scenario '{name}' has no timestamps, skipping")
            return {}
        
        logger.info(f"Analyzing scenario: {name}")
        
        analysis = {
            'name': name,
            'bitrate': scenario.get('bitrate', 'N/A'),
            'resolution': scenario.get('resolution', 'N/A'),
            'fps': scenario.get('fps', 'N/A'),
            'duration': end - start
        }
        
        # Query power consumption
        power_query = 'sum(rapl_power_watts{zone=~"package.*"})'
        power_data = self.client.query_range(power_query, start, end)
        power_stats = self.get_metric_stats(power_data)
        
        if power_stats:
            analysis['power'] = {
                'mean_watts': round(power_stats['mean'], 2),
                'median_watts': round(power_stats['median'], 2),
                'min_watts': round(power_stats['min'], 2),
                'max_watts': round(power_stats['max'], 2),
                'stdev_watts': round(power_stats['stdev'], 2),
                'total_energy_joules': round(power_stats['mean'] * (end - start), 2),
                'total_energy_wh': round((power_stats['mean'] * (end - start)) / 3600, 4)
            }
        
        # Query DRAM power
        dram_query = 'sum(rapl_power_watts{zone=~".*dram.*"})'
        dram_data = self.client.query_range(dram_query, start, end)
        dram_stats = self.get_metric_stats(dram_data)
        
        if dram_stats:
            analysis['dram_power'] = {
                'mean_watts': round(dram_stats['mean'], 2),
                'total_energy_wh': round((dram_stats['mean'] * (end - start)) / 3600, 4)
            }
        
        # Query Docker overhead
        docker_query = 'docker_engine_cpu_percent'
        docker_data = self.client.query_range(docker_query, start, end)
        docker_stats = self.get_metric_stats(docker_data)
        
        if docker_stats and power_stats:
            docker_watts = (docker_stats['mean'] / 100) * power_stats['mean']
            analysis['docker_overhead'] = {
                'cpu_percent': round(docker_stats['mean'], 2),
                'estimated_watts': round(docker_watts, 2),
                'percentage_of_total': round((docker_watts / power_stats['mean']) * 100, 2)
            }
        
        # Query container CPU
        container_query = 'docker_containers_total_cpu_percent'
        container_data = self.client.query_range(container_query, start, end)
        container_stats = self.get_metric_stats(container_data)
        
        if container_stats and power_stats:
            container_watts = (container_stats['mean'] / 100) * power_stats['mean']
            analysis['container_usage'] = {
                'cpu_percent': round(container_stats['mean'], 2),
                'estimated_watts': round(container_watts, 2)
            }
        
        return analysis
    
    def generate_report(self) -> List[Dict]:
        """Generate full analysis report"""
        logger.info("Generating analysis report...")
        
        results = []
        
        for scenario in self.scenarios:
            analysis = self.analyze_scenario(scenario)
            if analysis:
                results.append(analysis)
        
        return results
    
    def print_summary(self, results: List[Dict]):
        """Print summary to console"""
        print("\n" + "=" * 100)
        print("STREAMING ENERGY CONSUMPTION ANALYSIS REPORT")
        print("=" * 100)
        
        if not results:
            print("No results to display")
            return
        
        # Print detailed results
        for result in results:
            print(f"\n{'─' * 100}")
            print(f"Scenario: {result['name']}")
            print(f"  Configuration: {result['bitrate']} @ {result['resolution']} {result['fps']}fps")
            print(f"  Duration: {result['duration']:.1f}s")
            
            if 'power' in result:
                p = result['power']
                print(f"\n  Power Consumption:")
                print(f"    Mean:   {p['mean_watts']:>8.2f} W")
                print(f"    Median: {p['median_watts']:>8.2f} W")
                print(f"    Min:    {p['min_watts']:>8.2f} W")
                print(f"    Max:    {p['max_watts']:>8.2f} W")
                print(f"    StdDev: {p['stdev_watts']:>8.2f} W")
                print(f"    Total Energy: {p['total_energy_wh']:.4f} Wh ({p['total_energy_joules']:.0f} J)")
            
            if 'dram_power' in result:
                d = result['dram_power']
                print(f"\n  DRAM Power:")
                print(f"    Mean: {d['mean_watts']:.2f} W")
                print(f"    Total Energy: {d['total_energy_wh']:.4f} Wh")
            
            if 'docker_overhead' in result:
                do = result['docker_overhead']
                print(f"\n  Docker Engine Overhead:")
                print(f"    CPU Usage: {do['cpu_percent']:.2f}%")
                print(f"    Power: {do['estimated_watts']:.2f} W ({do['percentage_of_total']:.1f}% of total)")
            
            if 'container_usage' in result:
                cu = result['container_usage']
                print(f"\n  Container Usage:")
                print(f"    CPU: {cu['cpu_percent']:.2f}%")
                print(f"    Estimated Power: {cu['estimated_watts']:.2f} W")
        
        # Comparison table
        print(f"\n{'=' * 100}")
        print("COMPARISON TABLE")
        print("=" * 100)
        
        # Filter scenarios with power data
        power_results = [r for r in results if 'power' in r]
        
        if power_results:
            print(f"\n{'Scenario':<30} {'Bitrate':<12} {'Mean Power':<12} {'Energy (Wh)':<14} {'Docker OH':<12}")
            print("─" * 100)
            
            for r in power_results:
                docker_oh = f"{r['docker_overhead']['percentage_of_total']:.1f}%" if 'docker_overhead' in r else "N/A"
                print(f"{r['name']:<30} {r['bitrate']:<12} "
                      f"{r['power']['mean_watts']:>10.2f} W "
                      f"{r['power']['total_energy_wh']:>12.4f} Wh "
                      f"{docker_oh:>10}")
            
            # Calculate relative differences from baseline
            baseline = next((r for r in power_results if 'baseline' in r['name'].lower()), None)
            
            if baseline and len(power_results) > 1:
                print(f"\n{'─' * 100}")
                print(f"RELATIVE TO BASELINE ({baseline['name']})")
                print("─" * 100)
                print(f"{'Scenario':<30} {'Power Diff':<15} {'Energy Diff':<15} {'% Increase':<12}")
                print("─" * 100)
                
                for r in power_results:
                    if r['name'] == baseline['name']:
                        continue
                    
                    power_diff = r['power']['mean_watts'] - baseline['power']['mean_watts']
                    energy_diff = r['power']['total_energy_wh'] - baseline['power']['total_energy_wh']
                    pct_increase = (power_diff / baseline['power']['mean_watts']) * 100
                    
                    print(f"{r['name']:<30} {power_diff:>+13.2f} W "
                          f"{energy_diff:>+13.4f} Wh "
                          f"{pct_increase:>+10.1f}%")
        
        print("\n" + "=" * 100 + "\n")
    
    def export_csv(self, output_file: str = None):
        """Export results to CSV"""
        if output_file is None:
            output_file = self.results_file.parent / f"{self.results_file.stem}_analysis.csv"
        
        results = self.generate_report()
        
        if not results:
            logger.warning("No results to export")
            return
        
        with open(output_file, 'w', newline='') as f:
            writer = csv.DictWriter(f, fieldnames=[
                'name', 'bitrate', 'resolution', 'fps', 'duration',
                'mean_power_w', 'median_power_w', 'total_energy_wh',
                'docker_overhead_w', 'docker_overhead_pct',
                'container_cpu_pct', 'container_power_w'
            ])
            
            writer.writeheader()
            
            for r in results:
                row = {
                    'name': r['name'],
                    'bitrate': r['bitrate'],
                    'resolution': r['resolution'],
                    'fps': r['fps'],
                    'duration': r['duration']
                }
                
                if 'power' in r:
                    row['mean_power_w'] = r['power']['mean_watts']
                    row['median_power_w'] = r['power']['median_watts']
                    row['total_energy_wh'] = r['power']['total_energy_wh']
                
                if 'docker_overhead' in r:
                    row['docker_overhead_w'] = r['docker_overhead']['estimated_watts']
                    row['docker_overhead_pct'] = r['docker_overhead']['percentage_of_total']
                
                if 'container_usage' in r:
                    row['container_cpu_pct'] = r['container_usage']['cpu_percent']
                    row['container_power_w'] = r['container_usage']['estimated_watts']
                
                writer.writerow(row)
        
        logger.info(f"CSV exported to {output_file}")


def main():
    if len(sys.argv) < 2:
        # Find most recent results file
        results_dir = Path('./test_results')
        if results_dir.exists():
            results_files = sorted(results_dir.glob('test_results_*.json'), reverse=True)
            if results_files:
                results_file = results_files[0]
                logger.info(f"Using most recent results file: {results_file}")
            else:
                logger.error("No results files found in ./test_results")
                print("Usage: python3 analyze_results.py [results_file.json]")
                return 1
        else:
            logger.error("No results directory found")
            print("Usage: python3 analyze_results.py [results_file.json]")
            return 1
    else:
        results_file = Path(sys.argv[1])
        if not results_file.exists():
            logger.error(f"Results file not found: {results_file}")
            return 1
    
    try:
        analyzer = ResultsAnalyzer(results_file)
        results = analyzer.generate_report()
        analyzer.print_summary(results)
        analyzer.export_csv()
        
        return 0
    except Exception as e:
        logger.error(f"Analysis failed: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    exit(main())
