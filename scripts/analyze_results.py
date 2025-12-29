#!/usr/bin/env python3
"""
Automated analysis of test results
Queries Prometheus and generates comprehensive reports
Includes energy-aware transcoding recommendations
"""

import argparse
import csv
import json
import logging
import statistics
import sys
from pathlib import Path
from typing import Dict, List, Optional

# Add parent directory to path to allow imports from advisor package
sys.path.insert(0, str(Path(__file__).parent.parent))

import requests

from advisor import MultivariatePredictor, PowerPredictor, TranscodingRecommender

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class PrometheusClient:
    """Client for querying Prometheus API"""
    
    def __init__(self, base_url: str = 'http://localhost:9090'):
        self.base_url = base_url
        
    def query_range(
        self, query: str, start: float, end: float, step: str = '15s'
    ) -> Optional[Dict]:
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

    def query(self, query: str, ts: float = None) -> Optional[Dict]:
        """Execute instant query (optionally at a specific unix timestamp)."""
        url = f"{self.base_url}/api/v1/query"
        params = {'query': query}
        if ts is not None:
            params['time'] = int(ts)

        try:
            response = requests.get(url, params=params, timeout=30)
            response.raise_for_status()
            return response.json()
        except Exception as e:
            logger.error(f"Error querying Prometheus instant: {e}")
            return None


class ResultsAnalyzer:
    """Analyzes test results and generates reports"""
    
    def __init__(self, results_file: Path, prometheus_url: str = 'http://localhost:9090'):
        self.results_file = results_file
        self.client = PrometheusClient(prometheus_url)
        self.recommender = TranscodingRecommender()
        
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

    def get_instant_value(self, data: Optional[Dict]) -> Optional[float]:
        if not data or 'data' not in data or 'result' not in data['data']:
            return None
        results = data['data']['result']
        if not results:
            return None
        value = results[0].get('value')
        if not value or len(value) < 2:
            return None
        try:
            return float(value[1])
        except Exception:
            return None

    def get_energy_joules(self, zone_regex: str, start: float, end: float) -> Optional[float]:
        duration = max(0.0, float(end) - float(start))
        if duration <= 0:
            return None
        window = f"{max(1, int(duration))}s"
        query = f'sum(increase(rapl_energy_joules_total{{zone=~"{zone_regex}"}}[{window}]))'
        data = self.client.query(query, ts=end)
        return self.get_instant_value(data)
    
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
        power_data = self.client.query_range(power_query, start, end, step='5s')
        power_stats = self.get_metric_stats(power_data)

        package_energy_j = self.get_energy_joules('package.*', start, end)
        mean_power_from_energy = None
        if package_energy_j is not None and analysis['duration'] > 0:
            mean_power_from_energy = package_energy_j / analysis['duration']
        
        if power_stats:
            total_energy_j = None
            if package_energy_j is not None:
                total_energy_j = package_energy_j
            else:
                total_energy_j = power_stats['mean'] * (end - start)

            mean_watts = power_stats['mean']
            if mean_power_from_energy is not None:
                mean_watts = mean_power_from_energy

            analysis['power'] = {
                'mean_watts': round(mean_watts, 2),
                'median_watts': round(power_stats['median'], 2),
                'min_watts': round(power_stats['min'], 2),
                'max_watts': round(power_stats['max'], 2),
                'stdev_watts': round(power_stats['stdev'], 2),
                'total_energy_joules': (
                    round(total_energy_j, 2) if total_energy_j is not None else None
                ),
                'total_energy_wh': (
                    round((total_energy_j / 3600), 4) if total_energy_j is not None else None
                ),
            }
        
        # Query DRAM power
        dram_query = 'sum(rapl_power_watts{zone=~".*dram.*"})'
        dram_data = self.client.query_range(dram_query, start, end, step='5s')
        dram_stats = self.get_metric_stats(dram_data)

        dram_energy_j = self.get_energy_joules('.*dram.*', start, end)
        
        if dram_stats:
            total_dram_j = None
            if dram_energy_j is not None:
                total_dram_j = dram_energy_j
            else:
                total_dram_j = dram_stats['mean'] * (end - start)

            mean_dram_watts = dram_stats['mean']
            if total_dram_j is not None and analysis['duration'] > 0:
                mean_dram_watts = total_dram_j / analysis['duration']

            dram_energy_wh = round((total_dram_j / 3600), 4) if total_dram_j else None
            analysis['dram_power'] = {
                'mean_watts': round(mean_dram_watts, 2),
                'total_energy_wh': dram_energy_wh,
            }
        
        # Query Docker overhead
        docker_query = 'docker_engine_cpu_percent'
        docker_data = self.client.query_range(docker_query, start, end, step='5s')
        docker_stats = self.get_metric_stats(docker_data)
        
        if docker_stats and power_stats:
            base_watts = power_stats['mean']
            if mean_power_from_energy is not None:
                base_watts = mean_power_from_energy
            docker_watts = (docker_stats['mean'] / 100) * base_watts
            docker_pct = (docker_watts / base_watts) * 100 if base_watts > 0 else 0.0
            analysis['docker_overhead'] = {
                'cpu_percent': round(docker_stats['mean'], 2),
                'estimated_watts': round(docker_watts, 2),
                'percentage_of_total': round(docker_pct, 2),
            }
        
        # Query container CPU
        container_query = 'docker_containers_total_cpu_percent'
        container_data = self.client.query_range(container_query, start, end, step='5s')
        container_stats = self.get_metric_stats(container_data)
        
        if container_stats and power_stats:
            base_watts = power_stats['mean']
            if mean_power_from_energy is not None:
                base_watts = mean_power_from_energy
            container_watts = (container_stats['mean'] / 100) * base_watts
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

        power_results = [
            r for r in results
            if 'power' in r and r['power'].get('mean_watts') is not None
        ]
        baseline = next((r for r in power_results if 'baseline' in r['name'].lower()), None)
        if baseline:
            for r in results:
                r['net'] = {
                    'power_w': None,
                    'energy_wh': None,
                    'container_cpu_pct': None,
                }

                if 'power' in r and r['power'].get('mean_watts') is not None:
                    if baseline.get('power'):
                        baseline_watts = baseline['power']['mean_watts']
                        r['net']['power_w'] = round(r['power']['mean_watts'] - baseline_watts, 2)
                        
                        r_energy = r['power'].get('total_energy_wh')
                        baseline_energy = baseline['power'].get('total_energy_wh')
                        if r_energy is not None and baseline_energy is not None:
                            r['net']['energy_wh'] = round(r_energy - baseline_energy, 4)

                if 'container_usage' in r and 'container_usage' in baseline:
                    r_cpu = r['container_usage']['cpu_percent']
                    baseline_cpu = baseline['container_usage']['cpu_percent']
                    r['net']['container_cpu_pct'] = round(r_cpu - baseline_cpu, 2)

                if r['name'] == baseline['name']:
                    r['net']['power_w'] = 0.0
                    if r['net']['energy_wh'] is not None:
                        r['net']['energy_wh'] = 0.0
                    if r['net']['container_cpu_pct'] is not None:
                        r['net']['container_cpu_pct'] = 0.0

        # Compute efficiency scores and rank scenarios
        logger.info("Computing energy efficiency scores...")
        results = self.recommender.analyze_and_rank(results)
        
        # Also compute ladder-aware rankings if applicable
        by_ladder = self.recommender.analyze_and_rank_by_ladder(results)
        
        # Store both in the results for reporting
        for result in results:
            result['_by_ladder'] = by_ladder

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
            config = f"{result['bitrate']} @ {result['resolution']} {result['fps']}fps"
            print(f"  Configuration: {config}")
            print(f"  Duration: {result['duration']:.1f}s")
            
            if 'power' in result:
                p = result['power']
                print("\n  Power Consumption:")
                print(f"    Mean:   {p['mean_watts']:>8.2f} W")
                print(f"    Median: {p['median_watts']:>8.2f} W")
                print(f"    Min:    {p['min_watts']:>8.2f} W")
                print(f"    Max:    {p['max_watts']:>8.2f} W")
                print(f"    StdDev: {p['stdev_watts']:>8.2f} W")
                if p.get('total_energy_wh') is not None:
                    if p.get('total_energy_joules') is not None:
                        wh = p['total_energy_wh']
                        joules = p['total_energy_joules']
                        print(f"    Total Energy: {wh:.4f} Wh ({joules:.0f} J)")
                else:
                    print("    Total Energy: N/A")

            if 'net' in result:
                n = result['net']
                if n.get('power_w') is not None or n.get('energy_wh') is not None:
                    print("\n  Net vs Baseline:")
                    if n.get('power_w') is not None:
                        print(f"    Net Power:  {n['power_w']:+.2f} W")
                    if n.get('energy_wh') is not None:
                        print(f"    Net Energy: {n['energy_wh']:+.4f} Wh")
                    if n.get('container_cpu_pct') is not None:
                        print(f"    Net Container CPU: {n['container_cpu_pct']:+.2f}%")
            
            if 'dram_power' in result:
                d = result['dram_power']
                print("\n  DRAM Power:")
                print(f"    Mean: {d['mean_watts']:.2f} W")
                if d.get('total_energy_wh') is not None:
                    print(f"    Total Energy: {d['total_energy_wh']:.4f} Wh")
                else:
                    print("    Total Energy: N/A")
            
            if 'docker_overhead' in result:
                do = result['docker_overhead']
                print("\n  Docker Engine Overhead:")
                print(f"    CPU Usage: {do['cpu_percent']:.2f}%")
                pct_total = do['percentage_of_total']
                print(f"    Power: {do['estimated_watts']:.2f} W ({pct_total:.1f}% of total)")
            
            if 'container_usage' in result:
                cu = result['container_usage']
                print("\n  Container Usage:")
                print(f"    CPU: {cu['cpu_percent']:.2f}%")
                print(f"    Estimated Power: {cu['estimated_watts']:.2f} W")
        
        # Comparison table
        print(f"\n{'=' * 100}")
        print("COMPARISON TABLE")
        print("=" * 100)
        
        # Filter scenarios with power data
        power_results = [r for r in results if 'power' in r]
        
        if power_results:
            header = (
                f"\n{'Scenario':<30} {'Bitrate':<12} "
                f"{'Mean Power':<12} {'Energy (Wh)':<14} {'Docker OH':<12}"
            )
            print(header)
            print("─" * 100)
            
            for r in power_results:
                if 'docker_overhead' in r:
                    docker_oh = f"{r['docker_overhead']['percentage_of_total']:.1f}%"
                else:
                    docker_oh = "N/A"
                energy_wh = r['power'].get('total_energy_wh')
                if energy_wh is None:
                    energy_str = "N/A"
                else:
                    energy_str = f"{energy_wh:>12.4f} Wh"
                print(f"{r['name']:<30} {r['bitrate']:<12} "
                      f"{r['power']['mean_watts']:>10.2f} W "
                      f"{energy_str} "
                      f"{docker_oh:>10}")
            
            # Calculate relative differences from baseline
            baseline = next(
                (r for r in power_results if 'baseline' in r['name'].lower()), None
            )
            
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

                    energy_diff = None
                    r_energy = r['power'].get('total_energy_wh')
                    baseline_energy = baseline['power'].get('total_energy_wh')
                    if r_energy is not None and baseline_energy is not None:
                        energy_diff = r_energy - baseline_energy

                    baseline_watts = baseline['power']['mean_watts']
                    if baseline_watts:
                        pct_increase = (power_diff / baseline_watts) * 100
                    else:
                        pct_increase = 0.0

                    if energy_diff is None:
                        energy_str = "N/A"
                    else:
                        energy_str = f"{energy_diff:>+13.4f} Wh"

                    print(
                        f"{r['name']:<30} {power_diff:>+13.2f} W "
                        f"{energy_str} "
                        f"{pct_increase:>+10.1f}%"
                    )

        # Print energy efficiency rankings
        scored_results = [r for r in results if r.get('efficiency_score') is not None]
        if scored_results:
            print(f"\n{'─' * 100}")
            print("ENERGY EFFICIENCY RANKINGS")
            print("─" * 100)
            header = (
                f"{'Rank':<6} {'Scenario':<35} "
                f"{'Efficiency':<18} {'Power':<12} {'Bitrate':<12}"
            )
            print(header)
            print("─" * 100)
            
            for r in scored_results:
                rank = r.get('efficiency_rank', '-')
                score = r.get('efficiency_score', 0)
                power_w = r.get('power', {}).get('mean_watts', 0)
                bitrate = r.get('bitrate', 'N/A')
                
                row = (
                    f"{rank:<6} {r['name']:<35} "
                    f"{score:>10.4f} Mbps/W   {power_w:>10.2f} W  {bitrate:<12}"
                )
                print(row)
            
            # Print recommendation
            best = scored_results[0]
            print(f"\n{'─' * 100}")
            print("RECOMMENDATION")
            print("─" * 100)
            print(f"Most energy-efficient configuration: {best['name']}")
            print(f"  Efficiency Score: {best['efficiency_score']:.4f} Mbps/W")
            print(f"  Mean Power: {best['power']['mean_watts']:.2f} W")
            print(f"  Bitrate: {best['bitrate']}")
            if best.get('resolution') != 'N/A':
                print(f"  Resolution: {best['resolution']}")
            if best.get('fps') != 'N/A':
                print(f"  FPS: {best['fps']}")
            msg = (
                "\nThis configuration delivers the most video throughput "
                "per watt of energy consumed."
            )
            print(msg)
        
        # Print per-ladder rankings if available
        if results and results[0].get('_by_ladder'):
            by_ladder = results[0]['_by_ladder']
            
            # Filter out internal keys and scenarios without scores
            valid_ladders = {
                k: v for k, v in by_ladder.items()
                if k != '_no_ladder_' and v and v[0].get('efficiency_score') is not None
            }
            
            if len(valid_ladders) > 1:
                print(f"\n{'─' * 100}")
                print("PER-LADDER RANKINGS")
                print("─" * 100)
                print(
                    "\nScenarios grouped by output ladder "
                    "(identical resolution/fps combinations)"
                )
                print("are ranked separately to ensure fair comparison.\n")
                
                for ladder_key, ladder_scenarios in sorted(valid_ladders.items()):
                    print(f"\n{'─' * 100}")
                    print(f"Output Ladder: {ladder_key}")
                    print("─" * 100)
                    
                    # Show top 3 for this ladder
                    top_n = min(3, len(ladder_scenarios))
                    for i, scenario in enumerate(ladder_scenarios[:top_n], start=1):
                        score = scenario.get('efficiency_score', 0)
                        power_w = scenario.get('power', {}).get('mean_watts', 0)
                        bitrate = scenario.get('bitrate', 'N/A')
                        
                        print(f"{i}. {scenario['name']}")
                        if score > 1000:
                            print(f"   Efficiency: {score:.4e} pixels/J")
                        else:
                            print(f"   Efficiency: {score:.4f} Mbps/W")
                        print(f"   Power: {power_w:.2f} W")
                        print(f"   Bitrate: {bitrate}")

        print("\n" + "=" * 100 + "\n")
    
    def print_power_predictions(self, results: List[Dict], predictor):
        """
        Print power scalability predictions and comparison table.
        
        This method displays:
        1. Model metadata (type, training samples, stream range)
        2. Predicted power for standard stream counts (1, 2, 4, 8, 12)
        3. Comparison table showing measured vs predicted for training data
        
        The comparison table helps assess model quality by showing how well
        predictions match actual measurements on training data. Large differences
        may indicate:
        - Poor model fit (low R² score)
        - Non-linear effects not captured by linear model
        - Inconsistent measurements in training data
        
        Args:
            results: List of analyzed scenario dicts (from generate_report)
            predictor: Trained PowerPredictor instance
        """
        print(f"\n{'=' * 100}")
        print("POWER SCALABILITY PREDICTIONS")
        print("=" * 100)
        
        # Get model info
        model_info = predictor.get_model_info()
        
        if not model_info['trained']:
            print("\nPower prediction model could not be trained (insufficient data)")
            return
        
        # Display model metadata
        print(f"\nModel Type: {model_info['model_type'].upper()}")
        print(f"Training Samples: {model_info['n_samples']}")
        if model_info['stream_range']:
            min_s, max_s = model_info['stream_range']
            print(f"Stream Range: {min_s} - {max_s} streams")
        
        # Predict for key stream counts (standard capacity planning points)
        # These represent typical workload sizes: single stream, small (2-4),
        # medium (8), and large (12) deployments
        target_streams = [1, 2, 4, 8, 12]
        predictions = {}
        
        print("\nPredicted Power Consumption:")
        print("─" * 100)
        for streams in target_streams:
            power = predictor.predict(streams)
            if power is not None:
                predictions[streams] = power
                print(f"  {streams:>2} streams: {power:>8.2f} W")
        
        # Create comparison table: Streams | Measured (W) | Predicted (W) | Diff (W)
        # This shows model accuracy on training data (should be close to 0 diff)
        print(f"\n{'─' * 100}")
        print("MEASURED vs PREDICTED COMPARISON")
        print("─" * 100)
        print("(Shows model fit quality on training data)")
        
        # Extract measured data from training
        # Group by stream count and average if multiple measurements exist
        measured_data = {}
        for streams, power in predictor.training_data:
            if streams not in measured_data:
                measured_data[streams] = []
            measured_data[streams].append(power)
        
        # Average multiple measurements for same stream count
        # This handles cases where different scenarios have same stream count
        # (e.g., "4 Streams @ 2500k" and "4 Streams @ 1080p")
        measured_avg = {s: sum(powers) / len(powers) for s, powers in measured_data.items()}
        
        # Print comparison table header
        print(f"{'Streams':<10} {'Measured (W)':<15} {'Predicted (W)':<15} {'Diff (W)':<12}")
        print("─" * 100)
        
        # Show all measured stream counts with predictions
        for streams in sorted(measured_avg.keys()):
            measured = measured_avg[streams]
            predicted = predictor.predict(streams)
            diff = predicted - measured if predicted is not None else None
            
            measured_str = f"{measured:.2f}"
            predicted_str = f"{predicted:.2f}" if predicted is not None else "N/A"
            diff_str = f"{diff:+.2f}" if diff is not None else "N/A"
            
            print(f"{streams:<10} {measured_str:<15} {predicted_str:<15} {diff_str:<12}")
        
        print("─" * 100)

    def export_csv(self, output_file: str = None, predictor=None):
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
                'predicted_mean_power_w',
                'net_power_w', 'net_energy_wh', 'net_container_cpu_pct',
                'docker_overhead_w', 'docker_overhead_pct',
                'container_cpu_pct', 'container_power_w',
                'efficiency_score', 'efficiency_rank',
                'output_ladder', 'total_pixels'
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
                
                # Add predicted power if predictor is available
                if predictor is not None:
                    streams = predictor._infer_stream_count(r['name'])
                    if streams is not None:
                        predicted_power = predictor.predict(streams)
                        row['predicted_mean_power_w'] = predicted_power
                    else:
                        row['predicted_mean_power_w'] = None
                else:
                    row['predicted_mean_power_w'] = None

                if 'net' in r:
                    row['net_power_w'] = r['net'].get('power_w')
                    row['net_energy_wh'] = r['net'].get('energy_wh')
                    row['net_container_cpu_pct'] = r['net'].get('container_cpu_pct')

                if 'docker_overhead' in r:
                    row['docker_overhead_w'] = r['docker_overhead']['estimated_watts']
                    row['docker_overhead_pct'] = r['docker_overhead']['percentage_of_total']

                if 'container_usage' in r:
                    row['container_cpu_pct'] = r['container_usage']['cpu_percent']
                    row['container_power_w'] = r['container_usage']['estimated_watts']

                # Add efficiency score and rank
                row['efficiency_score'] = r.get('efficiency_score')
                row['efficiency_rank'] = r.get('efficiency_rank')
                
                # Add output ladder information
                row['output_ladder'] = r.get('output_ladder')
                
                # Compute total pixels using scorer's method
                total_pixels = self.recommender.scorer._compute_total_pixels(r)
                row['total_pixels'] = total_pixels

                writer.writerow(row)

        logger.info(f"CSV exported to {output_file}")
    
    def print_multivariate_predictions(
        self,
        results: List[Dict],
        predictor: MultivariatePredictor,
        stream_counts: List[int],
    ):
        """
        Print multivariate model predictions for specified stream counts.
        
        Args:
            results: List of analyzed scenario dicts
            predictor: Trained MultivariatePredictor instance
            stream_counts: List of stream counts to predict for
        """
        print(f"\n{'=' * 100}")
        print("MULTIVARIATE MODEL PREDICTIONS")
        print("=" * 100)
        
        info = predictor.get_model_info()
        
        if not info['trained']:
            print("\nMultivariate predictor could not be trained (insufficient data)")
            return
        
        print(f"\nModel: {info['best_model'].upper()}")
        print(f"Training Samples: {info['n_samples']}")
        print(f"Features: {', '.join(info['feature_names'])}")
        print(f"R² Score: {info['best_score']['r2']:.4f}")
        print(f"RMSE: {info['best_score']['rmse']:.2f} W")
        print(f"Confidence Level: {info['confidence_level']*100:.0f}%")
        
        # Generate predictions for each stream count
        print(f"\n{'─' * 100}")
        print("Predicted Power Consumption with Confidence Intervals:")
        print("─" * 100)
        print(
            f"{'Streams':<10} {'Mean Power (W)':<18} "
            f"{'CI Low (W)':<15} {'CI High (W)':<15} {'CI Width (W)':<15}"
        )
        print("─" * 100)
        
        # Use representative scenario for base features
        if not results:
            print("No results available for feature extraction")
            return
        
        # Find a typical scenario (exclude baseline)
        typical_scenario = next(
            (r for r in results if 'baseline' not in r['name'].lower()),
            results[0]
        )
        
        for stream_count in stream_counts:
            # Create feature dict based on typical scenario scaled to stream count
            features = {
                'stream_count': stream_count,
                'bitrate_mbps': 2.5,  # Default bitrate
                'total_pixels': 1920 * 1080 * 30 * 60,  # 1080p30 for 60s
                'cpu_usage_pct': min(95.0, 20.0 + stream_count * 10),  # Estimated
                'encoder_type': typical_scenario.get('encoder_type', 'x264'),
                'hardware_cpu_model': typical_scenario.get('hardware', {}).get(
                    'cpu_model', 'unknown'
                ),
                'container_cpu_pct': min(10.0, 2.0 + stream_count * 0.5),  # Estimated
            }
            
            prediction = predictor.predict(features, return_confidence=True)
            
            mean_power = prediction.get('mean', 0)
            ci_low = prediction.get('ci_low', 0)
            ci_high = prediction.get('ci_high', 0)
            ci_width = prediction.get('ci_width', 0)
            
            print(
                f"{stream_count:<10} {mean_power:>16.2f} "
                f"{ci_low:>13.2f} {ci_high:>13.2f} {ci_width:>13.2f}"
            )
        
        print("─" * 100)
        
        # Model comparison table
        print(f"\n{'─' * 100}")
        print("Model Performance Comparison:")
        print("─" * 100)
        print(f"{'Model':<20} {'R² Score':<15} {'RMSE (W)':<15}")
        print("─" * 100)
        
        for model_name, scores in info['models'].items():
            print(f"{model_name:<20} {scores['r2']:>13.4f} {scores['rmse']:>13.2f}")
        
        print("─" * 100)
        print(f"\nBest Model: {info['best_model']} (highest R²)")
        print("=" * 100 + "\n")


def main():
    parser = argparse.ArgumentParser(
        description='Analyze FFmpeg power test results and generate predictions',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Analyze latest test results
  python3 analyze_results.py
  
  # Analyze specific test file
  python3 analyze_results.py test_results/test_results_20231215_143022.json
  
  # Generate predictions for future stream counts
  python3 analyze_results.py --predict-future 1,2,4,8,12
  
  # Use multivariate predictor instead of simple linear model
  python3 analyze_results.py --multivariate --predict-future 1,2,4,8,12,16
        """
    )
    
    parser.add_argument(
        'results_file',
        nargs='?',
        type=Path,
        help='Path to test results JSON file (default: latest in ./test_results/)'
    )
    
    parser.add_argument(
        '--predict-future',
        type=str,
        metavar='STREAMS',
        help='Comma-separated list of stream counts to predict (e.g., "1,2,4,8,12")'
    )
    
    parser.add_argument(
        '--multivariate',
        action='store_true',
        help='Use multivariate ML predictor (more features, ensemble models)'
    )
    
    args = parser.parse_args()
    
    # Determine results file
    if args.results_file:
        results_file = args.results_file
        if not results_file.exists():
            logger.error(f"Results file not found: {results_file}")
            return 1
    else:
        # Find most recent results file
        results_dir = Path('./test_results')
        if results_dir.exists():
            results_files = sorted(results_dir.glob('test_results_*.json'), reverse=True)
            if results_files:
                results_file = results_files[0]
                logger.info(f"Using most recent results file: {results_file}")
            else:
                logger.error("No results files found in ./test_results")
                parser.print_help()
                return 1
        else:
            logger.error("No results directory found")
            parser.print_help()
            return 1
    
    # Parse prediction stream counts
    predict_streams = None
    if args.predict_future:
        try:
            predict_streams = [int(s.strip()) for s in args.predict_future.split(',')]
            logger.info(f"Will predict for stream counts: {predict_streams}")
        except ValueError:
            logger.error(f"Invalid stream counts: {args.predict_future}")
            return 1
    
    try:
        analyzer = ResultsAnalyzer(results_file)
        results = analyzer.generate_report()
        
        # Print summary
        analyzer.print_summary(results)
        
        # Train and use predictor
        if args.multivariate:
            # Use advanced multivariate predictor
            logger.info("Training multivariate ML predictor...")
            mv_predictor = MultivariatePredictor(
                models=['linear', 'poly2', 'poly3', 'rf', 'gbm'],
                confidence_level=0.95,
                n_bootstrap=100,
                cv_folds=5
            )
            
            success = mv_predictor.fit(results, target='mean_power_watts')
            
            if success:
                if predict_streams:
                    analyzer.print_multivariate_predictions(results, mv_predictor, predict_streams)
                else:
                    # Default predictions
                    analyzer.print_multivariate_predictions(results, mv_predictor, [1, 2, 4, 8, 12])
            else:
                logger.warning(
                    "Multivariate predictor training failed, "
                    "falling back to simple predictor"
                )
                args.multivariate = False
        
        if not args.multivariate:
            # Use simple PowerPredictor (backward compatible)
            predictor = PowerPredictor()
            predictor.fit(results)
            
            # Print power predictions
            analyzer.print_power_predictions(results, predictor)
            
            # Export CSV with predictions
            analyzer.export_csv(predictor=predictor)
        
        return 0
    except Exception as e:
        logger.error(f"Analysis failed: {e}")
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    exit(main())
