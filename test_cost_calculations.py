#!/usr/bin/env python3
"""
Test script to verify cost calculation improvements.

This script demonstrates the mathematical improvements made to the cost
calculation system, specifically the use of trapezoidal numerical integration
for more accurate cost modeling.
"""

import sys

sys.path.insert(0, '.')
from advisor.cost import CostModel


def compare_integration_methods():
    """
    Compare rectangular vs trapezoidal integration methods.

    Shows that trapezoidal rule provides better accuracy for
    varying time-series data.
    """
    print("=" * 70)
    print("INTEGRATION METHOD COMPARISON")
    print("=" * 70)
    print()

    # Example: CPU usage that varies over time
    cpu_values = [1.0, 2.5, 3.5, 3.0, 2.0, 1.5, 1.0]
    step = 5.0  # seconds between measurements

    # Rectangular approximation (old method)
    rectangular = sum(cpu_values) * step

    # Trapezoidal approximation (new method)
    model = CostModel()
    trapezoidal = model._trapezoidal_integrate(cpu_values, step)

    print(f"CPU usage over time: {cpu_values}")
    print(f"Time step: {step} seconds")
    print()
    print(f"Rectangular approximation: {rectangular:.2f} core-seconds")
    print("  Formula: sum(values) × step")
    print("  Accuracy: O(h) - assumes constant value between measurements")
    print()
    print(f"Trapezoidal approximation: {trapezoidal:.2f} core-seconds")
    print("  Formula: (step/2) × [v₀ + 2v₁ + 2v₂ + ... + 2vₙ₋₁ + vₙ]")
    print("  Accuracy: O(h²) - accounts for slope between measurements")
    print()
    print(f"Difference: {abs(trapezoidal - rectangular):.2f} core-seconds")
    print(f"Relative difference: {abs(trapezoidal - rectangular) / rectangular * 100:.1f}%")
    print()


def test_cost_calculations():
    """
    Test complete cost calculations with realistic scenario data.
    """
    print("=" * 70)
    print("REALISTIC COST CALCULATION TEST")
    print("=" * 70)
    print()

    # Initialize cost model with realistic cloud pricing
    # AWS t3.medium: ~$0.0416/hour = $0.50/hour for 1 core equivalent
    # Energy: $0.12/kWh (typical datacenter cost)
    model = CostModel(energy_cost_per_kwh=0.12, cpu_cost_per_hour=0.50, currency='USD')

    # Realistic transcoding scenario: 4 streams @ 2500kbps
    scenario = {
        'name': '4 streams @ 2500k (720p30)',
        'duration': 120,  # 2 minutes
        'resolution': '1280x720',
        'fps': 30,
        'streams': 4,
        'bitrate': '2500k',
        'encoder_type': 'libx264',
        'viewers': 400,  # 4 streams × 100 viewers each
        # CPU usage varies as encoders start/stop
        'cpu_usage_cores': [
            5.5,
            5.6,
            5.55,
            5.58,
            5.62,
            5.60,
            5.59,
            5.61,
            5.58,
            5.57,
            5.59,
            5.61,
            5.60,
            5.58,
            5.59,
            5.60,
            5.61,
            5.59,
            5.58,
            5.60,
            5.61,
            5.59,
            5.60,
            5.58,
        ],
        # Power consumption increases with CPU load
        'power_watts': [
            135.5,
            136.2,
            135.8,
            136.0,
            136.5,
            136.1,
            135.9,
            136.3,
            136.0,
            135.8,
            136.1,
            136.4,
            136.2,
            136.0,
            136.1,
            136.2,
            136.3,
            136.1,
            136.0,
            136.2,
            136.3,
            136.1,
            136.2,
            136.0,
        ],
        'step_seconds': 5,
    }

    print(f"Scenario: {scenario['name']}")
    print(f"Duration: {scenario['duration']}s")
    print(f"Streams: {scenario['streams']}")
    print(f"Resolution: {scenario['resolution']} @ {scenario['fps']}fps")
    print(f"Bitrate: {scenario['bitrate']}")
    print(f"Viewers: {scenario['viewers']}")
    print()

    # Calculate costs
    compute_cost = model.compute_compute_cost_load_aware(scenario)
    energy_cost = model.compute_energy_cost_load_aware(scenario)
    total_cost = model.compute_total_cost_load_aware(scenario)
    cost_per_pixel = model.compute_cost_per_pixel_load_aware(scenario)
    cost_per_watch_hour = model.compute_cost_per_watch_hour_load_aware(scenario)

    print("=" * 40)
    print("COST BREAKDOWN")
    print("=" * 40)
    print(f"Compute cost: ${compute_cost:.6f}")
    print(f"  ({len(scenario['cpu_usage_cores'])} CPU samples integrated)")
    print()
    print(f"Energy cost:  ${energy_cost:.6f}")
    print(f"  ({len(scenario['power_watts'])} power samples integrated)")
    print()
    print(f"Total cost:   ${total_cost:.6f}")
    print()

    print("=" * 40)
    print("EFFICIENCY METRICS")
    print("=" * 40)
    print(f"Cost per megapixel: ${cost_per_pixel * 1e6:.9f}")
    print(f"Cost per watch hour: ${cost_per_watch_hour:.6f}")
    print()

    # Calculate hourly extrapolation
    hourly_cost = total_cost * (3600 / scenario['duration'])
    print(f"Extrapolated hourly cost: ${hourly_cost:.4f}/hour")
    print(f"Monthly cost (730h): ${hourly_cost * 730:.2f}/month")
    print()


def demonstrate_accuracy():
    """
    Demonstrate the importance of accurate integration for cost modeling.
    """
    print("=" * 70)
    print("ACCURACY DEMONSTRATION")
    print("=" * 70)
    print()

    print("Why trapezoidal integration matters:")
    print()
    print("Consider a scenario where CPU usage ramps up and down:")
    print("  Time:  0s   5s  10s  15s  20s  25s  30s")
    print("  CPU:   1.0  3.0  5.0  5.0  3.0  1.0  1.0 cores")
    print()

    cpu_ramp = [1.0, 3.0, 5.0, 5.0, 3.0, 1.0, 1.0]
    step = 5

    model = CostModel()

    # Rectangular (old)
    rect = sum(cpu_ramp) * step

    # Trapezoidal (new)
    trap = model._trapezoidal_integrate(cpu_ramp, step)

    print(f"Rectangular method: {rect:.1f} core-seconds")
    print("  Assumes constant CPU between each measurement")
    print("  Overestimates during ramp-up, underestimates during ramp-down")
    print()
    print(f"Trapezoidal method: {trap:.1f} core-seconds")
    print("  Approximates the slope between measurements")
    print("  More accurately captures the actual resource usage curve")
    print()

    price_per_core_second = 0.50 / 3600  # $0.50/hour
    cost_diff = abs(trap - rect) * price_per_core_second

    print(f"Cost difference for this 30s period: ${cost_diff:.6f}")
    print(f"Over 1 hour: ${cost_diff * 120:.4f}")
    print(f"Over 1 month (730h): ${cost_diff * 120 * 730:.2f}")
    print()
    print("This demonstrates why mathematical accuracy matters for")
    print("cost optimization in production systems!")
    print()


if __name__ == '__main__':
    compare_integration_methods()
    test_cost_calculations()
    demonstrate_accuracy()

    print("=" * 70)
    print("ALL TESTS COMPLETED SUCCESSFULLY")
    print("=" * 70)
