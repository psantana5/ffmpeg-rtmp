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

from typing import Dict, Optional
import logging

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
        """
        self.algorithm = algorithm
        self.supported_algorithms = {'throughput_per_watt'}
        
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
        
        Returns:
            Energy efficiency score (float) or None if insufficient data.
            
        Formula (v0.1):
            efficiency_score = throughput_mbps / mean_watts
            
            Where:
                throughput_mbps = parsed_bitrate_mbps * num_streams
                mean_watts = CPU power + GPU power (if available)
        
        Design notes:
            - Returns None for baseline/idle scenarios (bitrate = "0k")
            - Returns None if power data is missing or invalid
            - GPU power is automatically included if present in scenario data
            - Future versions will incorporate video quality metrics
        """
        if self.algorithm == 'throughput_per_watt':
            return self._compute_throughput_per_watt(scenario)
        
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
        import re
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
            elif value.isdigit():
                # Assume kbps if no unit
                return float(value) / 1000.0
            else:
                logger.warning(f"Unknown bitrate format: {bitrate}, returning 0")
                return 0.0
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
