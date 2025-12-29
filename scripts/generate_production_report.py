#!/usr/bin/env python3
"""
Production Benchmark Runner

Runs production streaming benchmarks and generates detailed reports with:
- Power consumption (avg, peak, kWh)
- Cost analysis (USD/hour)
- Quality metrics (VMAF scores)
- CO₂ emissions
"""

import argparse
import json
import sys
from datetime import datetime
from pathlib import Path
from typing import Dict, List

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent.parent))

from advisor.regional_pricing import RegionalPricing


def load_benchmark_results(results_dir: Path) -> Dict:
    """Load latest benchmark results JSON file."""
    json_files = sorted(results_dir.glob('test_results_*.json'), reverse=True)
    
    if not json_files:
        print(f"No test results found in {results_dir}")
        return {}
    
    with open(json_files[0]) as f:
        return json.load(f)


def compute_scenario_metrics(scenario: Dict, regional_pricing: RegionalPricing) -> Dict:
    """Compute comprehensive metrics for a scenario."""
    metrics = {
        'name': scenario.get('name', 'Unknown'),
        'bitrate': scenario.get('bitrate', 'N/A'),
        'resolution': scenario.get('resolution', 'N/A'),
        'fps': scenario.get('fps', 'N/A'),
        'duration': scenario.get('duration', 0),
    }
    
    # Power metrics
    power = scenario.get('power', {})
    if power:
        metrics['avg_watts'] = power.get('mean_watts', 0)
        metrics['peak_watts'] = power.get('max_watts', 0)
        metrics['min_watts'] = power.get('min_watts', 0)
        
        # Energy (kWh)
        duration_hours = metrics['duration'] / 3600.0
        metrics['energy_kwh'] = (metrics['avg_watts'] * duration_hours) / 1000.0
    else:
        metrics['avg_watts'] = 0
        metrics['peak_watts'] = 0
        metrics['energy_kwh'] = 0
    
    # Cost (USD/hour)
    if metrics['avg_watts'] > 0:
        energy_kwh_per_hour = metrics['avg_watts'] / 1000.0
        metrics['usd_per_hour'] = energy_kwh_per_hour * regional_pricing.get_electricity_price()
    else:
        metrics['usd_per_hour'] = 0
    
    # CO₂ emissions
    if metrics['avg_watts'] > 0:
        energy_kwh_per_hour = metrics['avg_watts'] / 1000.0
        metrics['co2_kg_per_hour'] = regional_pricing.compute_co2_emissions(energy_kwh_per_hour)
    else:
        metrics['co2_kg_per_hour'] = 0
    
    # Quality metrics
    metrics['vmaf_score'] = scenario.get('vmaf_score', 'N/A')
    metrics['psnr_score'] = scenario.get('psnr_score', 'N/A')
    
    return metrics


def generate_markdown_report(
    results: Dict,
    output_file: Path,
    regional_pricing: RegionalPricing
) -> None:
    """Generate markdown report from benchmark results."""
    
    scenarios = results.get('scenarios', [])
    if not scenarios:
        print("No scenarios found in results")
        return
    
    # Find baseline for savings calculations
    baseline_power = None
    for scenario in scenarios:
        if 'baseline' in scenario.get('name', '').lower() or 'idle' in scenario.get('name', '').lower():
            power = scenario.get('power', {})
            if power:
                baseline_power = power.get('mean_watts', 0)
                break
    
    # Compute metrics for all scenarios
    scenario_metrics = []
    for scenario in scenarios:
        if 'baseline' not in scenario.get('name', '').lower():
            metrics = compute_scenario_metrics(scenario, regional_pricing)
            scenario_metrics.append(metrics)
    
    # Generate report
    report = []
    report.append("# Production Streaming Benchmarks Report")
    report.append("")
    report.append(f"**Generated**: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    report.append(f"**Region**: {regional_pricing.region}")
    report.append(f"**Electricity Price**: ${regional_pricing.get_electricity_price():.3f}/kWh")
    report.append(f"**CO₂ Intensity**: {regional_pricing.get_co2_intensity():.0f} gCO₂/kWh")
    if baseline_power:
        report.append(f"**Baseline Power**: {baseline_power:.1f}W")
    report.append("")
    
    report.append("## Executive Summary")
    report.append("")
    
    if scenario_metrics:
        avg_power = sum(m['avg_watts'] for m in scenario_metrics) / len(scenario_metrics)
        avg_cost = sum(m['usd_per_hour'] for m in scenario_metrics) / len(scenario_metrics)
        avg_co2 = sum(m['co2_kg_per_hour'] for m in scenario_metrics) / len(scenario_metrics)
        
        report.append(f"- **Average Power**: {avg_power:.1f}W")
        report.append(f"- **Average Cost**: ${avg_cost:.4f}/hour")
        report.append(f"- **Average CO₂**: {avg_co2:.3f} kg/hour")
        report.append("")
        
        if baseline_power and baseline_power > 0:
            avg_savings_watts = avg_power - baseline_power
            savings_pct = (avg_savings_watts / baseline_power) * 100 if baseline_power > 0 else 0
            report.append(f"- **Power vs Baseline**: {avg_savings_watts:+.1f}W ({savings_pct:+.1f}%)")
    
    report.append("")
    report.append("## Detailed Results")
    report.append("")
    
    # Sort by resolution and fps for better grouping
    scenario_metrics.sort(key=lambda x: (x['resolution'], x['fps']))
    
    report.append("| Scenario | Resolution | FPS | Bitrate | Avg Power | Peak Power | Energy | Cost | CO₂ | VMAF |")
    report.append("|----------|------------|-----|---------|-----------|------------|--------|------|-----|------|")
    
    for metrics in scenario_metrics:
        vmaf_str = f"{metrics['vmaf_score']:.1f}" if isinstance(metrics['vmaf_score'], (int, float)) else "N/A"
        
        report.append(
            f"| {metrics['name'][:40]} | "
            f"{metrics['resolution']} | "
            f"{metrics['fps']} | "
            f"{metrics['bitrate']} | "
            f"{metrics['avg_watts']:.1f}W | "
            f"{metrics['peak_watts']:.1f}W | "
            f"{metrics['energy_kwh']:.4f} kWh | "
            f"${metrics['usd_per_hour']:.4f}/h | "
            f"{metrics['co2_kg_per_hour']:.3f} kg/h | "
            f"{vmaf_str} |"
        )
    
    report.append("")
    report.append("## Cost Analysis by Platform")
    report.append("")
    
    # Group by platform
    platforms = {}
    for metrics in scenario_metrics:
        name = metrics['name']
        if 'Twitch' in name:
            platform = 'Twitch'
        elif 'YouTube' in name:
            platform = 'YouTube'
        elif 'Zoom' in name:
            platform = 'Zoom'
        elif 'Facebook' in name:
            platform = 'Facebook'
        elif 'LinkedIn' in name:
            platform = 'LinkedIn'
        else:
            platform = 'Other'
        
        if platform not in platforms:
            platforms[platform] = []
        platforms[platform].append(metrics)
    
    for platform, metrics_list in sorted(platforms.items()):
        report.append(f"### {platform}")
        report.append("")
        
        for metrics in metrics_list:
            report.append(f"**{metrics['name']}**")
            report.append(f"- Resolution: {metrics['resolution']} @ {metrics['fps']} fps")
            report.append(f"- Bitrate: {metrics['bitrate']}")
            report.append(f"- Power: {metrics['avg_watts']:.1f}W (peak: {metrics['peak_watts']:.1f}W)")
            report.append(f"- Cost: ${metrics['usd_per_hour']:.4f}/hour (${metrics['usd_per_hour'] * 24 * 30:.2f}/month @ 24/7)")
            report.append(f"- CO₂: {metrics['co2_kg_per_hour']:.3f} kg/hour ({metrics['co2_kg_per_hour'] * 24 * 30:.1f} kg/month)")
            if isinstance(metrics['vmaf_score'], (int, float)):
                report.append(f"- Quality: VMAF {metrics['vmaf_score']:.1f}")
            report.append("")
    
    report.append("## ROI Calculator")
    report.append("")
    report.append("Monthly cost projections for different server counts (24/7 operation):")
    report.append("")
    
    if scenario_metrics:
        # Use the most common scenario for ROI calculation
        example_metrics = scenario_metrics[len(scenario_metrics)//2]  # Middle scenario
        monthly_cost = example_metrics['usd_per_hour'] * 24 * 30
        
        report.append("| Servers | Monthly Cost | Annual Cost |")
        report.append("|---------|--------------|-------------|")
        for count in [1, 10, 50, 100, 500]:
            report.append(f"| {count} | ${monthly_cost * count:.2f} | ${monthly_cost * count * 12:.2f} |")
    
    report.append("")
    report.append("## Recommendations")
    report.append("")
    
    # Find most efficient scenario
    if scenario_metrics:
        # Efficiency = quality per watt
        efficient_scenarios = [
            (m, m.get('vmaf_score', 0) / m['avg_watts'] if m['avg_watts'] > 0 and isinstance(m.get('vmaf_score'), (int, float)) else 0)
            for m in scenario_metrics
        ]
        efficient_scenarios.sort(key=lambda x: x[1], reverse=True)
        
        if efficient_scenarios[0][1] > 0:
            best = efficient_scenarios[0][0]
            report.append(f"**Most Efficient Configuration**: {best['name']}")
            report.append(f"- Power: {best['avg_watts']:.1f}W")
            report.append(f"- Cost: ${best['usd_per_hour']:.4f}/hour")
            if isinstance(best['vmaf_score'], (int, float)):
                report.append(f"- Quality: VMAF {best['vmaf_score']:.1f}")
            report.append(f"- Efficiency: {efficient_scenarios[0][1]:.3f} VMAF/W")
    
    report.append("")
    report.append("---")
    report.append("*Report generated by production benchmark runner*")
    
    # Write report
    with open(output_file, 'w') as f:
        f.write('\n'.join(report))
    
    print(f"Report generated: {output_file}")


def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(description='Generate production benchmark report')
    parser.add_argument(
        '--results-dir',
        type=Path,
        default=Path('./test_results'),
        help='Directory containing test results'
    )
    parser.add_argument(
        '--output',
        type=Path,
        default=Path('./results/PRODUCTION.md'),
        help='Output markdown file'
    )
    parser.add_argument(
        '--region',
        type=str,
        default='us-east-1',
        help='AWS region for pricing'
    )
    parser.add_argument(
        '--pricing-config',
        type=Path,
        default=None,
        help='Custom pricing configuration'
    )
    
    args = parser.parse_args()
    
    # Create output directory
    args.output.parent.mkdir(exist_ok=True)
    
    # Load results
    print(f"Loading results from {args.results_dir}...")
    results = load_benchmark_results(args.results_dir)
    
    if not results:
        return 1
    
    # Initialize regional pricing
    regional_pricing = RegionalPricing(
        region=args.region,
        config_file=args.pricing_config
    )
    
    # Generate report
    print(f"Generating report for region {args.region}...")
    generate_markdown_report(results, args.output, regional_pricing)
    
    return 0


if __name__ == '__main__':
    sys.exit(main())
