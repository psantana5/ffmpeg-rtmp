# Contributing to FFmpeg RTMP Power Monitoring

Thank you for considering contributing to this project! We welcome contributions from the community.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When creating a bug report, include as many details as possible using the bug report template.

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, use the feature request template and provide:

- A clear and descriptive title
- A detailed description of the proposed functionality
- Explain why this enhancement would be useful
- List any alternatives you've considered

### Pull Requests

1. Fork the repository and create your branch from `main`
2. Make your changes following the style guidelines
3. Add tests for new functionality
4. Ensure all tests pass
5. Update documentation as needed
6. Submit a pull request using the template

## Development Setup

### Prerequisites

- Python 3.10 or higher
- Docker and Docker Compose
- Git
- FFmpeg (for running tests)

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
```

2. Create a virtual environment:
```bash
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
```

3. Install dependencies:
```bash
pip install -r requirements.txt
pip install -r requirements-dev.txt
```

4. Install pre-commit hooks:
```bash
pip install pre-commit
pre-commit install
```

5. Start the development stack:
```bash
make up-build
```

## Style Guidelines

### Python Code Style

We use Ruff for linting and formatting. Please ensure your code passes all checks:

```bash
# Run linting
make lint

# Run formatting
make format
```

Key style points:
- Follow PEP 8 guidelines
- Maximum line length: 100 characters
- Use type hints where appropriate
- Write descriptive docstrings for functions and classes
- Keep functions focused and single-purpose

### Code Quality

- Write clear, self-documenting code
- Add comments for complex logic
- Avoid code duplication
- Handle errors appropriately
- Use meaningful variable and function names

### Git Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters
- Reference issues and pull requests when relevant
- Consider using conventional commits format:
  - `feat:` - New feature
  - `fix:` - Bug fix
  - `docs:` - Documentation changes
  - `style:` - Code style changes (formatting, etc.)
  - `refactor:` - Code refactoring
  - `test:` - Adding or updating tests
  - `chore:` - Maintenance tasks

Example:
```
feat: add support for H.265 codec benchmarking

- Implement H.265 encoder options in test runner
- Add HEVC metrics to exporters
- Update dashboards with codec comparison panels

Closes #123
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
python -m pytest --cov=. --cov-report=term

# Run specific test file
python -m pytest tests/test_cost.py

# Run tests with verbose output
python -m pytest -v
```

### Writing Tests

- Place tests in the `tests/` directory
- Name test files with `test_` prefix
- Name test functions with `test_` prefix
- Use descriptive test names that explain what is being tested
- Follow the Arrange-Act-Assert pattern
- Mock external dependencies
- Aim for high test coverage on new code

Example:
```python
def test_cost_calculation_with_custom_rates():
    # Arrange
    power_watts = 100
    duration_seconds = 3600
    rate_per_kwh = 0.15
    
    # Act
    result = calculate_cost(power_watts, duration_seconds, rate_per_kwh)
    
    # Assert
    assert result == 0.015
```

## Documentation

### Code Documentation

- Add docstrings to all public functions, classes, and modules
- Use Google-style or NumPy-style docstrings
- Include parameter types and return types
- Document exceptions that can be raised
- Provide usage examples for complex functionality

Example:
```python
def calculate_efficiency_score(power: float, quality: float, bitrate: int) -> float:
    """Calculate the energy efficiency score for a transcoding configuration.
    
    Args:
        power: Power consumption in watts
        quality: Video quality score (0-100)
        bitrate: Target bitrate in bits per second
        
    Returns:
        Efficiency score normalized to 0-100 range
        
    Raises:
        ValueError: If any parameter is negative or zero
        
    Example:
        >>> calculate_efficiency_score(75.5, 85.0, 2000000)
        78.3
    """
```

### Project Documentation

- Update README.md for user-facing changes
- Update relevant documentation in `docs/` directory
- Include examples and usage instructions
- Add screenshots for UI/dashboard changes
- Keep documentation concise and clear

## Project Structure

```
ffmpeg-rtmp/
├── .github/              # GitHub workflows and templates
├── advisor/              # ML models and energy advisor
├── alertmanager/         # Alert configuration
├── docs/                 # Documentation
├── grafana/             # Grafana dashboards and provisioning
├── scripts/             # Test runners and analysis scripts
├── src/exporters/       # Prometheus exporters
├── tests/               # Test suite
├── docker-compose.yml   # Stack configuration
├── prometheus.yml       # Prometheus configuration
└── requirements*.txt    # Python dependencies
```

## Adding New Exporters

1. Create a new directory under `src/exporters/`
2. Add the exporter Python script
3. Create a Dockerfile
4. Add service to `docker-compose.yml`
5. Add scrape config to `prometheus.yml`
6. Create or update Grafana dashboard
7. Add tests in `tests/`
8. Update documentation

## Release Process

Releases are managed by project maintainers:

1. Update version numbers
2. Update CHANGELOG.md
3. Create release branch
4. Run full test suite
5. Create GitHub release with release notes
6. Tag the release
7. Build and push Docker images

## Questions?

- Check the [documentation](docs/)
- Look at existing issues and pull requests
- Open a new issue with the question label

## Recognition

Contributors will be recognized in:
- GitHub contributors page
- Release notes
- Project documentation (for significant contributions)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to make this project better!
