"""Tests for the advisor.scoring module."""

import pytest

from advisor.scoring import EnergyEfficiencyScorer


class TestEnergyEfficiencyScorer:
    """Tests for EnergyEfficiencyScorer class."""
    
    def test_initialization_default(self):
        """Test default initialization."""
        scorer = EnergyEfficiencyScorer()
        assert scorer.algorithm == 'throughput_per_watt'
    
    def test_initialization_invalid_algorithm(self):
        """Test initialization with invalid algorithm raises error."""
        with pytest.raises(ValueError, match="Unsupported algorithm"):
            EnergyEfficiencyScorer(algorithm='invalid_algo')
    
    def test_compute_score_single_stream(self):
        """Test score computation for single stream scenario."""
        scorer = EnergyEfficiencyScorer()
        
        scenario = {
            'name': '1 Mbps Stream',
            'bitrate': '1000k',
            'power': {'mean_watts': 50.0}
        }
        
        score = scorer.compute_score(scenario)
        
        # 1000k = 1 Mbps, 1 stream, 50W
        # Expected: 1 / 50 = 0.02 Mbps/W
        assert score is not None
        assert pytest.approx(score, rel=1e-3) == 0.02
    
    def test_compute_score_multi_stream(self):
        """Test score computation for multi-stream scenario."""
        scorer = EnergyEfficiencyScorer()
        
        scenario = {
            'name': '4 streams @ 2500k',
            'bitrate': '2500k',
            'power': {'mean_watts': 100.0}
        }
        
        score = scorer.compute_score(scenario)
        
        # 2500k = 2.5 Mbps, 4 streams, 100W
        # Expected: (2.5 * 4) / 100 = 0.1 Mbps/W
        assert score is not None
        assert pytest.approx(score, rel=1e-3) == 0.1
    
    def test_compute_score_with_gpu_power(self):
        """Test score computation with GPU power included."""
        scorer = EnergyEfficiencyScorer()
        
        scenario = {
            'name': '5 Mbps Stream',
            'bitrate': '5M',
            'power': {'mean_watts': 80.0},
            'gpu_power': {'mean_watts': 20.0}
        }
        
        score = scorer.compute_score(scenario)
        
        # 5M = 5 Mbps, 1 stream, 80W CPU + 20W GPU = 100W total
        # Expected: 5 / 100 = 0.05 Mbps/W
        assert score is not None
        assert pytest.approx(score, rel=1e-3) == 0.05
    
    def test_compute_score_baseline_scenario(self):
        """Test that baseline scenarios return None."""
        scorer = EnergyEfficiencyScorer()
        
        scenario = {
            'name': 'Baseline (Idle)',
            'bitrate': '0k',
            'power': {'mean_watts': 30.0}
        }
        
        score = scorer.compute_score(scenario)
        assert score is None
    
    def test_compute_score_missing_power(self):
        """Test that scenarios without power data return None."""
        scorer = EnergyEfficiencyScorer()
        
        scenario = {
            'name': 'Test Stream',
            'bitrate': '2500k'
        }
        
        score = scorer.compute_score(scenario)
        assert score is None
    
    def test_compute_score_zero_power(self):
        """Test that scenarios with zero power return None."""
        scorer = EnergyEfficiencyScorer()
        
        scenario = {
            'name': 'Test Stream',
            'bitrate': '2500k',
            'power': {'mean_watts': 0.0}
        }
        
        score = scorer.compute_score(scenario)
        assert score is None
    
    def test_parse_bitrate_various_formats(self):
        """Test bitrate parsing with various formats."""
        scorer = EnergyEfficiencyScorer()
        
        # Test kbps format
        assert scorer._parse_bitrate_to_mbps('1000k') == 1.0
        assert scorer._parse_bitrate_to_mbps('2500k') == 2.5
        
        # Test Mbps format
        assert scorer._parse_bitrate_to_mbps('5M') == 5.0
        assert scorer._parse_bitrate_to_mbps('10M') == 10.0
        
        # Test numeric-only (assumes kbps)
        assert scorer._parse_bitrate_to_mbps('3000') == 3.0
        
        # Test decimal values
        assert scorer._parse_bitrate_to_mbps('1500.5') == 1.5005
        assert scorer._parse_bitrate_to_mbps('2500.0') == 2.5
        
        # Test invalid/edge cases
        assert scorer._parse_bitrate_to_mbps('0k') == 0.0
        assert scorer._parse_bitrate_to_mbps('N/A') == 0.0
        assert scorer._parse_bitrate_to_mbps('') == 0.0
    
    def test_extract_stream_count(self):
        """Test stream count extraction from scenario names."""
        scorer = EnergyEfficiencyScorer()
        
        # Test various naming patterns
        assert scorer._extract_stream_count({'name': '1 Stream @ 2500k'}) == 1
        assert scorer._extract_stream_count({'name': '4 streams @ 5M'}) == 4
        assert scorer._extract_stream_count({'name': '8 Streams @ 1000k'}) == 8
        
        # Test single stream (default)
        assert scorer._extract_stream_count({'name': '2.5 Mbps Stream'}) == 1
        assert scorer._extract_stream_count({'name': 'Baseline'}) == 1
    
    def test_placeholder_methods_not_implemented(self):
        """Test that placeholder methods raise NotImplementedError."""
        scorer = EnergyEfficiencyScorer()
        scenario = {'name': 'Test', 'bitrate': '2500k', 'power': {'mean_watts': 50}}
        
        with pytest.raises(NotImplementedError):
            scorer.compute_quality_adjusted_score(scenario, vmaf_score=95.0, psnr_score=40.0)
        
        with pytest.raises(NotImplementedError):
            scorer.compute_cost_adjusted_score(scenario, cost_per_kwh=0.12)
        
        with pytest.raises(NotImplementedError):
            scorer.compute_hardware_normalized_score(scenario, hardware_profile={})
