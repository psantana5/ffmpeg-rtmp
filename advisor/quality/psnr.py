"""
PSNR (Peak Signal-to-Noise Ratio) Computation

Computes PSNR quality metric by comparing reference and distorted video files.
PSNR is measured in decibels (dB), where higher values indicate better quality.

PSNR is a traditional objective quality metric based on pixel-level differences.
While not as perceptually accurate as VMAF, it's computationally efficient and
widely supported.
"""

import logging
import re
import subprocess
from pathlib import Path
from typing import Optional

logger = logging.getLogger(__name__)


def compute_psnr(input_path: str, output_path: str) -> Optional[float]:
    """
    Compute PSNR (Peak Signal-to-Noise Ratio) between reference and distorted video.
    
    PSNR is a traditional objective video quality metric that measures the ratio
    between maximum possible signal power and corrupting noise power. It's based
    on mean squared error (MSE) between pixel values.
    
    Formula:
        PSNR = 10 * log10(MAX^2 / MSE)
        
        Where:
        - MAX = maximum possible pixel value (255 for 8-bit video)
        - MSE = mean squared error between reference and distorted frames
    
    Score Interpretation (for 8-bit video):
        - < 20 dB: Very poor quality
        - 20-25 dB: Poor quality
        - 25-30 dB: Fair quality
        - 30-35 dB: Good quality
        - 35-40 dB: Very good quality
        - > 40 dB: Excellent quality
        
    Note: PSNR above 40-50 dB often indicates near-lossless or lossless quality.
    
    The function uses FFmpeg's psnr filter to compute scores. This is more
    efficient than VMAF and always available in standard FFmpeg builds.
    
    Args:
        input_path: Path to reference (original) video file
        output_path: Path to distorted (transcoded) video file
        
    Returns:
        PSNR score in dB as float, or None if computation fails
        
    Design Notes:
        - Available in all standard FFmpeg builds (no special compilation needed)
        - Computationally efficient compared to perceptual metrics
        - Less correlated with human perception than VMAF
        - Good for quick quality checks and regression testing
        
    Example:
        >>> psnr = compute_psnr('reference.mp4', 'transcoded.mp4')
        >>> if psnr is not None:
        ...     print(f"PSNR: {psnr:.2f} dB")
        ...     if psnr > 35:
        ...         print("Good quality!")
    """
    # Validate input files exist
    if not Path(input_path).exists():
        logger.error(f"Reference video not found: {input_path}")
        return None
    
    if not Path(output_path).exists():
        logger.error(f"Distorted video not found: {output_path}")
        return None
    
    try:
        # Build FFmpeg command for PSNR computation
        # Format: ffmpeg -i distorted.mp4 -i reference.mp4 -lavfi psnr -f null -
        cmd = [
            'ffmpeg',
            '-i', output_path,  # Distorted video (main input)
            '-i', input_path,   # Reference video (comparison input)
            '-lavfi', '[0:v][1:v]psnr=stats_file=/dev/stdout',
            '-f', 'null',
            '-'
        ]
        
        logger.debug(f"Running PSNR computation: {' '.join(cmd)}")
        
        # Execute FFmpeg with timeout
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=300,  # 5 minute timeout
            check=False
        )
        
        # Check if FFmpeg execution failed
        if result.returncode != 0:
            logger.error(f"PSNR computation failed: {result.stderr}")
            return None
        
        # Parse PSNR score from output
        # PSNR filter outputs to stderr in format:
        # n:1 mse_avg:123.45 mse_y:120.00 mse_u:125.00 mse_v:125.00
        # psnr_avg:28.21 psnr_y:28.34 psnr_u:28.16 psnr_v:28.16
        # We want the final average PSNR value
        
        output = result.stderr  # PSNR stats go to stderr
        psnr_values = []
        
        for line in output.split('\n'):
            if 'psnr_avg:' in line.lower():
                # Extract PSNR average value
                match = re.search(r'psnr_avg:(\d+\.?\d*)', line.lower())
                if match:
                    psnr_value = float(match.group(1))
                    psnr_values.append(psnr_value)
        
        if not psnr_values:
            logger.warning("Could not parse PSNR score from FFmpeg output")
            return None
        
        # Return the mean PSNR across all frames
        avg_psnr = sum(psnr_values) / len(psnr_values)
        logger.info(f"PSNR score: {avg_psnr:.2f} dB")
        return float(avg_psnr)
        
    except subprocess.TimeoutExpired:
        logger.error("PSNR computation timed out after 5 minutes")
        return None
    
    except FileNotFoundError:
        logger.error("FFmpeg not found. Please install FFmpeg to enable PSNR computation.")
        return None
    
    except Exception as e:
        logger.error(f"Unexpected error computing PSNR: {e}")
        return None


def is_psnr_available() -> bool:
    """
    Check if PSNR computation is available.
    
    Tests whether FFmpeg is installed. PSNR filter is available in all
    standard FFmpeg builds, so this mainly checks for FFmpeg installation.
    
    Returns:
        True if PSNR is available, False otherwise
        
    Example:
        >>> if is_psnr_available():
        ...     psnr = compute_psnr('ref.mp4', 'out.mp4')
        ... else:
        ...     print("FFmpeg not available")
    """
    try:
        # Check if FFmpeg is available
        result = subprocess.run(
            ['ffmpeg', '-version'],
            capture_output=True,
            timeout=5,
            check=False
        )
        
        return result.returncode == 0
        
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return False
    except Exception:
        return False
