"""
VMAF (Video Multimethod Assessment Fusion) Integration

Computes VMAF quality scores by comparing reference and distorted video files.
VMAF scores range from 0-100, where higher values indicate better quality.

VMAF is designed to correlate with human perception of video quality and is
widely used in the streaming industry for quality assessment.
"""

import json
import logging
import re
import subprocess
from pathlib import Path
from typing import Optional

logger = logging.getLogger(__name__)


def compute_vmaf(input_path: str, output_path: str) -> Optional[float]:
    """
    Compute VMAF quality score between reference and distorted video.

    VMAF (Video Multimethod Assessment Fusion) is a perceptual video quality
    assessment algorithm developed by Netflix. It combines multiple elementary
    quality metrics to predict subjective video quality.

    Score Interpretation:
        - 0-20: Poor quality
        - 20-40: Fair quality
        - 40-60: Good quality
        - 60-80: Very good quality
        - 80-100: Excellent quality

    The function uses FFmpeg's libvmaf filter to compute scores. If VMAF
    computation is not available (missing libvmaf), it returns None gracefully.

    Args:
        input_path: Path to reference (original) video file
        output_path: Path to distorted (transcoded) video file

    Returns:
        VMAF score (0-100) as float, or None if computation fails

    Design Notes:
        - Requires FFmpeg compiled with libvmaf support
        - Falls back gracefully if libvmaf is not available
        - Handles missing files and invalid paths
        - Logs detailed error messages for troubleshooting

    Example:
        >>> score = compute_vmaf('reference.mp4', 'transcoded.mp4')
        >>> if score is not None:
        ...     print(f"VMAF Score: {score:.2f}")
        ...     if score > 80:
        ...         print("Excellent quality!")
    """
    # Validate input files exist
    if not Path(input_path).exists():
        logger.error(f"Reference video not found: {input_path}")
        return None

    if not Path(output_path).exists():
        logger.error(f"Distorted video not found: {output_path}")
        return None

    try:
        # Build FFmpeg command for VMAF computation
        # Format: ffmpeg -i distorted.mp4 -i reference.mp4 -lavfi libvmaf -f null -
        cmd = [
            'ffmpeg',
            '-i',
            output_path,  # Distorted video (main input)
            '-i',
            input_path,  # Reference video (comparison input)
            '-lavfi',
            '[0:v][1:v]libvmaf=log_fmt=json:log_path=/dev/stdout',
            '-f',
            'null',
            '-',
        ]

        logger.debug(f"Running VMAF computation: {' '.join(cmd)}")

        # Execute FFmpeg with timeout to prevent hanging
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=300,  # 5 minute timeout
            check=False,
        )

        # Check if FFmpeg execution failed
        if result.returncode != 0:
            stderr = result.stderr.lower()

            # Check for common failure modes
            if 'libvmaf' in stderr and ('unknown filter' in stderr or 'not found' in stderr):
                logger.warning(
                    "VMAF computation not available: FFmpeg not compiled with libvmaf. "
                    "Install FFmpeg with libvmaf support for quality metrics."
                )
                return None

            logger.error(f"VMAF computation failed: {result.stderr}")
            return None

        # Parse VMAF score from output
        # Look for VMAF score in JSON format or text output
        output = result.stdout

        # Try parsing JSON output first (preferred format)
        try:
            vmaf_data = json.loads(output)
            if 'pooled_metrics' in vmaf_data:
                vmaf_score = vmaf_data['pooled_metrics']['vmaf']['mean']
                logger.info(f"VMAF score: {vmaf_score:.2f}")
                return float(vmaf_score)
        except (json.JSONDecodeError, KeyError, ValueError):
            pass

        # Fallback: Parse text output
        # Look for lines like "VMAF score: 85.234"
        for line in output.split('\n'):
            if 'vmaf' in line.lower() and 'score' in line.lower():
                # Extract numeric value
                match = re.search(r'(\d+\.?\d*)', line)
                if match:
                    vmaf_score = float(match.group(1))
                    logger.info(f"VMAF score: {vmaf_score:.2f}")
                    return vmaf_score

        logger.warning("Could not parse VMAF score from FFmpeg output")
        return None

    except subprocess.TimeoutExpired:
        logger.error("VMAF computation timed out after 5 minutes")
        return None

    except FileNotFoundError:
        logger.error("FFmpeg not found. Please install FFmpeg to enable VMAF computation.")
        return None

    except Exception as e:
        logger.error(f"Unexpected error computing VMAF: {e}")
        return None


def is_vmaf_available() -> bool:
    """
    Check if VMAF computation is available.

    Tests whether FFmpeg is installed and compiled with libvmaf support.

    Returns:
        True if VMAF is available, False otherwise

    Example:
        >>> if is_vmaf_available():
        ...     score = compute_vmaf('ref.mp4', 'out.mp4')
        ... else:
        ...     print("VMAF not available, skipping quality metrics")
    """
    try:
        # Check if FFmpeg is available
        result = subprocess.run(['ffmpeg', '-version'], capture_output=True, timeout=5, check=False)

        if result.returncode != 0:
            return False

        # Check if libvmaf filter is available
        result = subprocess.run(
            ['ffmpeg', '-filters'], capture_output=True, text=True, timeout=5, check=False
        )

        return 'libvmaf' in result.stdout.lower()

    except (FileNotFoundError, subprocess.TimeoutExpired):
        return False
    except Exception:
        return False
