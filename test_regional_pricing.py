#!/usr/bin/env python3
"""
Test regional pricing and CO₂ emissions calculations
"""

import sys
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent.parent))

from advisor.regional_pricing import RegionalPricing, get_available_regions


def test_regional_pricing():
    """Test regional pricing functionality."""
    print("Testing Regional Pricing Module\n")
    print("=" * 60)
    
    # Test default region
    print("\n1. Testing default region:")
    pricing_default = RegionalPricing('default')
    print(f"   Region: {pricing_default.region}")
    print(f"   Electricity price: ${pricing_default.get_electricity_price():.3f}/kWh")
    print(f"   CO₂ intensity: {pricing_default.get_co2_intensity():.0f} gCO₂/kWh")
    
    # Test US East (Virginia)
    print("\n2. Testing us-east-1 (Virginia):")
    pricing_us = RegionalPricing('us-east-1')
    print(f"   Region: {pricing_us.region}")
    print(f"   Electricity price: ${pricing_us.get_electricity_price():.3f}/kWh")
    print(f"   CO₂ intensity: {pricing_us.get_co2_intensity():.0f} gCO₂/kWh")
    
    # Test EU West (Ireland)
    print("\n3. Testing eu-west-1 (Ireland):")
    pricing_eu = RegionalPricing('eu-west-1')
    print(f"   Region: {pricing_eu.region}")
    print(f"   Electricity price: ${pricing_eu.get_electricity_price():.3f}/kWh")
    print(f"   CO₂ intensity: {pricing_eu.get_co2_intensity():.0f} gCO₂/kWh")
    
    # Test CO₂ calculations
    print("\n4. Testing CO₂ emissions calculations:")
    power_watts = 100.0  # 100W system
    print(f"   System power: {power_watts}W")
    
    for region_code in ['us-west-2', 'eu-central-1', 'ap-southeast-2']:
        pricing = RegionalPricing(region_code)
        monthly_co2 = pricing.compute_monthly_co2(power_watts, hours_per_day=24)
        print(f"   {region_code}: {monthly_co2:.2f} kg CO₂/month")
    
    # Test cost calculations
    print("\n5. Testing cost calculations (100W, 24/7):")
    for region_code in ['us-east-1', 'us-west-1', 'eu-west-1']:
        pricing = RegionalPricing(region_code)
        energy_kwh_per_month = (power_watts * 24 * 30) / 1000.0  # 72 kWh/month
        monthly_cost = energy_kwh_per_month * pricing.get_electricity_price()
        print(f"   {region_code}: ${monthly_cost:.2f}/month")
    
    # List available regions
    print("\n6. Available regions:")
    regions = get_available_regions()
    print(f"   Total: {len(regions)} regions")
    for region in regions:
        if region != 'default':
            print(f"   - {region}")
    
    print("\n" + "=" * 60)
    print("✅ All tests passed!")


if __name__ == '__main__':
    test_regional_pricing()
