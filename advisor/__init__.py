"""
Energy-Aware Transcoding Advisor

Provides intelligent recommendations for transcoding configurations based on:
- Power consumption and energy efficiency
- Video quality metrics (VMAF, PSNR)
- Load-aware cost analysis and TCO
- Machine learning-based predictions
- Hardware-aware optimization (CPU & GPU)

Main Components:
- PowerPredictor: Single-variable power prediction
- MultivariatePredictor: Advanced multi-feature prediction with confidence intervals
- EnergyEfficiencyScorer: Scoring algorithms for comparing configurations
- TranscodingRecommender: High-level recommendation engine
- CostModel: Load-aware cost analysis and TCO calculations

Example:
    >>> from advisor import PowerPredictor, TranscodingRecommender, CostModel
    >>> predictor = PowerPredictor()
    >>> predictor.fit(scenarios)
    >>> power = predictor.predict(streams=8)
    >>>
    >>> recommender = TranscodingRecommender()
    >>> recommendations = recommender.recommend(scenarios)
    >>>
    >>> cost_model = CostModel(
    ...     price_per_core_second=0.000138889,
    ...     price_per_joule=3.33e-8
    ... )
    >>> total_cost = cost_model.compute_total_cost_load_aware(scenario)
"""

from .cost import CostModel
from .modeling import MultivariatePredictor, PowerPredictor
from .recommender import TranscodingRecommender
from .scoring import EnergyEfficiencyScorer

__all__ = [
    'EnergyEfficiencyScorer',
    'TranscodingRecommender',
    'PowerPredictor',
    'MultivariatePredictor',
    'CostModel',
]

__version__ = '0.3.0'
