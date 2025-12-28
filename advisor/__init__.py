"""
Energy-Aware Transcoding Advisor

This module provides scoring and recommendation logic for FFmpeg transcoding configurations
based on energy efficiency, resource utilization, and throughput.

The advisor transforms raw measurement data into actionable recommendations,
helping operators select optimal transcoding pipelines for their hardware.
"""

<<<<<<< HEAD
from .modeling import PowerPredictor
from .recommender import TranscodingRecommender
from .scoring import EnergyEfficiencyScorer

__all__ = ['EnergyEfficiencyScorer', 'TranscodingRecommender', 'PowerPredictor']
=======
from .modeling import MultivariatePredictor, PowerPredictor
from .recommender import TranscodingRecommender
from .scoring import EnergyEfficiencyScorer

__all__ = [
    'EnergyEfficiencyScorer',
    'TranscodingRecommender',
    'PowerPredictor',
    'MultivariatePredictor',
]
>>>>>>> feature/ml-regression

__version__ = '0.2.0'
