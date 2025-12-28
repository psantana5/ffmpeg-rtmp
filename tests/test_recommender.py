"""Tests for the advisor.recommender module."""

import pytest

from advisor.recommender import TranscodingRecommender
from advisor.scoring import EnergyEfficiencyScorer


class TestTranscodingRecommender:
    """Tests for TranscodingRecommender class."""

    @pytest.fixture
    def sample_scenarios(self):
        """Sample scenario data for testing."""
        return [
            {
                'name': 'Baseline (Idle)',
                'bitrate': '0k',
                'power': {'mean_watts': 30.0},
                'resolution': '1280x720',
                'fps': 30
            },
            {
                'name': '1 Mbps Stream',
                'bitrate': '1000k',
                'power': {'mean_watts': 50.0},
                'resolution': '1280x720',
                'fps': 30
            },
            {
                'name': '2.5 Mbps Stream',
                'bitrate': '2500k',
                'power': {'mean_watts': 60.0},
                'resolution': '1280x720',
                'fps': 30
            },
            {
                'name': '5 Mbps Stream',
                'bitrate': '5M',
                'power': {'mean_watts': 80.0},
                'resolution': '1280x720',
                'fps': 30
            },
            {
                'name': '4 streams @ 2500k',
                'bitrate': '2500k',
                'power': {'mean_watts': 150.0},
                'resolution': '1280x720',
                'fps': 30
            },
        ]

    def test_initialization_default(self):
        """Test default initialization."""
        recommender = TranscodingRecommender()
        assert recommender.scorer is not None
        assert isinstance(recommender.scorer, EnergyEfficiencyScorer)

    def test_initialization_custom_scorer(self):
        """Test initialization with custom scorer."""
        custom_scorer = EnergyEfficiencyScorer()
        recommender = TranscodingRecommender(scorer=custom_scorer)
        assert recommender.scorer is custom_scorer

    def test_analyze_and_rank(self, sample_scenarios):
        """Test analyze_and_rank method."""
        recommender = TranscodingRecommender()
        ranked = recommender.analyze_and_rank(sample_scenarios)

        # All scenarios should be present
        assert len(ranked) == len(sample_scenarios)

        # Scenarios with scores should come first
        scored_count = sum(1 for s in ranked if s.get('efficiency_score') is not None)
        assert scored_count == 4  # All except baseline

        # Check that scored scenarios have ranks
        for i, scenario in enumerate(ranked[:scored_count]):
            assert 'efficiency_score' in scenario
            assert 'efficiency_rank' in scenario
            assert scenario['efficiency_rank'] == i + 1

    def test_analyze_and_rank_sorting(self, sample_scenarios):
        """Test that scenarios are sorted by efficiency score."""
        recommender = TranscodingRecommender()
        ranked = recommender.analyze_and_rank(sample_scenarios)

        # Get scenarios with scores
        scored = [s for s in ranked if s.get('efficiency_score') is not None]

        # Verify descending order
        for i in range(len(scored) - 1):
            assert scored[i]['efficiency_score'] >= scored[i + 1]['efficiency_score']

    def test_analyze_and_rank_preserves_input(self, sample_scenarios):
        """Test that input scenarios are not modified."""
        recommender = TranscodingRecommender()
        original_count = len(sample_scenarios)
        original_names = [s['name'] for s in sample_scenarios]

        # Call analyze_and_rank (return value not needed for this test)
        _ = recommender.analyze_and_rank(sample_scenarios)

        # Original list should be unchanged
        assert len(sample_scenarios) == original_count
        assert [s['name'] for s in sample_scenarios] == original_names

        # Original scenarios should not have efficiency_score
        for scenario in sample_scenarios:
            assert 'efficiency_score' not in scenario

    def test_get_best_configuration(self, sample_scenarios):
        """Test get_best_configuration method."""
        recommender = TranscodingRecommender()
        best = recommender.get_best_configuration(sample_scenarios)

        assert best is not None
        assert 'efficiency_score' in best
        assert 'efficiency_rank' in best
        assert best['efficiency_rank'] == 1

        # Best should be 4 streams @ 2500k: (2.5 * 4) / 150 = 0.0667 Mbps/W
        assert best['name'] == '4 streams @ 2500k'

    def test_get_best_configuration_no_valid_scenarios(self):
        """Test get_best_configuration with no valid scores."""
        recommender = TranscodingRecommender()
        scenarios = [
            {'name': 'Baseline', 'bitrate': '0k', 'power': {'mean_watts': 30}}
        ]

        best = recommender.get_best_configuration(scenarios)
        assert best is None

    def test_get_top_n(self, sample_scenarios):
        """Test get_top_n method."""
        recommender = TranscodingRecommender()

        # Get top 3
        top_3 = recommender.get_top_n(sample_scenarios, n=3)
        assert len(top_3) == 3

        # All should have scores
        for scenario in top_3:
            assert scenario.get('efficiency_score') is not None
            assert scenario.get('efficiency_rank') is not None

        # Should be in order
        assert top_3[0]['efficiency_rank'] == 1
        assert top_3[1]['efficiency_rank'] == 2
        assert top_3[2]['efficiency_rank'] == 3

    def test_get_top_n_exceeds_available(self, sample_scenarios):
        """Test get_top_n when n exceeds available scenarios."""
        recommender = TranscodingRecommender()

        # Request more than available (only 4 have scores)
        top_10 = recommender.get_top_n(sample_scenarios, n=10)
        assert len(top_10) == 4  # Should only return 4

    def test_get_recommendation_summary(self, sample_scenarios):
        """Test get_recommendation_summary method."""
        recommender = TranscodingRecommender()
        summary = recommender.get_recommendation_summary(sample_scenarios)

        # Check structure
        assert 'best_config' in summary
        assert 'top_5' in summary
        assert 'total_analyzed' in summary
        assert 'scorable' in summary
        assert 'efficiency_range' in summary
        assert 'power_range' in summary

        # Check values
        assert summary['total_analyzed'] == 5
        assert summary['scorable'] == 4
        assert summary['best_config'] is not None
        assert len(summary['top_5']) == 4

        # Check ranges
        assert summary['efficiency_range'] is not None
        assert len(summary['efficiency_range']) == 2
        assert summary['efficiency_range'][0] <= summary['efficiency_range'][1]

        assert summary['power_range'] is not None
        assert len(summary['power_range']) == 2
        assert summary['power_range'][0] == 30.0  # Baseline
        assert summary['power_range'][1] == 150.0  # 4 streams

    def test_compare_configurations(self, sample_scenarios):
        """Test compare_configurations method."""
        recommender = TranscodingRecommender()

        config_a = sample_scenarios[1]  # 1 Mbps @ 50W
        config_b = sample_scenarios[2]  # 2.5 Mbps @ 60W

        comparison = recommender.compare_configurations(config_a, config_b)

        assert 'winner' in comparison
        assert 'efficiency_diff' in comparison
        assert 'efficiency_pct_diff' in comparison
        assert 'power_diff' in comparison
        assert 'justification' in comparison

        # 2.5 Mbps @ 60W should be more efficient than 1 Mbps @ 50W
        # config_a: 1/50 = 0.02, config_b: 2.5/60 = 0.0417
        assert comparison['winner'] == 'config_b'
        assert comparison['efficiency_diff'] < 0  # config_a - config_b is negative
        assert comparison['power_diff'] == -10.0  # 50 - 60

    def test_compare_configurations_missing_scores(self):
        """Test compare_configurations with missing data."""
        recommender = TranscodingRecommender()

        config_a = {'name': 'Config A', 'bitrate': '0k', 'power': {'mean_watts': 30}}
        config_b = {'name': 'Config B', 'bitrate': '2500k'}  # Missing power

        comparison = recommender.compare_configurations(config_a, config_b)

        assert comparison['winner'] == 'tie'
        assert comparison['efficiency_diff'] is None
        assert 'Cannot compare' in comparison['justification']

    def test_generate_cli_report_not_implemented(self, sample_scenarios):
        """Test that generate_cli_report raises NotImplementedError."""
        recommender = TranscodingRecommender()

        with pytest.raises(NotImplementedError):
            recommender.generate_cli_report(sample_scenarios)
