"""
Energy Efficiency Scoring Module

Provides scoring algorithms to evaluate transcoding configurations based on:
- Energy consumption (watts, watt-hours)
- Resource utilization (CPU %, GPU %, cores)
- Throughput (bitrate, stream count)
- (Future) Video quality metrics (VMAF, PSNR)

Design principles:
- Use measured metrics only (no synthetic/estimated values)
- Pluggable scoring algorithms for easy extension
- Production-grade numerical stability
- Clear documentation of formulas and assumptions
"""

import logging
import re
from typing import Dict, Optional, Tuple

logger = logging.getLogger(__name__)


class EnergyEfficiencyScorer:
    """
    Computes energy efficiency scores for transcoding scenarios.
    
    Version 0.1: Simple deterministic scoring based on throughput/power ratio.
    Designed to be extended with multi-objective scoring, quality metrics, etc.
    
    The efficiency score represents: "How much video throughput per watt?"
    Higher scores indicate better energy efficiency.
    """
    
    def __init__(self, algorithm: str = 'throughput_per_watt'):
        """
        Initialize the scorer.
        
        Args:
            algorithm: Scoring algorithm to use. Currently supports:
                - 'throughput_per_watt': efficiency_score = throughput / power
                - 'pixels_per_joule': efficiency_score = total_pixels / total_energy_joules
        """
        self.algorithm = algorithm
        self.supported_algorithms = {'throughput_per_watt', 'pixels_per_joule'}
        
        if algorithm not in self.supported_algorithms:
            raise ValueError(
                f"Unsupported algorithm '{algorithm}'. "
                f"Supported: {self.supported_algorithms}"
            )
    
    def compute_score(self, scenario: Dict) -> Optional[float]:
        """
        Compute energy efficiency score for a scenario.
        
        Args:
            scenario: Analysis result dict containing:
                - bitrate: str (e.g., "2500k", "5M")
                - power: dict with 'mean_watts'
                - Optional: stream count embedded in scenario name or metadata
                - Optional: outputs: list of output resolutions/fps for pixel-based scoring
                - Optional: duration: duration in seconds for pixel calculation
        
        Returns:
            Energy efficiency score (float) or None if insufficient data.
            
        Formula (v0.1):
            efficiency_score = throughput_mbps / mean_watts
            
            Where:
                throughput_mbps = parsed_bitrate_mbps * num_streams
                mean_watts = CPU power + GPU power (if available)
        
        Formula (v0.2 - pixels_per_joule):
            efficiency_score = total_pixels_delivered / total_energy_joules
            
            Where:
                total_pixels_delivered = sum(width * height * fps * duration for each output)
                total_energy_joules = power['total_energy_joules']
        
        Design notes:
            - Returns None for baseline/idle scenarios (bitrate = "0k")
            - Returns None if power data is missing or invalid
            - GPU power is automatically included if present in scenario data
            - Future versions will incorporate video quality metrics
        """
        if self.algorithm == 'throughput_per_watt':
            return self._compute_throughput_per_watt(scenario)
        elif self.algorithm == 'pixels_per_joule':
            return self._compute_pixels_per_joule(scenario)
        
        return None
    
    def _compute_throughput_per_watt(self, scenario: Dict) -> Optional[float]:
        """
        Compute throughput-per-watt efficiency score.
        
        This is the v0.1 scoring function: simple, interpretable, deterministic.
        """
        # Extract power consumption (CPU + GPU if available)
        power = scenario.get('power')
        if not power or power.get('mean_watts') is None:
            logger.debug(f"Scenario '{scenario.get('name')}': No power data available")
            return None
        
        mean_watts = power['mean_watts']
        
        # Add GPU power if available
        # (Future: DCGM exporter integration would add gpu_power to scenario dict)
        gpu_power = scenario.get('gpu_power', {}).get('mean_watts')
        if gpu_power is not None:
            mean_watts += gpu_power
        
        if mean_watts <= 0:
            logger.debug(f"Scenario '{scenario.get('name')}': Invalid power value {mean_watts}")
            return None
        
        # Parse bitrate and stream count to compute throughput
        bitrate_str = scenario.get('bitrate', '0k')
        num_streams = self._extract_stream_count(scenario)
        
        throughput_mbps = self._parse_bitrate_to_mbps(bitrate_str) * num_streams
        
        # Skip baseline scenarios (no actual streaming)
        if throughput_mbps <= 0:
            logger.debug(f"Scenario '{scenario.get('name')}': Baseline scenario (0 throughput)")
            return None
        
        # Compute efficiency: Mbps per watt
        efficiency_score = throughput_mbps / mean_watts
        
        logger.debug(
            f"Scenario '{scenario.get('name')}': "
            f"throughput={throughput_mbps:.2f} Mbps, "
            f"power={mean_watts:.2f} W, "
            f"score={efficiency_score:.4f} Mbps/W"
        )
        
        return efficiency_score
    
    def _compute_pixels_per_joule(self, scenario: Dict) -> Optional[float]:
        """
        Compute pixels-per-joule efficiency score (output ladder aware).
        
        This scoring function is designed for scenarios with multiple output resolutions.
        It calculates the total pixels delivered across all outputs and divides by
        the total energy consumed.
        
        Formula:
            efficiency_score = total_pixels_delivered / total_energy_joules
            
        Where:
            total_pixels_delivered = sum(width * height * fps * duration for each output)
            total_energy_joules = scenario['power']['total_energy_joules']
        """
        # Extract energy consumption in joules
        power = scenario.get('power')
        if not power or power.get('total_energy_joules') is None:
            logger.debug(f"Scenario '{scenario.get('name')}': No energy data available")
            return None
        
        total_energy_joules = power['total_energy_joules']
        
        if total_energy_joules <= 0:
            logger.debug(
                f"Scenario '{scenario.get('name')}': "
                f"Invalid energy value {total_energy_joules}"
            )
            return None
        
        # Calculate total pixels delivered
        total_pixels = self._compute_total_pixels(scenario)
        
        if total_pixels is None or total_pixels <= 0:
            logger.debug(f"Scenario '{scenario.get('name')}': No valid pixel data")
            return None
        
        # Compute efficiency: pixels per joule
        efficiency_score = total_pixels / total_energy_joules
        
        logger.debug(
            f"Scenario '{scenario.get('name')}': "
            f"total_pixels={total_pixels:.2e}, "
            f"energy={total_energy_joules:.2f} J, "
            f"score={efficiency_score:.4e} pixels/J"
        )
        
        return efficiency_score
    
    def _compute_total_pixels(self, scenario: Dict) -> Optional[float]:
        """
        Compute total pixels delivered for a scenario.
        
        Supports two modes:
        1. Output ladder mode: If 'outputs' is present, sum pixels across all outputs
        2. Legacy mode: Use single resolution/fps from scenario metadata
        
        Args:
            scenario: Scenario dict
            
        Returns:
            Total pixels delivered (float) or None if insufficient data
        """
        duration = scenario.get('duration')
        if not duration or duration <= 0:
            logger.debug(f"Scenario '{scenario.get('name')}': No valid duration")
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
                    logger.warning(f"Output missing resolution or fps: {output}")
                    continue
                
                width, height = self._parse_resolution(resolution)
                if width is None or height is None:
                    logger.warning(f"Failed to parse resolution: {resolution}")
                    continue
                
                # pixels = width * height * fps * duration
                pixels = width * height * fps * duration
                total_pixels += pixels
            
            return total_pixels if total_pixels > 0 else None
        
        # Legacy mode: single resolution/fps
        resolution = scenario.get('resolution')
        fps = scenario.get('fps')
        
        if not resolution or resolution == 'N/A' or not fps or fps == 'N/A':
            logger.debug(f"Scenario '{scenario.get('name')}': No resolution/fps data")
            return None
        
        width, height = self._parse_resolution(resolution)
        if width is None or height is None:
            return None
        
        total_pixels = width * height * fps * duration
        return total_pixels
    
    def _parse_resolution(self, resolution: str) -> Tuple[Optional[int], Optional[int]]:
        """
        Parse resolution string to width and height.
        
        Supports formats:
            - "1920x1080" -> (1920, 1080)
            - "1280x720" -> (1280, 720)
            - "854x480" -> (854, 480)
        
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
        except (ValueError, AttributeError) as e:
            logger.warning(f"Failed to parse resolution '{resolution}': {e}")
        
        return (None, None)
    
    def get_output_ladder(self, scenario: Dict) -> Optional[str]:
        """
        Get a normalized output ladder identifier for grouping scenarios.
        
        Scenarios with identical output ladders should be compared against each other.
        The ladder is a string representation of the list of (resolution, fps) tuples.
        
        Args:
            scenario: Scenario dict
            
        Returns:
            Ladder identifier string (e.g., "1920x1080@30,1280x720@30,854x480@30")
            or None if no valid output ladder is found
        """
        outputs = scenario.get('outputs')
        
        if outputs and isinstance(outputs, list) and len(outputs) > 0:
            # Sort outputs by resolution (descending) for consistent ordering
            sorted_outputs = []
            for output in outputs:
                resolution = output.get('resolution')
                fps = output.get('fps')
                
                if not resolution or not fps:
                    continue
                
                width, height = self._parse_resolution(resolution)
                if width is None or height is None:
                    continue
                
                sorted_outputs.append((width, height, fps, resolution))
            
            # Sort by width (descending), then height (descending)
            sorted_outputs.sort(key=lambda x: (x[0], x[1]), reverse=True)
            
            # Build ladder string
            ladder_parts = [f"{res}@{fps}" for _, _, fps, res in sorted_outputs]
            return ','.join(ladder_parts) if ladder_parts else None
        
        # Legacy mode: single resolution/fps
        resolution = scenario.get('resolution')
        fps = scenario.get('fps')
        
        if resolution and resolution != 'N/A' and fps and fps != 'N/A':
            return f"{resolution}@{fps}"
        
        return None
    
    def _extract_stream_count(self, scenario: Dict) -> int:
        """
        Extract number of concurrent streams from scenario metadata.
        
        Heuristics:
            - Look for "N streams" or "N Streams" in scenario name
            - Default to 1 stream for single-stream scenarios
        
        Args:
            scenario: Scenario dict
            
        Returns:
            Number of streams (int, minimum 1)
        """
        name = scenario.get('name', '').lower()
        
        # Look for patterns like "2 streams", "4 Streams", etc.
        match = re.search(r'(\d+)\s+streams?', name)
        if match:
            return int(match.group(1))
        
        # Default to 1 stream
        return 1
    
    def _parse_bitrate_to_mbps(self, bitrate: str) -> float:
        """
        Parse bitrate string to Mbps (megabits per second).
        
        Supports formats:
            - "2500k" -> 2.5 Mbps
            - "5M" -> 5.0 Mbps
            - "1000" -> 1.0 Mbps (assumes kbps if no unit)
            - "0k" -> 0.0 Mbps (baseline)
        
        Args:
            bitrate: Bitrate string
            
        Returns:
            Bitrate in Mbps (float)
        """
        value = bitrate.strip().upper()
        
        if not value or value == "N/A":
            return 0.0
        
        try:
            if value.endswith('M'):
                return float(value[:-1])
            elif value.endswith('K'):
                return float(value[:-1]) / 1000.0
            else:
                # Try to parse as numeric value (assumes kbps if no unit)
                # This handles both integers and decimals
                numeric_value = float(value)
                return numeric_value / 1000.0
        except ValueError:
            logger.warning(f"Failed to parse bitrate: {bitrate}, returning 0")
            return 0.0
    
    # ============================================================================
    # Placeholder hooks for future enhancements
    # ============================================================================
    
    def compute_quality_adjusted_score(
        self, scenario: Dict, vmaf_score: float, psnr_score: float
    ) -> Optional[float]:
        """
        PLACEHOLDER: Multi-objective scoring with video quality.
        
        Future formula:
            score = (quality_weight * vmaf_normalized + 
                    efficiency_weight * throughput_per_watt_normalized)
        
        This will enable answering:
        "Which configuration delivers the best quality per watt?"
        
        Args:
            scenario: Scenario dict
            vmaf_score: VMAF quality score (0-100)
            psnr_score: PSNR quality score (dB)
            
        Returns:
            Quality-adjusted efficiency score (not implemented yet)
        """
        raise NotImplementedError(
            "Quality-adjusted scoring (VMAF/PSNR) will be implemented in v0.2. "
            "Current version uses throughput-only scoring."
        )
    
    def compute_cost_adjusted_score(
        self, scenario: Dict, cost_per_kwh: float
    ) -> Optional[float]:
        """
        PLACEHOLDER: Cost-aware scoring for cloud/edge deployments.
        
        Future formula:
            score = throughput / (energy_cost + compute_cost)
        
        This will enable:
        - Cloud instance type selection
        - Edge vs data center placement decisions
        - Total cost of ownership (TCO) optimization
        
        Args:
            scenario: Scenario dict
            cost_per_kwh: Energy cost ($/kWh)
            
        Returns:
            Cost-adjusted score (not implemented yet)
        """
        raise NotImplementedError(
            "Cost-aware scoring will be implemented in future versions. "
            "This requires cloud pricing models and TCO calculation."
        )
    
    def compute_hardware_normalized_score(
        self, scenario: Dict, hardware_profile: Dict
    ) -> Optional[float]:
        """
        PLACEHOLDER: Hardware-aware normalization for cross-platform comparison.
        
        Future capability:
        - Normalize scores across different CPU/GPU models
        - Enable fair comparison: "Which setup is most efficient for this workload?"
        - Support heterogeneous hardware fleets
        
        Args:
            scenario: Scenario dict
            hardware_profile: Hardware metadata (CPU model, GPU model, TDP, etc.)
            
        Returns:
            Hardware-normalized score (not implemented yet)
        """
        raise NotImplementedError(
            "Hardware-normalized scoring will be implemented in future versions. "
            "This requires hardware capability profiling and normalization factors."
        )
