# Test Suite Documentation

This directory contains the comprehensive test suite for the FFmpeg-RTMP energy monitoring and advisory system. The tests validate the energy efficiency scoring, recommendation logic, power prediction models, and integration with the analysis pipeline.

---

## Table of Contents

- [Overview](#overview)
- [Test Structure](#test-structure)
- [Installation](#installation)
- [Running Tests](#running-tests)
- [Test Coverage](#test-coverage)
- [Writing Tests](#writing-tests)
- [Continuous Integration](#continuous-integration)

---

## Overview

The test suite uses **pytest** as the testing framework and covers the following components:

- **Energy Efficiency Scoring** - Tests for throughput-per-watt and pixels-per-joule algorithms
- **Recommendation Logic** - Tests for ranking and selecting optimal configurations
- **Power Prediction Models** - Tests for both simple univariate and advanced multivariate models
- **Output Ladder Support** - Tests for multi-resolution transcoding scenarios
- **Results Export** - Tests for Prometheus metrics export
- **Integration** - End-to-end tests with the analysis pipeline

**Total Test Count**: 94 tests (as of last run)

---

## Test Structure

### Test Files

The test suite is organized into focused test files, each covering a specific module or feature:

| Test File | Module Tested | Description | Test Count |
|-----------|--------------|-------------|------------|
| `test_scoring.py` | `advisor.scoring` | Energy efficiency scoring algorithms (throughput-per-watt, pixels-per-joule) | 11 tests |
| `test_recommender.py` | `advisor.recommender` | Configuration ranking and recommendation logic | 11 tests |
| `test_modeling.py` | `advisor.modeling` | Simple power prediction model (PowerPredictor v0.1) | 14 tests |
| `test_multivariate_modeling.py` | `advisor.modeling` | Advanced multivariate power prediction (MultivariatePredictor v0.2) | 22 tests |
| `test_output_ladder.py` | Output ladder support | Multi-resolution transcoding and pixel-based scoring | 18 tests |
| `test_results_exporter.py` | `results-exporter/` | Prometheus metrics export and feature extraction | 9 tests |
| `test_integration.py` | End-to-end | Integration tests with the analysis pipeline | 9 tests |

### Test Organization

Each test file follows a consistent structure:

```python
"""Tests for the advisor.module_name module."""

import pytest
from advisor.module_name import ClassName

class TestClassName:
    """Tests for ClassName class."""

    @pytest.fixture
    def sample_data(self):
        """Fixture providing sample test data."""
        return {...}

    def test_specific_functionality(self, sample_data):
        """Test a specific aspect of the functionality."""
        # Arrange
        instance = ClassName()

        # Act
        result = instance.method(sample_data)

        # Assert
        assert result == expected_value
```

---

## Installation

### Prerequisites

- **Python 3.11+** (Python 3.12 recommended)
- **pip** for package management

### Install Test Dependencies

All test dependencies are defined in `requirements-dev.txt`:

```bash
# From the repository root
pip install -r requirements-dev.txt
```

This installs:
- `pytest>=8.0.0` - Testing framework
- `ruff>=0.6.0` - Linting and formatting
- `pre-commit>=3.6.0` - Git hooks for code quality
- All dependencies from `requirements.txt` (scikit-learn, requests, etc.)

### Verify Installation

Check that pytest is installed correctly:

```bash
pytest --version
# Expected output: pytest 9.0.2 (or newer)
```

---

## Running Tests

### Quick Start

Run all tests from the repository root:

```bash
# Using pytest directly
pytest tests/

# Using pytest with verbose output
pytest tests/ -v

# Using the Makefile (shortcut)
make test
```

### Run Specific Test Files

Run tests from a single file:

```bash
# Test scoring module
pytest tests/test_scoring.py

# Test recommender module
pytest tests/test_recommender.py -v

# Test power prediction models
pytest tests/test_modeling.py
pytest tests/test_multivariate_modeling.py
```

### Run Specific Test Classes

Run all tests in a specific test class:

```bash
# Run only EnergyEfficiencyScorer tests
pytest tests/test_scoring.py::TestEnergyEfficiencyScorer

# Run only TranscodingRecommender tests
pytest tests/test_recommender.py::TestTranscodingRecommender -v

# Run only PowerPredictor tests
pytest tests/test_modeling.py::TestPowerPredictor
```

### Run Specific Test Methods

Run a single test method:

```bash
# Test specific scoring functionality
pytest tests/test_scoring.py::TestEnergyEfficiencyScorer::test_compute_score_single_stream

# Test recommendation ranking
pytest tests/test_recommender.py::TestTranscodingRecommender::test_analyze_and_rank

# Test multivariate prediction
pytest tests/test_multivariate_modeling.py::TestMultivariatePredictor::test_predict_with_confidence
```

### Run Tests Matching a Pattern

Use the `-k` flag to run tests matching a keyword:

```bash
# Run all tests related to "scoring"
pytest tests/ -k scoring

# Run all tests related to "prediction"
pytest tests/ -k predict

# Run all tests related to "ladder"
pytest tests/ -k ladder

# Run tests for multivariate models only
pytest tests/ -k multivariate
```

### Test Output Options

Control the verbosity and detail of test output:

```bash
# Quiet mode (minimal output)
pytest tests/ -q

# Verbose mode (detailed test names)
pytest tests/ -v

# Very verbose mode (full output including print statements)
pytest tests/ -vv

# Show local variables on failures
pytest tests/ -l

# Stop on first failure
pytest tests/ -x

# Show test durations (slowest 10)
pytest tests/ --durations=10
```

### Test Coverage

Run tests with coverage reporting:

```bash
# Run tests with coverage (requires pytest-cov)
pip install pytest-cov
pytest tests/ --cov=advisor --cov-report=term-missing

# Generate HTML coverage report
pytest tests/ --cov=advisor --cov-report=html
# Open htmlcov/index.html in browser
```

### Makefile Commands

The repository provides convenient Makefile targets:

```bash
# Run all tests (quiet mode)
make test

# Run linter checks
make lint

# Auto-format code
make format

# Run pre-commit hooks on all files
make pre-commit
```

---

## Test Coverage

### Energy Efficiency Scoring (`test_scoring.py`)

Tests for the `EnergyEfficiencyScorer` class:

-  Algorithm initialization (default and custom)
-  Invalid algorithm handling
-  Single stream score computation
-  Multi-stream score computation
-  GPU power inclusion in calculations
-  Baseline scenario handling
-  Missing power data handling
-  Zero power edge case
-  Bitrate parsing (various formats: k, M, numeric)
-  Stream count extraction from scenario names
-  Not-implemented placeholder methods

**Example Test Run**:
```bash
$ pytest tests/test_scoring.py -v
tests/test_scoring.py::TestEnergyEfficiencyScorer::test_initialization_default PASSED
tests/test_scoring.py::TestEnergyEfficiencyScorer::test_compute_score_single_stream PASSED
tests/test_scoring.py::TestEnergyEfficiencyScorer::test_compute_score_multi_stream PASSED
...
11 passed in 0.12s
```

### Recommendation Logic (`test_recommender.py`)

Tests for the `TranscodingRecommender` class:

-  Recommender initialization (default and custom scorer)
-  Scenario analysis and ranking
-  Rank sorting (highest efficiency first)
-  Input preservation (non-destructive operations)
-  Best configuration selection
-  Handling scenarios with no valid scores
-  Top-N configuration retrieval
-  Handling N exceeding available scenarios
-  Recommendation summary generation
-  Configuration comparison
-  Missing score handling

**Example Test Run**:
```bash
$ pytest tests/test_recommender.py -v
tests/test_recommender.py::TestTranscodingRecommender::test_analyze_and_rank PASSED
tests/test_recommender.py::TestTranscodingRecommender::test_get_best_configuration PASSED
...
11 passed in 0.08s
```

### Power Prediction - Simple Model (`test_modeling.py`)

Tests for the `PowerPredictor` class (v0.1 - univariate):

-  Predictor initialization
-  Linear regression training
-  Polynomial regression training (degree 2)
-  Power prediction for new stream counts
-  Prediction extrapolation beyond training range
-  Model information retrieval (untrained, linear, polynomial)
-  Small dataset fallback (< 4 samples)
-  Single datapoint handling
-  Duplicate stream count handling
-  Zero stream count prediction
-  Large stream count prediction

**Example Test Run**:
```bash
$ pytest tests/test_modeling.py -v
tests/test_modeling.py::TestPowerPredictor::test_initialization PASSED
tests/test_modeling.py::TestPowerPredictor::test_fit_linear PASSED
tests/test_modeling.py::TestPowerPredictor::test_fit_polynomial PASSED
...
14 passed in 0.18s
```

### Power Prediction - Advanced Model (`test_multivariate_modeling.py`)

Tests for the `MultivariatePredictor` class (v0.2 - multivariate ensemble):

-  Predictor initialization (default and custom parameters)
-  Feature extraction (stream count, bitrate, resolution, CPU, encoder, hardware)
-  Multi-resolution scenario feature extraction
-  Missing name handling
-  Model training for power, energy, and efficiency prediction
-  Insufficient data handling (< 3 samples)
-  No valid data handling
-  Power prediction for new configurations
-  Prediction with confidence intervals
-  Prediction before training error
-  Batch prediction
-  Model information retrieval
-  Model selection (Linear, Polynomial, RandomForest, GradientBoosting)
-  Model persistence (save/load)
-  Different encoder types (CPU vs GPU)
-  Cross-validation scores
-  Hardware ID tracking

**Example Test Run**:
```bash
$ pytest tests/test_multivariate_modeling.py -v
tests/test_multivariate_modeling.py::TestMultivariatePredictor::test_fit_power_prediction PASSED
tests/test_multivariate_modeling.py::TestMultivariatePredictor::test_predict_with_confidence PASSED
tests/test_multivariate_modeling.py::TestMultivariatePredictor::test_model_selection PASSED
...
22 passed in 0.45s
```

### Output Ladder Support (`test_output_ladder.py`)

Tests for multi-resolution transcoding scenarios:

-  Resolution parsing (1920x1080, 1280x720, etc.)
-  Output ladder extraction (single resolution)
-  Output ladder extraction (multi-resolution)
-  Unsorted outputs handling (automatic sorting)
-  No data scenarios
-  Total pixels computation (single resolution)
-  Total pixels computation (multi-resolution)
-  Missing duration handling
-  Pixels-per-joule scoring
-  Pixels-per-joule for multi-resolution
-  Missing energy data handling
-  Ranking by output ladder groups
-  Ranking within ladder groups
-  Best configuration per ladder
-  Output ladder field in results
-  Backward compatibility (scenarios without outputs)
-  Mixed scenarios (with and without outputs)
-  End-to-end pixels scoring
-  Algorithm selection impact on ranking

**Example Test Run**:
```bash
$ pytest tests/test_output_ladder.py -v
tests/test_output_ladder.py::TestOutputLadderScoring::test_pixels_per_joule_scoring PASSED
tests/test_output_ladder.py::TestOutputLadderRecommender::test_analyze_and_rank_by_ladder PASSED
...
18 passed in 0.14s
```

### Results Exporter (`test_results_exporter.py`)

Tests for the Prometheus metrics exporter:

-  Stream count extraction from scenario names
-  Encoder type detection (GPU vs CPU)
-  Output ladder ID generation (single resolution)
-  Output ladder ID generation (multi-resolution)
-  Resolution parsing
-  Efficiency score computation (pixels-per-joule)
-  Efficiency score for multi-resolution
-  Scenario labels including new fields

**Example Test Run**:
```bash
$ pytest tests/test_results_exporter.py -v
tests/test_results_exporter.py::TestResultsExporterEnhancements::test_extract_stream_count PASSED
tests/test_results_exporter.py::TestResultsExporterEnhancements::test_compute_efficiency_score_pixels_per_joule PASSED
...
9 passed in 0.11s
```

### Integration Tests (`test_integration.py`)

End-to-end tests verifying the advisor works with the analysis pipeline:

-  Sample test results creation
-  Integration with `analyze_results.py`
-  Prometheus metric queries (mocked)
-  Energy efficiency recommendation generation
-  CSV export with efficiency scores
-  Ranking and best configuration selection
-  Complete analysis workflow

**Example Test Run**:
```bash
$ pytest tests/test_integration.py -v
tests/test_integration.py::TestAnalyzeResultsIntegration::test_analyzer_with_advisor PASSED
...
9 passed in 0.22s
```

---

## Writing Tests

### Test Naming Conventions

Follow these naming conventions for consistency:

- **Test files**: `test_<module_name>.py`
- **Test classes**: `Test<ClassName>`
- **Test methods**: `test_<functionality_being_tested>`

Example:
```python
# File: tests/test_scoring.py
class TestEnergyEfficiencyScorer:
    def test_compute_score_single_stream(self):
        ...

    def test_compute_score_multi_stream(self):
        ...
```

### Using Fixtures

Use pytest fixtures for reusable test data:

```python
@pytest.fixture
def sample_scenarios(self):
    """Sample scenario data for testing."""
    return [
        {
            'name': '1 Mbps Stream',
            'bitrate': '1000k',
            'power': {'mean_watts': 50.0},
            'resolution': '1280x720',
            'fps': 30
        },
        # More scenarios...
    ]

def test_analyze_and_rank(self, sample_scenarios):
    """Test scenario ranking."""
    recommender = TranscodingRecommender()
    ranked = recommender.analyze_and_rank(sample_scenarios)
    assert len(ranked) == len(sample_scenarios)
```

### Test Structure (AAA Pattern)

Follow the Arrange-Act-Assert pattern:

```python
def test_compute_score_single_stream(self):
    """Test score computation for single stream scenario."""
    # Arrange
    scorer = EnergyEfficiencyScorer()
    scenario = {
        'name': '1 Mbps Stream',
        'bitrate': '1000k',
        'power': {'mean_watts': 50.0}
    }

    # Act
    score = scorer.compute_score(scenario)

    # Assert
    assert score is not None
    assert pytest.approx(score, rel=1e-3) == 0.02
```

### Testing Exceptions

Test error handling with `pytest.raises`:

```python
def test_initialization_invalid_algorithm(self):
    """Test initialization with invalid algorithm raises error."""
    with pytest.raises(ValueError, match="Unsupported algorithm"):
        EnergyEfficiencyScorer(algorithm='invalid_algo')
```

### Adding New Tests

When adding new tests:

1. **Choose the appropriate test file** based on the module you're testing
2. **Add tests to existing test classes** when extending existing functionality
3. **Create new test classes** for new modules or major features
4. **Use descriptive test names** that explain what is being tested
5. **Include docstrings** explaining the test's purpose
6. **Follow existing patterns** in the test suite

Example:
```python
def test_new_functionality(self, sample_scenarios):
    """Test the new functionality added in PR #123.

    This test verifies that the new feature correctly handles
    multi-stream scenarios with GPU acceleration.
    """
    # Your test implementation
    ...
```

---

## Test Configuration

### pytest Configuration

Test configuration is defined in `pyproject.toml`:

```toml
[tool.pytest.ini_options]
testpaths = ["tests"]
```

This tells pytest to automatically discover tests in the `tests/` directory.

### Additional Configuration Options

You can add more pytest options to `pyproject.toml` if needed:

```toml
[tool.pytest.ini_options]
testpaths = ["tests"]
python_files = ["test_*.py"]
python_classes = ["Test*"]
python_functions = ["test_*"]
addopts = "-ra -q"  # Show summary of all test outcomes, quiet mode
```

---

## Continuous Integration

### Pre-commit Hooks

The repository uses pre-commit hooks to ensure code quality:

```bash
# Install pre-commit hooks
pre-commit install

# Run pre-commit on all files manually
make pre-commit
```

The pre-commit configuration runs:
- **ruff** for linting and formatting
- Automatically formats code before commits

### Running Tests in CI

Tests should be run in CI/CD pipelines before merging code:

```bash
# Typical CI workflow
pip install -r requirements-dev.txt
make lint    # Check code style
make test    # Run all tests
```

### Test Stability

All tests are designed to be:
-  **Deterministic** - Same input always produces same output
-  **Isolated** - Tests don't depend on each other
-  **Fast** - Complete test suite runs in < 2 seconds
-  **Reliable** - No flaky tests or random failures

---

## Troubleshooting

### Common Issues

**Issue**: `ImportError: No module named 'advisor'`
**Solution**: Run tests from the repository root, not from inside the `tests/` directory:
```bash
cd /path/to/ffmpeg-rtmp
pytest tests/
```

**Issue**: `ModuleNotFoundError: No module named 'pytest'`
**Solution**: Install dev dependencies:
```bash
pip install -r requirements-dev.txt
```

**Issue**: Tests fail with `scikit-learn` import errors
**Solution**: Install runtime dependencies:
```bash
pip install -r requirements.txt
```

**Issue**: Test discovery finds 0 tests
**Solution**: Ensure you're running pytest from the repository root and test files start with `test_`:
```bash
cd /path/to/ffmpeg-rtmp
pytest tests/ -v
```

---

## Resources

### Related Documentation

- **Main README**: `../README.md` - Overall project documentation
- **Power Prediction Model**: `../docs/power-prediction-model.md` - Detailed model documentation
- **Makefile**: `../Makefile` - Available make commands

### Pytest Documentation

- [Pytest Documentation](https://docs.pytest.org/)
- [Pytest Fixtures](https://docs.pytest.org/en/stable/fixture.html)
- [Pytest Parametrize](https://docs.pytest.org/en/stable/parametrize.html)

### Testing Best Practices

- [Test-Driven Development (TDD)](https://en.wikipedia.org/wiki/Test-driven_development)
- [AAA Pattern](https://medium.com/@pjbgf/title-testing-code-ocd-and-the-aaa-pattern-df453975ab80)
- [Python Testing Best Practices](https://docs.python-guide.org/writing/tests/)

---

## Summary

This test suite provides comprehensive coverage of the FFmpeg-RTMP energy monitoring and advisory system. With 94 tests covering scoring, recommendation, prediction, and integration, it ensures the system delivers accurate energy efficiency recommendations.

**Quick Reference**:
```bash
# Run all tests
pytest tests/ -v

# Run specific test file
pytest tests/test_scoring.py

# Run tests matching a keyword
pytest tests/ -k scoring

# Run with coverage
pytest tests/ --cov=advisor

# Using Makefile
make test
```

For questions or issues with tests, please refer to the main README or open an issue on GitHub.
