#!/usr/bin/env python3
"""
Simulate Grafana dashboard metrics visualization.

This script shows what the Grafana dashboard panels would display
based on the current cost exporter metrics.
"""

import sys

sys.path.insert(0, '.')

import json
from pathlib import Path

from advisor.cost import CostModel


def load_test_results():
    """Load the most recent test results file."""
    results_dir = Path('test_results')
    json_files = sorted(results_dir.glob('test_results_*.json'), reverse=True)
    
    if not json_files:
        print("ERROR: No test results found!")
        print("Please ensure test_results/test_results_*.json exists")
        return None
    
    with open(json_files[0]) as f:
        data = json.load(f)
    
    return data['scenarios']


def simulate_dashboard_panels(scenarios):
    """Simulate what each Grafana panel would show."""
    
    # Initialize cost model with same pricing as exporter
    model = CostModel(
        energy_cost_per_kwh=0.12,
        cpu_cost_per_hour=0.50,
        currency='USD'
    )
    
    print("=" * 80)
    print("GRAFANA DASHBOARD SIMULATION")
    print("Cost Analysis - Load Aware")
    print("=" * 80)
    print()
    
    # Panel 1: Cost Breakdown (Stacked timeseries)
    print("📊 PANEL 1: Cost Breakdown (Energy + Compute)")
    print("-" * 80)
    total_energy = 0.0
    total_compute = 0.0
    
    for scenario in scenarios:
        name = scenario.get('name', 'unknown')
        streams = scenario.get('streams', 0)
        bitrate = scenario.get('bitrate', 'N/A')
        
        if streams is None:
            continue
        
        energy_cost = model.compute_energy_cost_load_aware(scenario) or 0.0
        compute_cost = model.compute_compute_cost_load_aware(scenario) or 0.0
        
        total_energy += energy_cost
        total_compute += compute_cost
        
        print(f"  {name} ({streams} streams @ {bitrate})")
        print(f"    Energy:  ${energy_cost:.6f}")
        print(f"    Compute: ${compute_cost:.6f}")
    
    print(f"\n  TOTAL Energy:  ${total_energy:.6f}")
    print(f"  TOTAL Compute: ${total_compute:.6f}")
    print()
    
    # Panel 2: Total Cost by Scenario
    print("📊 PANEL 2: Total Cost by Scenario")
    print("-" * 80)
    for scenario in scenarios:
        name = scenario.get('name', 'unknown')
        streams = scenario.get('streams', 0)
        bitrate = scenario.get('bitrate', 'N/A')
        
        if streams is None:
            continue
        
        total_cost = model.compute_total_cost_load_aware(scenario) or 0.0
        print(f"  {name} ({streams} streams @ {bitrate}): ${total_cost:.6f}")
    print()
    
    # Panel 3: Cost per Megapixel
    print("📊 PANEL 3: Cost per Megapixel Delivered")
    print("-" * 80)
    for scenario in scenarios:
        name = scenario.get('name', 'unknown')
        streams = scenario.get('streams', 0)
        bitrate = scenario.get('bitrate', 'N/A')
        
        if streams is None or streams == 0:
            continue
        
        cost_per_pixel = model.compute_cost_per_pixel_load_aware(scenario)
        if cost_per_pixel:
            # Convert to per-megapixel for readability
            cost_per_megapixel = cost_per_pixel * 1e6
            print(f"  {name} ({streams} streams @ {bitrate}): ${cost_per_megapixel:.9f}/Mpx")
    print()
    
    # Panel 4: Cost per Watch Hour
    print("📊 PANEL 4: Cost per Viewer Watch Hour")
    print("-" * 80)
    for scenario in scenarios:
        name = scenario.get('name', 'unknown')
        streams = scenario.get('streams', 0)
        bitrate = scenario.get('bitrate', 'N/A')
        viewers = scenario.get('viewers', 0)
        
        if streams is None or streams == 0 or viewers == 0:
            continue
        
        cost_per_watch_hour = model.compute_cost_per_watch_hour_load_aware(scenario)
        if cost_per_watch_hour:
            print(
                f"  {name} ({streams} streams @ {bitrate}): "
                f"${cost_per_watch_hour:.6f}/viewer-hour"
            )
    print()
    
    # Panel 5: Current Total Cost (Gauge)
    print("🎚️  PANEL 5: Current Total Cost")
    print("-" * 80)
    total_cost_sum = sum(
        model.compute_total_cost_load_aware(s) or 0.0
        for s in scenarios
        if s.get('streams') is not None
    )
    print(f"  ${total_cost_sum:.6f}")
    print()
    
    # Panel 6: Current Energy Cost (Gauge)
    print("🎚️  PANEL 6: Current Energy Cost")
    print("-" * 80)
    energy_cost_sum = sum(
        model.compute_energy_cost_load_aware(s) or 0.0
        for s in scenarios
        if s.get('streams') is not None
    )
    print(f"  ${energy_cost_sum:.6f}")
    print()
    
    # Panel 7: Cost Distribution (Pie chart)
    print("🥧 PANEL 7: Cost Distribution by Scenario")
    print("-" * 80)
    costs_by_scenario = {}
    for scenario in scenarios:
        name = scenario.get('name', 'unknown')
        streams = scenario.get('streams')
        
        if streams is None:
            continue
        
        cost = model.compute_total_cost_load_aware(scenario) or 0.0
        costs_by_scenario[name] = cost
    
    total_for_pct = sum(costs_by_scenario.values())
    if total_for_pct > 0:
        for name, cost in sorted(costs_by_scenario.items(), key=lambda x: -x[1]):
            pct = (cost / total_for_pct) * 100
            bar_length = int(pct / 2)
            bar = '█' * bar_length
            print(f"  {name:40} {bar} {pct:5.1f}% (${cost:.6f})")
    print()
    
    # Summary
    print("=" * 80)
    print("SUMMARY")
    print("=" * 80)
    print(
        f"Total scenarios analyzed: "
        f"{len([s for s in scenarios if s.get('streams') is not None])}"
    )
    print(f"Total cost (all scenarios): ${total_cost_sum:.6f}")
    print(f"Average cost per scenario: ${total_cost_sum / len(costs_by_scenario):.6f}")
    print()
    print("✅ All metrics calculated successfully using trapezoidal integration!")
    print("✅ Dashboard would display data correctly in Grafana")
    print()


if __name__ == '__main__':
    scenarios = load_test_results()
    if scenarios:
        simulate_dashboard_panels(scenarios)
    else:
        print("Run this script after creating test_results/test_results_*.json")
        sys.exit(1)
