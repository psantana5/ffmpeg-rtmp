# Production Deployment Checklist ✅

Use this checklist when deploying to production environments.

## Pre-Deployment

### System Validation
- [ ] Run validator: `./deployment/validate-and-rollback.sh --validate --worker`
- [ ] Check OS is supported (Ubuntu/Debian with systemd)
- [ ] Verify cgroups v2 is enabled: `stat -fc %T /sys/fs/cgroup` (should show `cgroup2fs`)
- [ ] Confirm minimum 1GB disk space available
- [ ] Check required ports are free (8080 for master, 9091 for worker)
- [ ] Verify Go 1.24+ is installed if building from source

### Preparation
- [ ] Build binaries: `make build-master build-agent build-cli`
- [ ] Verify binaries exist in `bin/` directory
- [ ] Review dry-run output: `./deployment/validate-and-rollback.sh --dry-run --worker`
- [ ] Have master URL ready (e.g., `http://10.0.0.1:8080`)
- [ ] Have API key ready (from master: `sudo cat /etc/ffrtmp-master/api-key`)

### Documentation Review
- [ ] Read `DEPLOY_QUICKREF.md` for quick commands
- [ ] Review `deployment/WATCH_DEPLOYMENT.md` for watch daemon details
- [ ] Understand rollback process: `docs/DEPLOYMENT_ENHANCEMENTS.md`

## Master Node Deployment

### Installation
- [ ] Run pre-deployment validation: `./deployment/validate-and-rollback.sh --validate --master`
- [ ] Deploy master: `sudo ./deploy.sh --master`
- [ ] Note the generated API key location: `/etc/ffrtmp-master/api-key`
- [ ] Save API key securely for worker deployment

### Verification
- [ ] Check service status: `sudo systemctl status ffrtmp-master`
- [ ] Verify binary: `ls -l /opt/ffrtmp-master/bin/ffrtmp-master`
- [ ] Check logs: `sudo journalctl -u ffrtmp-master -n 50`
- [ ] Test API endpoint: `curl http://localhost:8080/health` (if available)
- [ ] Verify database created: `ls -l /var/lib/ffrtmp-master/master.db`
- [ ] Confirm API key exists: `sudo cat /etc/ffrtmp-master/api-key`

### Configuration
- [ ] Review master config: `sudo cat /etc/ffrtmp-master/master.env`
- [ ] Configure firewall to allow port 8080
- [ ] Set up TLS certificates if using HTTPS (optional)
- [ ] Enable service on boot: `sudo systemctl enable ffrtmp-master`

## Worker Node Deployment

### Installation
- [ ] Run pre-deployment validation: `./deployment/validate-and-rollback.sh --validate --worker`
- [ ] Preview changes: `./deployment/validate-and-rollback.sh --dry-run --worker`
- [ ] Deploy worker: `sudo ./deploy.sh --worker --master-url <url> --api-key <key>`

### Verification
- [ ] Check worker service: `sudo systemctl status ffrtmp-worker`
- [ ] Check watch service: `sudo systemctl status ffrtmp-watch`
- [ ] Verify binaries installed:
  - [ ] `ls -l /opt/ffrtmp/bin/agent`
  - [ ] `ls -l /opt/ffrtmp/bin/ffrtmp`
- [ ] Check service files use correct binary:
  - [ ] `grep ExecStart /etc/systemd/system/ffrtmp-worker.service | grep agent`
- [ ] Verify logs: `sudo journalctl -u ffrtmp-worker -n 50`
- [ ] Check watch daemon: `sudo journalctl -u ffrtmp-watch -n 50`

### Configuration
- [ ] Review worker config: `sudo cat /etc/ffrtmp/worker.env`
- [ ] Verify master URL is correct
- [ ] Verify API key is correct
- [ ] Review watch config: `sudo cat /etc/ffrtmp/watch-config.yaml`
- [ ] Adjust resource limits if needed (CPU quota, memory limit)
- [ ] Enable services on boot:
  - [ ] `sudo systemctl enable ffrtmp-worker`
  - [ ] `sudo systemctl enable ffrtmp-watch`

### Testing
- [ ] Submit test job to master (if available)
- [ ] Verify worker picks up job
- [ ] Check FFmpeg process is wrapped with cgroups
- [ ] Test watch daemon discovers running FFmpeg processes
- [ ] Verify retry mechanism works (Phase 3 feature)
- [ ] Check health status is being tracked

## Post-Deployment

### Monitoring
- [ ] Set up log monitoring: `sudo journalctl -u ffrtmp-* -f`
- [ ] Verify Prometheus metrics (if enabled): `curl localhost:9091/metrics`
- [ ] Check cgroup hierarchy: `systemd-cgls`
- [ ] Monitor resource usage: `systemctl status ffrtmp-worker`

### Documentation
- [ ] Document master URL for team
- [ ] Store API key securely (password manager/vault)
- [ ] Record worker node hostnames/IPs
- [ ] Note any custom configurations made
- [ ] Update internal deployment wiki/docs

### Backup
- [ ] Backup master database: `/var/lib/ffrtmp-master/master.db`
- [ ] Backup API key: `/etc/ffrtmp-master/api-key`
- [ ] Backup configuration files: `/etc/ffrtmp*/`
- [ ] Note backup location for rollback: `/tmp/ffrtmp-rollback-*`

## Troubleshooting

### Common Issues

**Worker service fails to start:**
- [ ] Check logs: `sudo journalctl -u ffrtmp-worker -n 100 --no-pager`
- [ ] Verify binary exists: `ls -l /opt/ffrtmp/bin/agent`
- [ ] Check service file: `sudo systemctl cat ffrtmp-worker`
- [ ] Ensure using correct binary (agent, not ffrtmp-worker)
- [ ] Verify flags are correct (`-master`, not `--master-url`)

**Watch daemon not discovering processes:**
- [ ] Check if cgroups v2 enabled
- [ ] Verify watch config: `sudo cat /etc/ffrtmp/watch-config.yaml`
- [ ] Check watch service logs: `sudo journalctl -u ffrtmp-watch -f`
- [ ] Verify target commands match: `ps aux | grep ffmpeg`

**Connection to master fails:**
- [ ] Verify master URL is correct: `curl http://<master>:8080/health`
- [ ] Check firewall rules allow port 8080
- [ ] Verify API key matches master's key
- [ ] Check network connectivity: `ping <master-ip>`

**Rollback needed:**
- [ ] Stop services: `sudo systemctl stop ffrtmp-worker ffrtmp-watch`
- [ ] Run rollback: `sudo ./deployment/validate-and-rollback.sh --rollback --worker`
- [ ] Review backup directory: `/tmp/ffrtmp-rollback-*`

## Update/Upgrade

### Idempotent Re-deployment
- [ ] Existing configs will be preserved
- [ ] State files will be backed up
- [ ] Services will be restarted
- [ ] Run: `sudo ./deployment/install-edge.sh`

### Manual Update
- [ ] Stop services: `sudo systemctl stop ffrtmp-worker ffrtmp-watch`
- [ ] Backup configs: `sudo cp -r /etc/ffrtmp /etc/ffrtmp.backup`
- [ ] Update binaries: `sudo cp bin/* /opt/ffrtmp/bin/`
- [ ] Reload systemd: `sudo systemctl daemon-reload`
- [ ] Start services: `sudo systemctl start ffrtmp-worker ffrtmp-watch`

## Security Checklist

- [ ] Change default API key if using demo key
- [ ] Use HTTPS for master URL (recommended for production)
- [ ] Set up TLS certificates (optional but recommended)
- [ ] Restrict firewall rules to known worker IPs
- [ ] Set proper file permissions: `chmod 600 /etc/ffrtmp-master/api-key`
- [ ] Review systemd security directives (NoNewPrivileges, ProtectSystem, etc.)
- [ ] Enable SELinux/AppArmor if available
- [ ] Set up log rotation: `/etc/logrotate.d/ffrtmp`

## Performance Tuning

- [ ] Adjust CPU quota based on workload: `cpu_quota` in watch config
- [ ] Set memory limits appropriately: `memory_limit` in watch config
- [ ] Tune scan interval: `scan_interval` (10s default, 5s for busy, 30s for light)
- [ ] Configure max concurrent jobs: `MAX_JOBS` in worker.env
- [ ] Enable retry for reliability: `enable_retry: true` in watch config
- [ ] Adjust max retry attempts: `max_retry_attempts` (default 3)

## Sign-Off

Deployment completed by: _____________________ Date: _____________

Verified by: _____________________ Date: _____________

Notes:
_________________________________________________________________
_________________________________________________________________
_________________________________________________________________

---

**Status:** ☐ Deployed ☐ Verified ☐ Documented ☐ Monitored
