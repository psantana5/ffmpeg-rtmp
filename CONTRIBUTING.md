# Contributing to FFmpeg-RTMP

Thank you for your interest in contributing to this reference system! Contributions that improve documentation, demonstrate additional patterns, or validate existing implementations are welcome.

## About This Project

FFmpeg-RTMP is a **production-validated reference system** designed for research and education. The primary goal is to document distributed systems patterns and operational tradeoffs, not to build a commercial platform. Contributions should align with this educational mission.

## How to Contribute

### Documentation Improvements

Documentation is the most valuable contribution to a reference system:

- Clarify design decisions and tradeoffs
- Add examples demonstrating specific patterns
- Document failure modes or edge cases you've observed
- Improve explanations of architectural choices
- Add references to related research or implementations

### Implementation Validation

Help verify that code matches documentation:

- Test documented behaviors under different conditions
- Validate performance characteristics
- Document observed failure modes
- Contribute test cases demonstrating patterns
- Report discrepancies between docs and implementation

### Pattern Demonstrations

Add examples showing specific distributed systems patterns:

- Additional failure recovery scenarios
- Resource isolation techniques
- Retry boundary examples
- State machine transitions
- Observability patterns

### Bug Reports

Report implementation bugs or documentation inaccuracies:

- Check existing issues first to avoid duplicates
- Provide minimal reproduction steps
- Include relevant logs, metrics, or traces
- Explain expected vs actual behavior
- Reference documentation if applicable

## What We're NOT Looking For

To maintain focus on the educational mission:

- ❌ Features for commercial use cases
- ❌ General-purpose abstractions or frameworks
- ❌ Support for every possible deployment scenario
- ❌ Performance optimizations without documented tradeoffs
- ❌ Features that obscure the underlying patterns

## Development Setup

### Prerequisites

- Go 1.24+ (for building master/worker binaries)
- Python 3.10+ (optional, for analysis scripts)
- Docker and Docker Compose (for local development)
- Git
- FFmpeg (for testing)

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
```

2. Build components:
```bash
make build-distributed  # Build all Go binaries
```

3. Run local stack:
```bash
./scripts/run_local_stack.sh
```

## Contribution Guidelines

### Code Style

**Go code**:
- Follow standard Go conventions (`gofmt`, `go vet`)
- Use meaningful variable names
- Document exported functions and types
- Keep functions focused and testable

**Python code** (analysis scripts):
- Follow PEP 8 guidelines
- Maximum line length: 100 characters
- Use type hints where appropriate
- Write descriptive docstrings

```bash
# Lint Go code
make lint

# Format Python code
make format
```

### Git Commit Messages

- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit first line to 72 characters
- Reference issues when relevant
- Use conventional commits format:
  - `feat:` - New feature or pattern demonstration
  - `fix:` - Bug fix or correction
  - `docs:` - Documentation improvements
  - `test:` - Test additions or updates
  - `refactor:` - Code restructuring without behavior change

Example:
```
docs: clarify retry boundary semantics in failure recovery

- Add explicit examples of transient vs terminal errors
- Document FFmpeg failure handling in detail
- Update dashboards with codec comparison panels

Closes #123
```

## Testing

### Running Tests

### Testing

```bash
# Run all Go tests
make test

# Run with race detector
go test -race ./...

# Run specific package tests
go test ./shared/pkg/scheduler/...

# View test coverage
go test -cover ./...
```

### Writing Tests

- Place tests alongside code (`*_test.go` files)
- Focus on testing observable behavior and invariants
- Test failure modes and edge cases
- Document what pattern or guarantee the test validates
- Use table-driven tests for multiple scenarios

Example:
```go
func TestJobAssignment_PreventsDuplicates(t *testing.T) {
    // Tests that FOR UPDATE locking prevents race conditions
    // Pattern: Row-level locking for distributed coordination
    
    // Test implementation...
}
```

## Documentation Standards

### Code Documentation

**Go code**:
- Document exported functions, types, and packages
- Explain *why* not just *what* (rationale for design choices)
- Reference related patterns or papers when applicable
- Document invariants and failure modes

Example:
```go
// AssignJobToWorker assigns a queued job to a worker using row-level locking.
// This prevents race conditions when multiple workers poll simultaneously.
// 
// Pattern: Optimistic concurrency control with FOR UPDATE
// Failure mode: Returns ErrNoJobsAvailable if all jobs assigned
//
// Invariant: A job can only be assigned to one worker at a time
func AssignJobToWorker(ctx context.Context, workerID string) (*Job, error) {
    // Implementation...
}
```

### Project Documentation

- Update README.md for architectural changes
- Document design decisions and tradeoffs in detail
- Add examples demonstrating patterns
- Include failure mode analysis
- Reference related research or implementations
- Keep documentation honest about limitations

## Project Structure

```
ffmpeg-rtmp/
├── master/              # Master node orchestration
├── worker/              # Worker node execution
├── shared/              # Common libraries (FSM, retry, DB)
├── deployment/          # Systemd configs, deployment scripts
├── docs/                # Architecture and design documentation
├── scripts/             # Test runners and analysis tools
├── tests/               # Integration tests
└── cmd/                 # CLI tools
```

## Questions?

For questions about:
- **Design decisions**: See [ARCHITECTURE.md](docs/ARCHITECTURE.md) and [CODE_VERIFICATION_REPORT.md](CODE_VERIFICATION_REPORT.md)
- **Implementation details**: Read the code - it's documented inline
- **Patterns demonstrated**: Check [docs/](docs/) directory
- **Contributing**: Open an issue for discussion before starting major work

## License

By contributing, you agree that your contributions will be licensed under the same MIT License that covers the project.
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
