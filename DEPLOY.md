# Production Deployment Guide

##  Quick Start (5 Minutes)

### Option 1: Interactive Wizard (Easiest)

```bash
# Clone the repository
git clone <repo-url>
cd ffmpeg-rtmp

# Run the interactive deployment wizard
./deployment/deployment-wizard.sh
```

The wizard will guide you through:
1. **Deployment type** - Master, worker, or both
2. **Environment** - Development, staging, or production
3. **Pre-flight checks** - System validation
4. **Configuration** - TLS, database, resources
5. **Deployment** - Automated installation
6. **Verification** - Health checks

**That's it!** The wizard handles everything automatically.

---

### Option 2: Quick Deployment (Advanced Users)

```bash
# 1. Clone and build
git clone <repo-url>
cd ffmpeg-rtmp
make build-master build-agent build-cli

# 2. Run pre-flight checks
./deployment/checks/preflight-check.sh --master

# 3. Deploy master node
sudo ./deploy.sh --master --non-interactive

# 4. Verify deployment
./deployment/checks/health-check.sh --master

# 5. Test it
./bin/ffrtmp jobs submit --scenario test
./bin/ffrtmp jobs list
```

**System will be running with:**
-  Master node on port 8080 (HTTPS)
-  Worker agent registered and polling
-  API authentication enabled
-  Metrics on ports 9090-9091
-  SQLite or PostgreSQL database
-  Health checks passed
-  Systemd services configured

---

##  Deployment Methods

### Method 1: Interactive Wizard (Recommended)

Best for: First-time deployments, manual setup

```bash
./deployment/deployment-wizard.sh
```

Features:
- Step-by-step guided deployment
- Automatic system validation
- Configuration generation
- Health checks
- User-friendly prompts

---

### Method 2: Unified Deployment Script

Best for: Automated deployments, scripts, CI/CD

```bash
# Deploy master node
sudo ./deploy.sh --master --non-interactive

# Deploy worker node
sudo ./deploy.sh --worker \
  --master-url https://master.example.com:8080 \
  --api-key YOUR_API_KEY \
  --worker-id worker-01

# Deploy both on same server
sudo ./deploy.sh --both --non-interactive

# With TLS certificate generation
sudo ./deploy.sh --master \
  --generate-certs \
  --master-ip 10.0.1.10 \
  --master-host master.example.com
```

**Options:**
- `--master` - Deploy master node
- `--worker` - Deploy worker node
- `--both` - Deploy both on single server
- `--master-url URL` - Master server URL (for workers)
- `--api-key KEY` - Master API key (for workers)
- `--worker-id ID` - Worker identifier
- `--generate-certs` - Generate self-signed TLS certificates
- `--master-ip IP` - Master server IP for certificates
- `--master-host HOST` - Master server hostname for certificates
- `--non-interactive` - Skip prompts (for automation)
- `--skip-build` - Use existing binaries

---

### Method 3: Blue-Green Deployment (Zero Downtime)

Best for: Production master updates with no downtime

```bash
# 1. Deploy new version to inactive environment
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --deploy --master --version v2.0.0

# 2. Test new version (it's not active yet)
# Health checks run automatically

# 3. Switch traffic to new version
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --switch --master

# 4. If something goes wrong, instant rollback
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --rollback --master
```

**How it works:**
- Maintains two parallel environments (blue and green)
- Deploys to inactive environment
- Tests before switching
- Symlink switch for instant activation
- Previous version ready for immediate rollback

---

### Method 4: Rolling Updates (Worker Nodes)

Best for: Updating multiple workers safely

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
1. Drains each worker (stops accepting new jobs)
2. Waits for running jobs to complete
3. Creates backup of current installation
4. Deploys new version
5. Runs health checks
6. Activates worker
7. Moves to next worker

**Options:**
- `--workers` - Comma-separated list of worker hosts
- `--version` - Version tag
- `--max-parallel` - Update multiple workers simultaneously
- `--drain-timeout` - Seconds to wait for jobs (default: 300)
- `--ssh-user` - SSH user (default: root)
- `--ssh-key` - Path to SSH private key

---

### Method 5: Ansible Automation (Multiple Servers)

Best for: Large-scale deployments, infrastructure as code

```bash
# Configure inventory
cd ansible/
cp inventory/production.ini.example inventory/production.ini
vim inventory/production.ini

# Deploy everything
ansible-playbook -i inventory/production.ini playbooks/site.yml

# Deploy only master
ansible-playbook -i inventory/production.ini playbooks/master.yml

# Deploy only workers
ansible-playbook -i inventory/production.ini playbooks/workers.yml
```

See `ansible/ANSIBLE_GUIDE.md` for detailed instructions.

---

##  Pre-Deployment Validation

### Pre-flight Checks

Always run pre-flight checks before deployment:

```bash
# Check master node requirements
./deployment/checks/preflight-check.sh --master

# Check worker node requirements
./deployment/checks/preflight-check.sh --worker \
  --master-url https://master.example.com:8080
```

**Validates:**
-  Operating system compatibility (Ubuntu, Debian, Rocky, AlmaLinux)
-  CPU cores (2+ for master, 2+ for worker)
-  Memory (4GB+ for master, 8GB+ for worker)
-  Disk space (20GB+ root, 10GB+ /var for master, 100GB+ for worker)
-  Port availability (8080 for master, 1935 optional)
-  Required commands (curl, wget, git, tar, gzip)
-  Go version (1.24+)
-  FFmpeg installation and codecs (workers)
-  Cgroups v2 support
-  Network connectivity and DNS
-  Firewall configuration
-  SELinux status (RHEL-based systems)

### Configuration Validation

Validate configuration files before deployment:

```bash
# Validate master config
./deployment/checks/config-validator.sh /etc/ffrtmp-master/config.yaml

# Validate worker config
./deployment/checks/config-validator.sh /etc/ffrtmp/worker.env

# Validate watch daemon config
./deployment/checks/config-validator.sh /etc/ffrtmp/watch-config.yaml
```

**Checks:**
- YAML/ENV syntax correctness
- Required fields present
- Sensitive data configured
- File permissions secure (600/640)
- Type-specific validation

---

## 🏥 Post-Deployment Verification

### Health Checks

Verify deployment success with comprehensive health checks:

```bash
# Check master node health
./deployment/checks/health-check.sh --master

# Check worker node health
./deployment/checks/health-check.sh --worker \
  --url https://master.example.com:8080 \
  --api-key YOUR_API_KEY
```

**Verifies:**
-  Service status (systemd)
-  Port listening (8080 for master)
-  HTTP endpoints responding
-  API authentication working
-  File and directory structure
-  Configuration files present
-  Disk space available
-  Log files accessible
-  No critical errors in logs
-  Database connectivity (master)
-  FFmpeg installation (workers)
-  Cgroups v2 enabled (workers)
-  Master connectivity (workers)
-  Worker registration successful

**Output:**
```
═══════════════════════════════════════
  Health Check Summary
═══════════════════════════════════════
Passed:   25
Warnings: 2
Failed:   0
```

---

##  Configuration Management

### Environment-Specific Configs

Pre-configured templates for different environments:

```
deployment/configs/
├── master-dev.yaml       # Development master
├── master-prod.yaml      # Production master
├── worker-dev.env        # Development worker
└── worker-prod.env       # Production worker
```

**Development:**
- SQLite database
- Debug logging
- Relaxed rate limits
- TLS optional
- Local paths

**Production:**
- PostgreSQL database with SSL
- JSON structured logging
- Strict rate limits
- TLS required with client verification
- Monitoring enabled
- Backup configured
- Alert webhooks

### Using Configuration Templates

```bash
# Copy and customize for your environment
sudo cp deployment/configs/master-prod.yaml /etc/ffrtmp-master/config.yaml
sudo vim /etc/ffrtmp-master/config.yaml

# Validate before use
./deployment/checks/config-validator.sh /etc/ffrtmp-master/config.yaml

# Deploy with custom config
sudo ./deploy.sh --master --config /etc/ffrtmp-master/config.yaml
```

---

##  Deployment Scenarios

### Scenario 1: Single Server (Development)

**Use Case**: Development, testing, small deployments (<100 jobs/day)

```bash
# Interactive wizard (easiest)
./deployment/deployment-wizard.sh

# Or manual deployment
make build-master build-agent build-cli
sudo ./deploy.sh --both --non-interactive
./deployment/checks/health-check.sh --master

# Access
curl http://localhost:8080/health
./bin/ffrtmp jobs submit --scenario test
```

**Pros**: Simple, easy setup, single command  
**Cons**: Single point of failure, limited scale

---

### Scenario 2: Distributed Setup (Production)

**Use Case**: Production workloads, high availability, horizontal scaling

#### Master Server

```bash
# 1. Pre-flight checks
./deployment/checks/preflight-check.sh --master

# 2. Deploy master
sudo ./deploy.sh --master \
  --generate-certs \
  --master-ip 10.0.1.10 \
  --master-host master.example.com \
  --non-interactive

# 3. Health check
./deployment/checks/health-check.sh --master

# 4. Get API key
sudo cat /etc/ffrtmp-master/.api-key
```

#### Worker Servers (3+ nodes)

```bash
# Deploy to multiple workers with rolling update
./deployment/orchestration/rolling-update.sh \
  --workers worker1,worker2,worker3 \
  --version v1.0.0 \
  --master-url https://10.0.1.10:8080 \
  --api-key <API_KEY_FROM_MASTER> \
  --ssh-user root

# Or deploy individually
ssh worker1
sudo ./deploy.sh --worker \
  --master-url https://10.0.1.10:8080 \
  --api-key <API_KEY> \
  --worker-id worker1

# Verify
./deployment/checks/health-check.sh --worker \
  --url https://10.0.1.10:8080 \
  --api-key <API_KEY>
```

**Pros**: Scalable, fault-tolerant, independent worker updates  
**Cons**: Requires multiple servers, more complex setup

---

### Scenario 3: High-Availability Master (Blue-Green)

**Use Case**: Mission-critical deployments, zero-downtime updates

```bash
# Initial deployment
sudo ./deploy.sh --master --non-interactive

# Later: Deploy update with zero downtime
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --deploy --master --version v2.0.0

# Test in inactive environment
# (automatic health checks run)

# Switch when ready
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --switch --master

# Instant rollback if issues
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --rollback --master
```

**Pros**: Zero downtime, instant rollback, safe testing  
**Cons**: Requires double disk space for environments

---

### Scenario 4: Multi-Region Deployment (Ansible)

**Use Case**: Geographic distribution, 10+ servers, infrastructure as code

```bash
# 1. Configure inventory
cd ansible/
cp inventory/production.ini.example inventory/production.ini

# Edit with your servers:
# [master]
# master.us-east.example.com
# 
# [workers]
# worker1.us-east.example.com
# worker2.us-east.example.com
# worker3.us-west.example.com

# 2. Configure variables
vim group_vars/all.yml        # Global settings
vim group_vars/master.yml     # Master-specific
vim group_vars/workers.yml    # Worker-specific

# 3. Deploy everything
ansible-playbook -i inventory/production.ini playbooks/site.yml

# 4. Or deploy incrementally
ansible-playbook -i inventory/production.ini playbooks/master.yml
ansible-playbook -i inventory/production.ini playbooks/workers.yml --limit us-east
```

**Pros**: Repeatable, version controlled, multi-region support  
**Cons**: Ansible knowledge required, initial setup complexity

See `ansible/ANSIBLE_GUIDE.md` for complete Ansible documentation.

---

##  TLS/SSL Configuration

### Automated Certificate Generation

```bash
# Generate self-signed certificates (development/testing)
sudo ./deployment/generate-certs.sh \
  --type master \
  --ip 10.0.1.10 \
  --hostname master.example.com \
  --output /etc/ffrtmp-master/certs

# Generate CA and client certificates (mTLS)
sudo ./deployment/generate-certs.sh \
  --type ca \
  --output /etc/ffrtmp-master/certs

sudo ./deployment/generate-certs.sh \
  --type worker \
  --output /etc/ffrtmp/certs \
  --ca-cert /etc/ffrtmp-master/certs/ca.crt \
  --ca-key /etc/ffrtmp-master/certs/ca.key
```

### Production Certificates

For production, use CA-signed certificates:

```bash
# 1. Generate CSR
openssl req -new -key master.key -out master.csr

# 2. Get certificate from CA (Let's Encrypt, etc.)

# 3. Install certificates
sudo cp master.crt /etc/ffrtmp-master/certs/
sudo cp master.key /etc/ffrtmp-master/certs/
sudo chmod 600 /etc/ffrtmp-master/certs/master.key

# 4. Configure master to use them
sudo ./deploy.sh --master \
  --master-ip $(hostname -I | awk '{print $1}')
```

See `docs/TLS_SETUP_GUIDE.md` for complete TLS documentation.

---

## 🗄️ Database Configuration

### SQLite (Default - Development)

Best for: Development, testing, <1000 jobs/day

```bash
# Automatically configured in development mode
./deployment/deployment-wizard.sh
# Select: Development environment

# Or manually
sudo ./deploy.sh --master --non-interactive

# Database location: /var/lib/ffrtmp-master/master.db
```

### PostgreSQL (Production)

Best for: Production, >1000 jobs/day, >10 workers

```bash
# 1. Install PostgreSQL
sudo apt-get install postgresql postgresql-contrib

# Or with Docker
docker run -d \
    --name ffrtmp-postgres \
    -e POSTGRES_DB=ffrtmp \
    -e POSTGRES_USER=ffrtmp_user \
    -e POSTGRES_PASSWORD=secure_password \
    -p 5432:5432 \
    postgres:15

# 2. Create database and user
sudo -u postgres psql << EOF
CREATE DATABASE ffrtmp;
CREATE USER ffrtmp_user WITH PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE ffrtmp TO ffrtmp_user;
\q
EOF

# 3. Run migrations
psql -U ffrtmp_user -d ffrtmp -f shared/pkg/store/migrations/001_initial_schema.sql

# 4. Configure master
sudo cp deployment/configs/master-prod.yaml /etc/ffrtmp-master/config.yaml
sudo vim /etc/ffrtmp-master/config.yaml

# Update database section:
# database:
#   type: postgres
#   host: localhost
#   port: 5432
#   database: ffrtmp
#   user: ffrtmp_user
#   password: secure_password
#   ssl_mode: require

# 5. Restart master
sudo systemctl restart ffrtmp-master
```

See `deployment/postgres/README.md` for PostgreSQL high-availability setup.

---

##  Monitoring and Observability

### Built-in Metrics (Prometheus)

```bash
# Master metrics
curl http://localhost:9090/metrics

# Worker metrics  
curl http://localhost:9091/metrics

# Available metrics:
# - ffrtmp_jobs_total
# - ffrtmp_jobs_duration_seconds
# - ffrtmp_workers_active
# - ffrtmp_queue_length
# - go_goroutines, go_memstats_*
# - process_cpu_seconds_total
```

### Health Endpoints

```bash
# Master health
curl http://localhost:8080/health

# Returns:
# {"status":"healthy","database":"ok","workers":3,"version":"2.0.0"}

# Detailed status
curl http://localhost:8080/api/v1/status
```

### Grafana Dashboards

```bash
# 1. Start monitoring stack
cd deployment/grafana
docker-compose up -d

# 2. Access Grafana
open http://localhost:3000
# Credentials: admin/admin

# 3. Pre-configured dashboards:
# - FFmpeg-RTMP Overview
# - Job Processing Metrics
# - Worker Node Status
# - System Resources
```

Dashboards auto-loaded from `deployment/grafana/dashboards/`

### Prometheus Configuration

```bash
# Prometheus config at: deployment/prometheus/prometheus.yml
# Scrapes:
# - Master: localhost:9090
# - Workers: auto-discovery or static config
# - Node Exporter: 9100 (if installed)

# View targets
open http://localhost:9090/targets
```

### Log Aggregation

```bash
# View logs
sudo journalctl -u ffrtmp-master -f
sudo journalctl -u ffrtmp-worker -f
sudo journalctl -u ffrtmp-watch -f

# Or file-based logs
tail -f /var/log/ffrtmp/master.log
tail -f /var/log/ffrtmp/worker.log

# Search for errors
grep -i error /var/log/ffrtmp/*.log
```

**Log rotation** configured automatically:
- Daily rotation
- 14-day retention
- Gzip compression
- Location: `/var/log/ffrtmp/`

See `deployment/logrotate/` for logrotate configs.

---

## 🔒 Security Hardening

### Pre-Deployment Security Checklist

Run automated security checks:

```bash
# Included in pre-flight checks
./deployment/checks/preflight-check.sh --master
```

### Essential Security Steps

1. **Strong API Keys**
   ```bash
   # Generate cryptographically secure key
   export MASTER_API_KEY="$(openssl rand -hex 32)"
   
   # Or let deployment script generate one
   sudo ./deploy.sh --master --non-interactive
   # Key saved to: /etc/ffrtmp-master/.api-key
   ```

2. **TLS Encryption**
   ```bash
   # Production: Use CA-signed certificates
   sudo cp /path/to/ca-signed.crt /etc/ffrtmp-master/certs/server.crt
   sudo cp /path/to/ca-signed.key /etc/ffrtmp-master/certs/server.key
   
   # Development: Auto-generate self-signed
   sudo ./deploy.sh --master --generate-certs \
     --master-ip 10.0.1.10 \
     --master-host master.example.com
   ```

3. **Firewall Rules**
   ```bash
   # UFW (Ubuntu/Debian)
   sudo ufw allow 8080/tcp   # Master API
   sudo ufw allow from 10.0.0.0/8 to any port 9090  # Metrics (internal only)
   sudo ufw enable
   
   # Firewalld (RHEL/Rocky)
   sudo firewall-cmd --permanent --add-port=8080/tcp
   sudo firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="10.0.0.0/8" port port="9090" protocol="tcp" accept'
   sudo firewall-cmd --reload
   ```

4. **File Permissions**
   ```bash
   # Automatically set by deployment scripts
   # Verify:
   ls -la /etc/ffrtmp-master/
   # Expect: 600 or 640 for sensitive files
   
   # Fix if needed:
   sudo chmod 600 /etc/ffrtmp-master/config.yaml
   sudo chmod 600 /etc/ffrtmp-master/.api-key
   sudo chown ffrtmp-master:ffrtmp-master /etc/ffrtmp-master/*
   ```

5. **Database Security**
   ```bash
   # PostgreSQL: Require SSL
   # In config.yaml:
   database:
     ssl_mode: require
     
   # Restrict PostgreSQL access
   sudo vim /etc/postgresql/*/main/pg_hba.conf
   # Add: hostssl ffrtmp ffrtmp_user 10.0.0.0/8 md5
   ```

6. **Rate Limiting**
   ```bash
   # Already enabled by default in production config
   # Adjust in config.yaml:
   api:
     rate_limit: 1000  # requests per minute
     burst: 2000       # burst allowance
   ```

7. **SELinux (RHEL-based)**
   ```bash
   # If SELinux is enforcing, set contexts
   sudo semanage fcontext -a -t bin_t "/opt/ffrtmp(-master)?/bin(/.*)?"
   sudo restorecon -Rv /opt/ffrtmp(-master)?/bin
   ```

### Security Validation

```bash
# Run security audit
./deployment/checks/security-audit.sh

# Check for exposed secrets
grep -r "password\|secret\|key" /etc/ffrtmp* 2>/dev/null | grep -v "CHANGE_ME"

# Verify TLS
openssl s_client -connect localhost:8080 -showcerts

# Test API authentication
curl -X GET http://localhost:8080/api/v1/jobs  # Should return 401
curl -X GET -H "X-API-Key: wrong-key" http://localhost:8080/api/v1/jobs  # Should return 403
```

---

##  System Requirements

### Minimum (Development/Testing)
- **CPU**: 2 cores
- **RAM**: 4 GB
- **Disk**: 10 GB
- **OS**: Linux (Ubuntu 20.04+, Debian 10+, Rocky Linux 8+)
- **Network**: Internet connectivity

### Recommended (Production Master)
- **CPU**: 4-8 cores
- **RAM**: 8-16 GB
- **Disk**: 50-100 GB SSD
- **OS**: Ubuntu 22.04 LTS or Rocky Linux 9
- **Network**: 1 Gbps, low latency to workers

### Recommended (Production Worker)
- **CPU**: 8-16 cores (more for video encoding)
- **RAM**: 16-32 GB
- **Disk**: 200-500 GB SSD (for temporary files)
- **GPU**: NVIDIA GPU with NVENC (optional, recommended)
- **OS**: Ubuntu 22.04 LTS or Rocky Linux 9
- **Network**: 1 Gbps minimum, 10 Gbps for 4K workloads

### Software Requirements
- **Go**: 1.24+ (for building from source)
- **FFmpeg**: 4.4+ with libx264, libx265
- **PostgreSQL**: 15+ (production master)
- **Docker**: 20.10+ (optional, for monitoring stack)
- **Ansible**: 2.15+ (optional, for automated deployment)

### Optional Components
- **NVIDIA GPU + drivers** - For NVENC hardware encoding
- **Intel QSV** - Intel Quick Sync Video encoding
- **VAAPI** - Intel/AMD hardware acceleration
- **Prometheus** - Metrics collection
- **Grafana** - Monitoring dashboards
- **Victoria Metrics** - Long-term metrics storage

---

##  File and Directory Structure

```
ffmpeg-rtmp/
├── deployment/                    # Deployment scripts and tools
│   ├── checks/                    # Validation and health checks
│   │   ├── health-check.sh        # Post-deployment verification
│   │   ├── preflight-check.sh     # Pre-deployment validation
│   │   ├── config-validator.sh    # Configuration validation
│   │   └── security-audit.sh      # Security checks
│   ├── configs/                   # Configuration templates
│   │   ├── master-dev.yaml        # Development master config
│   │   ├── master-prod.yaml       # Production master config
│   │   ├── worker-dev.env         # Development worker config
│   │   └── worker-prod.env        # Production worker config
│   ├── orchestration/             # Deployment orchestration
│   │   ├── blue-green-deploy.sh   # Zero-downtime deployment
│   │   └── rolling-update.sh      # Worker rolling updates
│   ├── systemd/                   # Systemd service files
│   │   ├── ffrtmp-master.service
│   │   ├── ffrtmp-worker.service
│   │   ├── ffrtmp-watch.service
│   │   └── *.env.example
│   ├── logrotate/                 # Log rotation configs
│   ├── grafana/                   # Grafana dashboards
│   ├── prometheus/                # Prometheus configuration
│   ├── postgres/                  # PostgreSQL setup scripts
│   ├── generate-certs.sh          # TLS certificate generator
│   ├── validate-and-rollback.sh   # Backup and restore
│   ├── deployment-wizard.sh       # Interactive deployment
│   ├── test-deployment-scripts.sh # Deployment tests
│   └── simulate-deployment.sh     # Deployment simulation
├── ansible/                       # Ansible automation
│   ├── playbooks/                 # Ansible playbooks
│   ├── roles/                     # Ansible roles
│   ├── inventory/                 # Inventory files
│   ├── group_vars/                # Variable files
│   └── ANSIBLE_GUIDE.md           # Ansible documentation
├── deploy.sh                      # Unified deployment script
├── bin/                           # Compiled binaries
│   ├── master                     # Master server binary
│   ├── agent                      # Worker agent binary
│   └── ffrtmp                     # CLI tool with watch daemon
├── docs/                          # Documentation
│   ├── DEPLOYMENT_IMPROVEMENTS.md # Deployment guide
│   ├── TLS_SETUP_GUIDE.md         # TLS/SSL guide
│   ├── PRODUCTION_CHECKLIST.md    # Production checklist
│   ├── API.md                     # API documentation
│   ├── ARCHITECTURE.md            # System architecture
│   └── ...
├── /etc/ffrtmp-master/            # Master configuration
│   ├── config.yaml                # Main configuration
│   ├── .api-key                   # API key (generated)
│   └── certs/                     # TLS certificates
│       ├── server.crt
│       ├── server.key
│       └── ca.crt
├── /etc/ffrtmp/                   # Worker configuration
│   ├── worker.env                 # Environment variables
│   ├── watch-config.yaml          # Watch daemon config
│   └── certs/                     # TLS certificates
├── /var/lib/ffrtmp-master/        # Master data
│   ├── master.db                  # SQLite database
│   └── archive/                   # Archived jobs
├── /var/lib/ffrtmp/               # Worker data
│   └── results/                   # Job output files
├── /var/log/ffrtmp/               # Application logs
│   ├── master.log
│   ├── worker.log
│   ├── watch.log
│   └── *.log.[1-14].gz            # Rotated logs
├── /var/backups/ffrtmp/           # Automatic backups
│   ├── master-*.tar.gz
│   └── worker-*.tar.gz
└── /opt/ffrtmp(-master)/          # Installation directory
    └── bin/                       # Binaries

# Blue-Green Deployment Structure
/opt/ffrtmp-blue/                  # Blue environment
/opt/ffrtmp-green/                 # Green environment
/opt/ffrtmp -> blue                # Current active (symlink)
```

---

##  Troubleshooting

### Quick Diagnostics

```bash
# Run comprehensive health check
./deployment/checks/health-check.sh --master

# Check service status
sudo systemctl status ffrtmp-master
sudo systemctl status ffrtmp-worker
sudo systemctl status ffrtmp-watch

# View recent logs
sudo journalctl -u ffrtmp-master -n 100 --no-pager
sudo journalctl -u ffrtmp-worker -n 100 --no-pager

# Check connectivity
curl -k https://localhost:8080/health
```

### Common Issues

#### Issue: Pre-flight checks fail

**Symptom**: `preflight-check.sh` reports errors

**Solutions**:
```bash
# Insufficient memory
free -g  # Check available memory
# Add swap or upgrade server

# Port in use
sudo ss -tlnp | grep :8080
sudo systemctl stop <conflicting-service>

# Missing dependencies
sudo apt-get install curl wget git  # Debian/Ubuntu
sudo yum install curl wget git      # RHEL/Rocky

# Go version too old
# Install Go 1.24+ from https://go.dev/dl/

# Cgroups v2 not enabled
sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
sudo reboot
```

#### Issue: Master service won't start

**Symptom**: `systemctl status ffrtmp-master` shows failed

**Solutions**:
```bash
# Check logs
sudo journalctl -u ffrtmp-master -n 50 --no-pager

# Common causes:
# 1. Port already in use
sudo ss -tlnp | grep :8080
sudo lsof -i:8080

# 2. Database migration failed
ls -la /var/lib/ffrtmp-master/master.db
sudo -u ffrtmp-master sqlite3 /var/lib/ffrtmp-master/master.db ".schema"

# 3. Permission issues
sudo chown -R ffrtmp-master:ffrtmp-master /var/lib/ffrtmp-master
sudo chmod 755 /var/lib/ffrtmp-master

# 4. Configuration syntax error
./deployment/checks/config-validator.sh /etc/ffrtmp-master/config.yaml

# 5. Certificate issues
ls -la /etc/ffrtmp-master/certs/
openssl x509 -in /etc/ffrtmp-master/certs/server.crt -text -noout
```

#### Issue: Worker can't register with master

**Symptom**: Worker logs show "failed to register" or connection errors

**Solutions**:
```bash
# 1. Check master is reachable
curl -k https://master.example.com:8080/health

# 2. Verify API key matches
sudo cat /etc/ffrtmp/worker.env | grep API_KEY
# Compare with master:
sudo cat /etc/ffrtmp-master/.api-key

# 3. TLS certificate verification issues
# Test with curl
curl -k https://master.example.com:8080/health  # -k skips verification
curl https://master.example.com:8080/health     # Should work if certs valid

# 4. Firewall blocking
sudo ufw status
sudo firewall-cmd --list-all

# 5. Network connectivity
ping master.example.com
telnet master.example.com 8080
```

#### Issue: Jobs not processing

**Symptom**: Jobs stay in "pending" state

**Solutions**:
```bash
# 1. Check workers are registered
./bin/ffrtmp nodes list
# Should show your workers

# 2. Check worker service is running
sudo systemctl status ffrtmp-worker
sudo journalctl -u ffrtmp-worker -f

# 3. Check worker capacity
# In worker logs, look for:
# - "max concurrent jobs reached"
# - "worker is draining"

# 4. Check job queue
./bin/ffrtmp jobs list --status pending

# 5. Check for errors in master logs
sudo grep -i error /var/log/ffrtmp/master.log
```

#### Issue: Health checks fail after deployment

**Symptom**: `health-check.sh` reports failures

**Solutions**:
```bash
# Re-run with verbose output
./deployment/checks/health-check.sh --master --verbose

# Address each failed check:
# - Service not running: sudo systemctl start ffrtmp-master
# - Port not listening: Check service logs
# - HTTP endpoint error: Check TLS certificates
# - Database error: Check PostgreSQL is running
# - Disk space: Clean up old files

# If multiple checks fail, consider rollback:
sudo ./deployment/orchestration/blue-green-deploy.sh --rollback --master
```

#### Issue: Deployment hangs or times out

**Symptom**: Deployment script stops responding

**Solutions**:
```bash
# 1. Check system resources
top
df -h

# 2. Check for prompts waiting for input
# Use --non-interactive flag:
sudo ./deploy.sh --master --non-interactive

# 3. Check network connectivity (if downloading packages)
ping 8.8.8.8
ping github.com

# 4. Increase timeouts
# Edit script or use:
export DEPLOY_TIMEOUT=600
```

### Rollback Procedures

#### Rollback Master (Blue-Green)

```bash
# Instant rollback to previous version
sudo ./deployment/orchestration/blue-green-deploy.sh --rollback --master

# Manual rollback
sudo systemctl stop ffrtmp-master
sudo rm /opt/ffrtmp
sudo ln -s /opt/ffrtmp-green /opt/ffrtmp  # Or blue
sudo systemctl start ffrtmp-master
```

#### Rollback Worker

```bash
# SSH to worker
ssh worker1

# Find backup
ls -lt /opt/ffrtmp.backup-*

# Restore
sudo systemctl stop ffrtmp-worker ffrtmp-watch
sudo rm -rf /opt/ffrtmp
sudo mv /opt/ffrtmp.backup-20260107-103000 /opt/ffrtmp
sudo systemctl start ffrtmp-worker ffrtmp-watch

# Verify
./deployment/checks/health-check.sh --worker \
  --url https://master.example.com:8080
```

### Getting Help

1. **Check logs first**:
   ```bash
   sudo journalctl -u ffrtmp-* -n 200 --no-pager
   ```

2. **Run diagnostics**:
   ```bash
   ./deployment/checks/health-check.sh --master
   ./deployment/checks/preflight-check.sh --master
   ```

3. **Enable debug logging**:
   ```bash
   # Edit config.yaml
   logging:
     level: debug
   
   sudo systemctl restart ffrtmp-master
   ```

4. **Check documentation**:
   - `docs/DEPLOYMENT_IMPROVEMENTS.md` - Complete deployment guide
   - `docs/TLS_SETUP_GUIDE.md` - TLS troubleshooting
   - `ansible/ANSIBLE_GUIDE.md` - Ansible-specific issues
   - `DEPLOY_QUICKREF.md` - Quick reference

5. **Report issues**:
   - GitHub Issues: Include logs, config (redacted), system info
   - Provide output from health checks

---

##  Maintenance and Operations

### Regular Maintenance Tasks

#### Daily
- Monitor service health
- Check disk space
- Review error logs

```bash
# Automated daily health check
./deployment/checks/health-check.sh --master | tee /var/log/ffrtmp/health-$(date +%Y%m%d).log
```

#### Weekly
- Review job success rates
- Check worker capacity
- Database maintenance

```bash
# SQLite maintenance
sudo sqlite3 /var/lib/ffrtmp-master/master.db "VACUUM; ANALYZE;"

# PostgreSQL maintenance
sudo -u postgres psql ffrtmp -c "VACUUM ANALYZE;"

# Clean up old job results
find /var/lib/ffrtmp/results -mtime +30 -delete
```

#### Monthly
- Review and rotate API keys
- Update system packages
- Test backup restoration
- Review security logs

```bash
# System updates
sudo apt-get update && sudo apt-get upgrade -y  # Debian/Ubuntu
sudo yum update -y                               # RHEL/Rocky

# Test backups
./deployment/validate-and-rollback.sh --validate
```

### Log Management

**Automatic log rotation** (configured during deployment):

```bash
# Configuration
/etc/logrotate.d/ffrtmp-master
/etc/logrotate.d/ffrtmp-worker
/etc/logrotate.d/ffrtmp-watch

# Settings:
# - Daily rotation
# - 14-day retention
# - Gzip compression
# - Copytruncate mode (no service restart needed)

# Test logrotate
sudo logrotate -d /etc/logrotate.d/ffrtmp-master  # Dry run
sudo logrotate -f /etc/logrotate.d/ffrtmp-master  # Force rotation

# View rotated logs
ls -lh /var/log/ffrtmp/
zcat /var/log/ffrtmp/master.log.1.gz | less
```

### Backup and Restore

#### Automated Backups

```bash
# Backups created automatically during deployment
ls -lh /var/backups/ffrtmp/

# Backup locations:
# /var/backups/ffrtmp/master-YYYYMMDD-HHMMSS.tar.gz
# /var/backups/ffrtmp/worker-YYYYMMDD-HHMMSS.tar.gz
```

#### Manual Backup

```bash
# Master node backup
sudo tar -czf /var/backups/ffrtmp/master-$(date +%Y%m%d).tar.gz \
  /etc/ffrtmp-master \
  /var/lib/ffrtmp-master \
  /opt/ffrtmp-master/bin

# Worker node backup
sudo tar -czf /var/backups/ffrtmp/worker-$(date +%Y%m%d).tar.gz \
  /etc/ffrtmp \
  /opt/ffrtmp/bin

# Database-only backup
sudo cp /var/lib/ffrtmp-master/master.db /var/backups/ffrtmp/master-$(date +%Y%m%d).db
# Or PostgreSQL:
pg_dump -U ffrtmp_user ffrtmp > /var/backups/ffrtmp/ffrtmp-$(date +%Y%m%d).sql
```

#### Restore from Backup

```bash
# Interactive rollback wizard
./deployment/validate-and-rollback.sh --rollback

# Or manual restore
sudo systemctl stop ffrtmp-master
sudo tar -xzf /var/backups/ffrtmp/master-20260107.tar.gz -C /
sudo systemctl start ffrtmp-master
./deployment/checks/health-check.sh --master
```

### Database Maintenance

#### SQLite

```bash
# Compact database
sudo -u ffrtmp-master sqlite3 /var/lib/ffrtmp-master/master.db "VACUUM;"

# Analyze for query optimization
sudo -u ffrtmp-master sqlite3 /var/lib/ffrtmp-master/master.db "ANALYZE;"

# Check integrity
sudo -u ffrtmp-master sqlite3 /var/lib/ffrtmp-master/master.db "PRAGMA integrity_check;"

# View database size
du -sh /var/lib/ffrtmp-master/master.db
```

#### PostgreSQL

```bash
# Vacuum and analyze
sudo -u postgres psql ffrtmp << EOF
VACUUM VERBOSE ANALYZE;
REINDEX DATABASE ffrtmp;
EOF

# Check bloat
sudo -u postgres psql ffrtmp -c "SELECT schemaname, tablename, pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size FROM pg_tables ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC LIMIT 10;"

# Backup
pg_dump -U ffrtmp_user -F c ffrtmp > /var/backups/ffrtmp/ffrtmp-$(date +%Y%m%d).dump
```

### Cleanup Old Data

```bash
# Clean completed jobs older than 30 days
./bin/ffrtmp jobs cleanup --older-than 30d

# Or manually
sudo sqlite3 /var/lib/ffrtmp-master/master.db << EOF
DELETE FROM jobs WHERE status='completed' AND completed_at < datetime('now', '-30 days');
DELETE FROM jobs WHERE status='failed' AND completed_at < datetime('now', '-7 days');
VACUUM;
EOF

# Clean old log files (beyond logrotate retention)
find /var/log/ffrtmp -name "*.log.*" -mtime +30 -delete

# Clean old job output files
find /var/lib/ffrtmp/results -type f -mtime +30 -delete
```

---

##  Upgrading to New Versions

### Zero-Downtime Upgrade (Master - Recommended)

```bash
# 1. Build new version
git pull origin main
make build-master build-agent build-cli

# 2. Deploy to inactive environment
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --deploy --master --version v2.1.0

# 3. Health checks run automatically
# Review output

# 4. Switch to new version
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --switch --master

# 5. Verify new version
./deployment/checks/health-check.sh --master
./bin/ffrtmp version

# 6. If issues, instant rollback
sudo ./deployment/orchestration/blue-green-deploy.sh \
  --rollback --master
```

### Rolling Upgrade (Workers)

```bash
# Upgrade all workers with zero downtime
./deployment/orchestration/rolling-update.sh \
  --workers worker1,worker2,worker3 \
  --version v2.1.0 \
  --master-url https://master.example.com:8080 \
  --api-key YOUR_API_KEY \
  --max-parallel 1

# Workers are updated one at a time:
# 1. Drains worker
# 2. Waits for jobs to complete
# 3. Backs up current version
# 4. Deploys new version
# 5. Verifies health
# 6. Moves to next worker
```

### Standard Upgrade (With Downtime)

```bash
# 1. Backup everything
sudo ./deployment/validate-and-rollback.sh --validate

# 2. Stop services
sudo systemctl stop ffrtmp-master ffrtmp-worker ffrtmp-watch

# 3. Backup database
sudo cp /var/lib/ffrtmp-master/master.db /var/backups/ffrtmp/master-pre-upgrade-$(date +%Y%m%d).db

# 4. Pull and build
git pull origin main
make clean build-master build-agent build-cli

# 5. Run any database migrations
# Check CHANGELOG.md for migration steps

# 6. Redeploy
sudo ./deploy.sh --master --non-interactive

# 7. Verify
./deployment/checks/health-check.sh --master
./bin/ffrtmp version
```

### Upgrade Checklist

Before upgrading:
- [ ] Read CHANGELOG.md for breaking changes
- [ ] Backup database
- [ ] Test upgrade in development environment
- [ ] Schedule maintenance window (if not using blue-green)
- [ ] Notify users of potential downtime
- [ ] Verify disk space for new version

After upgrading:
- [ ] Run health checks
- [ ] Verify API functionality
- [ ] Check worker registration
- [ ] Submit test job
- [ ] Monitor logs for errors
- [ ] Update monitoring dashboards (if needed)

---

##  Performance Tuning

### Master Node Optimization

```bash
# Production configuration (deployment/configs/master-prod.yaml)

# 1. Use PostgreSQL for high throughput
database:
  type: postgres
  max_connections: 100
  max_idle_connections: 25
  connection_max_lifetime: 3600s

# 2. Increase API rate limits
api:
  rate_limit: 5000      # requests per minute
  burst: 10000          # burst capacity

# 3. Tune scheduler
scheduler:
  interval: 5s          # How often to check for jobs
  batch_size: 100       # Jobs to schedule per cycle

# 4. Increase worker heartbeat tolerance
worker:
  heartbeat_timeout: 30s
  max_missed_heartbeats: 5
  cleanup_interval: 60s
```

### Worker Node Optimization

```bash
# Worker configuration (deployment/configs/worker-prod.env)

# 1. Maximize concurrent jobs (based on CPU cores)
MAX_CONCURRENT_JOBS=16     # Typically cores * 2

# 2. Faster heartbeat for responsiveness
HEARTBEAT_INTERVAL=30s     # Default: 30s (3 missed = 90s timeout)

# 3. Enable hardware acceleration
ENABLE_NVENC=true          # NVIDIA GPU
ENABLE_VAAPI=true          # Intel/AMD
ENABLE_QSV=true            # Intel Quick Sync

# 4. FFmpeg threading
FFMPEG_THREADS=0           # Auto-detect (recommended)

# 5. Resource limits (adjust based on hardware)
MAX_MEMORY_MB=32768        # 32GB
MAX_CPU_CORES=16
```

### PostgreSQL Connection Pool Scaling

**Critical for 50+ workers**: Tune connection pool to prevent bottlenecks.

#### Default Configuration (up to 50 workers)

```yaml
# config-postgres.yaml
database:
  max_open_conns: 25      # Total connections to database
  max_idle_conns: 5       # Idle connections kept warm
  conn_max_lifetime: 5m   # Recycle old connections
  conn_max_idle_time: 1m  # Close idle connections
```

**Performance:**
- 25 connections @ 2s poll rate = **50 workers** max
- Each query takes ~500ms including lock contention

#### Production Configuration (50-100 workers)

```yaml
# config-postgres.yaml
database:
  max_open_conns: 50      # 2× default
  max_idle_conns: 10      # 2× idle pool
  conn_max_lifetime: 5m   # Keep existing
  conn_max_idle_time: 1m  # Keep existing
```

**Performance:**
- 50 connections @ 2s poll rate = **100 workers** max
- Handles burst load during job assignments

#### High-Scale Configuration (100-200 workers)

```yaml
# config-postgres.yaml
database:
  max_open_conns: 100     # High concurrency
  max_idle_conns: 20      # Larger warm pool
  conn_max_lifetime: 3m   # Faster recycling
  conn_max_idle_time: 30s # Aggressive cleanup
```

**Performance:**
- 100 connections @ 2s poll rate = **200 workers** max
- Requires PostgreSQL `max_connections = 150+`

#### PostgreSQL Server Configuration

Match your connection pool to PostgreSQL limits:

```bash
# Edit /etc/postgresql/15/main/postgresql.conf
max_connections = 150          # 100 (app) + 50 (admin/monitoring)
shared_buffers = 4GB           # 25% of RAM (16GB server)
effective_cache_size = 12GB    # 75% of RAM
work_mem = 64MB                # Per-query memory
maintenance_work_mem = 512MB   # For VACUUM/ANALYZE

# Restart PostgreSQL
sudo systemctl restart postgresql
```

#### Connection Pool Sizing Formula

```
Required Connections = (Workers × Poll Frequency) / Avg Query Time

Example:
- 100 workers
- Poll every 2 seconds
- Avg query time: 500ms (0.5s)

Connections = (100 workers × 0.5 queries/sec) / (1 / 0.5s)
            = 50 queries/sec / 2 queries/conn/sec
            = 25 connections minimum

Add 20% overhead: 25 × 1.2 = 30 connections
```

#### Monitoring Connection Pool Health

```bash
# 1. Check PostgreSQL active connections
sudo -u postgres psql -c "
  SELECT state, COUNT(*) 
  FROM pg_stat_activity 
  WHERE datname = 'ffrtmp' 
  GROUP BY state;
"

# 2. Watch connection pool metrics (Prometheus)
curl localhost:9090/metrics | grep database_connections

# Expected output:
# database_connections_open 18
# database_connections_idle 4
# database_connections_in_use 14
# database_connections_wait_duration_ms 2.5

# 3. Check for connection starvation
# If wait_duration > 100ms, increase max_open_conns
```

#### Symptoms of Undersized Pool

 **Connection starvation signs:**
- Workers report "timeout waiting for connection"
- Increased job assignment latency (>5s)
- Prometheus metric: `database_connections_wait_duration_ms > 100`
- PostgreSQL logs: "remaining connection slots reserved"

**Fix:** Increase `max_open_conns` by 50%

 **Connection exhaustion signs:**
- PostgreSQL error: "FATAL: sorry, too many clients already"
- Master crashes with "connection refused"
- Zero idle connections in pool

**Fix:** Increase PostgreSQL `max_connections` in `postgresql.conf`

#### Connection Pool Best Practices

 **Do:**
- Start with defaults (25 conns) and scale up as needed
- Monitor `database_connections_wait_duration_ms` metric
- Use `conn_max_lifetime` to prevent stale connections
- Set PostgreSQL `max_connections` = 1.5× `max_open_conns`
- Enable connection pool metrics in production

 **Don't:**
- Set `max_open_conns` higher than PostgreSQL allows
- Use `max_open_conns > 200` (indicates architectural issue)
- Set `max_idle_conns = 0` (causes reconnection overhead)
- Ignore connection wait times in metrics

#### Scaling Beyond 200 Workers

If you need >200 workers, consider:

1. **PgBouncer connection pooler**
   ```bash
   # Install PgBouncer between app and PostgreSQL
   max_client_conn = 200      # Application connections
   default_pool_size = 50     # Database connections
   reserve_pool_size = 10     # Emergency connections
   ```

2. **Redis job queue** (architectural change)
   - Offload job assignment to Redis
   - Reduce database polling load
   - See: `docs/ARCHITECTURE_IMPROVEMENTS.md`

3. **Read replicas** (for read-heavy queries)
   - Route worker polling to read replicas
   - Keep writes on primary
   - Requires query routing logic

### Worker Node Optimization (continued)

### System-Level Tuning

```bash
# 1. Increase file descriptors
echo "* soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "* hard nofile 65536" | sudo tee -a /etc/security/limits.conf

# 2. TCP tuning for high throughput
sudo tee -a /etc/sysctl.conf << EOF
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864
EOF
sudo sysctl -p

# 3. Cgroups v2 optimization
# Adjust in systemd service files:
CPUQuota=1600%             # 16 cores = 1600%
MemoryMax=32G
IOWeight=1000

# 4. Disable swap for consistent performance (optional)
sudo swapoff -a
```

### High-Throughput Configuration

For >10,000 jobs/day:

```bash
# 1. Horizontal scaling
# Add more workers:
./deployment/orchestration/rolling-update.sh \
  --workers worker1,worker2,worker3,worker4,worker5 \
  ...

# 2. Database optimization
# PostgreSQL connection pooling
database:
  max_connections: 200
  max_idle_connections: 50

# 3. Dedicated queue for high-priority jobs
# Use LIVE queue for low-latency jobs
./bin/ffrtmp jobs submit --scenario test --queue live

# 4. Redis for job queue (future enhancement)
# Currently uses database, Redis would be faster

# 5. Monitoring and auto-scaling
# Use Prometheus + Alertmanager to trigger scaling
```

### Low-Latency Configuration

For sub-minute job processing:

```bash
# Master: Aggressive scheduling
scheduler:
  interval: 2s
  batch_size: 50

# Workers: Frequent polling
HEARTBEAT_INTERVAL=5s
JOB_TIMEOUT=300s

# Use LIVE queue
api:
  default_queue: live
```

---

##  Additional Resources

### Documentation

- **`docs/DEPLOYMENT_IMPROVEMENTS.md`** - Complete deployment system guide
- **`docs/TLS_SETUP_GUIDE.md`** - TLS/SSL configuration and troubleshooting
- **`docs/PRODUCTION_CHECKLIST.md`** - Production readiness checklist
- **`DEPLOY_QUICKREF.md`** - Quick reference guide
- **`ansible/ANSIBLE_GUIDE.md`** - Ansible automation guide
- **`docs/API.md`** - REST API documentation
- **`docs/ARCHITECTURE.md`** - System architecture overview
- **`docs/SECURITY.md`** - Security best practices
- **`CHANGELOG.md`** - Version history and breaking changes

### Deployment Tools

| Tool | Purpose | Documentation |
|------|---------|---------------|
| `deployment-wizard.sh` | Interactive guided deployment | Built-in help |
| `deploy.sh` | Unified deployment script | `./deploy.sh --help` |
| `preflight-check.sh` | Pre-deployment validation | `docs/DEPLOYMENT_IMPROVEMENTS.md` |
| `health-check.sh` | Post-deployment verification | `docs/DEPLOYMENT_IMPROVEMENTS.md` |
| `blue-green-deploy.sh` | Zero-downtime deployment | `docs/DEPLOYMENT_IMPROVEMENTS.md` |
| `rolling-update.sh` | Worker rolling updates | `docs/DEPLOYMENT_IMPROVEMENTS.md` |
| `validate-and-rollback.sh` | Backup and restore | `docs/DEPLOYMENT_IMPROVEMENTS.md` |
| `generate-certs.sh` | TLS certificate generation | `docs/TLS_SETUP_GUIDE.md` |

### Configuration Files

| File | Description | Template |
|------|-------------|----------|
| `/etc/ffrtmp-master/config.yaml` | Master configuration | `deployment/configs/master-prod.yaml` |
| `/etc/ffrtmp/worker.env` | Worker environment | `deployment/configs/worker-prod.env` |
| `/etc/ffrtmp/watch-config.yaml` | Watch daemon config | `deployment/systemd/watch-config.yaml.example` |

### Service Management

```bash
# Check service status
sudo systemctl status ffrtmp-master
sudo systemctl status ffrtmp-worker
sudo systemctl status ffrtmp-watch

# View logs
sudo journalctl -u ffrtmp-master -f
sudo journalctl -u ffrtmp-worker -f
sudo journalctl -u ffrtmp-watch -f

# Restart services
sudo systemctl restart ffrtmp-master
sudo systemctl restart ffrtmp-worker
sudo systemctl restart ffrtmp-watch

# Enable/disable auto-start
sudo systemctl enable ffrtmp-master
sudo systemctl disable ffrtmp-worker
```

### CLI Commands

```bash
# Job management
./bin/ffrtmp jobs submit --scenario test
./bin/ffrtmp jobs list
./bin/ffrtmp jobs get <job-id>
./bin/ffrtmp jobs cancel <job-id>

# Node management
./bin/ffrtmp nodes list
./bin/ffrtmp nodes get <node-id>
./bin/ffrtmp nodes stats

# System info
./bin/ffrtmp version
./bin/ffrtmp health
```

### Support and Community

- **Issues**: [GitHub Issues](https://github.com/YOUR-ORG/ffmpeg-rtmp/issues)
- **Discussions**: [GitHub Discussions](https://github.com/YOUR-ORG/ffmpeg-rtmp/discussions)
- **Security**: See `SECURITY.md` for reporting vulnerabilities
- **Contributing**: See `CONTRIBUTING.md` for contribution guidelines

---

##  Quick Reference by Use Case

### I want to... Then use...

| Goal | Command/Method |
|------|----------------|
| **Deploy for the first time** | `./deployment/deployment-wizard.sh` |
| **Deploy master only** | `sudo ./deploy.sh --master --non-interactive` |
| **Deploy worker only** | `sudo ./deploy.sh --worker --master-url URL --api-key KEY` |
| **Check if system is ready** | `./deployment/checks/preflight-check.sh --master` |
| **Verify deployment worked** | `./deployment/checks/health-check.sh --master` |
| **Update master with no downtime** | `./deployment/orchestration/blue-green-deploy.sh` |
| **Update multiple workers** | `./deployment/orchestration/rolling-update.sh` |
| **Rollback to previous version** | `./deployment/orchestration/blue-green-deploy.sh --rollback` |
| **Deploy to 10+ servers** | Ansible: `ansible-playbook playbooks/site.yml` |
| **Generate TLS certificates** | `./deployment/generate-certs.sh --type master` |
| **Validate configuration** | `./deployment/checks/config-validator.sh config.yaml` |
| **Backup before upgrade** | `./deployment/validate-and-rollback.sh --validate` |
| **Restore from backup** | `./deployment/validate-and-rollback.sh --rollback` |
| **Test the system** | `./bin/ffrtmp jobs submit --scenario test` |
| **Check logs for errors** | `sudo journalctl -u ffrtmp-master -n 100 --no-pager` |
| **Monitor metrics** | `curl http://localhost:9090/metrics` |

---

##  Deployment Checklist

### Pre-Deployment
- [ ] Read `docs/DEPLOYMENT_IMPROVEMENTS.md`
- [ ] Choose deployment method (wizard, manual, Ansible)
- [ ] Run pre-flight checks
- [ ] Prepare configuration files
- [ ] Generate or obtain TLS certificates
- [ ] Set up firewall rules
- [ ] Configure backups

### During Deployment
- [ ] Deploy master node first
- [ ] Verify master health
- [ ] Generate and save API key
- [ ] Deploy worker nodes
- [ ] Verify worker registration
- [ ] Test job submission
- [ ] Configure monitoring

### Post-Deployment
- [ ] Run health checks
- [ ] Verify all services running
- [ ] Test API endpoints
- [ ] Submit test job and verify completion
- [ ] Review logs for errors
- [ ] Set up log rotation
- [ ] Configure database backups
- [ ] Set up monitoring alerts
- [ ] Document deployment (IPs, keys, etc.)
- [ ] Test rollback procedure

### Production Readiness
- [ ] Review `PRODUCTION_CHECKLIST.md`
- [ ] Enable TLS with proper certificates
- [ ] Rotate default API keys
- [ ] Configure firewall rules
- [ ] Set up monitoring and alerting
- [ ] Configure automated backups
- [ ] Test disaster recovery
- [ ] Document runbooks
- [ ] Train operators
- [ ] Schedule regular maintenance

---

##  Deployment Status

**Version**: 2.0.0+  
**Status**:  Production-Ready  
**Last Updated**: 2026-01-07

### What's New in Deployment System v2.0

-  Interactive deployment wizard
-  Pre-flight system validation
-  Post-deployment health checks
-  Configuration validation
-  Blue-green deployments (zero downtime)
-  Rolling worker updates
-  Automated backups and rollback
-  TLS certificate generation
-  Environment-specific configs (dev/prod)
-  Comprehensive documentation
-  GitHub Actions CI/CD integration
-  Ansible automation for multi-server deployments

### Previous Versions

See `CHANGELOG.md` for complete version history.

---

**Next Steps**:

1.  **Quick Start**: Run `./deployment/deployment-wizard.sh`
2.  **Learn More**: Read `docs/DEPLOYMENT_IMPROVEMENTS.md`
3.  **Advanced**: Explore `ansible/ANSIBLE_GUIDE.md`
4.  **Troubleshoot**: See troubleshooting section above
5.  **Get Help**: Open a GitHub issue

---

**Happy Deploying! **
