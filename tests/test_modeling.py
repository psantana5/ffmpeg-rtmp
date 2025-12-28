"""Tests for the advisor.modeling module."""

import pytest

from advisor.modeling import PowerPredictor


class TestPowerPredictor:
    """Tests for PowerPredictor class."""
    
    @pytest.fixture
    def sample_scenarios(self):
        """Sample scenario data for testing."""
        return [
            {
                'name': '1 Stream @ 2500k',
                'power': {'mean_watts': 45.0}
            },
            {
                'name': '2 Streams @ 2500k',
                'power': {'mean_watts': 80.0}
            },
            {
                'name': '4 Streams @ 2500k',
                'power': {'mean_watts': 150.0}
            },
            {
                'name': '8 streams @ 1080p',
                'power': {'mean_watts': 280.0}
            },
        ]
    
    @pytest.fixture
    def large_dataset(self):
        """Larger dataset for polynomial regression testing."""
        return [
            {'name': '1 Stream @ 2500k', 'power': {'mean_watts': 45.0}},
            {'name': '2 Streams @ 2500k', 'power': {'mean_watts': 80.0}},
            {'name': '3 streams test', 'power': {'mean_watts': 110.0}},
            {'name': '4 Streams @ 2500k', 'power': {'mean_watts': 150.0}},
            {'name': '6 streams @ 1080p', 'power': {'mean_watts': 210.0}},
            {'name': '8 streams @ 1080p', 'power': {'mean_watts': 280.0}},
            {'name': '12 Streams @ 720p', 'power': {'mean_watts': 380.0}},
        ]
    
    def test_initialization(self):
        """Test PowerPredictor initialization."""
        predictor = PowerPredictor()
        assert predictor.model is None
        assert predictor.poly_features is None
        assert predictor.is_polynomial is False
        assert predictor.training_data == []
    
    def test_infer_stream_count_basic(self):
        """Test stream count inference from scenario names."""
        predictor = PowerPredictor()
        
        # Standard patterns
        assert predictor._infer_stream_count("4 Streams @ 2500k") == 4
        assert predictor._infer_stream_count("8 streams @ 1080p") == 8
        assert predictor._infer_stream_count("2 Stream Test") == 2
        assert predictor._infer_stream_count("12 Streams") == 12
        
        # Hyphenated
        assert predictor._infer_stream_count("4-stream test") == 4
        
        # Single stream
        assert predictor._infer_stream_count("Single Stream @ 2500k") == 1
        
        # Starting with number
        assert predictor._infer_stream_count("6 concurrent streams") == 6
    
    def test_infer_stream_count_edge_cases(self):
        """Test edge cases in stream count inference."""
        predictor = PowerPredictor()
        
        # Cannot infer
        assert predictor._infer_stream_count("Baseline (Idle)") is None
        assert predictor._infer_stream_count("Multi Stream Test") is None
        assert predictor._infer_stream_count("High Quality Test") is None
        assert predictor._infer_stream_count("") is None
    
    def test_fit_linear_regression(self, sample_scenarios):
        """Test fitting with linear regression (< 6 unique stream counts)."""
        predictor = PowerPredictor()
        success = predictor.fit(sample_scenarios)
        
        assert success is True
        assert predictor.model is not None
        assert predictor.is_polynomial is False
        assert predictor.poly_features is None
        assert len(predictor.training_data) == 4
    
    def test_fit_polynomial_regression(self, large_dataset):
        """Test fitting with polynomial regression (>= 6 unique stream counts)."""
        predictor = PowerPredictor()
        success = predictor.fit(large_dataset)
        
        assert success is True
        assert predictor.model is not None
        assert predictor.is_polynomial is True
        assert predictor.poly_features is not None
        assert len(predictor.training_data) == 7
    
    def test_fit_no_valid_data(self):
        """Test fitting with no valid training data."""
        predictor = PowerPredictor()
        scenarios = [
            {'name': 'Baseline (Idle)', 'power': {'mean_watts': 30.0}},
            {'name': 'Test Scenario', 'power': {'mean_watts': 50.0}},
        ]
        
        success = predictor.fit(scenarios)
        assert success is False
        assert predictor.model is None
    
    def test_fit_missing_power_data(self):
        """Test fitting with scenarios missing power data."""
        predictor = PowerPredictor()
        scenarios = [
            {'name': '2 Streams @ 2500k', 'power': {'mean_watts': 80.0}},
            {'name': '4 Streams @ 2500k'},  # Missing power
            {'name': '8 streams @ 1080p', 'power': {}},  # Empty power
        ]
        
        success = predictor.fit(scenarios)
        assert success is True
        assert len(predictor.training_data) == 1
    
    def test_predict_linear(self, sample_scenarios):
        """Test prediction with linear regression model."""
        predictor = PowerPredictor()
        predictor.fit(sample_scenarios)
        
        # Predict for various stream counts
        power_1 = predictor.predict(1)
        power_4 = predictor.predict(4)
        power_8 = predictor.predict(8)
        
        assert power_1 is not None
        assert power_4 is not None
        assert power_8 is not None
        
        # Power should increase with stream count
        assert power_1 < power_4 < power_8
        
        # Should be non-negative
        assert power_1 >= 0
        assert power_4 >= 0
        assert power_8 >= 0
    
    def test_predict_polynomial(self, large_dataset):
        """Test prediction with polynomial regression model."""
        predictor = PowerPredictor()
        predictor.fit(large_dataset)
        
        # Predict for various stream counts
        power_2 = predictor.predict(2)
        power_6 = predictor.predict(6)
        power_12 = predictor.predict(12)
        
        assert power_2 is not None
        assert power_6 is not None
        assert power_12 is not None
        
        # Power should increase with stream count
        assert power_2 < power_6 < power_12
        
        # Should be non-negative
        assert power_2 >= 0
    
    def test_predict_before_training(self):
        """Test prediction before model is trained."""
        predictor = PowerPredictor()
        power = predictor.predict(4)
        
        assert power is None
    
    def test_predict_interpolation(self, sample_scenarios):
        """Test prediction within training range."""
        predictor = PowerPredictor()
        predictor.fit(sample_scenarios)
        
        # Predict for stream count within range (between 2 and 4)
        power_3 = predictor.predict(3)
        power_2 = predictor.predict(2)
        power_4 = predictor.predict(4)
        
        # 3 streams should be between 2 and 4 streams
        assert power_2 < power_3 < power_4
    
    def test_predict_extrapolation(self, sample_scenarios):
        """Test prediction outside training range."""
        predictor = PowerPredictor()
        predictor.fit(sample_scenarios)
        
        # Predict for stream counts outside training range
        power_16 = predictor.predict(16)
        power_8 = predictor.predict(8)
        
        # Should still produce valid predictions
        assert power_16 is not None
        assert power_8 is not None
        
        # Should be increasing
        assert power_8 < power_16
    
    def test_get_model_info_untrained(self):
        """Test get_model_info before training."""
        predictor = PowerPredictor()
        info = predictor.get_model_info()
        
        assert info['trained'] is False
        assert info['model_type'] is None
        assert info['n_samples'] == 0
        assert info['stream_range'] is None
    
    def test_get_model_info_linear(self, sample_scenarios):
        """Test get_model_info after linear training."""
        predictor = PowerPredictor()
        predictor.fit(sample_scenarios)
        info = predictor.get_model_info()
        
        assert info['trained'] is True
        assert info['model_type'] == 'linear'
        assert info['n_samples'] == 4
        assert info['stream_range'] == (1, 8)
    
    def test_get_model_info_polynomial(self, large_dataset):
        """Test get_model_info after polynomial training."""
        predictor = PowerPredictor()
        predictor.fit(large_dataset)
        info = predictor.get_model_info()
        
        assert info['trained'] is True
        assert info['model_type'] == 'polynomial'
        assert info['n_samples'] == 7
        assert info['stream_range'] == (1, 12)
    
    def test_small_dataset_fallback(self):
        """Test that small datasets use linear regression."""
        predictor = PowerPredictor()
        
        # Only 3 unique stream counts (< 6)
        scenarios = [
            {'name': '1 Stream @ 2500k', 'power': {'mean_watts': 45.0}},
            {'name': '2 Streams @ 2500k', 'power': {'mean_watts': 80.0}},
            {'name': '4 Streams @ 2500k', 'power': {'mean_watts': 150.0}},
        ]
        
        success = predictor.fit(scenarios)
        assert success is True
        assert predictor.is_polynomial is False
    
    def test_single_datapoint(self):
        """Test with only one training point."""
        predictor = PowerPredictor()
        scenarios = [
            {'name': '2 Streams @ 2500k', 'power': {'mean_watts': 80.0}},
        ]
        
        success = predictor.fit(scenarios)
        assert success is True
        
        # Should still be able to predict
        power = predictor.predict(4)
        assert power is not None
    
    def test_duplicate_stream_counts(self):
        """Test with duplicate stream counts (same count, different scenarios)."""
        predictor = PowerPredictor()
        scenarios = [
            {'name': '2 Streams @ 2500k', 'power': {'mean_watts': 80.0}},
            {'name': '2 streams @ 1080p', 'power': {'mean_watts': 85.0}},
            {'name': '4 Streams @ 2500k', 'power': {'mean_watts': 150.0}},
            {'name': '4 streams @ 720p', 'power': {'mean_watts': 145.0}},
        ]
        
        success = predictor.fit(scenarios)
        assert success is True
        
        # Should have 4 training points but only 2 unique stream counts
        assert len(predictor.training_data) == 4
        
        # Model should still work
        power = predictor.predict(3)
        assert power is not None
    
    def test_predict_zero_streams(self, sample_scenarios):
        """Test prediction for zero streams."""
        predictor = PowerPredictor()
        predictor.fit(sample_scenarios)
        
        power = predictor.predict(0)
        assert power is not None
        assert power >= 0  # Should be non-negative
    
    def test_predict_large_stream_count(self, sample_scenarios):
        """Test prediction for very large stream count."""
        predictor = PowerPredictor()
        predictor.fit(sample_scenarios)
        
        power = predictor.predict(100)
        assert power is not None
        assert power >= 0  # Should be non-negative
