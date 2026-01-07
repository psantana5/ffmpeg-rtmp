# Deployment Quick Reference

## Pre-Deployment Validation

```bash
# Validate system readiness (no root needed)
./deployment/validate-and-rollback.sh --validate --worker
./deployment/validate-and-rollback.sh --validate --master

# Preview what will be done (dry-run)
./deployment/validate-and-rollback.sh --dry-run --worker
./deployment/validate-and-rollback.sh --dry-run --master

# Run all tests
./deployment/test-deployment-scripts.sh
./deployment/simulate-deployment.sh
```

## Deploy Master Node

```bash
# Interactive mode
sudo ./deploy.sh --master

# With TLS certificate generation
sudo ./deploy.sh --master --generate-certs \
  --master-ip 10.0.0.1 \
  --master-host master.example.com

# Automated mode
sudo ./deploy.sh --master --non-interactive

# Get API key after installation
sudo cat /etc/ffrtmp-master/api-key
```

## Deploy Worker Node

```bash
# Step 1: Validate first
./deployment/validate-and-rollback.sh --validate --worker

# Step 2: Interactive mode (will prompt for master URL and API key)
sudo ./deploy.sh --worker

# Step 3: With TLS certificates
sudo ./deploy.sh --worker --generate-certs \
  --master-url https://10.0.0.1:8443 \
  --api-key $(cat /etc/ffrtmp-master/api-key)

# Step 4: Automated mode
sudo ./deploy.sh --worker \
  --master-url https://10.0.0.1:8080 \
  --api-key $(cat /etc/ffrtmp-master/api-key) \
  --worker-id edge-node-01 \
  --non-interactive
```

## TLS/SSL Certificate Generation

```bash
# Generate master certificates
./deployment/generate-certs.sh --master \
  --master-ip 10.0.0.1 \
  --master-host master.example.com

# Generate worker certificates
./deployment/generate-certs.sh --worker \
  --worker-ip 10.0.0.10 \
  --worker-host worker01.example.com

# Generate both with CA (for mTLS)
./deployment/generate-certs.sh --both --ca

# See: docs/TLS_SETUP_GUIDE.md for complete guide
```

## Deploy Both (Development)

```bash
sudo ./deploy.sh --both --non-interactive
```

## Verify Installation

```bash
# Check services
sudo systemctl status ffrtmp-master
sudo systemctl status ffrtmp-worker
sudo systemctl status ffrtmp-watch

# View logs
sudo journalctl -u ffrtmp-master -f
sudo journalctl -u ffrtmp-worker -f
sudo journalctl -u ffrtmp-watch -f

# Test binaries
/opt/ffrtmp/bin/ffrtmp --version
/opt/ffrtmp/bin/ffrtmp watch --help
```

## Common Operations

```bash
# Start services
sudo systemctl start ffrtmp-master
sudo systemctl start ffrtmp-worker
sudo systemctl start ffrtmp-watch

# Stop services
sudo systemctl stop ffrtmp-watch
sudo systemctl stop ffrtmp-worker
sudo systemctl stop ffrtmp-master

# Restart services
sudo systemctl restart ffrtmp-watch

# Enable on boot
sudo systemctl enable ffrtmp-master
sudo systemctl enable ffrtmp-worker
sudo systemctl enable ffrtmp-watch

# Disable on boot
sudo systemctl disable ffrtmp-watch
```

## Configuration Files

**Master:**
- Config: `/etc/ffrtmp-master/master.env`
- API Key: `/etc/ffrtmp-master/api-key`
- Database: `/var/lib/ffrtmp-master/master.db`
- Logs: `/var/log/ffrtmp-master/`

**Worker:**
- Config: `/etc/ffrtmp/worker.env`
- Watch Config: `/etc/ffrtmp/watch-config.yaml`
- State: `/var/lib/ffrtmp/watch-state.json`
- Logs: `/var/log/ffrtmp/`

## Troubleshooting

```bash
# Validate installation
./deployment/validate-and-rollback.sh --validate --worker

# Check if cgroups v2 is enabled
stat -fc %T /sys/fs/cgroup

# Verify binaries are correct
ls -l /opt/ffrtmp/bin/
grep ExecStart /etc/systemd/system/ffrtmp-worker.service

# Should show: /opt/ffrtmp/bin/agent (not ffrtmp-worker)

# Check binary permissions
ls -l /opt/ffrtmp/bin/

# View systemd service details
sudo systemctl cat ffrtmp-watch

# Check for errors
sudo systemctl status ffrtmp-watch -l --no-pager
```

## Rollback Failed Deployment

```bash
# Rollback worker deployment
sudo ./deployment/validate-and-rollback.sh --rollback --worker

# Rollback master deployment
sudo ./deployment/validate-and-rollback.sh --rollback --master

# Manual cleanup if needed
sudo systemctl stop ffrtmp-worker ffrtmp-watch
sudo systemctl disable ffrtmp-worker ffrtmp-watch
sudo rm /etc/systemd/system/ffrtmp-*.service
sudo systemctl daemon-reload
```

## Update Existing Installation

```bash
# Safe to re-run (idempotent)
# Will preserve configs and state
sudo ./deployment/install-edge.sh

# Output shows:
[⚠] Existing installation detected - running in UPDATE mode
[INFO] Configuration and state files will be preserved
```

## Watch Daemon Configuration

Edit `/etc/ffrtmp/watch-config.yaml`:

```yaml
# Scan settings
scan_interval: 10s
target_commands:
  - ffmpeg
  - gst-launch-1.0

# Resource limits
cpu_quota: 150    # 150% CPU (1.5 cores)
cpu_weight: 100   # Fair share
memory_limit: 2048 # 2GB RAM

# Reliability (Phase 3)
enable_retry: true
max_retry_attempts: 3
enable_state: true
state_path: /var/lib/ffrtmp/watch-state.json
```

Then restart:
```bash
sudo systemctl restart ffrtmp-watch
```

## Performance Tuning

**Light workload (1-5 streams):**
```yaml
cpu_quota: 100
memory_limit: 1024
scan_interval: 10s
```

**Medium workload (5-20 streams):**
```yaml
cpu_quota: 200
memory_limit: 2048
scan_interval: 5s
```

**Heavy workload (20+ streams):**
```yaml
cpu_quota: 400
memory_limit: 4096
scan_interval: 3s
```

## Uninstall

```bash
# Stop and disable services
sudo systemctl stop ffrtmp-watch ffrtmp-worker ffrtmp-master
sudo systemctl disable ffrtmp-watch ffrtmp-worker ffrtmp-master

# Remove service files
sudo rm /etc/systemd/system/ffrtmp-*.service
sudo systemctl daemon-reload

# Remove binaries and configs (optional)
sudo rm -rf /opt/ffrtmp/
sudo rm -rf /opt/ffrtmp-master/
sudo rm -rf /etc/ffrtmp/
sudo rm -rf /etc/ffrtmp-master/
sudo rm -rf /var/lib/ffrtmp/
sudo rm -rf /var/lib/ffrtmp-master/
sudo rm -rf /var/log/ffrtmp/
sudo rm -rf /var/log/ffrtmp-master/

# Remove users (optional)
sudo userdel ffrtmp
sudo userdel ffrtmp-master
```

## Getting Help

```bash
# Script help
./deploy.sh --help
./deployment/install-edge.sh --help

# Binary help
/opt/ffrtmp/bin/ffrtmp --help
/opt/ffrtmp/bin/ffrtmp watch --help
/opt/ffrtmp-master/bin/ffrtmp-master --help
```

## Documentation

- Full Deployment Guide: `deployment/WATCH_DEPLOYMENT.md`
- Quickstart: `QUICKSTART.md`
- Testing Report: `docs/DEPLOYMENT_TESTING_COMPLETE.md`
- Phase 3 Features: `docs/AUTO_DISCOVERY_PHASE3_COMPLETE.md`

---

**Need help?** Check the logs first: `sudo journalctl -u ffrtmp-watch -n 100`
