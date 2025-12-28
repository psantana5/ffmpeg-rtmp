# Contributing to FFmpeg RTMP Power Monitoring

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Project Structure](#project-structure)

## Code of Conduct

This project adheres to a code of conduct that we expect all contributors to follow. Be respectful, inclusive, and constructive in all interactions.

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Python 3.11+
- FFmpeg installed on the host
- Intel CPU with RAPL support (for power monitoring)
- Optional: NVIDIA GPU with nvidia-container-toolkit (for GPU monitoring)

### Quick Start

1. Fork and clone the repository:
   ```bash
   git clone https://github.com/YOUR-USERNAME/ffmpeg-rtmp.git
   cd ffmpeg-rtmp
   ```

2. Install development dependencies:
   ```bash
   pip install -r requirements-dev.txt
   ```

3. Install pre-commit hooks:
   ```bash
   pre-commit install
   ```

4. Start the monitoring stack:
   ```bash
   make up-build
   ```

## Development Setup

### Installing Dependencies

- **Runtime dependencies**: `pip install -r requirements.txt`
- **Development dependencies**: `pip install -r requirements-dev.txt`
- **Plot generation**: `pip install -r requirements-plots.txt` (optional)

### Pre-commit Hooks

We use pre-commit hooks to ensure code quality:

```bash
# Install hooks
pre-commit install

# Run manually on all files
make pre-commit
```

Hooks include:
- `ruff` for linting and formatting
- Trailing whitespace removal
- End-of-file fixes
- YAML/JSON validation

## Making Changes

### Branch Naming Convention

Use descriptive branch names:
- `feature/add-thermal-monitoring`
- `fix/outlier-detection-edge-case`
- `docs/improve-readme-examples`
- `refactor/simplify-model-training`

### Commit Messages

Follow conventional commit format:

```
<type>: <short summary>

<longer description if needed>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `style`: Code style changes (formatting, etc.)
- `chore`: Maintenance tasks

Example:
```
feat: add cross-validation for power prediction model

Implements k-fold cross-validation to assess model generalization.
Uses 3-fold CV for small datasets (<10 samples) and 5-fold for larger.

Closes #42
```

### Code Style

We use `ruff` for both linting and formatting:

```bash
# Check for issues
make lint

# Auto-format code
make format
```

Key style guidelines:
- Line length: 100 characters (configured in `pyproject.toml`)
- Use type hints for function signatures
- Document public functions with docstrings
- Prefer descriptive variable names over comments

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run specific test file
python3 -m pytest tests/test_modeling.py -v

# Run specific test
python3 -m pytest tests/test_modeling.py::TestPowerPredictor::test_fit_linear_regression -v

# Run with coverage
python3 -m pytest --cov=advisor --cov-report=html
```

### Writing Tests

- Place tests in the `tests/` directory
- Name test files `test_*.py`
- Use descriptive test function names: `test_<what>_<condition>_<expected>`
- Group related tests in classes
- Use pytest fixtures for shared setup

Example:
```python
class TestOutlierDetection:
    def test_filter_outliers_iqr_with_outliers(self):
        """Test that outliers are correctly identified and removed"""
        values = [98.0, 99.0, 100.0, 101.0, 102.0, 200.0, 5.0]
        filtered, removed = filter_outliers_iqr(values)
        
        assert removed == 2
        assert 200.0 not in filtered
        assert 5.0 not in filtered
```

### Test Coverage

Aim for:
- **80%+ overall coverage**
- **100% coverage for critical paths** (modeling, scoring, recommendations)
- Edge cases and error handling

## Pull Request Process

### Before Submitting

1. **Run the test suite**: `make test`
2. **Run the linter**: `make lint`
3. **Update documentation** if adding features
4. **Add tests** for new functionality
5. **Verify pre-commit hooks pass**

### Submitting a PR

1. Push your branch to your fork
2. Open a PR against the `main` branch
3. Fill out the PR template (if provided)
4. Link related issues: `Closes #123`

### PR Checklist

- [ ] Tests pass locally
- [ ] Linter passes
- [ ] Documentation updated
- [ ] Changelog entry added (if significant change)
- [ ] Backward compatibility considered
- [ ] Performance impact assessed

### Review Process

- Maintainers will review your PR
- Address feedback with additional commits
- Once approved, maintainers will merge

## Coding Standards

### Python Code

```python
# Good: Type hints and docstrings
def predict_power(streams: int) -> Optional[float]:
    """
    Predict power consumption for a given stream count.
    
    Args:
        streams: Number of concurrent streams
        
    Returns:
        Predicted power in watts, or None if model not trained
    """
    ...

# Bad: No type hints or docstring
def predict_power(streams):
    ...
```

### Error Handling

```python
# Good: Specific exceptions with context
try:
    result = process_data(input)
except ValueError as e:
    logger.error(f"Invalid input format: {e}")
    return None

# Bad: Bare except
try:
    result = process_data(input)
except:
    return None
```

### Logging

Use appropriate log levels:
- `logger.debug()`: Detailed diagnostic information
- `logger.info()`: General informational messages
- `logger.warning()`: Warning messages (recoverable issues)
- `logger.error()`: Error messages (failures)

## Project Structure

```
ffmpeg-rtmp/
├── advisor/              # Energy efficiency analysis modules
│   ├── modeling.py       # Power prediction models
│   ├── scoring.py        # Efficiency scoring algorithms
│   └── recommender.py    # Configuration recommendations
├── rapl-exporter/        # Intel RAPL power metrics exporter
├── results-exporter/     # Test results Prometheus exporter
├── docker-stats-exporter/  # Docker overhead metrics
├── grafana/              # Grafana dashboards and provisioning
├── tests/                # Test suite
├── analyze_results.py    # Analysis and reporting tool
├── run_tests.py          # Test orchestration
└── docs/                 # Documentation

```

### Key Modules

- **advisor/**: Core energy efficiency logic
- **analyze_results.py**: Entry point for analysis
- **run_tests.py**: Entry point for test execution
- **tests/**: Comprehensive test coverage

## Areas for Contribution

We welcome contributions in these areas:

### High Priority

- **Data quality improvements**: Outlier detection, noise reduction
- **Model enhancements**: Feature engineering, confidence intervals
- **Observability**: Grafana dashboards, Prometheus metrics
- **Documentation**: Examples, tutorials, API docs
- **Test coverage**: Edge cases, integration tests

### Medium Priority

- **GPU support**: NVIDIA/AMD GPU power tracking
- **Quality metrics**: VMAF/PSNR integration
- **CLI enhancements**: Interactive mode, JSON output
- **Performance**: Optimization, profiling

### Nice to Have

- **Model drift detection**: Automatic retraining triggers
- **Hardware normalization**: Cross-platform comparisons
- **Cloud integration**: AWS, Azure, GCP cost estimation

## Questions or Need Help?

- **Issues**: Open an issue for bugs or feature requests
- **Discussions**: Use GitHub Discussions for questions
- **Email**: Contact maintainers for sensitive issues

## License

By contributing, you agree that your contributions will be licensed under the same license as the project.

---

Thank you for contributing! 🚀
