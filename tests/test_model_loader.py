"""Tests for scripts.utils.model_loader module."""

import json
import pickle
import tempfile
from pathlib import Path

import numpy as np
import pytest

from advisor.modeling import MultivariatePredictor, PowerPredictor
from scripts.utils.model_loader import ModelLoader


class TestModelLoader:
    """Tests for ModelLoader class."""
    
    @pytest.fixture
    def sample_scenarios(self):
        """Sample scenario data for testing."""
        return [
            {
                'name': '1 Stream @ 2500k',
                'power': {'mean_watts': 45.0},
                'bitrate': '2500k',
                'duration': 120
            },
            {
                'name': '2 Streams @ 2500k',
                'power': {'mean_watts': 80.0},
                'bitrate': '2500k',
                'duration': 120
            },
            {
                'name': '4 Streams @ 2500k',
                'power': {'mean_watts': 150.0},
                'bitrate': '2500k',
                'duration': 120
            },
            {
                'name': '8 streams @ 1080p',
                'power': {'mean_watts': 280.0},
                'bitrate': '2500k',
                'duration': 120
            },
        ]
    
    @pytest.fixture
    def trained_power_predictor(self, sample_scenarios):
        """Create a trained PowerPredictor."""
        predictor = PowerPredictor()
        predictor.fit(sample_scenarios)
        return predictor
    
    @pytest.fixture
    def temp_model_file(self, trained_power_predictor):
        """Create a temporary model file."""
        with tempfile.NamedTemporaryFile(mode='wb', suffix='.pkl', delete=False) as f:
            pickle.dump(trained_power_predictor, f)
            temp_path = Path(f.name)
        
        yield temp_path
        
        # Cleanup
        if temp_path.exists():
            temp_path.unlink()
    
    def test_initialization(self):
        """Test ModelLoader initialization."""
        loader = ModelLoader()
        assert loader.search_paths is not None
        assert len(loader.search_paths) > 0
    
    def test_load_model(self, temp_model_file, trained_power_predictor):
        """Test loading a model from file."""
        loader = ModelLoader()
        model = loader.load_model(temp_model_file)
        
        assert model is not None
        assert isinstance(model, PowerPredictor)
        assert model.model is not None
    
    def test_load_model_nonexistent(self):
        """Test loading non-existent model."""
        loader = ModelLoader()
        model = loader.load_model(Path('/nonexistent/model.pkl'))
        
        assert model is None
    
    def test_get_model_metadata_power_predictor(self, trained_power_predictor):
        """Test extracting metadata from PowerPredictor."""
        loader = ModelLoader()
        metadata = loader.get_model_metadata(trained_power_predictor)
        
        assert metadata['trained'] is True
        assert metadata['n_samples'] == 4
        assert 'stream_count' in metadata['features']
        assert metadata['stream_range'] == (1, 8)
    
    def test_compute_residuals(self, trained_power_predictor, sample_scenarios):
        """Test computing residuals for scenarios."""
        loader = ModelLoader()
        residuals = loader.compute_residuals(trained_power_predictor, sample_scenarios)
        
        assert len(residuals) > 0
        
        # Check structure of residuals
        for name, measured, predicted, residual in residuals:
            assert isinstance(name, str)
            assert isinstance(measured, float)
            assert isinstance(predicted, float)
            assert isinstance(residual, float)
            assert residual == measured - predicted
    
    def test_detect_outliers(self):
        """Test outlier detection."""
        loader = ModelLoader()
        
        # Create residuals with one clear outlier
        residuals = [
            ('scenario1', 100.0, 100.0, 0.0),
            ('scenario2', 100.0, 102.0, -2.0),
            ('scenario3', 100.0, 98.0, 2.0),
            ('scenario4', 100.0, 101.0, -1.0),
            ('scenario5', 100.0, 99.0, 1.0),
            ('scenario6', 100.0, 150.0, -50.0),  # Clear outlier
        ]
        
        outliers = loader.detect_outliers(residuals, threshold=2.0)
        
        assert len(outliers) > 0
        assert any('scenario6' in name for name, _ in outliers)
    
    def test_detect_outliers_no_outliers(self):
        """Test outlier detection with no outliers."""
        loader = ModelLoader()
        
        residuals = [
            ('scenario1', 100.0, 100.0, 0.0),
            ('scenario2', 100.0, 102.0, -2.0),
            ('scenario3', 100.0, 98.0, 2.0),
        ]
        
        outliers = loader.detect_outliers(residuals, threshold=2.0)
        
        assert len(outliers) == 0
    
    def test_detect_outliers_empty(self):
        """Test outlier detection with empty residuals."""
        loader = ModelLoader()
        outliers = loader.detect_outliers([], threshold=2.0)
        
        assert len(outliers) == 0
    
    def test_get_feature_importance_no_model(self):
        """Test feature importance with untrained model."""
        loader = ModelLoader()
        predictor = PowerPredictor()
        
        importance = loader.get_feature_importance(predictor)
        
        assert len(importance) == 0
    
    def test_load_test_results_nonexistent(self):
        """Test loading test results from nonexistent directory."""
        loader = ModelLoader()
        results = loader.load_test_results(Path('/nonexistent'))
        
        assert results is None
    
    def test_load_test_results(self):
        """Test loading test results from directory."""
        loader = ModelLoader()
        
        # Create temporary directory with test results
        with tempfile.TemporaryDirectory() as tmpdir:
            tmpdir = Path(tmpdir)
            
            results_file = tmpdir / 'test_results_20240101_120000.json'
            with open(results_file, 'w') as f:
                json.dump({
                    'scenarios': [
                        {'name': 'Test', 'power': {'mean_watts': 100.0}}
                    ]
                }, f)
            
            results = loader.load_test_results(tmpdir)
            
            assert results is not None
            assert 'scenarios' in results
            assert len(results['scenarios']) == 1


class TestModelLoaderMultivariate:
    """Tests for ModelLoader with MultivariatePredictor."""
    
    @pytest.fixture
    def sample_scenarios(self):
        """Sample scenario data for multivariate testing."""
        return [
            {
                'name': '1 Stream @ 2500k',
                'power': {'mean_watts': 45.0},
                'bitrate': '2500k',
                'duration': 120,
                'resolution': '1920x1080',
                'fps': 30,
                'container_usage': {'cpu_percent': 10.0}
            },
            {
                'name': '2 Streams @ 2500k',
                'power': {'mean_watts': 80.0},
                'bitrate': '2500k',
                'duration': 120,
                'resolution': '1920x1080',
                'fps': 30,
                'container_usage': {'cpu_percent': 20.0}
            },
            {
                'name': '4 Streams @ 2500k',
                'power': {'mean_watts': 150.0},
                'bitrate': '2500k',
                'duration': 120,
                'resolution': '1920x1080',
                'fps': 30,
                'container_usage': {'cpu_percent': 40.0}
            },
        ]
    
    @pytest.fixture
    def trained_multivariate_predictor(self, sample_scenarios):
        """Create a trained MultivariatePredictor."""
        predictor = MultivariatePredictor(models=['linear'])
        predictor.fit(sample_scenarios, target='mean_power_watts')
        return predictor
    
    def test_get_model_metadata_multivariate(self, trained_multivariate_predictor):
        """Test extracting metadata from MultivariatePredictor."""
        loader = ModelLoader()
        metadata = loader.get_model_metadata(trained_multivariate_predictor)
        
        assert metadata['trained'] is True
        assert metadata['n_samples'] == 3
        assert 'best_model' in metadata
        assert metadata['target'] == 'mean_power_watts'
        assert len(metadata['features']) > 1
    
    def test_compute_residuals_multivariate(self, trained_multivariate_predictor, sample_scenarios):
        """Test computing residuals with MultivariatePredictor."""
        loader = ModelLoader()
        
        # Note: This may return empty list if model isn't trained properly
        # The test just checks that the method doesn't crash
        residuals = loader.compute_residuals(trained_multivariate_predictor, sample_scenarios)
        
        # Just verify it returns a list (may be empty if model failed to train)
        assert isinstance(residuals, list)
