# Exporters Quick Reference

Quick guide for all Go-based exporters in the ffmpeg-rtmp project.

## Master Node Exporters (Go)

All master exporters are implemented in Go for high performance.

### Start All Master Exporters (Docker)

```bash
docker compose up -d results-exporter qoe-exporter cost-exporter exporter-health-checker
```

### Run Master Exporters Manually

```bash
# Results Exporter - Port 9502
RESULTS_EXPORTER_PORT=9502 RESULTS_DIR=./test_results ./bin/results_exporter

# QoE Exporter - Port 9503
QOE_EXPORTER_PORT=9503 RESULTS_DIR=./test_results ./bin/qoe_exporter

# Cost Exporter - Port 9504
COST_EXPORTER_PORT=9504 RESULTS_DIR=./test_results \
ENERGY_COST=0.0 CPU_COST=0.50 CURRENCY=USD REGION=us-east-1 \
./bin/cost_exporter

# Health Checker - Port 9600
HEALTH_CHECK_PORT=9600 ./bin/health_checker
```

### Build Master Exporters

```bash
go build -o bin/results_exporter ./master/exporters/results_go/
go build -o bin/qoe_exporter ./master/exporters/qoe_go/
go build -o bin/cost_exporter ./master/exporters/cost_go/
go build -o bin/health_checker ./master/exporters/health_checker_go/
```

---

## Worker Node Exporters (Go)

All worker exporters are implemented in Go.

### Start All Worker Exporters (Docker)

```bash
docker compose up -d cpu-exporter-go docker-stats-exporter ffmpeg-exporter
```

### Run Worker Exporters Manually

```bash
# CPU Exporter - Port 9510 (requires sudo for RAPL access)
sudo CPU_EXPORTER_PORT=9500 ./bin/cpu-exporter

# Docker Stats Exporter - Port 9501
DOCKER_STATS_PORT=9501 ./bin/docker-stats-exporter

# FFmpeg Exporter - Port 9506
FFMPEG_EXPORTER_PORT=9506 ./bin/ffmpeg-exporter

# GPU Exporter - Port 9511 (NVIDIA only)
GPU_EXPORTER_PORT=9505 ./bin/gpu-exporter
```

### Build Worker Exporters

```bash
go build -o bin/cpu-exporter ./worker/exporters/cpu_exporter/
go build -o bin/docker-stats-exporter ./worker/exporters/docker_stats_go/
go build -o bin/ffmpeg-exporter ./worker/exporters/ffmpeg_exporter/
go build -o bin/gpu-exporter ./worker/exporters/gpu_exporter/
```

---

## Port Reference

| Exporter | Port | Type | Description |
|----------|------|------|-------------|
| **Master Exporters** |
| results-exporter | 9502 | Go | Test results and scenario metrics |
| qoe-exporter | 9503 | Go | Quality of Experience (VMAF, PSNR, SSIM) |
| cost-exporter | 9504 | Go | Cost analysis (energy + compute) |
| exporter-health-checker | 9600 | Go | Health monitoring for all exporters |
| **Worker Exporters** |
| cpu-exporter-go | 9510 | Go | CPU power via Intel RAPL |
| docker-stats-exporter | 9501 | Go | Docker container metrics |
| ffmpeg-exporter | 9506 | Go | FFmpeg encoding statistics |
| gpu-exporter-go | 9511 | Go | NVIDIA GPU metrics |
| **External Exporters** |
| nginx-exporter | 9728 | - | NGINX RTMP statistics |
| node-exporter | 9100 | - | System-level metrics |
| cadvisor | 8080 | - | Container metrics |
| dcgm-exporter | 9400 | - | NVIDIA DCGM metrics |

---

## Health Checks

Test all exporters are responding:

```bash
# Master exporters
curl http://localhost:9502/health  # results
curl http://localhost:9503/health  # qoe
curl http://localhost:9504/health  # cost
curl http://localhost:9600/health  # health-checker

# Worker exporters
curl http://localhost:9500/health  # cpu (internal port)
curl http://localhost:9501/health  # docker-stats
curl http://localhost:9506/health  # ffmpeg
curl http://localhost:9505/health  # gpu (internal port)
```

---

## View Metrics

All exporters expose Prometheus-formatted metrics:

```bash
# Master exporters
curl http://localhost:9502/metrics
curl http://localhost:9503/metrics
curl http://localhost:9504/metrics
curl http://localhost:9600/metrics

# Worker exporters
curl http://localhost:9500/metrics
curl http://localhost:9501/metrics
curl http://localhost:9506/metrics
curl http://localhost:9505/metrics
```

---

## Environment Variables Reference

### Master Exporters

**Results Exporter**
- `RESULTS_EXPORTER_PORT` (default: 9502)
- `RESULTS_DIR` (default: /results)

**QoE Exporter**
- `QOE_EXPORTER_PORT` (default: 9503)
- `RESULTS_DIR` (default: /results)

**Cost Exporter**
- `COST_EXPORTER_PORT` (default: 9504)
- `RESULTS_DIR` (default: /results)
- `ENERGY_COST` (cost per kWh, e.g., 0.12)
- `CPU_COST` (cost per hour, e.g., 0.50)
- `CURRENCY` (default: USD)
- `REGION` (default: us-east-1)

**Health Checker**
- `HEALTH_CHECK_PORT` (default: 9600)

### Worker Exporters

**CPU Exporter**
- `CPU_EXPORTER_PORT` (default: 9500)

**Docker Stats Exporter**
- `DOCKER_STATS_PORT` (default: 9501)

**FFmpeg Exporter**
- `FFMPEG_EXPORTER_PORT` (default: 9506)

**GPU Exporter**
- `GPU_EXPORTER_PORT` (default: 9505)

---

## Troubleshooting

### Exporter Won't Start

1. Check port availability: `sudo netstat -tlnp | grep <port>`
2. Check logs: `docker compose logs <service-name>`
3. Verify binary built correctly: `ls -lh bin/`

### No Metrics

1. Test health endpoint: `curl http://localhost:<port>/health`
2. Check metrics endpoint: `curl http://localhost:<port>/metrics`
3. For master exporters: ensure test result files exist in RESULTS_DIR
4. For worker exporters: ensure required hardware/permissions available

### Permission Errors

**CPU Exporter**: Requires root or capabilities to read `/sys/class/powercap`
```bash
sudo setcap cap_dac_read_search=+ep ./bin/cpu-exporter
```

**GPU Exporter**: Requires NVIDIA drivers and GPU access

**Docker Stats**: Requires access to `/var/run/docker.sock`

---

## Additional Resources

- [Master Exporters Documentation](../master/exporters/README.md)
- [Worker Exporters Documentation](../worker/exporters/README.md)
- [Go Exporters Summary](../GO_EXPORTERS_SUMMARY.md)
- [Docker Compose Configuration](../docker-compose.yml)
- [Grafana Dashboards](../master/monitoring/grafana/provisioning/dashboards/)
