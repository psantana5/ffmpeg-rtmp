"""
Energy-Aware Transcoding Advisor

This module provides scoring and recommendation logic for FFmpeg transcoding configurations
based on energy efficiency, resource utilization, and throughput.

The advisor transforms raw measurement data into actionable recommendations,
helping operators select optimal transcoding pipelines for their hardware.
"""

from .scoring import EnergyEfficiencyScorer
from .recommender import TranscodingRecommender

__all__ = ['EnergyEfficiencyScorer', 'TranscodingRecommender']

__version__ = '0.1.0'
