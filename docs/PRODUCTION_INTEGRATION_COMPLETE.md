# Production Integration Complete - Quick Reference

## What Was Delivered

Complete production deployment infrastructure for the auto-discovery watch daemon, including systemd integration, configuration templates, validation tools, and comprehensive documentation.

## Files Created

### Systemd Service
```
deployment/systemd/ffrtmp-watch.service
```
- Full reliability stack (state persistence + retry enabled by default)
- Automatic restarts with exponential backoff
- Cgroup delegation for process governance
- Security hardening (NoNewPrivileges, PrivateTmp, ProtectSystem)
- Runs as `ffrtmp` user (non-root)

### Configuration Templates
```
deployment/config/watch-config.production.yaml
deployment/systemd/watch.env.example
```
- Production-ready YAML configuration with examples
- Environment variables for easy tuning
- Commented with best practices
- Multiple workload profiles (light/medium/heavy)

### Deployment Tools
```
deployment/install-edge.sh (updated)
deployment/validate-watch.sh
deployment/WATCH_DEPLOYMENT.md
```
- Automated installation script
- 10-check validation script
- 10KB comprehensive deployment guide

## Quick Deployment

### 1. Install (5 minutes)
```bash
# On edge node (as root)
cd /path/to/ffmpeg-rtmp
sudo ./deployment/install-edge.sh
```

**Installs**:
- ✅ ffrtmp binary → `/usr/local/bin/ffrtmp`
- ✅ Watch daemon service → `/etc/systemd/system/ffrtmp-watch.service`
- ✅ Configuration templates → `/etc/ffrtmp/`
- ✅ State directory → `/var/lib/ffrtmp/`
- ✅ Cgroup delegation enabled

### 2. Configure (2 minutes)
```bash
# Edit main configuration
sudo nano /etc/ffrtmp/watch-config.yaml

# Key settings:
# - scan_interval: "10s"  (how often to scan)
# - target_commands: ["ffmpeg"]  (what to discover)
# - default_limits: cpu/memory for discovered processes
# - filters: min_runtime, blocked_dirs, user restrictions
```

### 3. Validate (1 minute)
```bash
# Run validation script
sudo ./deployment/validate-watch.sh

# Should show:
# ✓ Passed: 8-10 checks
# ✓ All checks passed!
```

### 4. Start (30 seconds)
```bash
# Enable and start service
sudo systemctl enable ffrtmp-watch
sudo systemctl start ffrtmp-watch

# Check status
sudo systemctl status ffrtmp-watch

# Monitor logs
sudo journalctl -u ffrtmp-watch -f
```

### 5. Verify (2 minutes)
```bash
# Start a test FFmpeg process
ffmpeg -f lavfi -i testsrc -f null - &

# Watch for discovery (within scan_interval)
sudo journalctl -u ffrtmp-watch -f

# Expected output:
# [watch] Discovered 1 new process(es)
# [watch] Attaching to PID <pid> (ffmpeg)
# [watch] Scan complete: new=1 tracked=1
```

## Configuration Profiles

### Light Workload (Dev/Test)
```yaml
scan_interval: "30s"
default_limits:
  cpu_quota: 100
  memory_limit: 2048
```

### Medium Workload (Production)
```yaml
scan_interval: "10s"
default_limits:
  cpu_quota: 200
  memory_limit: 4096
```

### Heavy Workload (High-Throughput)
```yaml
scan_interval: "5s"
default_limits:
  cpu_quota: 400
  memory_limit: 8192
```

## Key Commands

### Service Management
```bash
# Start/stop
sudo systemctl start ffrtmp-watch
sudo systemctl stop ffrtmp-watch
sudo systemctl restart ffrtmp-watch

# Enable/disable autostart
sudo systemctl enable ffrtmp-watch
sudo systemctl disable ffrtmp-watch

# Status and logs
sudo systemctl status ffrtmp-watch
sudo journalctl -u ffrtmp-watch -f
sudo journalctl -u ffrtmp-watch -n 100
```

### Configuration
```bash
# Edit main config
sudo nano /etc/ffrtmp/watch-config.yaml

# Edit environment
sudo nano /etc/ffrtmp/watch.env

# Validate config (if Python available)
python3 -c "import yaml; print(yaml.safe_load(open('/etc/ffrtmp/watch-config.yaml')))"

# Reload after config change
sudo systemctl restart ffrtmp-watch
```

### Monitoring
```bash
# Check state file
sudo cat /var/lib/ffrtmp/watch-state.json | jq

# View statistics
sudo jq '.statistics' /var/lib/ffrtmp/watch-state.json

# Check tracked processes
sudo jq '.processes' /var/lib/ffrtmp/watch-state.json

# Monitor for errors
sudo journalctl -u ffrtmp-watch | grep -i error
```

## Validation Checklist

Run validation script to verify:
- [x] Binary installed and executable
- [x] Systemd service exists
- [x] Configuration files present and valid
- [x] Directories created with correct permissions
- [x] ffrtmp user and group exist
- [x] Cgroup v2 supported and mounted
- [x] Cgroup delegation enabled
- [x] Kernel version >= 4.15
- [x] Service running without errors
- [x] State file exists (after first run)

## Troubleshooting

### Service Won't Start
```bash
# Check for errors
sudo journalctl -u ffrtmp-watch -n 50

# Common issues:
# - Binary not found: make build-cli
# - Config syntax error: validate YAML
# - Permission denied: check /usr/local/bin/ffrtmp permissions
```

### Processes Not Discovered
```bash
# Verify scan is running
sudo journalctl -u ffrtmp-watch | grep "Scan complete"

# Check target commands match
ps aux | grep ffmpeg

# Temporarily disable all filters
# Edit /etc/ffrtmp/watch-config.yaml
# Comment out filters section
sudo systemctl restart ffrtmp-watch
```

### Permission Errors on Cgroups
```bash
# Verify delegation
systemctl show -p Delegate ffrtmp-watch
# Should show: Delegate=yes

# Check cgroup v2
mount | grep cgroup2

# Reinstall delegate config
sudo cp deployment/systemd/user@.service.d-delegate.conf \
    /etc/systemd/system/user@.service.d/delegate.conf
sudo systemctl daemon-reload
sudo systemctl restart ffrtmp-watch
```

## Documentation Links

- **Quick Start**: `README.md` (watch daemon section)
- **Deployment Guide**: `deployment/WATCH_DEPLOYMENT.md`
- **Feature Documentation**: `docs/AUTO_ATTACH.md`
- **Phase 3 Summary**: `docs/AUTO_DISCOVERY_PHASE3_COMPLETE.md`
- **Configuration Examples**: `examples/watch-config.yaml`

## Next Steps

After successful deployment:

1. **Monitor for 24 hours**: Validate discovery patterns
2. **Tune resource limits**: Adjust based on actual workload
3. **Add Prometheus metrics**: Option 2 - coming next
4. **Scale to cluster**: Deploy on all worker nodes
5. **Centralized management**: Option 3 - distributed discovery

## Production Readiness Checklist

- [x] Systemd service installed
- [x] Configuration customized for environment
- [x] Validation script passed all checks
- [x] Service enabled and running
- [x] Test process discovered successfully
- [x] Logs showing no errors
- [x] State persistence working
- [ ] 24-hour stability validation
- [ ] Grafana dashboards (Option 2)
- [ ] Multi-node deployment (Option 3)

## Support

- **Issue**: GitHub Issues
- **Documentation**: `docs/` directory
- **Examples**: `examples/` directory
- **Deployment**: `deployment/` directory

---

**Status**: Production Integration (Option 1) COMPLETE ✅

**Next**: Option 2 - Prometheus Metrics Integration
