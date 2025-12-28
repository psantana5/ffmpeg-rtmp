"""
Cost Modeling Module

Provides load-aware cost analysis for energy-aware transcoding optimization:
- Cloud pricing models ($/core-second, $/joule)
- Load-aware compute cost based on actual CPU usage (not wall-clock time)
- Energy cost based on integrated power measurements (joules)
- Cost per pixel delivered
- Cost per watch hour
- Total cost of ownership (TCO) calculations

This enables cost-optimized transcoding decisions for cloud and edge deployments.

Cost Formulas (Load-Aware):
    compute_cost = sum(cpu_usage_cores[i] * step_seconds for each i) * PRICE_PER_CORE_SECOND
    energy_cost = sum(power_watts[i] * step_seconds for each i) * PRICE_PER_JOULE
    total_cost = compute_cost + energy_cost
"""

import logging
from typing import Dict, Optional

logger = logging.getLogger(__name__)


class CostModel:
    """
    Computes load-aware cost metrics for transcoding scenarios based on actual
    CPU usage and power consumption.
    
    Load-aware formulas:
        - Compute cost: sum(cpu_usage_cores * step_seconds) * price_per_core_second
        - Energy cost: sum(power_watts * step_seconds) * price_per_joule
        - Total cost: compute_cost + energy_cost
    
    Example:
        >>> model = CostModel(
        ...     price_per_core_second=0.000138889,  # $0.50/hour / 3600
        ...     price_per_joule=3.33e-8  # $0.12/kWh / 3.6e6
        ... )
        >>> scenario = {
        ...     'cpu_usage_cores': [2.5, 2.8, 3.0],  # CPU cores over time
        ...     'power_watts': [150.0, 155.0, 160.0],  # Power over time
        ...     'step_seconds': 5,  # Measurement interval
        ...     'resolution': '1920x1080',
        ...     'fps': 30
        ... }
        >>> cost = model.compute_total_cost_load_aware(scenario)
        >>> print(f"Total cost: ${cost:.4f}")
    """
    
    def __init__(
        self,
        price_per_core_second: float = 0.0,
        price_per_joule: float = 0.0,
        currency: str = 'USD'
    ):
        """
        Initialize cost model with load-aware pricing parameters.
        
        Args:
            price_per_core_second: Cost per core-second ($/core-second)
            price_per_joule: Cost per joule ($/J)
            currency: Currency code (USD, EUR, etc.)
        
        Note:
            Conversion helpers from hourly rates:
                - price_per_core_second = cpu_cost_per_hour / 3600
                - price_per_joule = energy_cost_per_kwh / 3_600_000
        """
        self.price_per_core_second = price_per_core_second
        self.price_per_joule = price_per_joule
        self.currency = currency
        
        logger.info(
            f"CostModel initialized (load-aware only): "
            f"{self.price_per_core_second:.9f} {currency}/core-second, "
            f"{self.price_per_joule:.2e} {currency}/joule"
        )
    
    def _compute_total_pixels(self, scenario: Dict) -> Optional[float]:
        """
        Compute total pixels delivered for a scenario.
        
        Supports two modes:
        1. Output ladder mode: If 'outputs' is present, sum pixels across all outputs
        2. Single resolution mode: Use resolution/fps from scenario metadata
        
        Args:
            scenario: Scenario dict
            
        Returns:
            Total pixels delivered (float) or None if insufficient data
        """
        duration = scenario.get('duration')
        if not duration or duration <= 0:
            return None
        
        # Check for output ladder mode
        outputs = scenario.get('outputs')
        if outputs and isinstance(outputs, list) and len(outputs) > 0:
            # Output ladder mode: sum pixels across all outputs
            total_pixels = 0.0
            for output in outputs:
                resolution = output.get('resolution')
                fps = output.get('fps')
                
                if not resolution or not fps:
                    continue
                
                width, height = self._parse_resolution(resolution)
                if width is None or height is None:
                    continue
                
                # pixels = width * height * fps * duration
                pixels = width * height * fps * duration
                total_pixels += pixels
            
            return total_pixels if total_pixels > 0 else None
        
        # Single resolution mode
        resolution = scenario.get('resolution')
        fps = scenario.get('fps')
        
        if not resolution or resolution == 'N/A':
            return None
        if not fps or fps == 'N/A':
            return None
        
        width, height = self._parse_resolution(resolution)
        if width is None or height is None:
            return None
        
        total_pixels = width * height * fps * duration
        return total_pixels
    
    def _parse_resolution(self, resolution: str) -> tuple:
        """
        Parse resolution string to width and height.
        
        Args:
            resolution: Resolution string (e.g., "1920x1080")
            
        Returns:
            Tuple of (width, height) or (None, None) if parsing fails
        """
        if not resolution or resolution == 'N/A':
            return (None, None)
        
        try:
            parts = resolution.lower().split('x')
            if len(parts) == 2:
                width = int(parts[0].strip())
                height = int(parts[1].strip())
                return (width, height)
        except (ValueError, AttributeError):
            pass
        
        return (None, None)
    
    # ========================================================================
    # Load-Aware Cost Calculation Methods
    # ========================================================================
    
    def compute_total_cost_load_aware(self, scenario: Dict) -> Optional[float]:
        """
        Compute total cost using load-aware approach (actual CPU usage + power).
        
        This is the recommended method for accurate cost analysis.
        
        Formula:
            total_cost = compute_cost_load_aware + energy_cost_load_aware
        
        Args:
            scenario: Scenario dict with CPU usage and power time series data
                Required fields:
                    - cpu_usage_cores: List of CPU core usage values over time
                    - power_watts: List of power measurements (watts) over time
                    - step_seconds: Time interval between measurements
                Optional fields:
                    - gpu_usage_cores: GPU usage (if applicable)
        
        Returns:
            Total cost in configured currency, or None if insufficient data
        """
        compute_cost = self.compute_compute_cost_load_aware(scenario)
        energy_cost = self.compute_energy_cost_load_aware(scenario)
        
        if compute_cost is None and energy_cost is None:
            return None
        
        total = (compute_cost or 0.0) + (energy_cost or 0.0)
        
        logger.debug(
            f"Scenario '{scenario.get('name')}' (load-aware): "
            f"compute_cost={compute_cost if compute_cost is not None else 0.0:.6f}, "
            f"energy_cost={energy_cost if energy_cost is not None else 0.0:.6f}, "
            f"total={total:.6f} {self.currency}"
        )
        
        return total
    
    def compute_compute_cost_load_aware(self, scenario: Dict) -> Optional[float]:
        """
        Compute compute cost based on actual CPU usage (load-aware).
        
        Formula:
            compute_cost = sum(cpu_usage_cores[i] * step_seconds) * price_per_core_second
        
        This scales with actual load, not wall-clock time:
            - More streams → higher CPU usage → higher cost
            - Higher bitrate → higher CPU usage → higher cost
            - Idle periods → minimal cost
        
        Args:
            scenario: Scenario dict with CPU usage time series
                Required fields:
                    - cpu_usage_cores: List of CPU core usage values
                    - step_seconds: Time interval between measurements
                Optional fields:
                    - gpu_usage_cores: GPU usage (if applicable)
        
        Returns:
            Compute cost in configured currency, or None if data missing
        """
        if self.price_per_core_second == 0.0:
            return 0.0
        
        cpu_usage_cores = scenario.get('cpu_usage_cores')
        step_seconds = scenario.get('step_seconds')
        
        if not cpu_usage_cores or not isinstance(cpu_usage_cores, list):
            logger.debug(f"Scenario '{scenario.get('name')}': No CPU usage data")
            return None
        
        if not step_seconds or step_seconds <= 0:
            logger.debug(f"Scenario '{scenario.get('name')}': No step_seconds data")
            return None
        
        # Sum CPU core-seconds
        total_core_seconds = sum(cpu_usage_cores) * step_seconds
        
        # Add GPU if available and pricing is configured
        gpu_usage_cores = scenario.get('gpu_usage_cores')
        if gpu_usage_cores and isinstance(gpu_usage_cores, list):
            # Use same pricing for GPU cores (can be extended later)
            total_core_seconds += sum(gpu_usage_cores) * step_seconds
        
        # Calculate cost
        cost = total_core_seconds * self.price_per_core_second
        
        logger.debug(
            f"Scenario '{scenario.get('name')}' (load-aware): "
            f"core_seconds={total_core_seconds:.2f}, "
            f"cost={cost:.6f} {self.currency}"
        )
        
        return cost
    
    def compute_energy_cost_load_aware(self, scenario: Dict) -> Optional[float]:
        """
        Compute energy cost from integrated power measurements (load-aware).
        
        Formula:
            energy_joules = sum(power_watts[i] * step_seconds)
            energy_cost = energy_joules * price_per_joule
        
        This integrates actual power consumption over time, not average power.
        
        Args:
            scenario: Scenario dict with power time series
                Required fields:
                    - power_watts: List of power measurements (watts)
                    - step_seconds: Time interval between measurements
        
        Returns:
            Energy cost in configured currency, or None if data missing
        """
        if self.price_per_joule == 0.0:
            return 0.0
        
        power_watts = scenario.get('power_watts')
        step_seconds = scenario.get('step_seconds')
        
        if not power_watts or not isinstance(power_watts, list):
            logger.debug(f"Scenario '{scenario.get('name')}': No power data")
            return None
        
        if not step_seconds or step_seconds <= 0:
            logger.debug(f"Scenario '{scenario.get('name')}': No step_seconds data")
            return None
        
        # Integrate power over time to get energy in joules
        energy_joules = sum(power_watts) * step_seconds
        
        # Calculate cost
        cost = energy_joules * self.price_per_joule
        
        logger.debug(
            f"Scenario '{scenario.get('name')}' (load-aware): "
            f"energy={energy_joules:.2f} J, "
            f"cost={cost:.6f} {self.currency}"
        )
        
        return cost
    
    def compute_cost_per_pixel_load_aware(self, scenario: Dict) -> Optional[float]:
        """
        Compute cost per pixel delivered using load-aware cost calculation.
        
        Args:
            scenario: Scenario dict with cost and pixel data
            
        Returns:
            Cost per pixel in configured currency, or None if data missing
        """
        total_cost = self.compute_total_cost_load_aware(scenario)
        if total_cost is None or total_cost == 0.0:
            return None
        
        total_pixels = self._compute_total_pixels(scenario)
        if total_pixels is None or total_pixels <= 0:
            logger.debug(
                f"Scenario '{scenario.get('name')}': No pixel data"
            )
            return None
        
        cost_per_pixel = total_cost / total_pixels
        
        logger.debug(
            f"Scenario '{scenario.get('name')}' (load-aware): "
            f"total_cost={total_cost:.6f}, "
            f"total_pixels={total_pixels:.2e}, "
            f"cost_per_pixel={cost_per_pixel:.2e} {self.currency}/pixel"
        )
        
        return cost_per_pixel
    
    def compute_cost_per_watch_hour_load_aware(
        self, scenario: Dict, viewers: Optional[int] = None
    ) -> Optional[float]:
        """
        Compute cost per watch hour using load-aware cost calculation.
        
        Note: Viewer count must be provided or present in scenario data.
        Do NOT use hardcoded values.
        
        Formula:
            cost_per_watch_hour = total_cost / (duration_hours * viewers)
        
        Args:
            scenario: Scenario dict with cost and duration
                Required fields:
                    - duration: Test duration in seconds
                Optional fields:
                    - viewers: Number of viewers (can also be passed as parameter)
            viewers: Number of concurrent viewers (overrides scenario value)
            
        Returns:
            Cost per watch hour in configured currency, or None if data missing
        """
        total_cost = self.compute_total_cost_load_aware(scenario)
        if total_cost is None:
            return None
        
        duration = scenario.get('duration')
        if not duration or duration <= 0:
            return None
        
        # Get viewer count from parameter or scenario
        if viewers is None:
            viewers = scenario.get('viewers')
        
        if viewers is None or viewers <= 0:
            logger.warning(
                f"Scenario '{scenario.get('name')}': No viewer count provided. "
                "Use viewers parameter or include 'viewers' in scenario data."
            )
            return None
        
        # Calculate watch hours
        duration_hours = duration / 3600.0
        watch_hours = duration_hours * viewers
        
        # Calculate cost per watch hour
        cost_per_watch_hour = total_cost / watch_hours
        
        logger.debug(
            f"Scenario '{scenario.get('name')}' (load-aware): "
            f"total_cost={total_cost:.6f}, "
            f"watch_hours={watch_hours:.2f}, "
            f"cost_per_watch_hour={cost_per_watch_hour:.6f} "
            f"{self.currency}/watch-hour"
        )
        
        return cost_per_watch_hour
    
    def get_pricing_info(self) -> Dict:
        """
        Get current pricing configuration.
        
        Returns:
            Dict with load-aware pricing information
        """
        return {
            'price_per_core_second': self.price_per_core_second,
            'price_per_joule': self.price_per_joule,
            'currency': self.currency
        }
