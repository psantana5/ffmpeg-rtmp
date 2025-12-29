# FFmpeg RTMP Power Monitoring

A comprehensive streaming test and power monitoring stack for analyzing energy consumption during video transcoding. Perfect for optimizing FFmpeg configurations for energy efficiency.

## Quick Start

### Prerequisites

- Docker + Docker Compose
- Python 3.11+
- FFmpeg (for running tests)
- Intel CPU with RAPL support (for power monitoring)

### Start the Stack

```bash
# Build and start all services
make up-build

# Or manually
docker compose up -d --build
```

### Access the Dashboards

- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **Alertmanager**: http://localhost:9093

### Run Your First Test

```bash
# Run a simple streaming test
python3 scripts/run_tests.py single --name "test1" --bitrate 2000k --duration 60

# Analyze the results
python3 scripts/analyze_results.py
```

## What This Project Does

This project helps you:

1. **Run FFmpeg streaming tests** with various configurations (bitrate, resolution, codec)
2. **Monitor power consumption** in real-time using Intel RAPL
3. **Collect system metrics** (CPU, memory, network, Docker overhead)
4. **Analyze energy efficiency** and get recommendations for optimal transcoding settings
5. **Visualize results** in Grafana dashboards
6. **Set up alerts** for power thresholds

## Architecture

The stack includes:

- **Nginx RTMP**: Streaming server for RTMP ingest
- **Prometheus**: Metrics collection and storage
- **Grafana**: Visualization dashboards
- **Custom Exporters**: Power (RAPL), Docker stats, QoE metrics, Cost analysis
- **Energy Advisor**: ML-based recommendations for optimal configurations

## Documentation

Detailed documentation is organized by topic:

- **[Getting Started Guide](docs/getting-started.md)** - Complete setup and first steps
- **[Running Tests](scripts/README.md)** - How to run different test scenarios
- **[Exporters](src/exporters/README.md)** - Understanding the metrics collectors
- **[Energy Advisor](advisor/README.md)** - ML models and efficiency scoring
- **[Architecture](docs/architecture.md)** - System design and data flow
- **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions

## Common Commands

```bash
# Stack management
make up-build          # Start stack with rebuild
make down              # Stop stack
make ps                # Show container status
make logs SERVICE=prometheus  # View logs

# Testing
make test-single       # Run single stream test
make test-batch        # Run batch test matrix
make analyze           # Analyze latest results

# Development
make lint              # Run code linting
make format            # Format code
make test              # Run test suite
```

## Example Use Cases

### Find the Most Energy-Efficient Bitrate

```bash
python3 scripts/run_tests.py batch --file batch_stress_matrix.json
python3 scripts/analyze_results.py
```

The analyzer will rank all configurations by energy efficiency and recommend the best settings for your hardware.

### Compare H.264 vs H.265 Power Consumption

Create a batch configuration testing both codecs at the same bitrates, run the tests, and compare results in Grafana.

### Monitor Production Streaming Power Usage

Set up the stack on your streaming server and configure Prometheus alerts to notify you when power consumption exceeds thresholds.

## Contributing

Contributions are welcome! See the detailed documentation for development guidelines.

## License

See [LICENSE](LICENSE) file for details.

## Quick Links

- [Full Documentation](docs/)
- [Test Runner Guide](scripts/README.md)
- [Exporter Documentation](src/exporters/README.md)
- [Energy Advisor](advisor/README.md)
