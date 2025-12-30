# Master Node Components

This directory contains all components that run on the **master node** in a distributed deployment.

## Purpose

The master node is responsible for:
- **Job orchestration**: Queuing and dispatching transcoding jobs to workers
- **Node registry**: Tracking available worker nodes and their capabilities
- **Results aggregation**: Collecting and storing results from all workers
- **Metrics visualization**: Running Grafana dashboards for monitoring
- **Alerting**: Running Alertmanager for threshold-based alerts

## Directory Structure

```
master/
├── cmd/                       # Master binary entry point
│   └── master/                # Go application main package
├── exporters/                 # Master-side metrics exporters
│   ├── results/               # Test results exporter (aggregates worker data)
│   ├── qoe/                   # Quality of Experience metrics
│   ├── cost/                  # Cost calculation metrics
│   └── health_checker/        # Exporter health monitoring
├── monitoring/                # Monitoring stack configuration
│   ├── grafana/               # Grafana dashboards and provisioning
│   ├── alertmanager/          # Alert routing configuration
│   ├── victoriametrics.yml    # Metrics scrape configuration
│   └── alert-rules.yml        # Alert threshold rules
└── deployment/                # Master deployment configs
    └── ffmpeg-master.service  # Systemd service file
```

## What Runs on Master

### 1. Master HTTP Service
- **Binary**: `cmd/master/main.go`
- **Port**: 8080 (default, configurable)
- **Purpose**: REST API for job queue, node registry, results collection

**Key endpoints**:
- `POST /nodes/register` - Register worker nodes
- `POST /jobs` - Submit new transcoding jobs
- `GET /jobs/next?node_id=X` - Workers poll for jobs
- `POST /results` - Workers submit results

### 2. VictoriaMetrics (TSDB)
- **Port**: 8428
- **Purpose**: Time-series database for metrics storage
- **Retention**: 30 days by default
- **Config**: `monitoring/victoriametrics.yml`

### 3. Grafana Dashboards
- **Port**: 3000
- **Purpose**: Visualize metrics from all workers
- **Config**: `monitoring/grafana/`

### 4. Alertmanager
- **Port**: 9093
- **Purpose**: Route alerts based on thresholds
- **Config**: `monitoring/alertmanager/`

### 5. Master-Side Exporters

#### Results Exporter (Port 9502)
- Exposes aggregated test results from all workers
- Metrics: power delta, energy delta, efficiency scores

#### QoE Exporter (Port 9503)
- Calculates Quality of Experience metrics
- Metrics: efficiency scores, pixels per joule, throughput per watt

#### Cost Exporter (Port 9504)
- Calculates cost metrics (energy + compute)
- Configurable pricing per region

#### Health Checker (Port 9600)
- Monitors health of all exporters
- Exposes exporter status metrics

## Building the Master

```bash
# From repository root
make build-master

# Or directly with Go
cd /home/runner/work/ffmpeg-rtmp/ffmpeg-rtmp
go build -o bin/master ./cmd/master
```

## Running the Master

### Development Mode (HTTP)
```bash
./bin/master --port 8080 --tls=false
```

### Production Mode (HTTPS with TLS)
```bash
# Set API key for authentication
export MASTER_API_KEY=$(openssl rand -base64 32)

# Start with auto-generated TLS certificate
./bin/master --port 8080 --tls=true --generate-cert

# Or provide your own certificate
./bin/master --port 8080 --tls=true --cert /path/to/cert.pem --key /path/to/key.pem
```

### With Systemd
```bash
# Copy service file
sudo cp deployment/ffmpeg-master.service /etc/systemd/system/

# Start and enable
sudo systemctl enable ffmpeg-master
sudo systemctl start ffmpeg-master

# Check status
sudo systemctl status ffmpeg-master
```

## Deploying Monitoring Stack

The monitoring stack (VictoriaMetrics + Grafana) can run via Docker Compose:

```bash
# From repository root
make vm-up-build

# Or manually
docker compose up -d victoriametrics grafana
```

Access:
- Grafana: http://localhost:3000 (admin/admin)
- VictoriaMetrics: http://localhost:8428

## Master-Only Deployment

For a master-only deployment (no local transcoding):

```bash
# 1. Build master binary
make build-master

# 2. Start master service
./bin/master --port 8080

# 3. Start monitoring stack
make vm-up-build

# 4. Master is ready to accept worker registrations
curl http://localhost:8080/nodes
```

## Hardware Requirements

**Minimum**:
- 2 CPU cores
- 4 GB RAM
- 20 GB disk (for VictoriaMetrics storage)

**Recommended**:
- 4+ CPU cores
- 8+ GB RAM
- 50+ GB SSD (for metrics retention)

## Network Requirements

**Inbound ports**:
- 8080: Master HTTP API (or 443 if using reverse proxy)
- 3000: Grafana (optional, can use reverse proxy)
- 8428: VictoriaMetrics (optional, can be internal-only)

**Firewall** (example with ufw):
```bash
sudo ufw allow 8080/tcp comment 'Master API'
sudo ufw allow 3000/tcp comment 'Grafana'
sudo ufw allow 8428/tcp comment 'VictoriaMetrics'
```

## Related Documentation

- [FOLDER_ORGANIZATION.md](../FOLDER_ORGANIZATION.md) - Overall project structure
- [../deployment/README.md](../deployment/README.md) - Production deployment guide
- [../docs/DEPLOYMENT_MODES.md](../docs/DEPLOYMENT_MODES.md) - Deployment modes comparison
- [../docs/distributed_architecture_v1.md](../docs/distributed_architecture_v1.md) - Architecture details

## Troubleshooting

### Master won't start
```bash
# Check if port is in use
sudo lsof -i :8080

# Check logs
sudo journalctl -u ffmpeg-master -n 50

# Verify binary
./bin/master --help
```

### Workers can't connect
```bash
# Test master health endpoint
curl http://localhost:8080/health

# Check firewall
sudo ufw status

# Check master logs for rejected connections
sudo journalctl -u ffmpeg-master -f
```

### VictoriaMetrics not scraping
```bash
# Check targets
curl http://localhost:8428/targets

# Check victoriametrics.yml for correct ports
cat monitoring/victoriametrics.yml

# Restart VictoriaMetrics
docker compose restart victoriametrics
```

## Production Considerations

1. **Use HTTPS**: Enable TLS for master API
2. **Set API key**: Require authentication for job submission
3. **Use reverse proxy**: Put nginx/Caddy in front of master
4. **Monitor disk**: VictoriaMetrics needs disk space for metrics
5. **Backup database**: If using SQLite persistence, backup master.db
6. **Set up alerts**: Configure Alertmanager for critical thresholds

## Support

For issues specific to master node components, please include:
- Master logs: `journalctl -u ffmpeg-master`
- Master version: `./bin/master --version`
- Node list: `curl http://localhost:8080/nodes`
- Job queue: `curl http://localhost:8080/jobs`
