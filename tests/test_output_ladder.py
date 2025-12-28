"""Tests for output ladder support in scoring and recommender modules."""

import pytest

from advisor.recommender import TranscodingRecommender
from advisor.scoring import EnergyEfficiencyScorer


class TestOutputLadderScoring:
    """Tests for output ladder scoring functionality."""

    def test_parse_resolution(self):
        """Test resolution parsing."""
        scorer = EnergyEfficiencyScorer()

        assert scorer._parse_resolution("1920x1080") == (1920, 1080)
        assert scorer._parse_resolution("1280x720") == (1280, 720)
        assert scorer._parse_resolution("854x480") == (854, 480)
        assert scorer._parse_resolution("N/A") == (None, None)
        assert scorer._parse_resolution("") == (None, None)

    def test_get_output_ladder_single_resolution(self):
        """Test output ladder extraction for single resolution scenario."""
        scorer = EnergyEfficiencyScorer()

        scenario = {
            'name': 'Single 720p stream',
            'resolution': '1280x720',
            'fps': 30
        }

        ladder = scorer.get_output_ladder(scenario)
        assert ladder == "1280x720@30"

    def test_get_output_ladder_multi_resolution(self):
        """Test output ladder extraction for multi-resolution scenario."""
        scorer = EnergyEfficiencyScorer()

        scenario = {
            'name': 'Multi-resolution ladder',
            'outputs': [
                {'resolution': '1920x1080', 'fps': 30},
                {'resolution': '1280x720', 'fps': 30},
                {'resolution': '854x480', 'fps': 30}
            ]
        }

        ladder = scorer.get_output_ladder(scenario)
        # Should be sorted by resolution (descending)
        assert ladder == "1920x1080@30,1280x720@30,854x480@30"

    def test_get_output_ladder_unsorted_outputs(self):
        """Test that output ladder is normalized (sorted)."""
        scorer = EnergyEfficiencyScorer()

        # Outputs provided in non-standard order
        scenario = {
            'name': 'Unsorted outputs',
            'outputs': [
                {'resolution': '854x480', 'fps': 30},
                {'resolution': '1920x1080', 'fps': 30},
                {'resolution': '1280x720', 'fps': 30}
            ]
        }

        ladder = scorer.get_output_ladder(scenario)
        # Should still be sorted correctly
        assert ladder == "1920x1080@30,1280x720@30,854x480@30"

    def test_get_output_ladder_no_data(self):
        """Test output ladder with no resolution data."""
        scorer = EnergyEfficiencyScorer()

        scenario = {'name': 'No data'}
        ladder = scorer.get_output_ladder(scenario)
        assert ladder is None

    def test_compute_total_pixels_single_resolution(self):
        """Test pixel computation for single resolution."""
        scorer = EnergyEfficiencyScorer()

        scenario = {
            'name': 'Single 720p',
            'resolution': '1280x720',
            'fps': 30,
            'duration': 120  # 2 minutes
        }

        # 1280 * 720 * 30 fps * 120 seconds = 3,317,760,000 pixels
        expected_pixels = 1280 * 720 * 30 * 120
        pixels = scorer._compute_total_pixels(scenario)

        assert pixels == expected_pixels

    def test_compute_total_pixels_multi_resolution(self):
        """Test pixel computation for multi-resolution ladder."""
        scorer = EnergyEfficiencyScorer()

        scenario = {
            'name': 'Multi-resolution',
            'duration': 120,
            'outputs': [
                {'resolution': '1920x1080', 'fps': 30},
                {'resolution': '1280x720', 'fps': 30},
                {'resolution': '854x480', 'fps': 30}
            ]
        }

        # Sum of pixels across all outputs
        pixels_1080p = 1920 * 1080 * 30 * 120
        pixels_720p = 1280 * 720 * 30 * 120
        pixels_480p = 854 * 480 * 30 * 120
        expected_pixels = pixels_1080p + pixels_720p + pixels_480p

        pixels = scorer._compute_total_pixels(scenario)
        assert pixels == expected_pixels

    def test_compute_total_pixels_no_duration(self):
        """Test pixel computation with missing duration."""
        scorer = EnergyEfficiencyScorer()

        scenario = {
            'name': 'No duration',
            'resolution': '1280x720',
            'fps': 30
        }

        pixels = scorer._compute_total_pixels(scenario)
        assert pixels is None

    def test_pixels_per_joule_scoring(self):
        """Test pixels-per-joule scoring algorithm."""
        scorer = EnergyEfficiencyScorer(algorithm='pixels_per_joule')

        scenario = {
            'name': 'Test scenario',
            'resolution': '1280x720',
            'fps': 30,
            'duration': 120,
            'power': {
                'mean_watts': 100.0,
                'total_energy_joules': 12000.0  # 100W * 120s
            }
        }

        # Total pixels: 1280 * 720 * 30 * 120 = 3,317,760,000
        # Score: 3,317,760,000 / 12000 = 276,480 pixels/joule
        expected_score = (1280 * 720 * 30 * 120) / 12000.0

        score = scorer.compute_score(scenario)
        assert score is not None
        assert pytest.approx(score, rel=1e-6) == expected_score

    def test_pixels_per_joule_multi_resolution(self):
        """Test pixels-per-joule for multi-resolution ladder."""
        scorer = EnergyEfficiencyScorer(algorithm='pixels_per_joule')

        scenario = {
            'name': 'Multi-res test',
            'duration': 120,
            'outputs': [
                {'resolution': '1920x1080', 'fps': 30},
                {'resolution': '1280x720', 'fps': 30}
            ],
            'power': {
                'total_energy_joules': 15000.0
            }
        }

        # Total pixels across both outputs
        total_pixels = (1920 * 1080 * 30 * 120) + (1280 * 720 * 30 * 120)
        expected_score = total_pixels / 15000.0

        score = scorer.compute_score(scenario)
        assert score is not None
        assert pytest.approx(score, rel=1e-6) == expected_score

    def test_pixels_per_joule_missing_energy(self):
        """Test that pixels-per-joule returns None without energy data."""
        scorer = EnergyEfficiencyScorer(algorithm='pixels_per_joule')

        scenario = {
            'name': 'No energy',
            'resolution': '1280x720',
            'fps': 30,
            'duration': 120,
            'power': {'mean_watts': 100.0}  # No total_energy_joules
        }

        score = scorer.compute_score(scenario)
        assert score is None


class TestOutputLadderRecommender:
    """Tests for ladder-aware recommendation functionality."""

    @pytest.fixture
    def ladder_scenarios(self):
        """Sample scenarios with different output ladders."""
        return [
            {
                'name': 'Single 720p @ 2500k',
                'bitrate': '2500k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 120,
                'power': {
                    'mean_watts': 60.0,
                    'total_energy_joules': 7200.0
                }
            },
            {
                'name': 'Single 720p @ 5000k',
                'bitrate': '5000k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 120,
                'power': {
                    'mean_watts': 80.0,
                    'total_energy_joules': 9600.0
                }
            },
            {
                'name': 'Multi-res ladder @ 2500k',
                'bitrate': '2500k',
                'duration': 120,
                'outputs': [
                    {'resolution': '1920x1080', 'fps': 30},
                    {'resolution': '1280x720', 'fps': 30},
                    {'resolution': '854x480', 'fps': 30}
                ],
                'power': {
                    'mean_watts': 100.0,
                    'total_energy_joules': 12000.0
                }
            },
            {
                'name': 'Multi-res ladder @ 5000k',
                'bitrate': '5000k',
                'duration': 120,
                'outputs': [
                    {'resolution': '1920x1080', 'fps': 30},
                    {'resolution': '1280x720', 'fps': 30},
                    {'resolution': '854x480', 'fps': 30}
                ],
                'power': {
                    'mean_watts': 150.0,
                    'total_energy_joules': 18000.0
                }
            }
        ]

    def test_analyze_and_rank_by_ladder(self, ladder_scenarios):
        """Test that scenarios are grouped by output ladder."""
        recommender = TranscodingRecommender()
        by_ladder = recommender.analyze_and_rank_by_ladder(ladder_scenarios)

        # Should have 2 ladder groups
        assert len(by_ladder) == 2

        # Check that single-res scenarios are grouped together
        single_res_ladder = "1280x720@30"
        assert single_res_ladder in by_ladder
        assert len(by_ladder[single_res_ladder]) == 2

        # Check that multi-res scenarios are grouped together
        multi_res_ladder = "1920x1080@30,1280x720@30,854x480@30"
        assert multi_res_ladder in by_ladder
        assert len(by_ladder[multi_res_ladder]) == 2

    def test_ranking_within_ladder_groups(self, ladder_scenarios):
        """Test that ranking is done within each ladder group."""
        recommender = TranscodingRecommender()
        by_ladder = recommender.analyze_and_rank_by_ladder(ladder_scenarios)

        for ladder_key, scenarios in by_ladder.items():
            # Scenarios within each ladder should be ranked
            for i, scenario in enumerate(scenarios):
                if scenario.get('efficiency_score') is not None:
                    assert scenario['efficiency_rank'] == i + 1

            # First scenario should have best score within ladder
            if len(scenarios) > 1 and scenarios[0].get('efficiency_score'):
                for scenario in scenarios[1:]:
                    if scenario.get('efficiency_score'):
                        assert scenarios[0]['efficiency_score'] >= scenario['efficiency_score']

    def test_get_best_per_ladder(self, ladder_scenarios):
        """Test getting best configuration per ladder."""
        recommender = TranscodingRecommender()
        best_per_ladder = recommender.get_best_per_ladder(ladder_scenarios)

        # Should have best for each ladder
        assert len(best_per_ladder) == 2

        for ladder_key, best in best_per_ladder.items():
            assert best is not None
            assert 'efficiency_score' in best
            assert 'efficiency_rank' in best
            assert best['efficiency_rank'] == 1

    def test_output_ladder_field_in_results(self, ladder_scenarios):
        """Test that output_ladder field is added to results."""
        recommender = TranscodingRecommender()
        by_ladder = recommender.analyze_and_rank_by_ladder(ladder_scenarios)

        for ladder_key, scenarios in by_ladder.items():
            for scenario in scenarios:
                assert 'output_ladder' in scenario
                assert scenario['output_ladder'] is not None or ladder_key == '_no_ladder_'

    def test_backward_compatibility_no_outputs(self):
        """Test that traditional scenarios without outputs still work."""
        recommender = TranscodingRecommender()

        traditional_scenario = {
            'name': 'Traditional 720p',
            'bitrate': '2500k',
            'resolution': '1280x720',
            'fps': 30,
            'duration': 120,
            'power': {
                'mean_watts': 60.0,
                'total_energy_joules': 7200.0
            }
        }

        by_ladder = recommender.analyze_and_rank_by_ladder([traditional_scenario])

        # Should work with traditional scenarios
        assert len(by_ladder) == 1
        ladder_key = "1280x720@30"
        assert ladder_key in by_ladder

    def test_mixed_scenarios_with_and_without_outputs(self):
        """Test handling mixed scenarios (some with outputs, some without)."""
        recommender = TranscodingRecommender()

        mixed_scenarios = [
            {
                'name': 'Traditional',
                'bitrate': '2500k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 120,
                'power': {'mean_watts': 60.0, 'total_energy_joules': 7200.0}
            },
            {
                'name': 'With outputs',
                'bitrate': '2500k',
                'duration': 120,
                'outputs': [
                    {'resolution': '1920x1080', 'fps': 30},
                    {'resolution': '1280x720', 'fps': 30}
                ],
                'power': {'mean_watts': 90.0, 'total_energy_joules': 10800.0}
            }
        ]

        by_ladder = recommender.analyze_and_rank_by_ladder(mixed_scenarios)

        # Should create separate ladder groups
        assert len(by_ladder) == 2


class TestPixelsPerJouleIntegration:
    """Integration tests for pixels-per-joule scoring."""

    def test_end_to_end_pixels_scoring(self):
        """Test complete workflow with pixels-per-joule scoring."""
        scorer = EnergyEfficiencyScorer(algorithm='pixels_per_joule')
        recommender = TranscodingRecommender(scorer=scorer)

        scenarios = [
            {
                'name': 'Low power 720p',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 120,
                'power': {
                    'mean_watts': 50.0,
                    'total_energy_joules': 6000.0
                }
            },
            {
                'name': 'High power 720p',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 120,
                'power': {
                    'mean_watts': 100.0,
                    'total_energy_joules': 12000.0
                }
            }
        ]

        ranked = recommender.analyze_and_rank(scenarios)

        # Low power should be ranked first (more pixels per joule)
        assert ranked[0]['name'] == 'Low power 720p'
        assert ranked[0]['efficiency_rank'] == 1
        assert ranked[1]['name'] == 'High power 720p'
        assert ranked[1]['efficiency_rank'] == 2

    def test_algorithm_selection_impacts_ranking(self):
        """Test that different algorithms produce different rankings."""
        # Use throughput_per_watt (default)
        recommender_throughput = TranscodingRecommender()

        # Use pixels_per_joule
        scorer_pixels = EnergyEfficiencyScorer(algorithm='pixels_per_joule')
        recommender_pixels = TranscodingRecommender(scorer=scorer_pixels)

        scenarios = [
            {
                'name': 'Scenario A',
                'bitrate': '2500k',
                'resolution': '1280x720',
                'fps': 30,
                'duration': 120,
                'power': {
                    'mean_watts': 60.0,
                    'total_energy_joules': 7200.0
                }
            }
        ]

        # Both should compute scores, but formulas differ
        ranked_throughput = recommender_throughput.analyze_and_rank(scenarios)
        ranked_pixels = recommender_pixels.analyze_and_rank(scenarios)

        # Both should produce valid scores
        assert ranked_throughput[0].get('efficiency_score') is not None
        assert ranked_pixels[0].get('efficiency_score') is not None

        # Scores should be different (different formulas)
        assert ranked_throughput[0]['efficiency_score'] != ranked_pixels[0]['efficiency_score']
