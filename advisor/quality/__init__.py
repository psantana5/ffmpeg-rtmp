"""
Quality Assessment Module

Provides video quality metrics for QoE-aware transcoding optimization:
- VMAF (Video Multimethod Assessment Fusion)
- PSNR (Peak Signal-to-Noise Ratio)

These metrics enable quality-per-watt analysis and QoE efficiency scoring.
"""

from .vmaf_integration import compute_vmaf
from .psnr import compute_psnr

__all__ = ['compute_vmaf', 'compute_psnr']
