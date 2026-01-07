# Production Deployment Guide - Master Node

Complete guide for deploying the FFmpeg RTMP master node in production using systemd.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [System Setup](#system-setup)
4. [Building and Installation](#building-and-installation)
5. [TLS Certificate Setup](#tls-certificate-setup)
6. [API Key Configuration](#api-key-configuration)
7. [Systemd Service Configuration](#systemd-service-configuration)
8. [Starting the Service](#starting-the-service)
9. [Monitoring Stack Setup](#monitoring-stack-setup)
10. [Verification and Testing](#verification-and-testing)
11. [Troubleshooting](#troubleshooting)
12. [Security Hardening](#security-hardening)
13. [Maintenance and Operations](#maintenance-and-operations)

---

## Overview

The master node is the central coordination service in a distributed FFmpeg RTMP deployment. It handles:
- Job orchestration and queuing
- Worker node registration and tracking
- Results aggregation
- Metrics collection
- Dashboard hosting

This guide focuses on deploying the master service as a systemd unit for production use.

---

## Prerequisites

### Hardware Requirements

**Minimum:**
- 2 CPU cores
- 4 GB RAM
- 20 GB disk space

**Recommended:**
- 4+ CPU cores
- 8+ GB RAM
- 50+ GB SSD (for metrics retention and database)

### Software Requirements

- **Operating System**: Linux with systemd (Ubuntu 20.04+, Debian 11+, CentOS 8+, etc.)
- **Go**: 1.21 or later (for building the binary)
- **Docker**: 20.10+ (for monitoring stack - VictoriaMetrics and Grafana)
- **Docker Compose**: 2.0+ (for monitoring stack)

### Network Requirements

**Inbound Ports:**
- `8080` - Master HTTP API (required for worker communication)
- `3000` - Grafana dashboard (optional, can use reverse proxy)
- `8428` - VictoriaMetrics (optional, can be internal-only)
- `9090` - Prometheus metrics endpoint (optional, can be internal-only)

**Firewall Configuration (example with ufw):**
```bash
sudo ufw allow 8080/tcp comment 'FFmpeg Master API'
sudo ufw allow 3000/tcp comment 'Grafana Dashboard'
sudo ufw allow 8428/tcp comment 'VictoriaMetrics'
sudo ufw allow 9090/tcp comment 'Prometheus Metrics'
```

---

## System Setup

### 1. Create a Dedicated User

Running the service as a dedicated user improves security:

```bash
# Create ffmpeg user and group
sudo useradd --system --no-create-home --shell /bin/false ffmpeg

# Verify user was created
id ffmpeg
```

### 2. Create Required Directories

```bash
# Installation directory
sudo mkdir -p /opt/ffmpeg-rtmp/{bin,certs}

# Data directory (for SQLite database)
sudo mkdir -p /var/lib/ffmpeg-rtmp

# Configuration directory
sudo mkdir -p /etc/ffmpeg-rtmp/certs

# Set ownership
sudo chown -R ffmpeg:ffmpeg /opt/ffmpeg-rtmp
sudo chown -R ffmpeg:ffmpeg /var/lib/ffmpeg-rtmp
sudo chown -R ffmpeg:ffmpeg /etc/ffmpeg-rtmp
```

### 3. Configure File Permissions

```bash
# Secure the directories
sudo chmod 750 /opt/ffmpeg-rtmp
sudo chmod 750 /var/lib/ffmpeg-rtmp
sudo chmod 750 /etc/ffmpeg-rtmp
sudo chmod 700 /etc/ffmpeg-rtmp/certs  # More restrictive for certificates
```

---

## Building and Installation

### 1. Clone the Repository

```bash
cd /tmp
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
```

### 2. Build the Master Binary

```bash
# Build using Make
make build-master

# Or build directly with Go
go build -o bin/master ./master/cmd/master

# Verify the binary
./bin/master --help
```

### 3. Install the Binary

```bash
# Copy binary to installation directory
sudo cp bin/master /opt/ffmpeg-rtmp/bin/master

# Set ownership and permissions
sudo chown ffmpeg:ffmpeg /opt/ffmpeg-rtmp/bin/master
sudo chmod 755 /opt/ffmpeg-rtmp/bin/master

# Verify installation
sudo -u ffmpeg /opt/ffmpeg-rtmp/bin/master --help
```

---

## TLS Certificate Setup

TLS is **strongly recommended** for production deployments to encrypt communication between the master and worker nodes.

### Option 1: Self-Signed Certificate (Development/Internal Use)

Generate a self-signed certificate with your server's IP address and hostname:

```bash
# Get your server's IP address
SERVER_IP=$(hostname -I | awk '{print $1}')
SERVER_HOSTNAME=$(hostname -s)

# Generate certificate as the ffmpeg user
sudo -u ffmpeg /opt/ffmpeg-rtmp/bin/master \
  --generate-cert \
  --cert /etc/ffmpeg-rtmp/certs/master.crt \
  --key /etc/ffmpeg-rtmp/certs/master.key \
  --cert-ips "$SERVER_IP" \
  --cert-hosts "$SERVER_HOSTNAME"
```

**Output:**
```
Generating self-signed certificate...
Certificate generated successfully
  Certificate: /etc/ffmpeg-rtmp/certs/master.crt
  Key: /etc/ffmpeg-rtmp/certs/master.key
  Additional SANs: [192.168.1.100 server1]
```

**Important Notes:**
- The certificate generation command **only creates the files** - it does not start the master service
- You must configure and start the systemd service separately (see next sections)
- The certificate is valid for 365 days by default

### Option 2: CA-Signed Certificate (Production)

For production with a proper Certificate Authority:

1. Generate a Certificate Signing Request (CSR):
```bash
openssl req -new -newkey rsa:2048 -nodes \
  -keyout /tmp/master.key \
  -out /tmp/master.csr \
  -subj "/CN=$(hostname -f)"
```

2. Submit the CSR to your CA and obtain the signed certificate

3. Install the certificate:
```bash
sudo cp /tmp/master.crt /etc/ffmpeg-rtmp/certs/master.crt
sudo cp /tmp/master.key /etc/ffmpeg-rtmp/certs/master.key
sudo chown ffmpeg:ffmpeg /etc/ffmpeg-rtmp/certs/master.{crt,key}
sudo chmod 600 /etc/ffmpeg-rtmp/certs/master.key
sudo chmod 644 /etc/ffmpeg-rtmp/certs/master.crt
```

### Verify Certificate Files

```bash
# Check files exist and have correct permissions
ls -l /etc/ffmpeg-rtmp/certs/

# Verify certificate content
sudo openssl x509 -in /etc/ffmpeg-rtmp/certs/master.crt -text -noout | grep -A 2 "Subject:"
sudo openssl x509 -in /etc/ffmpeg-rtmp/certs/master.crt -text -noout | grep -A 5 "Subject Alternative Name"
```

---

## API Key Configuration

An API key is **required for production** to authenticate worker nodes and API requests.

### 1. Generate a Secure API Key

```bash
# Generate a 32-byte random key
openssl rand -base64 32
```

**Example output:**
```
y40RRukB1910mHtLtzpXYtpeOsyl/KiInzMlJifRfGk=
```

### 2. Store API Key Securely

**Option A: Environment File (Recommended)**

Create a systemd environment file:

```bash
# Create environment file
sudo bash -c 'cat > /etc/ffmpeg-rtmp/master.env << EOF
MASTER_API_KEY=y40RRukB1910mHtLtzpXYtpeOsyl/KiInzMlJifRfGk=
EOF'

# Secure the file
sudo chown root:ffmpeg /etc/ffmpeg-rtmp/master.env
sudo chmod 640 /etc/ffmpeg-rtmp/master.env
```

**Option B: Direct in Service File**

Alternatively, set the environment variable directly in the systemd service file (less secure as it's visible in process lists).

### 3. Save the API Key

** IMPORTANT:** Save this API key securely. You will need to provide it to all worker nodes that connect to this master.

```bash
# Save to a password manager or secure location
echo "Master API Key: y40RRukB1910mHtLtzpXYtpeOsyl/KiInzMlJifRfGk=" >> ~/master-credentials.txt
chmod 600 ~/master-credentials.txt
```

---

## Systemd Service Configuration

### 1. Copy Service Template

The repository includes a systemd service template:

```bash
# Copy from repository
sudo cp /tmp/ffmpeg-rtmp/master/deployment/ffmpeg-master.service \
  /etc/systemd/system/ffmpeg-master.service
```

### 2. Review Service Configuration

View the service file:

```bash
sudo cat /etc/systemd/system/ffmpeg-master.service
```

### 3. Customize Service File

Edit the service file to match your environment:

**Option A: Using systemctl edit (Recommended)**

```bash
# Edit the service file with systemctl
sudo systemctl edit --full ffmpeg-master.service

# After saving your changes, reload systemd configuration
sudo systemctl daemon-reload
```

**Option B: Using a text editor directly**

```bash
sudo nano /etc/systemd/system/ffmpeg-master.service

# After saving changes, reload systemd configuration
sudo systemctl daemon-reload
```

**Key settings to verify/modify:**

```ini
[Service]
# User and working directory
User=ffmpeg
Group=ffmpeg
WorkingDirectory=/opt/ffmpeg-rtmp

# Environment file (if using Option A for API key)
EnvironmentFile=/etc/ffmpeg-rtmp/master.env

# Or set API key directly (if using Option B)
# Environment="MASTER_API_KEY=your-api-key-here"

# Master service command
ExecStart=/opt/ffmpeg-rtmp/bin/master \
    --port 8080 \
    --db /var/lib/ffmpeg-rtmp/master.db \
    --tls \
    --cert /etc/ffmpeg-rtmp/certs/master.crt \
    --key /etc/ffmpeg-rtmp/certs/master.key \
    --max-retries 3 \
    --metrics \
    --metrics-port 9090
```

### 4. Understand Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | HTTP API port |
| `--db` | `master.db` | SQLite database path (use empty string for in-memory) |
| `--tls` | `true` | Enable TLS/HTTPS |
| `--cert` | `certs/master.crt` | TLS certificate file path |
| `--key` | `certs/master.key` | TLS key file path |
| `--max-retries` | `3` | Maximum job retry attempts on failure |
| `--metrics` | `true` | Enable Prometheus metrics endpoint |
| `--metrics-port` | `9090` | Prometheus metrics port |
| `--api-key` | - | API key (prefer environment variable) |

### 5. Disable TLS for Development (Not Recommended)

If you need to disable TLS for development only:

```ini
ExecStart=/opt/ffmpeg-rtmp/bin/master \
    --port 8080 \
    --db /var/lib/ffmpeg-rtmp/master.db \
    --tls=false \
    --max-retries 3 \
    --metrics \
    --metrics-port 9090
```

** WARNING:** Running without TLS in production exposes your API key and data to network eavesdropping.

---

## Starting the Service

### 1. Reload Systemd Configuration

After creating or modifying the service file:

```bash
sudo systemctl daemon-reload
```

### 2. Enable Service (Start on Boot)

```bash
sudo systemctl enable ffmpeg-master.service
```

### 3. Start the Service

```bash
sudo systemctl start ffmpeg-master.service
```

### 4. Check Service Status

```bash
sudo systemctl status ffmpeg-master.service
```

**Expected output (healthy service):**
```
● ffmpeg-master.service - FFmpeg RTMP Master Coordination Service
     Loaded: loaded (/etc/systemd/system/ffmpeg-master.service; enabled; preset: enabled)
     Active: active (running) since Tue 2025-12-30 12:05:00 CET; 5s ago
       Docs: https://github.com/psantana5/ffmpeg-rtmp
   Main PID: 12345 (master)
      Tasks: 10 (limit: 4915)
     Memory: 15.2M
        CPU: 100ms
     CGroup: /system.slice/ffmpeg-master.service
             └─12345 /opt/ffmpeg-rtmp/bin/master --port 8080 ...

Dec 30 12:05:00 server1 systemd[1]: Started FFmpeg RTMP Master Coordination Service.
Dec 30 12:05:00 server1 ffmpeg-master[12345]: Starting FFmpeg RTMP Distributed Master Node (Production Mode)
Dec 30 12:05:00 server1 ffmpeg-master[12345]: Port: 8080
Dec 30 12:05:00 server1 ffmpeg-master[12345]: ✓ API authentication enabled
Dec 30 12:05:00 server1 ffmpeg-master[12345]: Master node listening on :8080
```

### 5. View Logs

```bash
# Follow logs in real-time
sudo journalctl -u ffmpeg-master.service -f

# View last 50 lines
sudo journalctl -u ffmpeg-master.service -n 50

# View logs since last boot
sudo journalctl -u ffmpeg-master.service -b
```

---

## Monitoring Stack Setup

The master node uses VictoriaMetrics and Grafana for monitoring and visualization.

### 1. Navigate to Repository

```bash
cd /tmp/ffmpeg-rtmp
```

### 2. Start Monitoring Stack

```bash
# Start VictoriaMetrics and Grafana with Docker Compose
make vm-up-build

# Or manually
docker compose up -d victoriametrics grafana
```

### 3. Verify Monitoring Services

```bash
# Check containers are running
docker compose ps

# Expected output:
# NAME                    STATUS
# victoriametrics         Up
# grafana                 Up
```

### 4. Access Dashboards

- **Grafana**: http://your-server-ip:3000
  - Default credentials: `admin` / `admin`
  - Change password on first login
  
- **VictoriaMetrics UI**: http://your-server-ip:8428

### 5. Configure VictoriaMetrics Scraping

Edit the scrape configuration if needed:

```bash
# Edit VictoriaMetrics config
nano master/monitoring/victoriametrics.yml
```

Default scrape targets include:
- Master metrics endpoint (port 9090)
- Node exporters from registered workers

---

## Verification and Testing

### 1. Test Health Endpoint

```bash
# Health check (no authentication required)
curl -k https://localhost:8080/health

# Expected output:
# {"status":"healthy"}
```

### 2. Test Authenticated Endpoints

```bash
# Set API key from your configuration
export MASTER_API_KEY="y40RRukB1910mHtLtzpXYtpeOsyl/KiInzMlJifRfGk="

# List registered nodes (should be empty initially)
curl -k -H "Authorization: Bearer $MASTER_API_KEY" https://localhost:8080/nodes

# Expected output:
# []

# List jobs
curl -k -H "Authorization: Bearer $MASTER_API_KEY" https://localhost:8080/jobs

# Expected output:
# []
```

### 3. Test Metrics Endpoint

```bash
# Get Prometheus metrics (no authentication required)
curl http://localhost:9090/metrics

# Expected output:
# # HELP ffmpeg_master_nodes_total Total number of registered nodes
# # TYPE ffmpeg_master_nodes_total gauge
# ffmpeg_master_nodes_total 0
# ...
```

### 4. Test from Remote Machine

```bash
# Replace with your master server's IP
MASTER_IP="192.168.1.100"
API_KEY="y40RRukB1910mHtLtzpXYtpeOsyl/KiInzMlJifRfGk="

# Test connectivity
curl -k https://$MASTER_IP:8080/health

# Test authentication
curl -k -H "Authorization: Bearer $API_KEY" https://$MASTER_IP:8080/nodes
```

---

## Troubleshooting

### Service Fails to Start

#### Issue: `status=226/NAMESPACE` Error

**Symptom:**
```
Process: 76694 ExecStart=/home/sanpau/Documents/projects/ffmpeg-rtmp/bin/master ... (code=exited, status=226/NAMESPACE)
```

**Cause:** The systemd service file has security hardening features (namespace isolation) that may not be supported on all systems or may conflict with the service requirements.

**Solutions:**

1. **Check systemd version** (namespace features require systemd 232+):
```bash
systemctl --version
```

2. **Temporarily disable namespace isolation** to diagnose:
```bash
# Edit the service file
sudo systemctl edit --full ffmpeg-master.service
```

Comment out or remove these lines:
```ini
# PrivateTmp=yes
# ProtectSystem=strict
# ProtectHome=yes
# NoNewPrivileges=true
```

Then reload and restart:
```bash
sudo systemctl daemon-reload
sudo systemctl restart ffmpeg-master.service
```

3. **Check filesystem permissions**:
```bash
# Verify the ffmpeg user can access required directories
sudo -u ffmpeg ls -l /opt/ffmpeg-rtmp/bin/master
sudo -u ffmpeg ls -l /etc/ffmpeg-rtmp/certs/
sudo -u ffmpeg touch /var/lib/ffmpeg-rtmp/test && sudo -u ffmpeg rm /var/lib/ffmpeg-rtmp/test
```

4. **Adjust `ReadWritePaths` in service file**:
```ini
[Service]
ReadWritePaths=/opt/ffmpeg-rtmp /var/lib/ffmpeg-rtmp /etc/ffmpeg-rtmp /tmp
```

#### Issue: Certificate Not Found

**Symptom:**
```
Failed to load TLS certificate: open /etc/ffmpeg-rtmp/certs/master.crt: no such file or directory
```

**Solution:**
```bash
# Verify certificates exist
ls -l /etc/ffmpeg-rtmp/certs/

# If missing, generate certificates
sudo -u ffmpeg /opt/ffmpeg-rtmp/bin/master \
  --generate-cert \
  --cert /etc/ffmpeg-rtmp/certs/master.crt \
  --key /etc/ffmpeg-rtmp/certs/master.key

# Restart service
sudo systemctl restart ffmpeg-master.service
```

#### Issue: Permission Denied on Database

**Symptom:**
```
Failed to create SQLite store: unable to open database file
```

**Solution:**
```bash
# Check directory permissions
ls -ld /var/lib/ffmpeg-rtmp

# Fix ownership
sudo chown -R ffmpeg:ffmpeg /var/lib/ffmpeg-rtmp

# Verify ffmpeg user can write
sudo -u ffmpeg touch /var/lib/ffmpeg-rtmp/test
sudo -u ffmpeg rm /var/lib/ffmpeg-rtmp/test

# Restart service
sudo systemctl restart ffmpeg-master.service
```

#### Issue: Port Already in Use

**Symptom:**
```
listen tcp :8080: bind: address already in use
```

**Solution:**
```bash
# Find process using the port
sudo lsof -i :8080

# Stop the conflicting process or change the port in service file
sudo systemctl edit --full ffmpeg-master.service
# Change --port 8080 to --port 8081

sudo systemctl daemon-reload
sudo systemctl restart ffmpeg-master.service
```

### Worker Cannot Connect

#### Issue: Connection Refused

**Symptom:**
```
Failed to register: connection refused
```

**Solution:**
```bash
# Check master is listening
sudo ss -tlnp | grep 8080

# Check firewall
sudo ufw status
sudo ufw allow 8080/tcp

# Test from master server
curl -k https://localhost:8080/health
```

#### Issue: Certificate Verification Failed

**Symptom:**
```
x509: certificate signed by unknown authority
```

**Solution for Workers:**
```bash
# Option 1: Use insecure flag for self-signed certificates (development only)
./bin/agent --register --master https://192.168.1.100:8080 \
  --api-key "$MASTER_API_KEY" \
  --insecure-skip-verify

# Option 2: Distribute CA certificate to workers (production)
# Copy master certificate to worker
scp master:/etc/ffmpeg-rtmp/certs/master.crt /tmp/master-ca.crt

# Use CA certificate
./bin/agent --register --master https://192.168.1.100:8080 \
  --api-key "$MASTER_API_KEY" \
  --ca /tmp/master-ca.crt
```

#### Issue: Authentication Failed

**Symptom:**
```
Missing Authorization header
Invalid API key
```

**Solution:**
```bash
# Verify API key matches between master and worker
# On master:
sudo cat /etc/ffmpeg-rtmp/master.env

# On worker, use the exact same key:
export MASTER_API_KEY="<same-key-from-master>"
./bin/agent --register --master https://master:8080
```

### Monitoring Stack Issues

#### Issue: Grafana Not Accessible

**Solution:**
```bash
# Check Grafana container status
docker compose ps grafana

# Check logs
docker compose logs grafana

# Restart if needed
docker compose restart grafana
```

#### Issue: VictoriaMetrics Not Scraping

**Solution:**
```bash
# Check VictoriaMetrics targets
curl http://localhost:8428/targets

# Verify master metrics endpoint is accessible
curl http://localhost:9090/metrics

# Check VictoriaMetrics logs
docker compose logs victoriametrics
```

### Performance Issues

#### Issue: High Memory Usage

**Solution:**
```bash
# Check current memory usage
sudo systemctl status ffmpeg-master.service

# Adjust memory limit in service file
sudo systemctl edit --full ffmpeg-master.service
# Modify: MemoryLimit=2G (increase if needed)

sudo systemctl daemon-reload
sudo systemctl restart ffmpeg-master.service
```

#### Issue: Database Growing Too Large

**Solution:**
```bash
# Check database size
ls -lh /var/lib/ffmpeg-rtmp/master.db

# Clean up old jobs and results (if supported)
# For now, backup and reset:
sudo systemctl stop ffmpeg-master.service
sudo mv /var/lib/ffmpeg-rtmp/master.db /var/lib/ffmpeg-rtmp/master.db.backup
sudo systemctl start ffmpeg-master.service
```

---

## Security Hardening

### 1. Use TLS/HTTPS Always

Never run in production without TLS:

```bash
# Verify TLS is enabled in service file
grep "tls" /etc/systemd/system/ffmpeg-master.service
```

### 2. Rotate API Keys Regularly

```bash
# Generate new key
NEW_KEY=$(openssl rand -base64 32)

# Update master environment file
sudo bash -c "echo 'MASTER_API_KEY=$NEW_KEY' > /etc/ffmpeg-rtmp/master.env"

# Restart master
sudo systemctl restart ffmpeg-master.service

# Update all workers with new key
```

### 3. Use Mutual TLS (mTLS) for Maximum Security

Generate client certificates for each worker and require them:

```bash
# Start master with mTLS
ExecStart=/opt/ffmpeg-rtmp/bin/master \
    --port 8080 \
    --tls \
    --mtls \
    --ca /etc/ffmpeg-rtmp/certs/ca.crt \
    --cert /etc/ffmpeg-rtmp/certs/master.crt \
    --key /etc/ffmpeg-rtmp/certs/master.key
```

### 4. Restrict Network Access

```bash
# Allow only specific worker IPs
sudo ufw allow from 192.168.1.0/24 to any port 8080 proto tcp

# Block all other access
sudo ufw default deny incoming
```

### 5. Enable Audit Logging

```bash
# View all API access
sudo journalctl -u ffmpeg-master.service | grep "POST\|GET\|PUT\|DELETE"
```

### 6. Keep System Updated

```bash
# Regular system updates
sudo apt update && sudo apt upgrade -y

# Rebuild master binary regularly for security patches
cd /tmp/ffmpeg-rtmp
git pull
make build-master
sudo systemctl stop ffmpeg-master.service
sudo cp bin/master /opt/ffmpeg-rtmp/bin/master
sudo systemctl start ffmpeg-master.service
```

---

## Maintenance and Operations

### Regular Backups

```bash
# Backup database
sudo systemctl stop ffmpeg-master.service
sudo cp /var/lib/ffmpeg-rtmp/master.db /backup/master-$(date +%Y%m%d).db
sudo systemctl start ffmpeg-master.service

# Backup certificates
sudo tar -czf /backup/certs-$(date +%Y%m%d).tar.gz /etc/ffmpeg-rtmp/certs/
```

### Log Rotation

Systemd/journald handles log rotation automatically, but you can configure retention:

```bash
# Edit journald configuration
sudo nano /etc/systemd/journald.conf

# Set retention (example: 30 days, 2GB max)
SystemMaxUse=2G
MaxRetentionSec=2592000

# Restart journald
sudo systemctl restart systemd-journald
```

### Monitoring Service Health

```bash
# Create a simple health check script
cat > /usr/local/bin/check-master-health.sh << 'EOF'
#!/bin/bash
if curl -sf -k https://localhost:8080/health > /dev/null; then
  echo "Master is healthy"
  exit 0
else
  echo "Master is unhealthy"
  exit 1
fi
EOF

chmod +x /usr/local/bin/check-master-health.sh

# Add to cron for periodic checks
(crontab -l 2>/dev/null; echo "*/5 * * * * /usr/local/bin/check-master-health.sh || systemctl restart ffmpeg-master.service") | crontab -
```

### Upgrading

```bash
# 1. Stop the service
sudo systemctl stop ffmpeg-master.service

# 2. Backup current binary
sudo cp /opt/ffmpeg-rtmp/bin/master /opt/ffmpeg-rtmp/bin/master.backup

# 3. Build and install new version
cd /tmp/ffmpeg-rtmp
git pull
make build-master
sudo cp bin/master /opt/ffmpeg-rtmp/bin/master
sudo chown ffmpeg:ffmpeg /opt/ffmpeg-rtmp/bin/master

# 4. Start the service
sudo systemctl start ffmpeg-master.service

# 5. Verify
sudo systemctl status ffmpeg-master.service
curl -k https://localhost:8080/health
```

### Uninstalling

```bash
# Stop and disable service
sudo systemctl stop ffmpeg-master.service
sudo systemctl disable ffmpeg-master.service

# Remove service file
sudo rm /etc/systemd/system/ffmpeg-master.service
sudo systemctl daemon-reload

# Remove files
sudo rm -rf /opt/ffmpeg-rtmp
sudo rm -rf /var/lib/ffmpeg-rtmp
sudo rm -rf /etc/ffmpeg-rtmp

# Remove user
sudo userdel ffmpeg
```

---

## Next Steps

After successfully deploying the master node:

1. **Deploy Worker Nodes** - See [../worker/README.md](../worker/README.md) for worker deployment guide
2. **Configure Dashboards** - Customize Grafana dashboards for your metrics
3. **Set Up Alerts** - Configure Alertmanager for critical thresholds
4. **Test Job Submission** - Submit test jobs to verify the full pipeline
5. **Configure Reverse Proxy** - Use nginx or Caddy for production SSL termination

---

## Additional Resources

- [Master Components Overview](../master/README.md)
- [Distributed Architecture Guide](../docs/QUICKSTART_DISTRIBUTED.md)
- [Worker Deployment Guide](../worker/README.md)
- [Troubleshooting Guide](../docs/troubleshooting.md)

---

## Support

If you encounter issues not covered in this guide:

1. Check logs: `sudo journalctl -u ffmpeg-master.service -n 100`
2. Verify configuration: Review all paths and environment variables
3. Test connectivity: Ensure firewall and network settings are correct
4. Open an issue: https://github.com/psantana5/ffmpeg-rtmp/issues

Include in your issue:
- Master logs (last 50 lines)
- Service status output
- System information (OS, systemd version)
- Configuration files (redact sensitive information)

---

**Document Version:** 1.0  
**Last Updated:** 2025-12-30  
**Tested On:** Ubuntu 22.04, Debian 12, CentOS Stream 9
