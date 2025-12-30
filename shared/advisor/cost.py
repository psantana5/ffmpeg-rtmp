"""
Cost Modeling Module

Provides cost analysis for energy-aware transcoding optimization:
- Cloud pricing models (€/kWh, €/GPU-hour, $/instance-hour, $/core-second)
- Load-aware compute cost based on actual CPU usage (not wall-clock time)
- Energy cost based on integrated power measurements (joules)
- Cost per pixel delivered
- Cost per watch hour
- Total cost of ownership (TCO) calculations

This enables cost-optimized transcoding decisions for cloud and edge deployments.

Cost Formulas (Load-Aware with Trapezoidal Integration):

    Compute Cost:
        compute_cost = (∫ cpu_usage_cores(t) dt) × PRICE_PER_CORE_SECOND

        Using trapezoidal rule for numerical integration:
        ≈ (Δt/2) × [cpu₀ + 2×cpu₁ + 2×cpu₂ + ... + 2×cpuₙ₋₁ + cpuₙ] × PRICE

    Energy Cost:
        energy_joules = ∫ power_watts(t) dt
        energy_cost = energy_joules × PRICE_PER_JOULE

        Using trapezoidal rule:
        ≈ (Δt/2) × [P₀ + 2×P₁ + 2×P₂ + ... + 2×Pₙ₋₁ + Pₙ] × PRICE

    Total Cost:
        total_cost = compute_cost + energy_cost

Mathematical Justification:
    The trapezoidal rule provides O(h²) accuracy for smooth functions,
    significantly better than rectangular approximation's O(h) accuracy.
    For time-series data with varying rates, this captures the actual
    area under the curve more accurately.
"""

import logging
from typing import Dict, List, Optional

logger = logging.getLogger(__name__)


class CostModel:
    """
    Computes cost metrics for transcoding scenarios based on energy consumption
    and cloud pricing.

    Supports two modes:
        1. Legacy mode (duration-based): Uses wall-clock duration and hourly pricing
        2. Load-aware mode (recommended): Uses actual CPU usage and power measurements

    Load-aware formulas:
        - Compute cost: sum(cpu_usage_cores * step_seconds) * price_per_core_second
        - Energy cost: sum(power_watts * step_seconds) * price_per_joule
        - Total cost: compute_cost + energy_cost

    Example (legacy mode):
        >>> model = CostModel(energy_cost_per_kwh=0.12, cpu_cost_per_hour=0.50)
        >>> scenario = {
        ...     'power': {'mean_watts': 100.0},
        ...     'duration': 3600,  # 1 hour
        ...     'resolution': '1920x1080',
        ...     'fps': 30
        ... }
        >>> cost = model.compute_total_cost(scenario)
        >>> print(f"Total cost: ${cost:.4f}")

    Example (load-aware mode):
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
        energy_cost_per_kwh: float = 0.0,
        cpu_cost_per_hour: float = 0.0,
        gpu_cost_per_hour: float = 0.0,
        currency: str = 'USD',
        # New load-aware pricing parameters
        price_per_core_second: Optional[float] = None,
        price_per_joule: Optional[float] = None,
    ):
        """
        Initialize cost model with pricing parameters.

        Args:
            energy_cost_per_kwh: Cost per kilowatt-hour (€/kWh or $/kWh) - legacy mode
            cpu_cost_per_hour: CPU/instance cost per hour (€/h or $/h) - legacy mode
            gpu_cost_per_hour: GPU cost per hour (€/h or $/h) - legacy mode
            currency: Currency code (USD, EUR, etc.)
            price_per_core_second: Cost per core-second ($/core-second) - load-aware mode
            price_per_joule: Cost per joule ($/J) - load-aware mode

        Note:
            For load-aware mode, set price_per_core_second and price_per_joule.
            Conversion helpers:
                - price_per_core_second = cpu_cost_per_hour / 3600
                - price_per_joule = energy_cost_per_kwh / 3_600_000
        """
        self.energy_cost_per_kwh = energy_cost_per_kwh
        self.cpu_cost_per_hour = cpu_cost_per_hour
        self.gpu_cost_per_hour = gpu_cost_per_hour
        self.currency = currency

        # Load-aware pricing
        if price_per_core_second is not None:
            self.price_per_core_second = price_per_core_second
        else:
            # Auto-derive from hourly rate if not provided
            self.price_per_core_second = (
                cpu_cost_per_hour / 3600.0 if cpu_cost_per_hour > 0 else 0.0
            )

        if price_per_joule is not None:
            self.price_per_joule = price_per_joule
        else:
            # Auto-derive from kWh rate if not provided
            # 1 kWh = 3,600,000 joules
            self.price_per_joule = (
                energy_cost_per_kwh / 3_600_000.0 if energy_cost_per_kwh > 0 else 0.0
            )

        logger.info(
            f"CostModel initialized: {energy_cost_per_kwh} {currency}/kWh, "
            f"{cpu_cost_per_hour} {currency}/h (CPU), "
            f"{gpu_cost_per_hour} {currency}/h (GPU)"
        )
        logger.info(
            f"Load-aware pricing: {self.price_per_core_second:.9f} {currency}/core-second, "
            f"{self.price_per_joule:.2e} {currency}/joule"
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
            f"Scenario '{scenario.get('name')}': hours={hours:.4f}, cost={cost:.6f} {self.currency}"
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
            logger.debug(f"Scenario '{scenario.get('name')}': No pixel data")
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

    def compute_cost_per_watch_hour(self, scenario: Dict, viewers: int = 1) -> Optional[float]:
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

    def _trapezoidal_integrate(self, values: List[float], step: float) -> float:
        """
        Numerically integrate time-series data using the trapezoidal rule.

        The trapezoidal rule provides better accuracy than rectangular approximation
        by treating each interval as a trapezoid rather than a rectangle.

        Formula:
            ∫[a,b] f(x)dx ≈ h/2 * [f(x₀) + 2f(x₁) + 2f(x₂) + ... + 2f(xₙ₋₁) + f(xₙ)]

        Where:
            h = step size (time interval between measurements)
            f(xᵢ) = value at measurement i

        Args:
            values: List of measurements at regular intervals
            step: Time interval between measurements (seconds)

        Returns:
            Integrated value (area under the curve)

        Example:
            values = [1.0, 2.0, 3.0, 2.5, 2.0]  # CPU cores over time
            step = 5.0  # 5 seconds between measurements
            result = _trapezoidal_integrate(values, step)
            # result ≈ 5.0/2 * (1.0 + 2*2.0 + 2*3.0 + 2*2.5 + 2.0) = 51.25 core-seconds
        """
        if not values or len(values) == 0:
            return 0.0

        if len(values) == 1:
            # Single point: area = value * step (rectangular)
            return values[0] * step

        # Trapezoidal rule: (step/2) * [first + 2*(middle terms) + last]
        integral = values[0] + values[-1]  # First and last terms
        integral += 2.0 * sum(values[1:-1])  # Middle terms (weighted by 2)
        integral *= step / 2.0

        return integral

    # ========================================================================
    # Load-Aware Cost Calculation Methods (Recommended)
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

        Formula (using trapezoidal integration for accuracy):
            compute_cost = ∫ cpu_usage_cores(t) dt * price_per_core_second
            ≈ (step_seconds / 2) * Σ(cpu[i] + cpu[i+1]) * price_per_core_second

        This uses the trapezoidal rule for numerical integration, which provides
        better accuracy than rectangular approximation by accounting for the
        slope between measurements.

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

        if len(cpu_usage_cores) == 0:
            return 0.0

        # Use trapezoidal integration for better accuracy
        # Integral ≈ (h/2) * [y0 + 2*y1 + 2*y2 + ... + 2*y(n-1) + yn]
        total_core_seconds = self._trapezoidal_integrate(cpu_usage_cores, step_seconds)

        # Add GPU if available and pricing is configured
        gpu_usage_cores = scenario.get('gpu_usage_cores')
        if gpu_usage_cores and isinstance(gpu_usage_cores, list) and len(gpu_usage_cores) > 0:
            # Use same pricing for GPU cores (can be extended later)
            total_core_seconds += self._trapezoidal_integrate(gpu_usage_cores, step_seconds)

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

        Formula (using trapezoidal integration for accuracy):
            energy_joules = ∫ power_watts(t) dt
            ≈ (step_seconds / 2) * Σ(power[i] + power[i+1])
            energy_cost = energy_joules * price_per_joule

        This uses the trapezoidal rule for numerical integration of power over time,
        which provides accurate energy consumption (in Joules) by accounting for the
        variation between measurements rather than assuming constant power within intervals.

        Mathematical basis:
            Energy = ∫ Power dt (Joules = Watts × seconds)
            Using trapezoidal rule: ∫[a,b] f(x)dx ≈ (b-a)/2 * [f(a) + f(b)]

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

        if len(power_watts) == 0:
            return 0.0

        # Use trapezoidal integration for accurate energy calculation
        energy_joules = self._trapezoidal_integrate(power_watts, step_seconds)

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
            logger.debug(f"Scenario '{scenario.get('name')}': No pixel data")
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
            Dict with pricing information (legacy and load-aware)
        """
        return {
            # Legacy pricing
            'energy_cost_per_kwh': self.energy_cost_per_kwh,
            'cpu_cost_per_hour': self.cpu_cost_per_hour,
            'gpu_cost_per_hour': self.gpu_cost_per_hour,
            'currency': self.currency,
            # Load-aware pricing
            'price_per_core_second': self.price_per_core_second,
            'price_per_joule': self.price_per_joule,
        }
