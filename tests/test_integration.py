"""
Integration test: verify advisor works with analyze_results.py
Creates sample test data and verifies the integration.
"""

import json
import sys
from pathlib import Path
from unittest.mock import Mock, patch

import pytest

# Add scripts directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))

from analyze_results import ResultsAnalyzer


class TestAnalyzeResultsIntegration:
    """Integration tests for advisor + analyze_results."""

    @pytest.fixture
    def sample_test_results(self):
        """Create sample test results JSON."""
        return {
            "test_date": "2024-01-15T10:00:00",
            "scenarios": [
                {
                    "name": "Baseline (Idle)",
                    "bitrate": "0k",
                    "resolution": "1280x720",
                    "fps": 30,
                    "duration": 120,
                    "start_time": 1705315200.0,
                    "end_time": 1705315320.0,
                },
                {
                    "name": "1 Mbps Stream",
                    "bitrate": "1000k",
                    "resolution": "1280x720",
                    "fps": 30,
                    "duration": 300,
                    "start_time": 1705315400.0,
                    "end_time": 1705315700.0,
                },
                {
                    "name": "2.5 Mbps Stream",
                    "bitrate": "2500k",
                    "resolution": "1280x720",
                    "fps": 30,
                    "duration": 300,
                    "start_time": 1705315800.0,
                    "end_time": 1705316100.0,
                },
            ],
        }

    @pytest.fixture
    def mock_prometheus_responses(self):
        """Mock Prometheus responses for power data."""

        def mock_query_range(query, start, end, step='15s'):
            # Return mock power data
            return {
                'status': 'success',
                'data': {
                    'result': [
                        {'values': [[start, '50.0'], [start + 10, '52.0'], [start + 20, '51.0']]}
                    ]
                },
            }

        def mock_query(query, ts=None):
            # Return mock energy data
            if 'increase(rapl_energy_joules_total' in query:
                return {
                    'status': 'success',
                    'data': {'result': [{'value': [ts or 1705315320.0, '6000.0']}]},
                }
            return None

        return mock_query_range, mock_query

    def test_analyzer_computes_efficiency_scores(
        self, sample_test_results, mock_prometheus_responses, tmp_path
    ):
        """Test that ResultsAnalyzer computes efficiency scores."""
        # Create temporary results file
        results_file = tmp_path / "test_results.json"
        with open(results_file, 'w') as f:
            json.dump(sample_test_results, f)

        # Mock Prometheus client
        mock_query_range, mock_query = mock_prometheus_responses

        with patch('analyze_results.PrometheusClient') as MockClient:
            mock_client = Mock()
            mock_client.query_range = Mock(side_effect=mock_query_range)
            mock_client.query = Mock(side_effect=mock_query)
            MockClient.return_value = mock_client

            # Create analyzer
            analyzer = ResultsAnalyzer(results_file)

            # Generate report
            results = analyzer.generate_report()

            # Verify results
            assert len(results) > 0

            # Check that efficiency scores were computed
            scored_results = [r for r in results if r.get('efficiency_score') is not None]
            assert len(scored_results) > 0

            # Verify baseline has no score (0k bitrate)
            baseline = next(r for r in results if 'baseline' in r['name'].lower())
            assert baseline.get('efficiency_score') is None

            # Verify streaming scenarios have scores and ranks
            for result in scored_results:
                assert 'efficiency_score' in result
                assert result['efficiency_score'] > 0
                assert 'efficiency_rank' in result
                assert result['efficiency_rank'] >= 1

    def test_csv_export_includes_efficiency_columns(
        self, sample_test_results, mock_prometheus_responses, tmp_path
    ):
        """Test that CSV export includes efficiency score and rank."""
        # Create temporary results file
        results_file = tmp_path / "test_results.json"
        with open(results_file, 'w') as f:
            json.dump(sample_test_results, f)

        csv_file = tmp_path / "test_analysis.csv"

        # Mock Prometheus client
        mock_query_range, mock_query = mock_prometheus_responses

        with patch('analyze_results.PrometheusClient') as MockClient:
            mock_client = Mock()
            mock_client.query_range = Mock(side_effect=mock_query_range)
            mock_client.query = Mock(side_effect=mock_query)
            MockClient.return_value = mock_client

            # Create analyzer and export CSV
            analyzer = ResultsAnalyzer(results_file)
            analyzer.export_csv(output_file=str(csv_file))

            # Verify CSV was created
            assert csv_file.exists()

            # Read and verify CSV content
            with open(csv_file, 'r') as f:
                header = f.readline()
                assert 'efficiency_score' in header
                assert 'efficiency_rank' in header
