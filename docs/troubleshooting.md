# Troubleshooting Guide

This guide covers common issues and their solutions.

## Stack Issues

### Services Not Starting

**Symptoms**: Containers exit immediately or show "Exited (1)" status

**Check**:
```bash
# Check container status
make ps

# Check logs for specific service
make logs SERVICE=prometheus
make logs SERVICE=rapl-exporter
```

**Common causes**:
1. Port already in use
2. Configuration file syntax error
3. Missing dependencies
4. Permission issues

**Solutions**:
```bash
# Restart specific service
docker compose restart <service>

# Rebuild and restart everything
make down
make up-build
```

### Port Already in Use

**Symptoms**: Error like "bind: address already in use"

**Check**:
```bash
# Find what's using the port
sudo lsof -i :9090  # Replace with your port
```

**Solutions**:
1. Stop the conflicting service
2. Change the port in `docker-compose.yml`

### Out of Disk Space

**Symptoms**: Build failures, containers crashing

**Check**:
```bash
# Check disk usage
df -h
du -sh test_results/
docker system df
```

**Solutions**:
```bash
# Clean old test results
rm test_results/test_results_2023*.json

# Clean Docker cache
docker system prune -a

# Clean unused volumes
docker volume prune
```

## RAPL Exporter Issues

### "No RAPL zones found"

**Cause**: RAPL interface not available or not accessible

**Verify RAPL availability**:
```bash
# Check if RAPL interface exists
ls -la /sys/class/powercap/intel-rapl:0/

# Check energy counter
cat /sys/class/powercap/intel-rapl:0/energy_uj
```

**Solutions**:

1. **Intel CPU check**:
   ```bash
   cat /proc/cpuinfo | grep "model name"
   # RAPL requires Intel Sandy Bridge (2011) or newer
   ```

2. **Grant permissions**:
   ```bash
   sudo chmod -R a+r /sys/class/powercap/
   ```

3. **Load kernel module**:
   ```bash
   sudo modprobe intel_rapl_msr
   lsmod | grep rapl
   ```

### RAPL Metrics Not Appearing

**Check**:
```bash
# Test exporter directly
curl http://localhost:9500/metrics | grep rapl_power_watts

# Check Prometheus targets
# Open http://localhost:9090/targets
```

**Solutions**:
- Verify container has privileged access
- Check `/sys/class/powercap` is mounted correctly
- Restart rapl-exporter: `docker compose restart rapl-exporter`

## Prometheus Issues

### Target is DOWN

**Check Prometheus targets**: http://localhost:9090/targets

**Common causes**:
1. Exporter container not running
2. Network connectivity issue
3. Wrong port/URL in prometheus.yml

**Solutions**:
```bash
# Check if exporter is running
docker compose ps | grep exporter

# Test exporter directly
curl http://localhost:9500/metrics

# Check container network
docker compose exec prometheus ping rapl-exporter

# Restart Prometheus
docker compose restart prometheus
```

### Metrics Not Showing in Grafana

**Check**:
1. Prometheus UI: http://localhost:9090
   - Query: `rapl_power_watts`
   - Should return data

2. Grafana datasource:
   - Settings → Data Sources → Prometheus
   - Click "Test" - should show "Data source is working"

**Solutions**:
```bash
# Restart Grafana
docker compose restart grafana

# Check Grafana logs
make logs SERVICE=grafana
```

### Prometheus Storage Issues

**Symptoms**: "out of disk space", "too many samples"

**Check**:
```bash
docker exec prometheus du -sh /prometheus
```

**Solutions**:
- Reduce retention time in docker-compose.yml
- Increase disk space
- Delete old data: `docker compose down && docker volume rm ffmpeg-rtmp_prometheus-data && docker compose up -d`

## Test Execution Issues

### FFmpeg Can't Connect to RTMP Server

**Symptoms**: 
```
Connection refused
Failed to open rtmp://localhost:1935/live
```

**Check**:
```bash
# Check nginx-rtmp is running
docker compose ps nginx-rtmp

# Check nginx status endpoint
curl http://localhost:8080/stat

# Check RTMP port
nc -zv localhost 1935
```

**Solutions**:
```bash
# Restart nginx-rtmp
docker compose restart nginx-rtmp

# Check nginx logs
make logs SERVICE=nginx-rtmp

# Wait for health check to pass
docker compose ps nginx-rtmp  # Should show "healthy"
```

### Test Results Not Appearing

**Symptoms**: Grafana dashboards show "No data"

**Check**:
```bash
# Verify test results exist
ls -la test_results/

# Check results-exporter
curl http://localhost:9502/metrics | grep results_

# Check results-exporter logs
make logs SERVICE=results-exporter
```

**Solutions**:
```bash
# Ensure directory exists and has proper permissions
mkdir -p test_results
chmod 755 test_results

# Restart results-exporter
docker compose restart results-exporter

# Wait 30 seconds for next scrape, then check Grafana
```

### Test Script Errors

**"FFmpeg not found"**:
```bash
# Install FFmpeg
# Ubuntu/Debian:
sudo apt-get update && sudo apt-get install ffmpeg

# macOS:
brew install ffmpeg
```

**"Permission denied" on scripts**:
```bash
chmod +x scripts/*.py scripts/*.sh
```

**Python import errors**:
```bash
# Install dependencies
pip install -r requirements.txt
```

## GPU Monitoring Issues

### DCGM Exporter Not Starting

**Symptoms**: dcgm-exporter shows "Exited (1)"

**Requirements check**:
```bash
# Check NVIDIA driver
nvidia-smi

# Check nvidia-container-toolkit
docker run --rm --gpus all nvidia/cuda:11.0-base nvidia-smi
```

**Solutions**:
1. Install nvidia-container-toolkit:
   ```bash
   # Ubuntu/Debian
   distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
   curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
   curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list
   sudo apt-get update && sudo apt-get install -y nvidia-docker2
   sudo systemctl restart docker
   ```

2. Start with NVIDIA profile:
   ```bash
   make nvidia-up-build
   ```

## Analysis Issues

### "No results files found"

**Symptoms**: `analyze_results.py` reports no results

**Check**:
```bash
ls -la test_results/
```

**Solutions**:
- Run a test first: `python3 scripts/run_tests.py single --name test --bitrate 1000k --duration 60`
- Specify results file explicitly: `python3 scripts/analyze_results.py test_results/test_results_*.json`

### ML Model Errors

**Symptoms**: Prediction errors, "not enough data"

**Requirements**:
- At least 3 test scenarios for basic model
- At least 10 scenarios recommended for multivariate model

**Solutions**:
```bash
# Run batch test to generate more data
python3 scripts/run_tests.py batch --file batch_stress_matrix.json

# Retrain models
python3 scripts/retrain_models.py --results-dir ./test_results
```

## Performance Issues

### Slow Container Performance

**Check resource usage**:
```bash
docker stats
```

**Solutions**:
1. Increase Docker resources (Docker Desktop → Settings → Resources)
   - Recommended: 4+ CPU cores, 8+ GB RAM

2. Reduce concurrent services:
   ```bash
   # Stop GPU monitoring if not needed
   docker compose stop dcgm-exporter
   ```

### High CPU Usage

**Check which container**:
```bash
docker stats --no-stream | sort -k3 -h
```

**Common culprits**:
- cAdvisor (monitoring all containers)
- Prometheus (during large queries)
- FFmpeg tests (expected during tests)

**Solutions**:
- Reduce Prometheus scrape frequency in prometheus.yml
- Reduce cAdvisor collection interval

### High Memory Usage

**Check**:
```bash
docker stats --format "table {{.Name}}\t{{.MemUsage}}"
```

**Solutions**:
- Reduce Prometheus retention time
- Limit memory in docker-compose.yml:
  ```yaml
  services:
    prometheus:
      deploy:
        resources:
          limits:
            memory: 2G
  ```

## Network Issues

### Cannot Access Web Interfaces

**Check**:
```bash
# Test each service
curl -I http://localhost:3000  # Grafana
curl -I http://localhost:9090  # Prometheus
curl -I http://localhost:9093  # Alertmanager
```

**Solutions**:
- Check firewall: `sudo ufw status`
- Check if ports are exposed: `docker compose ps`
- Try localhost vs 127.0.0.1 vs machine IP

### Containers Can't Communicate

**Check network**:
```bash
docker compose exec prometheus ping nginx-rtmp
```

**Solutions**:
```bash
# Recreate network
docker compose down
docker network prune
docker compose up -d
```

## Getting More Help

### Collect Diagnostic Information

```bash
# System info
uname -a
docker --version
docker compose version

# Container status
docker compose ps

# Logs (save to file)
docker compose logs > logs.txt

# Check disk space
df -h
docker system df
```

### Enable Debug Logging

Add to docker-compose.yml:
```yaml
services:
  prometheus:
    command:
      - "--log.level=debug"
```

### Still Having Issues?

1. Check the [Architecture documentation](architecture.md) to understand the system
2. Review [Getting Started Guide](getting-started.md) for setup steps
3. Search existing issues: https://github.com/psantana5/ffmpeg-rtmp/issues
4. Open a new issue with:
   - Description of the problem
   - Steps to reproduce
   - Relevant logs
   - System information
