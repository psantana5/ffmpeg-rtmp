"""Tests for the advisor.quality module."""

import subprocess
from unittest.mock import Mock, patch

import pytest

from advisor.quality import compute_psnr, compute_vmaf
from advisor.quality.psnr import is_psnr_available
from advisor.quality.vmaf_integration import is_vmaf_available


class TestVMAFIntegration:
    """Tests for VMAF computation."""

    @patch('advisor.quality.vmaf_integration.subprocess.run')
    @patch('advisor.quality.vmaf_integration.Path.exists')
    def test_compute_vmaf_success_json(self, mock_exists, mock_run):
        """Test successful VMAF computation with JSON output."""
        mock_exists.return_value = True

        # Mock successful FFmpeg execution with JSON output
        mock_result = Mock()
        mock_result.returncode = 0
        mock_result.stdout = '{"pooled_metrics": {"vmaf": {"mean": 85.234}}}'
        mock_result.stderr = ''
        mock_run.return_value = mock_result

        score = compute_vmaf('ref.mp4', 'out.mp4')

        assert score is not None
        assert pytest.approx(score, rel=1e-3) == 85.234
        mock_run.assert_called_once()

    @patch('advisor.quality.vmaf_integration.subprocess.run')
    @patch('advisor.quality.vmaf_integration.Path.exists')
    def test_compute_vmaf_success_text(self, mock_exists, mock_run):
        """Test successful VMAF computation with text output."""
        mock_exists.return_value = True

        # Mock successful FFmpeg execution with text output
        mock_result = Mock()
        mock_result.returncode = 0
        mock_result.stdout = 'VMAF score: 92.567'
        mock_result.stderr = ''
        mock_run.return_value = mock_result

        score = compute_vmaf('ref.mp4', 'out.mp4')

        assert score is not None
        assert pytest.approx(score, rel=1e-3) == 92.567

    @patch('advisor.quality.vmaf_integration.Path.exists')
    def test_compute_vmaf_missing_reference(self, mock_exists):
        """Test VMAF computation with missing reference file."""
        mock_exists.side_effect = [False, True]  # ref missing, out exists

        score = compute_vmaf('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.vmaf_integration.Path.exists')
    def test_compute_vmaf_missing_output(self, mock_exists):
        """Test VMAF computation with missing output file."""
        mock_exists.side_effect = [True, False]  # ref exists, out missing

        score = compute_vmaf('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.vmaf_integration.subprocess.run')
    @patch('advisor.quality.vmaf_integration.Path.exists')
    def test_compute_vmaf_libvmaf_not_available(self, mock_exists, mock_run):
        """Test VMAF computation when libvmaf is not available."""
        mock_exists.return_value = True

        # Mock FFmpeg failure due to missing libvmaf
        mock_result = Mock()
        mock_result.returncode = 1
        mock_result.stderr = 'Error: unknown filter: libvmaf'
        mock_run.return_value = mock_result

        score = compute_vmaf('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.vmaf_integration.subprocess.run')
    @patch('advisor.quality.vmaf_integration.Path.exists')
    def test_compute_vmaf_timeout(self, mock_exists, mock_run):
        """Test VMAF computation timeout."""
        mock_exists.return_value = True
        mock_run.side_effect = subprocess.TimeoutExpired('ffmpeg', 300)

        score = compute_vmaf('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.vmaf_integration.subprocess.run')
    @patch('advisor.quality.vmaf_integration.Path.exists')
    def test_compute_vmaf_ffmpeg_not_found(self, mock_exists, mock_run):
        """Test VMAF computation when FFmpeg is not installed."""
        mock_exists.return_value = True
        mock_run.side_effect = FileNotFoundError()

        score = compute_vmaf('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.vmaf_integration.subprocess.run')
    def test_is_vmaf_available_true(self, mock_run):
        """Test is_vmaf_available when VMAF is available."""
        # Mock FFmpeg version check
        mock_version_result = Mock()
        mock_version_result.returncode = 0

        # Mock filters check with libvmaf present
        mock_filters_result = Mock()
        mock_filters_result.returncode = 0
        mock_filters_result.stdout = 'Filter: libvmaf\nDescription: Calculate VMAF'

        mock_run.side_effect = [mock_version_result, mock_filters_result]

        assert is_vmaf_available() is True

    @patch('advisor.quality.vmaf_integration.subprocess.run')
    def test_is_vmaf_available_false(self, mock_run):
        """Test is_vmaf_available when VMAF is not available."""
        # Mock FFmpeg version check
        mock_version_result = Mock()
        mock_version_result.returncode = 0

        # Mock filters check without libvmaf
        mock_filters_result = Mock()
        mock_filters_result.returncode = 0
        mock_filters_result.stdout = 'Filter: scale\nDescription: Scale video'

        mock_run.side_effect = [mock_version_result, mock_filters_result]

        assert is_vmaf_available() is False


class TestPSNRComputation:
    """Tests for PSNR computation."""

    @patch('advisor.quality.psnr.subprocess.run')
    @patch('advisor.quality.psnr.Path.exists')
    def test_compute_psnr_success(self, mock_exists, mock_run):
        """Test successful PSNR computation."""
        mock_exists.return_value = True

        # Mock successful FFmpeg execution
        mock_result = Mock()
        mock_result.returncode = 0
        mock_result.stderr = (
            'n:1 mse_avg:50.00 psnr_avg:31.23\n'
            'n:2 mse_avg:48.00 psnr_avg:31.45\n'
            'n:3 mse_avg:52.00 psnr_avg:31.01\n'
        )
        mock_run.return_value = mock_result

        score = compute_psnr('ref.mp4', 'out.mp4')

        assert score is not None
        # Average of 31.23, 31.45, 31.01 = 31.23
        assert pytest.approx(score, rel=1e-2) == 31.23
        mock_run.assert_called_once()

    @patch('advisor.quality.psnr.subprocess.run')
    @patch('advisor.quality.psnr.Path.exists')
    def test_compute_psnr_single_value(self, mock_exists, mock_run):
        """Test PSNR computation with single value."""
        mock_exists.return_value = True

        # Mock successful FFmpeg execution with single frame
        mock_result = Mock()
        mock_result.returncode = 0
        mock_result.stderr = 'n:1 mse_avg:100.00 psnr_avg:28.13'
        mock_run.return_value = mock_result

        score = compute_psnr('ref.mp4', 'out.mp4')

        assert score is not None
        assert pytest.approx(score, rel=1e-3) == 28.13

    @patch('advisor.quality.psnr.Path.exists')
    def test_compute_psnr_missing_reference(self, mock_exists):
        """Test PSNR computation with missing reference file."""
        mock_exists.side_effect = [False, True]  # ref missing, out exists

        score = compute_psnr('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.psnr.Path.exists')
    def test_compute_psnr_missing_output(self, mock_exists):
        """Test PSNR computation with missing output file."""
        mock_exists.side_effect = [True, False]  # ref exists, out missing

        score = compute_psnr('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.psnr.subprocess.run')
    @patch('advisor.quality.psnr.Path.exists')
    def test_compute_psnr_ffmpeg_error(self, mock_exists, mock_run):
        """Test PSNR computation with FFmpeg error."""
        mock_exists.return_value = True

        # Mock FFmpeg execution failure
        mock_result = Mock()
        mock_result.returncode = 1
        mock_result.stderr = 'Error: Invalid input'
        mock_run.return_value = mock_result

        score = compute_psnr('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.psnr.subprocess.run')
    @patch('advisor.quality.psnr.Path.exists')
    def test_compute_psnr_no_psnr_data(self, mock_exists, mock_run):
        """Test PSNR computation with no PSNR data in output."""
        mock_exists.return_value = True

        # Mock FFmpeg execution with output lacking PSNR data
        mock_result = Mock()
        mock_result.returncode = 0
        mock_result.stderr = 'Some other output without PSNR'
        mock_run.return_value = mock_result

        score = compute_psnr('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.psnr.subprocess.run')
    @patch('advisor.quality.psnr.Path.exists')
    def test_compute_psnr_timeout(self, mock_exists, mock_run):
        """Test PSNR computation timeout."""
        mock_exists.return_value = True
        mock_run.side_effect = subprocess.TimeoutExpired('ffmpeg', 300)

        score = compute_psnr('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.psnr.subprocess.run')
    @patch('advisor.quality.psnr.Path.exists')
    def test_compute_psnr_ffmpeg_not_found(self, mock_exists, mock_run):
        """Test PSNR computation when FFmpeg is not installed."""
        mock_exists.return_value = True
        mock_run.side_effect = FileNotFoundError()

        score = compute_psnr('ref.mp4', 'out.mp4')

        assert score is None

    @patch('advisor.quality.psnr.subprocess.run')
    def test_is_psnr_available_true(self, mock_run):
        """Test is_psnr_available when FFmpeg is installed."""
        mock_result = Mock()
        mock_result.returncode = 0
        mock_run.return_value = mock_result

        assert is_psnr_available() is True

    @patch('advisor.quality.psnr.subprocess.run')
    def test_is_psnr_available_false(self, mock_run):
        """Test is_psnr_available when FFmpeg is not installed."""
        mock_run.side_effect = FileNotFoundError()

        assert is_psnr_available() is False


class TestQualityMetricsIntegration:
    """Integration tests for quality metrics."""

    def test_quality_module_imports(self):
        """Test that quality module can be imported and functions are available."""
        from advisor.quality import compute_psnr, compute_vmaf

        # Verify functions are callable
        assert callable(compute_vmaf)
        assert callable(compute_psnr)
