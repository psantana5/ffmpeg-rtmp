"""
Regional Pricing and CO₂ Emissions Data

Provides regional electricity pricing and carbon intensity for cost and environmental
impact calculations.

Data sources:
- Electricity prices: Average commercial/industrial rates by region (2024)
- CO₂ intensity: Grid carbon intensity in gCO₂/kWh by region
"""

from typing import Dict, Optional

# Regional electricity pricing (USD/kWh)
# Source: Average commercial rates for major cloud regions
REGIONAL_ELECTRICITY_PRICING = {
    'us-east-1': 0.12,      # Virginia
    'us-east-2': 0.11,      # Ohio
    'us-west-1': 0.19,      # California
    'us-west-2': 0.10,      # Oregon
    'eu-west-1': 0.22,      # Ireland
    'eu-central-1': 0.28,   # Germany
    'ap-northeast-1': 0.24, # Tokyo
    'ap-southeast-1': 0.18, # Singapore
    'ap-southeast-2': 0.21, # Sydney
    'default': 0.12,        # Default US average
}

# CO₂ emissions intensity (gCO₂/kWh)
# Source: Grid carbon intensity by region
CO2_EMISSIONS_INTENSITY = {
    'us-east-1': 390,       # Virginia - coal/gas mix
    'us-east-2': 450,       # Ohio - higher coal
    'us-west-1': 220,       # California - renewable heavy
    'us-west-2': 180,       # Oregon - hydro heavy
    'eu-west-1': 250,       # Ireland - wind + gas
    'eu-central-1': 380,    # Germany - coal/renewable mix
    'ap-northeast-1': 480,  # Tokyo - gas/coal
    'ap-southeast-1': 520,  # Singapore - gas heavy
    'ap-southeast-2': 650,  # Sydney - coal heavy
    'default': 400,         # Global average
}


class RegionalPricing:
    """Regional pricing and emissions data provider."""

    def __init__(self, region: str = 'default'):
        """
        Initialize regional pricing.

        Args:
            region: AWS region code or 'default' (e.g., 'us-east-1', 'eu-west-1')
        """
        self.region = region

    def get_electricity_price(self) -> float:
        """
        Get electricity price for the region in USD/kWh.

        Returns:
            Electricity price in USD/kWh
        """
        return REGIONAL_ELECTRICITY_PRICING.get(
            self.region,
            REGIONAL_ELECTRICITY_PRICING['default']
        )

    def get_co2_intensity(self) -> float:
        """
        Get CO₂ emissions intensity for the region in gCO₂/kWh.

        Returns:
            CO₂ intensity in gCO₂/kWh
        """
        return CO2_EMISSIONS_INTENSITY.get(
            self.region,
            CO2_EMISSIONS_INTENSITY['default']
        )

    def compute_co2_emissions(self, energy_kwh: float) -> float:
        """
        Compute CO₂ emissions for given energy consumption.

        Args:
            energy_kwh: Energy consumed in kWh

        Returns:
            CO₂ emissions in kg
        """
        intensity = self.get_co2_intensity()
        # Convert gCO₂ to kg: gCO₂ * kWh / 1000
        return (intensity * energy_kwh) / 1000.0

    def compute_monthly_co2(self, power_watts: float, hours_per_day: float = 24) -> float:
        """
        Compute monthly CO₂ emissions at given power draw.

        Args:
            power_watts: Average power consumption in watts
            hours_per_day: Operating hours per day (default: 24)

        Returns:
            Monthly CO₂ emissions in kg
        """
        # Energy per month: watts * hours_per_day * 30 days / 1000 (to kWh)
        energy_kwh_per_month = (power_watts * hours_per_day * 30) / 1000.0
        return self.compute_co2_emissions(energy_kwh_per_month)

    def get_pricing_info(self) -> Dict:
        """
        Get pricing and emissions information for the region.

        Returns:
            Dict with region, electricity price, and CO₂ intensity
        """
        return {
            'region': self.region,
            'electricity_price_usd_per_kwh': self.get_electricity_price(),
            'co2_intensity_g_per_kwh': self.get_co2_intensity(),
        }


def get_available_regions() -> list:
    """
    Get list of available regions.

    Returns:
        List of region codes
    """
    return list(REGIONAL_ELECTRICITY_PRICING.keys())
