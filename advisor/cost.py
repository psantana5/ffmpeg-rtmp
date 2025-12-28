"""
Cost Modeling Module

Provides cost analysis for energy-aware transcoding optimization:
- Cloud pricing models (€/kWh, €/GPU-hour, $/instance-hour)
- Cost per pixel delivered
- Cost per watch hour
- Total cost of ownership (TCO) calculations

This enables cost-optimized transcoding decisions for cloud and edge deployments.
"""

import logging
from typing import Dict, Optional

logger = logging.getLogger(__name__)


class CostModel:
    """
    Computes cost metrics for transcoding scenarios based on energy consumption
    and cloud pricing.
    
    Supports:
        - Energy cost (based on kWh pricing)
        - Compute cost (instance or GPU pricing)
        - Cost per pixel delivered
        - Cost per watch hour
        - TCO analysis
    
    Example:
        >>> model = CostModel(energy_cost_per_kwh=0.12, gpu_cost_per_hour=0.50)
        >>> scenario = {
        ...     'power': {'mean_watts': 100.0},
        ...     'duration': 3600,  # 1 hour
        ...     'resolution': '1920x1080',
        ...     'fps': 30
        ... }
        >>> cost = model.compute_total_cost(scenario)
        >>> print(f"Total cost: ${cost:.4f}")
    """
    
    def __init__(
        self,
        energy_cost_per_kwh: float = 0.0,
        cpu_cost_per_hour: float = 0.0,
        gpu_cost_per_hour: float = 0.0,
        currency: str = 'USD'
    ):
        """
        Initialize cost model with pricing parameters.
        
        Args:
            energy_cost_per_kwh: Cost per kilowatt-hour (€/kWh or $/kWh)
            cpu_cost_per_hour: CPU/instance cost per hour (€/h or $/h)
            gpu_cost_per_hour: GPU cost per hour (€/h or $/h)
            currency: Currency code (USD, EUR, etc.)
        """
        self.energy_cost_per_kwh = energy_cost_per_kwh
        self.cpu_cost_per_hour = cpu_cost_per_hour
        self.gpu_cost_per_hour = gpu_cost_per_hour
        self.currency = currency
        
        logger.info(
            f"CostModel initialized: {energy_cost_per_kwh} {currency}/kWh, "
            f"{cpu_cost_per_hour} {currency}/h (CPU), "
            f"{gpu_cost_per_hour} {currency}/h (GPU)"
        )
    
    def compute_total_cost(self, scenario: Dict) -> Optional[float]:
        """
        Compute total cost for a transcoding scenario.
        
        Total cost includes:
            - Energy cost (power consumption × time × price)
            - Compute cost (CPU + GPU instance time × price)
        
        Args:
            scenario: Scenario dict with 'power', 'duration', optionally 'gpu_power'
            
        Returns:
            Total cost in configured currency, or None if insufficient data
        """
        energy_cost = self.compute_energy_cost(scenario)
        compute_cost = self.compute_compute_cost(scenario)
        
        if energy_cost is None and compute_cost is None:
            return None
        
        total = (energy_cost or 0.0) + (compute_cost or 0.0)
        
        logger.debug(
            f"Scenario '{scenario.get('name')}': "
            f"energy_cost={energy_cost if energy_cost is not None else 0.0:.6f}, "
            f"compute_cost={compute_cost if compute_cost is not None else 0.0:.6f}, "
            f"total={total:.6f} {self.currency}"
        )
        
        return total
    
    def compute_energy_cost(self, scenario: Dict) -> Optional[float]:
        """
        Compute energy cost based on power consumption and duration.
        
        Formula:
            energy_cost = (mean_watts / 1000) * (duration / 3600) * cost_per_kwh
            
        Where:
            mean_watts = CPU power + GPU power (if available)
            duration = test duration in seconds
            cost_per_kwh = energy price
        
        Args:
            scenario: Scenario dict with 'power' and 'duration'
            
        Returns:
            Energy cost in configured currency, or None if data missing
        """
        if self.energy_cost_per_kwh == 0.0:
            return 0.0
        
        # Extract power consumption
        power = scenario.get('power')
        if not power or power.get('mean_watts') is None:
            logger.debug(f"Scenario '{scenario.get('name')}': No power data")
            return None
        
        mean_watts = power['mean_watts']
        
        # Add GPU power if available
        gpu_power = scenario.get('gpu_power', {}).get('mean_watts')
        if gpu_power is not None:
            mean_watts += gpu_power
        
        # Extract duration
        duration = scenario.get('duration')
        if not duration or duration <= 0:
            logger.debug(f"Scenario '{scenario.get('name')}': No duration data")
            return None
        
        # Calculate energy consumed (kWh)
        energy_kwh = (mean_watts / 1000.0) * (duration / 3600.0)
        
        # Calculate cost
        cost = energy_kwh * self.energy_cost_per_kwh
        
        logger.debug(
            f"Scenario '{scenario.get('name')}': "
            f"energy={energy_kwh:.6f} kWh, "
            f"cost={cost:.6f} {self.currency}"
        )
        
        return cost
    
    def compute_compute_cost(self, scenario: Dict) -> Optional[float]:
        """
        Compute compute (instance/GPU) cost based on duration and pricing.
        
        Formula:
            compute_cost = (duration / 3600) * (cpu_cost + gpu_cost)
            
        Args:
            scenario: Scenario dict with 'duration'
            
        Returns:
            Compute cost in configured currency, or None if data missing
        """
        if self.cpu_cost_per_hour == 0.0 and self.gpu_cost_per_hour == 0.0:
            return 0.0
        
        # Extract duration
        duration = scenario.get('duration')
        if not duration or duration <= 0:
            logger.debug(f"Scenario '{scenario.get('name')}': No duration data")
            return None
        
        # Calculate hours
        hours = duration / 3600.0
        
        # Calculate cost (CPU + GPU)
        cost = hours * (self.cpu_cost_per_hour + self.gpu_cost_per_hour)
        
        logger.debug(
            f"Scenario '{scenario.get('name')}': "
            f"hours={hours:.4f}, "
            f"cost={cost:.6f} {self.currency}"
        )
        
        return cost
    
    def compute_cost_per_pixel(self, scenario: Dict) -> Optional[float]:
        """
        Compute cost per pixel delivered.
        
        This metric helps compare cost efficiency across different resolutions
        and output ladders.
        
        Formula:
            cost_per_pixel = total_cost / total_pixels_delivered
            
        Where:
            total_pixels = sum(width * height * fps * duration for each output)
        
        Args:
            scenario: Scenario dict with cost and pixel data
            
        Returns:
            Cost per pixel in configured currency, or None if data missing
        """
        total_cost = self.compute_total_cost(scenario)
        if total_cost is None or total_cost == 0.0:
            return None
        
        # Calculate total pixels
        total_pixels = self._compute_total_pixels(scenario)
        if total_pixels is None or total_pixels <= 0:
            logger.debug(
                f"Scenario '{scenario.get('name')}': No pixel data"
            )
            return None
        
        # Calculate cost per pixel
        cost_per_pixel = total_cost / total_pixels
        
        logger.debug(
            f"Scenario '{scenario.get('name')}': "
            f"total_cost={total_cost:.6f}, "
            f"total_pixels={total_pixels:.2e}, "
            f"cost_per_pixel={cost_per_pixel:.2e} {self.currency}/pixel"
        )
        
        return cost_per_pixel
    
    def compute_cost_per_watch_hour(
        self, scenario: Dict, viewers: int = 1
    ) -> Optional[float]:
        """
        Compute cost per watch hour (viewer-hour).
        
        This metric is useful for understanding streaming costs relative to
        viewer engagement.
        
        Formula:
            cost_per_watch_hour = total_cost / (duration_hours * viewers)
            
        Args:
            scenario: Scenario dict with cost and duration
            viewers: Number of concurrent viewers (default: 1)
            
        Returns:
            Cost per watch hour in configured currency, or None if data missing
        """
        total_cost = self.compute_total_cost(scenario)
        if total_cost is None:
            return None
        
        duration = scenario.get('duration')
        if not duration or duration <= 0:
            return None
        
        if viewers <= 0:
            logger.warning(f"Invalid viewer count: {viewers}")
            return None
        
        # Calculate watch hours
        duration_hours = duration / 3600.0
        watch_hours = duration_hours * viewers
        
        # Calculate cost per watch hour
        cost_per_watch_hour = total_cost / watch_hours
        
        logger.debug(
            f"Scenario '{scenario.get('name')}': "
            f"total_cost={total_cost:.6f}, "
            f"watch_hours={watch_hours:.2f}, "
            f"cost_per_watch_hour={cost_per_watch_hour:.6f} "
            f"{self.currency}/watch-hour"
        )
        
        return cost_per_watch_hour
    
    def _compute_total_pixels(self, scenario: Dict) -> Optional[float]:
        """
        Compute total pixels delivered for a scenario.
        
        Supports two modes:
        1. Output ladder mode: If 'outputs' is present, sum pixels across all
           outputs
        2. Legacy mode: Use single resolution/fps from scenario metadata
        
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
        
        # Legacy mode: single resolution/fps
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
    
    def get_pricing_info(self) -> Dict:
        """
        Get current pricing configuration.
        
        Returns:
            Dict with pricing information
        """
        return {
            'energy_cost_per_kwh': self.energy_cost_per_kwh,
            'cpu_cost_per_hour': self.cpu_cost_per_hour,
            'gpu_cost_per_hour': self.gpu_cost_per_hour,
            'currency': self.currency
        }
