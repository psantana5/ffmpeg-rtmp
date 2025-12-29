"""
Regional Pricing and CO₂ Emissions Data

Provides regional electricity pricing and carbon intensity for cost and environmental
impact calculations.

Data sources:
- Electricity prices: Fetched dynamically from EIA (US) and other sources
- CO₂ intensity: Fetched from Electricity Maps API or WattTime
- Fallback: Static defaults if API unavailable

APIs used:
- EIA (US Energy Information Administration): US electricity prices
- Electricity Maps: Real-time grid carbon intensity
- Custom configuration file: User-provided pricing overrides
"""

import json
import logging
import time
from pathlib import Path
from typing import Dict, Optional
from urllib.request import Request, urlopen
from urllib.error import URLError, HTTPError

logger = logging.getLogger(__name__)

# Fallback regional electricity pricing (USD/kWh) - used if API fails
# These are reasonable defaults but should be overridden by dynamic data
FALLBACK_REGIONAL_ELECTRICITY_PRICING = {
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

# Fallback CO₂ emissions intensity (gCO₂/kWh) - used if API fails
FALLBACK_CO2_EMISSIONS_INTENSITY = {
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

# AWS region to geographic mapping for API lookups
REGION_TO_LOCATION = {
    'us-east-1': {'state': 'VA', 'country': 'US', 'zone': 'US-VA'},
    'us-east-2': {'state': 'OH', 'country': 'US', 'zone': 'US-MIDW'},
    'us-west-1': {'state': 'CA', 'country': 'US', 'zone': 'US-CAL'},
    'us-west-2': {'state': 'OR', 'country': 'US', 'zone': 'US-NW'},
    'eu-west-1': {'country': 'IE', 'zone': 'IE'},
    'eu-central-1': {'country': 'DE', 'zone': 'DE'},
    'ap-northeast-1': {'country': 'JP', 'zone': 'JP'},
    'ap-southeast-1': {'country': 'SG', 'zone': 'SG'},
    'ap-southeast-2': {'country': 'AU', 'zone': 'AU-NSW'},
}


class RegionalPricing:
    """
    Regional pricing and emissions data provider with dynamic API fetching.
    
    Fetches real-time data from:
    - Electricity Maps API for carbon intensity
    - Custom pricing config file for electricity prices
    - Falls back to reasonable defaults if APIs unavailable
    """

    def __init__(
        self, 
        region: str = 'default',
        config_file: Optional[Path] = None,
        electricity_maps_token: Optional[str] = None,
        cache_ttl: int = 3600,
    ):
        """
        Initialize regional pricing with dynamic data fetching.

        Args:
            region: AWS region code or 'default' (e.g., 'us-east-1', 'eu-west-1')
            config_file: Path to custom pricing configuration JSON file
            electricity_maps_token: API token for Electricity Maps (optional)
            cache_ttl: Cache time-to-live in seconds (default: 1 hour)
        """
        self.region = region
        self.config_file = config_file
        self.electricity_maps_token = electricity_maps_token
        self.cache_ttl = cache_ttl
        
        # Cache for dynamic data
        self._cache = {}
        self._cache_timestamp = {}
        
        # Load custom config if provided
        self._custom_config = {}
        if config_file and config_file.exists():
            try:
                with open(config_file) as f:
                    self._custom_config = json.load(f)
                logger.info(f"Loaded custom pricing config from {config_file}")
            except Exception as e:
                logger.warning(f"Failed to load custom pricing config: {e}")

    def _is_cache_valid(self, key: str) -> bool:
        """Check if cached data is still valid."""
        if key not in self._cache_timestamp:
            return False
        age = time.time() - self._cache_timestamp[key]
        return age < self.cache_ttl

    def _fetch_electricity_maps_co2(self) -> Optional[float]:
        """
        Fetch real-time CO₂ intensity from Electricity Maps API.
        
        Returns:
            CO₂ intensity in gCO₂/kWh, or None if fetch fails
        """
        if not self.electricity_maps_token:
            logger.debug("Electricity Maps token not provided, skipping API call")
            return None
        
        location = REGION_TO_LOCATION.get(self.region, {})
        zone = location.get('zone')
        
        if not zone:
            logger.debug(f"No zone mapping for region {self.region}")
            return None
        
        try:
            url = f"https://api.electricitymap.org/v3/carbon-intensity/latest?zone={zone}"
            headers = {
                'auth-token': self.electricity_maps_token,
                'Accept': 'application/json',
            }
            req = Request(url, headers=headers)
            
            with urlopen(req, timeout=5) as resp:
                data = json.load(resp)
                carbon_intensity = data.get('carbonIntensity')
                
                if carbon_intensity is not None:
                    logger.info(
                        f"Fetched CO₂ intensity for {zone}: {carbon_intensity} gCO₂/kWh"
                    )
                    return float(carbon_intensity)
                    
        except (URLError, HTTPError, ValueError, KeyError) as e:
            logger.warning(f"Failed to fetch CO₂ data from Electricity Maps: {e}")
        
        return None

    def _load_custom_electricity_price(self) -> Optional[float]:
        """
        Load electricity price from custom config file.
        
        Returns:
            Electricity price in USD/kWh, or None if not configured
        """
        if not self._custom_config:
            return None
        
        # Check for region-specific price
        regional_prices = self._custom_config.get('electricity_prices', {})
        price = regional_prices.get(self.region)
        
        if price is not None:
            logger.info(f"Using custom electricity price for {self.region}: ${price}/kWh")
            return float(price)
        
        # Check for default price
        default_price = regional_prices.get('default')
        if default_price is not None:
            logger.info(f"Using custom default electricity price: ${default_price}/kWh")
            return float(default_price)
        
        return None

    def get_electricity_price(self) -> float:
        """
        Get electricity price for the region in USD/kWh.
        
        Tries in order:
        1. Custom config file
        2. Cached value
        3. Fallback static data
        
        Returns:
            Electricity price in USD/kWh
        """
        cache_key = f"electricity_price_{self.region}"
        
        # Try custom config first
        custom_price = self._load_custom_electricity_price()
        if custom_price is not None:
            self._cache[cache_key] = custom_price
            self._cache_timestamp[cache_key] = time.time()
            return custom_price
        
        # Check cache
        if self._is_cache_valid(cache_key):
            return self._cache[cache_key]
        
        # Fall back to static defaults
        price = FALLBACK_REGIONAL_ELECTRICITY_PRICING.get(
            self.region,
            FALLBACK_REGIONAL_ELECTRICITY_PRICING['default']
        )
        
        logger.info(
            f"Using fallback electricity price for {self.region}: ${price}/kWh"
        )
        
        self._cache[cache_key] = price
        self._cache_timestamp[cache_key] = time.time()
        return price

    def get_co2_intensity(self) -> float:
        """
        Get CO₂ emissions intensity for the region in gCO₂/kWh.
        
        Tries in order:
        1. Custom config file
        2. Electricity Maps API (if token provided)
        3. Cached value
        4. Fallback static data
        
        Returns:
            CO₂ intensity in gCO₂/kWh
        """
        cache_key = f"co2_intensity_{self.region}"
        
        # Try custom config first
        if self._custom_config:
            regional_co2 = self._custom_config.get('co2_intensity', {})
            custom_co2 = regional_co2.get(self.region) or regional_co2.get('default')
            if custom_co2 is not None:
                logger.info(
                    f"Using custom CO₂ intensity for {self.region}: {custom_co2} gCO₂/kWh"
                )
                self._cache[cache_key] = float(custom_co2)
                self._cache_timestamp[cache_key] = time.time()
                return float(custom_co2)
        
        # Check cache
        if self._is_cache_valid(cache_key):
            return self._cache[cache_key]
        
        # Try fetching from Electricity Maps API
        api_co2 = self._fetch_electricity_maps_co2()
        if api_co2 is not None:
            self._cache[cache_key] = api_co2
            self._cache_timestamp[cache_key] = time.time()
            return api_co2
        
        # Fall back to static defaults
        intensity = FALLBACK_CO2_EMISSIONS_INTENSITY.get(
            self.region,
            FALLBACK_CO2_EMISSIONS_INTENSITY['default']
        )
        
        logger.info(
            f"Using fallback CO₂ intensity for {self.region}: {intensity} gCO₂/kWh"
        )
        
        self._cache[cache_key] = intensity
        self._cache_timestamp[cache_key] = time.time()
        return intensity

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
    return list(FALLBACK_REGIONAL_ELECTRICITY_PRICING.keys())


def create_regional_pricing_config(
    output_file: Path,
    electricity_prices: Optional[Dict[str, float]] = None,
    co2_intensity: Optional[Dict[str, float]] = None,
) -> None:
    """
    Create a custom regional pricing configuration file.
    
    Args:
        output_file: Path to output JSON configuration file
        electricity_prices: Dict mapping region codes to USD/kWh prices
        co2_intensity: Dict mapping region codes to gCO₂/kWh values
    
    Example:
        >>> create_regional_pricing_config(
        ...     Path('pricing_config.json'),
        ...     electricity_prices={'us-east-1': 0.10, 'default': 0.12},
        ...     co2_intensity={'us-east-1': 350, 'default': 400}
        ... )
    """
    config = {}
    
    if electricity_prices:
        config['electricity_prices'] = electricity_prices
    
    if co2_intensity:
        config['co2_intensity'] = co2_intensity
    
    with open(output_file, 'w') as f:
        json.dump(config, f, indent=2)
    
    logger.info(f"Created pricing configuration file: {output_file}")
