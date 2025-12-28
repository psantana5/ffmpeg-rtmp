"""
Quality Assessment Module

Provides video quality metrics for QoE-aware transcoding optimization:
- VMAF (Video Multimethod Assessment Fusion)
- PSNR (Peak Signal-to-Noise Ratio)

These metrics enable quality-per-watt analysis and QoE efficiency scoring.
"""

from .psnr import compute_psnr
from .vmaf_integration import compute_vmaf

__all__ = ['compute_vmaf', 'compute_psnr']
