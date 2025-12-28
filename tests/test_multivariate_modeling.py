"""Tests for the MultivariatePredictor class."""

import tempfile
from pathlib import Path

import pytest

from advisor.modeling import MultivariatePredictor


class TestMultivariatePredictor:
    """Tests for MultivariatePredictor class."""

    @pytest.fixture
    def sample_scenarios(self):
        """Sample scenario data with full feature set."""
        return [
            {
                'name': '2 Streams @ 2500k',
                'bitrate': '2500k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 60.0,
                'power': {'mean_watts': 80.0, 'total_energy_joules': 4800.0},
                'container_usage': {'cpu_percent': 40.0},
                'docker_overhead': {'cpu_percent': 5.0},
                'encoder_type': 'x264',
                'hardware': {'cpu_model': 'Intel_i7_9700K'},
                'efficiency_score': 0.05
            },
            {
                'name': '4 Streams @ 2500k',
                'bitrate': '2500k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 60.0,
                'power': {'mean_watts': 150.0, 'total_energy_joules': 9000.0},
                'container_usage': {'cpu_percent': 75.0},
                'docker_overhead': {'cpu_percent': 6.0},
                'encoder_type': 'x264',
                'hardware': {'cpu_model': 'Intel_i7_9700K'},
                'efficiency_score': 0.067
            },
            {
                'name': '8 streams @ 1080p',
                'bitrate': '5000k',
                'resolution': '1920x1080',
                'fps': 30,
                'duration': 60.0,
                'power': {'mean_watts': 280.0, 'total_energy_joules': 16800.0},
                'container_usage': {'cpu_percent': 95.0},
                'docker_overhead': {'cpu_percent': 8.0},
                'encoder_type': 'x264',
                'hardware': {'cpu_model': 'Intel_i7_9700K'},
                'efficiency_score': 0.143
            },
        ]

    @pytest.fixture
    def multiresolution_scenarios(self):
        """Scenarios with output ladders."""
        return [
            {
                'name': '2 Streams @ 2500k',
                'bitrate': '2500k',
                'duration': 60.0,
                'outputs': [
                    {'resolution': '1920x1080', 'fps': 30},
                    {'resolution': '1280x720', 'fps': 30},
                    {'resolution': '854x480', 'fps': 30},
                ],
                'power': {'mean_watts': 120.0, 'total_energy_joules': 7200.0},
                'container_usage': {'cpu_percent': 60.0},
                'docker_overhead': {'cpu_percent': 5.0},
                'encoder_type': 'x264',
                'hardware': {'cpu_model': 'Intel_i7_9700K'},
                'efficiency_score': 0.08
            },
        ]

    def test_initialization(self):
        """Test MultivariatePredictor initialization."""
        predictor = MultivariatePredictor()
        assert predictor.pipelines == {}
        assert predictor.best_model_name is None
        assert predictor.confidence_level == 0.95
        assert predictor.n_bootstrap == 100
        assert predictor.cv_folds == 5

    def test_initialization_custom_params(self):
        """Test initialization with custom parameters."""
        predictor = MultivariatePredictor(
            models=['linear', 'rf'],
            confidence_level=0.90,
            n_bootstrap=50,
            cv_folds=3
        )
        assert predictor.models == ['linear', 'rf']
        assert predictor.confidence_level == 0.90
        assert predictor.n_bootstrap == 50
        assert predictor.cv_folds == 3

    def test_extract_features(self, sample_scenarios):
        """Test feature extraction from scenarios."""
        predictor = MultivariatePredictor()
        features = predictor._extract_features(sample_scenarios[0])

        assert features is not None
        assert features['stream_count'] == 2
        assert features['bitrate_mbps'] == 2.5
        assert features['total_pixels'] > 0
        assert features['cpu_usage_pct'] == 40.0
        assert features['encoder_type'] == 'x264'
        assert features['hardware_cpu_model'] == 'Intel_i7_9700K'
        assert features['container_cpu_pct'] == 5.0

    def test_extract_features_multiresolution(self, multiresolution_scenarios):
        """Test feature extraction with output ladders."""
        predictor = MultivariatePredictor()
        features = predictor._extract_features(multiresolution_scenarios[0])

        assert features is not None
        assert features['stream_count'] == 2
        # Total pixels should be sum of all outputs
        expected_pixels = (1920*1080 + 1280*720 + 854*480) * 30 * 60
        assert features['total_pixels'] == expected_pixels

    def test_extract_features_missing_name(self):
        """Test feature extraction with missing stream count."""
        predictor = MultivariatePredictor()
        scenario = {'bitrate': '2500k'}
        features = predictor._extract_features(scenario)

        # Should return None if stream count cannot be inferred
        assert features is None

    def test_fit_power_prediction(self, sample_scenarios):
        """Test training for power prediction."""
        predictor = MultivariatePredictor(models=['linear', 'poly2'])
        success = predictor.fit(sample_scenarios, target='mean_power_watts')

        assert success is True
        assert predictor.target_name == 'mean_power_watts'
        assert len(predictor.pipelines) >= 1
        assert predictor.best_model_name is not None
        assert predictor.best_model_name in predictor.model_scores

    def test_fit_energy_prediction(self, sample_scenarios):
        """Test training for energy prediction."""
        predictor = MultivariatePredictor(models=['linear'])
        success = predictor.fit(sample_scenarios, target='total_energy_joules')

        assert success is True
        assert predictor.target_name == 'total_energy_joules'
        assert 'linear' in predictor.pipelines

    def test_fit_efficiency_prediction(self, sample_scenarios):
        """Test training for efficiency prediction."""
        predictor = MultivariatePredictor(models=['linear'])
        success = predictor.fit(sample_scenarios, target='efficiency_score')

        assert success is True
        assert predictor.target_name == 'efficiency_score'

    def test_fit_insufficient_data(self):
        """Test fitting with insufficient data."""
        predictor = MultivariatePredictor()
        scenarios = [
            {
                'name': '2 Streams @ 2500k',
                'bitrate': '2500k',
                'power': {'mean_watts': 80.0}
            }
        ]
        success = predictor.fit(scenarios, target='mean_power_watts')

        # Should fail with only 1 sample
        assert success is False

    def test_fit_no_valid_data(self):
        """Test fitting with no valid data."""
        predictor = MultivariatePredictor()
        scenarios = [
            {'name': 'Baseline', 'bitrate': '0k'},
            {'name': 'Test', 'bitrate': '2500k'},
        ]
        success = predictor.fit(scenarios, target='mean_power_watts')

        assert success is False

    def test_predict_power(self, sample_scenarios):
        """Test prediction after training."""
        predictor = MultivariatePredictor(models=['linear'])
        predictor.fit(sample_scenarios, target='mean_power_watts')

        features = {
            'stream_count': 6,
            'bitrate_mbps': 3.0,
            'total_pixels': 1920 * 1080 * 30 * 60,
            'cpu_usage_pct': 80.0,
            'encoder_type': 'x264',
            'hardware_cpu_model': 'Intel_i7_9700K',
            'container_cpu_pct': 7.0
        }

        prediction = predictor.predict(features, return_confidence=False)

        assert prediction['mean'] is not None
        assert prediction['mean'] >= 0
        assert prediction['model'] == 'linear'

    def test_predict_with_confidence(self, sample_scenarios):
        """Test prediction with confidence intervals."""
        predictor = MultivariatePredictor(
            models=['linear'],
            n_bootstrap=10  # Reduced for faster testing
        )
        predictor.fit(sample_scenarios, target='mean_power_watts')

        features = {
            'stream_count': 6,
            'bitrate_mbps': 3.0,
            'total_pixels': 1920 * 1080 * 30 * 60,
            'cpu_usage_pct': 80.0,
            'encoder_type': 'x264',
            'hardware_cpu_model': 'Intel_i7_9700K',
            'container_cpu_pct': 7.0
        }

        prediction = predictor.predict(features, return_confidence=True)

        assert prediction['mean'] is not None
        assert 'ci_low' in prediction
        assert 'ci_high' in prediction
        assert 'ci_width' in prediction
        # CI should be non-negative and reasonable
        assert prediction['ci_low'] >= 0
        assert prediction['ci_high'] >= prediction['ci_low']
        assert prediction['ci_width'] >= 0

    def test_predict_before_training(self):
        """Test prediction before model is trained."""
        predictor = MultivariatePredictor()
        features = {'stream_count': 4}

        prediction = predictor.predict(features)

        assert prediction['mean'] is None
        assert prediction['model'] is None

    def test_predict_batch(self, sample_scenarios):
        """Test batch prediction."""
        predictor = MultivariatePredictor(models=['linear'])
        predictor.fit(sample_scenarios, target='mean_power_watts')

        features_list = [
            {
                'stream_count': 3,
                'bitrate_mbps': 2.5,
                'total_pixels': 1280 * 720 * 30 * 60,
                'cpu_usage_pct': 50.0,
                'encoder_type': 'x264',
                'hardware_cpu_model': 'Intel_i7_9700K',
                'container_cpu_pct': 5.0
            },
            {
                'stream_count': 6,
                'bitrate_mbps': 3.0,
                'total_pixels': 1920 * 1080 * 30 * 60,
                'cpu_usage_pct': 80.0,
                'encoder_type': 'x264',
                'hardware_cpu_model': 'Intel_i7_9700K',
                'container_cpu_pct': 7.0
            },
        ]

        predictions = predictor.predict_batch(features_list)

        assert len(predictions) == 2
        assert predictions[0]['mean'] is not None
        assert predictions[1]['mean'] is not None
        assert predictions[0]['mean'] < predictions[1]['mean']  # More streams = more power

    def test_get_model_info_untrained(self):
        """Test get_model_info before training."""
        predictor = MultivariatePredictor()
        info = predictor.get_model_info()

        assert info['trained'] is False
        assert info['best_model'] is None
        assert info['n_samples'] == 0

    def test_get_model_info_trained(self, sample_scenarios):
        """Test get_model_info after training."""
        predictor = MultivariatePredictor(models=['linear', 'poly2'])
        predictor.fit(sample_scenarios, target='mean_power_watts')
        info = predictor.get_model_info()

        assert info['trained'] is True
        assert info['best_model'] in ['linear', 'poly2']
        assert info['best_score'] is not None
        assert 'r2' in info['best_score']
        assert 'rmse' in info['best_score']
        assert info['n_samples'] == 3
        assert info['target'] == 'mean_power_watts'
        assert info['version'] == '1.0'

    def test_model_selection(self, sample_scenarios):
        """Test that best model is selected based on R²."""
        predictor = MultivariatePredictor(models=['linear', 'poly2'])
        predictor.fit(sample_scenarios, target='mean_power_watts')

        # Check that model with highest R² is selected
        best_r2 = predictor.best_model_score['r2']
        for model_name, scores in predictor.model_scores.items():
            assert scores['r2'] <= best_r2

    def test_save_and_load(self, sample_scenarios):
        """Test model persistence."""
        predictor = MultivariatePredictor(models=['linear'])
        predictor.fit(sample_scenarios, target='mean_power_watts')

        # Save model
        with tempfile.TemporaryDirectory() as tmpdir:
            filepath = Path(tmpdir) / 'test_model.pkl'
            predictor.save(filepath)

            assert filepath.exists()

            # Load model
            loaded_predictor = MultivariatePredictor.load(filepath)

            assert loaded_predictor.target_name == 'mean_power_watts'
            assert loaded_predictor.best_model_name == 'linear'
            assert len(loaded_predictor.pipelines) == 1

            # Test prediction with loaded model
            features = {
                'stream_count': 6,
                'bitrate_mbps': 3.0,
                'total_pixels': 1920 * 1080 * 30 * 60,
                'cpu_usage_pct': 80.0,
                'encoder_type': 'x264',
                'hardware_cpu_model': 'Intel_i7_9700K',
                'container_cpu_pct': 7.0
            }

            original_pred = predictor.predict(features, return_confidence=False)
            loaded_pred = loaded_predictor.predict(features, return_confidence=False)

            assert abs(original_pred['mean'] - loaded_pred['mean']) < 0.01

    def test_different_encoder_types(self):
        """Test handling of different encoder types."""
        scenarios = [
            {
                'name': '2 Streams @ 2500k',
                'bitrate': '2500k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 60.0,
                'power': {'mean_watts': 80.0},
                'container_usage': {'cpu_percent': 40.0},
                'docker_overhead': {'cpu_percent': 5.0},
                'encoder_type': 'x264',
                'hardware': {'cpu_model': 'Intel_i7_9700K'},
            },
            {
                'name': '2 streams GPU',
                'bitrate': '2500k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 60.0,
                'power': {'mean_watts': 60.0},  # GPU encoding uses less CPU power
                'container_usage': {'cpu_percent': 20.0},
                'docker_overhead': {'cpu_percent': 3.0},
                'encoder_type': 'nvenc',
                'hardware': {'cpu_model': 'Intel_i7_9700K'},
            },
            {
                'name': '4 Streams @ 2500k',
                'bitrate': '2500k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 60.0,
                'power': {'mean_watts': 150.0},
                'container_usage': {'cpu_percent': 75.0},
                'docker_overhead': {'cpu_percent': 6.0},
                'encoder_type': 'x264',
                'hardware': {'cpu_model': 'Intel_i7_9700K'},
            },
        ]

        predictor = MultivariatePredictor(models=['linear'])
        success = predictor.fit(scenarios, target='mean_power_watts')

        assert success is True
        # Check that encoder types are captured
        assert 'x264' in predictor.encoder_categories['encoder_type']
        assert 'nvenc' in predictor.encoder_categories['encoder_type']

    def test_cross_validation_scores(self, sample_scenarios):
        """Test that cross-validation produces valid scores."""
        # Need enough samples for CV
        extended_scenarios = sample_scenarios * 3  # 9 samples

        predictor = MultivariatePredictor(models=['linear'], cv_folds=3)
        predictor.fit(extended_scenarios, target='mean_power_watts')

        info = predictor.get_model_info()
        assert 'linear' in info['models']
        assert 'r2' in info['models']['linear']
        assert 'rmse' in info['models']['linear']
        # R² should be reasonable for this data
        assert -1.0 <= info['models']['linear']['r2'] <= 1.0

    def test_hardware_id_tracking(self, sample_scenarios):
        """Test hardware ID is tracked correctly."""
        predictor = MultivariatePredictor()
        predictor.fit(
            sample_scenarios,
            target='mean_power_watts',
            hardware_id='Intel_i7_9700K'
        )

        info = predictor.get_model_info()
        assert info['hardware'] == 'Intel_i7_9700K'
