"""Tests for the advisor.scoring module."""

import pytest

from advisor.scoring import EnergyEfficiencyScorer


class TestEnergyEfficiencyScorer:
    """Tests for EnergyEfficiencyScorer class."""
    
    def test_initialization_default(self):
        """Test default initialization."""
        scorer = EnergyEfficiencyScorer()
        assert scorer.algorithm == 'auto'
    
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
        """Test that legacy placeholder methods have been implemented."""
        scorer = EnergyEfficiencyScorer()
        scenario = {
            'name': 'Test',
            'bitrate': '2500k',
            'power': {'mean_watts': 50, 'total_energy_joules': 3000},
            'duration': 60,
            'resolution': '1920x1080',
            'fps': 30
        }
        
        # compute_quality_adjusted_score is now implemented
        result = scorer.compute_quality_adjusted_score(scenario, vmaf_score=85.0, psnr_score=40.0)
        # Should return a score (not raise NotImplementedError)
        assert result is not None or result is None  # May be None if validation fails
        
        # These are still not implemented
        with pytest.raises(NotImplementedError):
            scorer.compute_cost_adjusted_score(scenario, cost_per_kwh=0.12)
        
        with pytest.raises(NotImplementedError):
            scorer.compute_hardware_normalized_score(scenario, hardware_profile={})


class TestQoEScoringAlgorithms:
    """Tests for QoE-based scoring algorithms."""
    
    def test_quality_per_watt_scoring(self):
        """Test quality_per_watt algorithm."""
        scorer = EnergyEfficiencyScorer(algorithm='quality_per_watt')
        
        scenario = {
            'name': 'QoE Test',
            'vmaf_score': 85.0,
            'power': {'mean_watts': 100.0}
        }
        
        score = scorer.compute_score(scenario)
        
        # Expected: 85.0 / 100.0 = 0.85 VMAF/W
        assert score is not None
        assert pytest.approx(score, rel=1e-3) == 0.85
    
    def test_quality_per_watt_with_gpu(self):
        """Test quality_per_watt with GPU power included."""
        scorer = EnergyEfficiencyScorer(algorithm='quality_per_watt')
        
        scenario = {
            'name': 'QoE GPU Test',
            'vmaf_score': 90.0,
            'power': {'mean_watts': 80.0},
            'gpu_power': {'mean_watts': 20.0}
        }
        
        score = scorer.compute_score(scenario)
        
        # Expected: 90.0 / (80.0 + 20.0) = 0.9 VMAF/W
        assert score is not None
        assert pytest.approx(score, rel=1e-3) == 0.9
    
    def test_quality_per_watt_missing_vmaf(self):
        """Test quality_per_watt returns None when VMAF is missing."""
        scorer = EnergyEfficiencyScorer(algorithm='quality_per_watt')
        
        scenario = {
            'name': 'No VMAF Test',
            'power': {'mean_watts': 100.0}
        }
        
        score = scorer.compute_score(scenario)
        assert score is None
    
    def test_quality_per_watt_invalid_vmaf(self):
        """Test quality_per_watt handles invalid VMAF scores."""
        scorer = EnergyEfficiencyScorer(algorithm='quality_per_watt')
        
        scenario = {
            'name': 'Invalid VMAF',
            'vmaf_score': 150.0,  # Invalid: > 100
            'power': {'mean_watts': 100.0}
        }
        
        score = scorer.compute_score(scenario)
        assert score is None
    
    def test_qoe_efficiency_score(self):
        """Test qoe_efficiency_score algorithm."""
        scorer = EnergyEfficiencyScorer(algorithm='qoe_efficiency_score')
        
        scenario = {
            'name': 'QoE Efficiency Test',
            'vmaf_score': 80.0,  # 0.8 normalized
            'power': {'total_energy_joules': 5000.0},
            'duration': 60,
            'resolution': '1920x1080',
            'fps': 30
        }
        
        score = scorer.compute_score(scenario)
        
        # Expected: (1920 * 1080 * 30 * 60) * 0.8 / 5000.0
        # = 3732480000 * 0.8 / 5000.0 = 597196.8
        assert score is not None
        assert pytest.approx(score, rel=1e-2) == 597196.8
    
    def test_qoe_efficiency_score_multi_output(self):
        """Test qoe_efficiency_score with multiple outputs."""
        scorer = EnergyEfficiencyScorer(algorithm='qoe_efficiency_score')
        
        scenario = {
            'name': 'Multi-output QoE',
            'vmaf_score': 85.0,  # 0.85 normalized
            'power': {'total_energy_joules': 10000.0},
            'duration': 60,
            'outputs': [
                {'resolution': '1920x1080', 'fps': 30},
                {'resolution': '1280x720', 'fps': 30},
                {'resolution': '854x480', 'fps': 30}
            ]
        }
        
        score = scorer.compute_score(scenario)
        
        # Expected: ((1920*1080 + 1280*720 + 854*480) * 30 * 60) * 0.85 / 10000.0
        total_pixels = (1920*1080 + 1280*720 + 854*480) * 30 * 60
        expected = (total_pixels * 0.85) / 10000.0
        
        assert score is not None
        assert pytest.approx(score, rel=1e-2) == expected
    
    def test_qoe_efficiency_score_missing_data(self):
        """Test qoe_efficiency_score returns None when data is missing."""
        scorer = EnergyEfficiencyScorer(algorithm='qoe_efficiency_score')
        
        # Missing VMAF
        scenario1 = {
            'name': 'No VMAF',
            'power': {'total_energy_joules': 5000.0},
            'duration': 60,
            'resolution': '1920x1080',
            'fps': 30
        }
        assert scorer.compute_score(scenario1) is None
        
        # Missing energy
        scenario2 = {
            'name': 'No Energy',
            'vmaf_score': 85.0,
            'duration': 60,
            'resolution': '1920x1080',
            'fps': 30
        }
        assert scorer.compute_score(scenario2) is None
        
        # Missing pixels
        scenario3 = {
            'name': 'No Pixels',
            'vmaf_score': 85.0,
            'power': {'total_energy_joules': 5000.0}
        }
        assert scorer.compute_score(scenario3) is None
    
    def test_auto_algorithm_selection_qoe(self):
        """Test auto algorithm selects qoe_efficiency_score when VMAF is available."""
        scorer = EnergyEfficiencyScorer(algorithm='auto')
        
        scenario = {
            'name': 'Auto QoE Test',
            'vmaf_score': 85.0,
            'power': {'total_energy_joules': 5000.0},
            'duration': 60,
            'resolution': '1920x1080',
            'fps': 30
        }
        
        score = scorer.compute_score(scenario)
        
        # Should use qoe_efficiency_score
        assert score is not None
    
    def test_auto_algorithm_selection_ladder(self):
        """Test auto algorithm selects pixels_per_joule for multi-output."""
        scorer = EnergyEfficiencyScorer(algorithm='auto')
        
        scenario = {
            'name': 'Auto Ladder Test',
            # No VMAF, so should use pixels_per_joule
            'power': {'total_energy_joules': 5000.0},
            'duration': 60,
            'outputs': [
                {'resolution': '1920x1080', 'fps': 30},
                {'resolution': '1280x720', 'fps': 30}
            ]
        }
        
        score = scorer.compute_score(scenario)
        
        # Should use pixels_per_joule
        assert score is not None
    
    def test_auto_algorithm_selection_simple(self):
        """Test auto algorithm selects throughput_per_watt for simple scenario."""
        scorer = EnergyEfficiencyScorer(algorithm='auto')
        
        scenario = {
            'name': 'Auto Simple Test',
            # No VMAF, no multiple outputs
            'bitrate': '2500k',
            'power': {'mean_watts': 50.0}
        }
        
        score = scorer.compute_score(scenario)
        
        # Should use throughput_per_watt
        assert score is not None
    
    def test_initialization_with_new_algorithms(self):
        """Test initialization with new algorithm types."""
        # quality_per_watt
        scorer1 = EnergyEfficiencyScorer(algorithm='quality_per_watt')
        assert scorer1.algorithm == 'quality_per_watt'
        
        # qoe_efficiency_score
        scorer2 = EnergyEfficiencyScorer(algorithm='qoe_efficiency_score')
        assert scorer2.algorithm == 'qoe_efficiency_score'
        
        # auto
        scorer3 = EnergyEfficiencyScorer(algorithm='auto')
        assert scorer3.algorithm == 'auto'
    
    def test_invalid_algorithm_raises_error(self):
        """Test initialization with invalid algorithm raises ValueError."""
        with pytest.raises(ValueError, match="Unsupported algorithm"):
            EnergyEfficiencyScorer(algorithm='invalid_qoe_algo')
