# Deployment System Improvements

This document describes the enhanced deployment system for FFmpeg-RTMP with enterprise-grade features.

## Overview

The improved deployment system includes:

1. **Health Checks & Monitoring** - Automated verification of deployments
2. **Pre-flight Checks** - System validation before deployment
3. **Configuration Management** - Environment-specific configs
4. **Blue-Green Deployments** - Zero-downtime master updates
5. **Rolling Updates** - Safe worker node updates
6. **CI/CD Integration** - Automated GitHub Actions workflows
7. **Interactive Wizard** - User-friendly guided deployment

## Quick Start

### Interactive Deployment

For the easiest deployment experience, use the interactive wizard:

```bash
./deployment/deployment-wizard.sh
```

This will guide you through:
- Deployment type selection (master/worker/both)
- Environment configuration (dev/staging/prod)
- Pre-flight system checks
- TLS certificate generation
- Configuration review
- Automated deployment
- Post-deployment verification

### Manual Deployment

For automation or CI/CD, use the individual tools:

```bash
# 1. Run pre-flight checks
sudo ./deployment/checks/preflight-check.sh --master

# 2. Deploy
sudo ./deploy.sh --master --non-interactive

# 3. Verify health
sudo ./deployment/checks/health-check.sh --master
```

## Features

### 1. Health Checks

Comprehensive post-deployment verification:

```bash
# Check master node
./deployment/checks/health-check.sh --master

# Check worker node
./deployment/checks/health-check.sh --worker \
  --url https://master.example.com:8080 \
  --api-key YOUR_API_KEY
```

**Checks include:**
- Service status (systemd)
- Port availability
- HTTP endpoint responses
- File/directory structure
- Disk space
- Log file analysis
- FFmpeg installation
- Cgroups v2 configuration
- Master connectivity (workers)

### 2. Pre-flight Checks

System validation before deployment:

```bash
# Check master requirements
./deployment/checks/preflight-check.sh --master

# Check worker requirements  
./deployment/checks/preflight-check.sh --worker \
  --master-url https://master.example.com:8080
```

**Validates:**
- Operating system compatibility
- CPU cores (minimum 2 for master, 2+ for worker)
- Memory (4GB+ for master, 8GB+ for worker)
- Disk space (20GB+ root, 10GB+ /var for master, 100GB+ for worker)
- Port availability (8080 for master, 1935 optional)
- Required commands (curl, wget, git, tar, gzip)
- Go version (1.24+)
- FFmpeg installation (workers)
- Cgroups v2 support
- Network connectivity
- DNS resolution
- Firewall configuration

### 3. Configuration Management

Environment-specific configuration templates:

```
deployment/configs/
├── master-dev.yaml       # Development master config
├── master-prod.yaml      # Production master config
├── worker-dev.env        # Development worker config
└── worker-prod.env       # Production worker config
```

**Validate configurations:**

```bash
./deployment/checks/config-validator.sh /etc/ffrtmp-master/config.yaml
```

**Features:**
- YAML/ENV syntax validation
- Required field verification
- Sensitive data detection
- File permission checks
- Type-specific validation (master/worker/watch)

### 4. Blue-Green Deployments

Zero-downtime deployments for master nodes:

```bash
# Deploy to inactive environment
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --deploy --master --version v2.0.0

# Switch traffic to new version
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --switch --master

# Rollback if needed
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --rollback --master
```

**How it works:**
1. Maintains two environments: blue and green
2. Deploys new version to inactive environment
3. Tests new environment
4. Switches symlink to new environment
5. Restarts service with new version
6. Keeps old version for instant rollback

**Directory structure:**
```
/opt/ffrtmp-blue/     # Environment 1
/opt/ffrtmp-green/    # Environment 2
/opt/ffrtmp -> blue   # Current active (symlink)
```

### 5. Rolling Updates

Safe worker node updates with zero downtime:

```bash
./deployment/orchestration/rolling-update.sh \
  --workers worker1,worker2,worker3 \
  --version v2.0.0 \
  --master-url https://master.example.com:8080 \
  --api-key YOUR_API_KEY \
  --max-parallel 2 \
  --ssh-user root
```

**Process:**
1. Checks connectivity to all workers
2. For each worker:
   - Drains worker (marks unavailable for new jobs)
   - Waits for running jobs to complete
   - Backs up current installation
   - Deploys new version
   - Runs health checks
   - Activates worker
3. Handles failures with rollback option
4. Reports summary

**Options:**
- `--workers` - Comma-separated list of worker hostnames/IPs
- `--version` - Version tag for deployment
- `--master-url` - Master API URL for draining workers
- `--api-key` - Master API key
- `--max-parallel` - Number of workers to update simultaneously
- `--ssh-user` - SSH user (default: root)
- `--ssh-key` - Path to SSH private key
- `--drain-timeout` - Seconds to wait for jobs to complete (default: 300)

### 6. CI/CD Integration

GitHub Actions workflows for automated deployments:

**.github/workflows/ci.yml** - Continuous Integration
- Code linting (golangci-lint)
- Unit tests
- Integration tests
- Coverage reporting
- Build binaries
- Deployment script validation
- Security scanning (Trivy)

**.github/workflows/deploy.yml** - Continuous Deployment
- Build and package
- Deploy to master (blue-green)
- Rolling update workers
- Health checks
- Slack notifications

**Secrets required:**
```
DEPLOY_SSH_KEY       # SSH private key for deployments
MASTER_HOST          # Master server hostname
WORKER_HOSTS         # Comma-separated worker hosts
MASTER_URL           # Master API URL
MASTER_API_KEY       # Master API key
SLACK_WEBHOOK_URL    # Slack webhook for notifications
```

**Trigger deployment:**
1. Create a release on GitHub, or
2. Manual workflow dispatch from Actions tab

### 7. Configuration Templates

Pre-configured templates for different environments:

**Development:**
- SQLite database
- Debug logging
- Relaxed rate limits
- TLS optional
- Local paths

**Production:**
- PostgreSQL database
- JSON structured logging
- Strict rate limits
- TLS required
- Monitoring enabled
- Backup configured
- Alert webhooks

## Best Practices

### Pre-Deployment

1. **Always run pre-flight checks:**
   ```bash
   ./deployment/checks/preflight-check.sh --master
   ```

2. **Validate configurations:**
   ```bash
   ./deployment/checks/config-validator.sh config.yaml
   ```

3. **Test in development first:**
   - Use `master-dev.yaml` and `worker-dev.env`
   - Deploy to test environment
   - Run full test suite

### During Deployment

1. **Use blue-green for masters:**
   - Zero downtime
   - Instant rollback capability
   - Test before switching

2. **Use rolling updates for workers:**
   - Maintain worker capacity
   - Drain before updating
   - Monitor for failures

3. **Monitor logs:**
   ```bash
   # Watch deployment
   journalctl -u ffrtmp-master -f
   
   # Check for errors
   grep -i error /var/log/ffrtmp/*.log
   ```

### Post-Deployment

1. **Run health checks:**
   ```bash
   ./deployment/checks/health-check.sh --master
   ```

2. **Verify functionality:**
   - Test API endpoints
   - Submit test job
   - Check worker registration
   - Verify monitoring metrics

3. **Monitor for issues:**
   - Watch logs for 30 minutes
   - Check Grafana dashboards
   - Review Prometheus metrics

## Troubleshooting

### Pre-flight Failures

**Insufficient memory:**
```bash
# Check memory
free -g

# Add swap if needed (temporary)
sudo fallocate -l 4G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

**Port already in use:**
```bash
# Find process using port
sudo ss -tlnp | grep :8080

# Stop conflicting service
sudo systemctl stop <service>
```

**Cgroups v2 not enabled:**
```bash
# Check kernel parameters
cat /proc/cmdline

# Enable cgroups v2 (requires reboot)
sudo grubby --update-kernel=ALL \
  --args="systemd.unified_cgroup_hierarchy=1"
sudo reboot
```

### Health Check Failures

**Service not running:**
```bash
# Check status
sudo systemctl status ffrtmp-master

# View logs
sudo journalctl -u ffrtmp-master -n 100

# Restart service
sudo systemctl restart ffrtmp-master
```

**Port not listening:**
```bash
# Check if binary is running
ps aux | grep ffrtmp

# Check firewall
sudo ufw status
sudo firewall-cmd --list-all

# Allow port
sudo ufw allow 8080
sudo firewall-cmd --add-port=8080/tcp --permanent
```

**Database connection failed:**
```bash
# Check PostgreSQL
sudo systemctl status postgresql

# Test connection
psql -h localhost -U ffrtmp -d ffrtmp

# Check credentials in config
cat /etc/ffrtmp-master/config.yaml | grep -A5 database
```

### Rollback Procedures

**Master rollback (blue-green):**
```bash
sudo ./deployment/orchestration/blue-green-deploy.sh --rollback --master
```

**Worker rollback:**
```bash
# SSH to worker
ssh worker1

# Find backup
ls -lth /opt/ffrtmp.backup-*

# Restore
sudo systemctl stop ffrtmp-worker
sudo rm -rf /opt/ffrtmp
sudo mv /opt/ffrtmp.backup-YYYYMMDD-HHMMSS /opt/ffrtmp
sudo systemctl start ffrtmp-worker
```

**Full rollback:**
1. Rollback master first
2. Rolling update workers to previous version
3. Verify all services healthy

## Security Considerations

1. **API Keys:**
   - Generate strong random keys
   - Rotate regularly
   - Store in secrets management system
   - Never commit to version control

2. **TLS Certificates:**
   - Use proper CA-signed certificates in production
   - Self-signed only for testing
   - Monitor expiration dates
   - Automate renewal

3. **SSH Access:**
   - Use key-based authentication
   - Disable password authentication
   - Restrict SSH access by IP
   - Use bastion hosts for production

4. **File Permissions:**
   - Config files: 600 or 640
   - Binaries: 755
   - Data directories: 750
   - Run services as non-root user

5. **Firewall:**
   - Only open required ports
   - Use IP allowlists
   - Enable connection rate limiting
   - Log rejected connections

## Monitoring Integration

### Prometheus Metrics

Deployment scripts export metrics:
- `deployment_duration_seconds` - Time to deploy
- `deployment_success_total` - Successful deployments
- `deployment_failure_total` - Failed deployments
- `health_check_status` - Health check results

### Grafana Dashboards

Pre-configured dashboards available:
- Deployment status
- Health check trends
- Rollback frequency
- Worker update progress

### Alerting

Configure alerts for:
- Deployment failures
- Health check failures
- Service downtime
- Resource exhaustion
- Certificate expiration (if implemented)

## Support

For issues or questions:
1. Check logs: `/var/log/ffrtmp/*.log`
2. Review documentation: `docs/`
3. Run health checks with verbose output
4. Check GitHub Issues
5. Contact support team

## Version History

- **v2.0** - Full deployment system overhaul
  - Added health checks
  - Added pre-flight checks
  - Added blue-green deployments
  - Added rolling updates
  - Added CI/CD integration
  - Added interactive wizard
  - Added configuration templates

## Contributing

See `CONTRIBUTING.md` for guidelines on:
- Adding new checks
- Extending deployment scripts
- Improving CI/CD workflows
- Writing documentation
