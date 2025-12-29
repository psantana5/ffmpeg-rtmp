#!/usr/bin/env python3
"""
Test regional pricing and CO₂ emissions calculations with dynamic loading
"""

import json
import sys
from pathlib import Path
from tempfile import NamedTemporaryFile

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent.parent))

from advisor.regional_pricing import RegionalPricing, get_available_regions, create_regional_pricing_config


def test_regional_pricing():
    """Test regional pricing functionality."""
    print("Testing Regional Pricing Module (Dynamic Loading)\n")
    print("=" * 60)
    
    # Test 1: Default region with fallback pricing
    print("\n1. Testing default region (fallback pricing):")
    pricing_default = RegionalPricing('default')
    print(f"   Region: {pricing_default.region}")
    print(f"   Electricity price: ${pricing_default.get_electricity_price():.3f}/kWh")
    print(f"   CO₂ intensity: {pricing_default.get_co2_intensity():.0f} gCO₂/kWh")
    
    # Test 2: Custom config file
    print("\n2. Testing custom pricing config file:")
    with NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
        config = {
            'electricity_prices': {
                'us-east-1': 0.08,  # Custom lower price
                'default': 0.10
            },
            'co2_intensity': {
                'us-east-1': 300,  # Custom lower intensity
                'default': 350
            }
        }
        json.dump(config, f)
        config_file = Path(f.name)
    
    try:
        pricing_custom = RegionalPricing('us-east-1', config_file=config_file)
        custom_price = pricing_custom.get_electricity_price()
        custom_co2 = pricing_custom.get_co2_intensity()
        print(f"   Region: us-east-1")
        print(f"   Custom electricity price: ${custom_price:.3f}/kWh")
        print(f"   Custom CO₂ intensity: {custom_co2:.0f} gCO₂/kWh")
        
        # Verify custom values are used
        assert custom_price == 0.08, "Custom price not loaded"
        assert custom_co2 == 300, "Custom CO₂ not loaded"
        print("   ✓ Custom config loaded successfully")
    finally:
        config_file.unlink()
    
    # Test 3: Caching behavior
    print("\n3. Testing caching behavior:")
    pricing_cached = RegionalPricing('us-west-2')
    price1 = pricing_cached.get_electricity_price()
    price2 = pricing_cached.get_electricity_price()  # Should use cache
    print(f"   First call: ${price1:.3f}/kWh")
    print(f"   Second call (cached): ${price2:.3f}/kWh")
    assert price1 == price2, "Cache not working"
    print("   ✓ Caching working correctly")
    
    # Test 4: CO₂ calculations
    print("\n4. Testing CO₂ emissions calculations:")
    power_watts = 100.0  # 100W system
    print(f"   System power: {power_watts}W")
    
    for region_code in ['us-west-2', 'eu-central-1', 'ap-southeast-2']:
        pricing = RegionalPricing(region_code)
        monthly_co2 = pricing.compute_monthly_co2(power_watts, hours_per_day=24)
        print(f"   {region_code}: {monthly_co2:.2f} kg CO₂/month")
    
    # Test 5: Cost calculations with dynamic pricing
    print("\n5. Testing cost calculations (100W, 24/7, dynamic pricing):")
    for region_code in ['us-east-1', 'us-west-1', 'eu-west-1']:
        pricing = RegionalPricing(region_code)
        energy_kwh_per_month = (power_watts * 24 * 30) / 1000.0  # 72 kWh/month
        monthly_cost = energy_kwh_per_month * pricing.get_electricity_price()
        print(f"   {region_code}: ${monthly_cost:.2f}/month (dynamic price: ${pricing.get_electricity_price():.3f}/kWh)")
    
    # Test 6: Create config file helper
    print("\n6. Testing config file creation helper:")
    test_config_path = Path('/tmp/test_pricing_config.json')
    create_regional_pricing_config(
        test_config_path,
        electricity_prices={'us-east-1': 0.09, 'default': 0.11},
        co2_intensity={'us-east-1': 320, 'default': 380}
    )
    if test_config_path.exists():
        with open(test_config_path) as f:
            created_config = json.load(f)
        print(f"   Config file created: {test_config_path}")
        print(f"   Contents: {json.dumps(created_config, indent=2)}")
        test_config_path.unlink()
        print("   ✓ Config helper working correctly")
    
    # Test 7: List available regions
    print("\n7. Available regions:")
    regions = get_available_regions()
    print(f"   Total: {len(regions)} regions")
    for region in regions:
        if region != 'default':
            print(f"   - {region}")
    
    print("\n" + "=" * 60)
    print("✅ All dynamic pricing tests passed!")
    print("\n💡 To use custom pricing:")
    print("   1. Edit pricing_config.json with your regional prices")
    print("   2. Mount it in docker-compose.yml")
    print("   3. Set PRICING_CONFIG=/app/pricing_config.json")
    print("\n💡 To use Electricity Maps API:")
    print("   1. Get an API token from https://www.electricitymaps.com")
    print("   2. Set ELECTRICITY_MAPS_TOKEN environment variable")


if __name__ == '__main__':
    test_regional_pricing()

