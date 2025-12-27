#!/usr/bin/env python3
"""
Tests for results_exporter enhancements
"""

import sys
import os
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent / "results-exporter"))

from results_exporter import ResultsExporter


class TestResultsExporterEnhancements:
    """Test new functionality in ResultsExporter"""
    
    def test_extract_stream_count(self):
        """Test stream count extraction from scenario names"""
        exporter = ResultsExporter()
        
        assert exporter._extract_stream_count({"name": "2 streams @ 2500k"}) == 2
        assert exporter._extract_stream_count({"name": "4 Streams @ 5000k"}) == 4
        assert exporter._extract_stream_count({"name": "8 streams"}) == 8
        assert exporter._extract_stream_count({"name": "Single stream test"}) == 1
        assert exporter._extract_stream_count({"name": "Baseline (Idle)"}) == 1
    
    def test_detect_encoder_type(self):
        """Test encoder type detection"""
        exporter = ResultsExporter()
        
        # GPU encoders
        assert exporter._detect_encoder_type({"name": "GPU transcode test"}) == "gpu"
        assert exporter._detect_encoder_type({"name": "NVENC encoding"}) == "gpu"
        assert exporter._detect_encoder_type({"name": "QSV test"}) == "gpu"
        
        # CPU encoders
        assert exporter._detect_encoder_type({"name": "CPU transcode"}) == "cpu"
        assert exporter._detect_encoder_type({"name": "x264 encoding"}) == "cpu"
        assert exporter._detect_encoder_type({"name": "libx264 test"}) == "cpu"
        
        # Default
        assert exporter._detect_encoder_type({"name": "Generic test"}) == "cpu"
    
    def test_get_output_ladder_id_single_resolution(self):
        """Test output ladder ID for single resolution scenarios"""
        exporter = ResultsExporter()
        
        scenario = {
            "resolution": "1280x720",
            "fps": 30
        }
        
        assert exporter._get_output_ladder_id(scenario) == "1280x720@30"
    
    def test_get_output_ladder_id_multi_resolution(self):
        """Test output ladder ID for multi-resolution scenarios"""
        exporter = ResultsExporter()
        
        scenario = {
            "outputs": [
                {"resolution": "1920x1080", "fps": 30},
                {"resolution": "1280x720", "fps": 30},
                {"resolution": "854x480", "fps": 30}
            ]
        }
        
        ladder_id = exporter._get_output_ladder_id(scenario)
        # Should be sorted descending
        assert "1920x1080@30" in ladder_id
        assert "1280x720@30" in ladder_id
        assert "854x480@30" in ladder_id
        
        # Check ordering (highest res first)
        parts = ladder_id.split(",")
        assert parts[0] == "1920x1080@30"
    
    def test_parse_resolution(self):
        """Test resolution parsing"""
        exporter = ResultsExporter()
        
        assert exporter._parse_resolution("1920x1080") == (1920, 1080)
        assert exporter._parse_resolution("1280x720") == (1280, 720)
        assert exporter._parse_resolution("854x480") == (854, 480)
        assert exporter._parse_resolution("N/A") == (None, None)
        assert exporter._parse_resolution("invalid") == (None, None)
    
    def test_compute_efficiency_score_pixels_per_joule(self):
        """Test efficiency score computation using pixels per joule"""
        exporter = ResultsExporter()
        
        scenario = {
            "name": "Test scenario",
            "resolution": "1280x720",
            "fps": 30,
            "bitrate": "2500k"
        }
        
        stats = {
            "mean_power_w": 50.0,
            "total_energy_j": 5000.0,
            "duration_s": 100.0
        }
        
        score = exporter._compute_efficiency_score(scenario, stats)
        
        # Should compute pixels per joule
        # total_pixels = 1280 * 720 * 30 * 100 = 2,764,800,000
        # efficiency = 2,764,800,000 / 5000 = 552,960
        assert score is not None
        assert score > 500000  # Should be in expected range
    
    def test_compute_efficiency_score_multi_resolution(self):
        """Test efficiency score for multi-resolution outputs"""
        exporter = ResultsExporter()
        
        scenario = {
            "name": "Multi-res test",
            "outputs": [
                {"resolution": "1920x1080", "fps": 30},
                {"resolution": "1280x720", "fps": 30}
            ]
        }
        
        stats = {
            "mean_power_w": 75.0,
            "total_energy_j": 7500.0,
            "duration_s": 100.0
        }
        
        score = exporter._compute_efficiency_score(scenario, stats)
        
        # Should compute total pixels across all outputs
        assert score is not None
        assert score > 0
    
    def test_scenario_labels_include_new_fields(self):
        """Test that scenario labels include new fields"""
        exporter = ResultsExporter()
        
        scenario = {
            "name": "4 streams @ 2500k",
            "bitrate": "2500k",
            "resolution": "1280x720",
            "fps": 30
        }
        
        labels = exporter._scenario_labels(scenario)
        
        assert "streams" in labels
        assert labels["streams"] == "4"
        assert "output_ladder" in labels
        assert "encoder_type" in labels
        assert labels["encoder_type"] == "cpu"  # default
