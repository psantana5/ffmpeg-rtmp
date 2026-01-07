# TLS/SSL Certificate Setup Guide

## Overview

This guide covers TLS/SSL certificate generation and configuration for FFmpeg-RTMP deployments. The system supports:

- **Self-signed certificates** - Quick setup for testing/development
- **CA-signed certificates** - Internal CA for production environments
- **Custom certificates** - Bring your own certificates from external CA
- **mTLS (Mutual TLS)** - Client certificate authentication

## Quick Start

### Generate Certificates

```bash
# Generate master server certificates
./deployment/generate-certs.sh --master \
  --master-ip 10.0.0.1 \
  --master-host master.example.com

# Generate worker certificates
./deployment/generate-certs.sh --worker \
  --worker-ip 10.0.0.10 \
  --worker-host worker01.example.com

# Generate both with CA (for mTLS)
./deployment/generate-certs.sh --both --ca
```

### Deploy with TLS

```bash
# Deploy master with automatic certificate generation
sudo ./deploy.sh --master --generate-certs \
  --master-ip 10.0.0.1 \
  --master-host master.example.com

# Deploy worker with TLS
sudo ./deploy.sh --worker \
  --master-url https://master.example.com:8443 \
  --api-key <key> \
  --generate-certs
```

## Certificate Generation Script

### Syntax

```bash
./deployment/generate-certs.sh [MODE] [OPTIONS]
```

### Modes

| Mode | Description |
|------|-------------|
| `--master` | Generate master server certificates |
| `--worker` or `--agent` | Generate worker/agent certificates |
| `--both` | Generate both master and worker certificates |
| `--ca` | Generate CA certificate (for mTLS) |

### Options

| Option | Description | Example |
|--------|-------------|---------|
| `--output DIR` | Output directory | `--output /etc/ffrtmp/certs` |
| `--master-cn NAME` | Master common name | `--master-cn master.local` |
| `--worker-cn NAME` | Worker common name | `--worker-cn worker01` |
| `--master-ip IP` | Master IP (multiple allowed) | `--master-ip 10.0.0.1` |
| `--master-host HOST` | Master hostname | `--master-host master.example.com` |
| `--worker-ip IP` | Worker IP | `--worker-ip 10.0.0.10` |
| `--worker-host HOST` | Worker hostname | `--worker-host worker01.example.com` |
| `--days DAYS` | Validity in days | `--days 730` (2 years) |

### Examples

#### Development (Single Machine)

```bash
# Simple self-signed certificates
./deployment/generate-certs.sh --both --output certs
```

#### Production (Multiple Machines)

```bash
# Master with specific IP and hostname
./deployment/generate-certs.sh --master \
  --master-ip 10.0.0.1 \
  --master-host master.example.com \
  --master-host master.internal \
  --output /etc/ffrtmp-master/certs

# Worker nodes (repeat for each worker)
./deployment/generate-certs.sh --worker \
  --worker-ip 10.0.0.10 \
  --worker-host worker01.example.com \
  --output /etc/ffrtmp/certs
```

#### Production with mTLS

```bash
# Generate CA and all certificates
./deployment/generate-certs.sh --both --ca \
  --master-ip 10.0.0.1 \
  --master-host master.example.com \
  --worker-ip 10.0.0.10 \
  --worker-host worker01.example.com \
  --output certs

# Copy CA to all nodes
sudo cp certs/ca.crt /etc/ffrtmp-master/certs/
sudo cp certs/ca.crt /etc/ffrtmp/certs/
```

## Configuration

### Master Node Configuration

#### Enable TLS

Edit `/etc/ffrtmp-master/master.env`:

```bash
# TLS Configuration
TLS_ENABLED=true
TLS_CERT=/etc/ffrtmp-master/certs/master.crt
TLS_KEY=/etc/ffrtmp-master/certs/master.key

# Port (typically 8443 for HTTPS)
PORT=8443

# Optional: Client certificate verification (mTLS)
# TLS_CLIENT_CA=/etc/ffrtmp-master/certs/ca.crt
# TLS_REQUIRE_CLIENT_CERT=true
```

#### Install Certificates

```bash
# Create directory
sudo mkdir -p /etc/ffrtmp-master/certs

# Copy certificates
sudo cp certs/master.crt /etc/ffrtmp-master/certs/
sudo cp certs/master.key /etc/ffrtmp-master/certs/

# Optional: CA certificate for mTLS
sudo cp certs/ca.crt /etc/ffrtmp-master/certs/

# Set permissions
sudo chmod 600 /etc/ffrtmp-master/certs/master.key
sudo chmod 644 /etc/ffrtmp-master/certs/master.crt
sudo chown -R ffrtmp-master:ffrtmp-master /etc/ffrtmp-master/certs

# Restart service
sudo systemctl restart ffrtmp-master
```

### Worker Node Configuration

#### Enable TLS

Edit `/etc/ffrtmp/worker.env`:

```bash
# Master URL with HTTPS
MASTER_URL=https://master.example.com:8443

# API Key
API_KEY=your-api-key-here

# Optional: CA certificate to verify master
TLS_CA=/etc/ffrtmp/certs/ca.crt

# Optional: Client certificate for mTLS
# TLS_CERT=/etc/ffrtmp/certs/agent.crt
# TLS_KEY=/etc/ffrtmp/certs/agent.key

# Optional: Skip TLS verification (insecure, dev only)
# INSECURE_SKIP_VERIFY=true
```

#### Install Certificates

```bash
# Create directory
sudo mkdir -p /etc/ffrtmp/certs

# Copy CA certificate (to verify master)
sudo cp certs/ca.crt /etc/ffrtmp/certs/

# Optional: Client certificate for mTLS
sudo cp certs/agent.crt /etc/ffrtmp/certs/
sudo cp certs/agent.key /etc/ffrtmp/certs/

# Set permissions
sudo chmod 644 /etc/ffrtmp/certs/*.crt
sudo chmod 600 /etc/ffrtmp/certs/*.key
sudo chown -R ffrtmp:ffrtmp /etc/ffrtmp/certs

# Restart services
sudo systemctl restart ffrtmp-worker ffrtmp-watch
```

## Deployment Integration

### Automatic Certificate Generation

The deployment scripts support automatic certificate generation with the `--generate-certs` flag:

```bash
# Master with certificates
sudo ./deploy.sh --master --generate-certs \
  --master-ip 10.0.0.1 \
  --master-host master.example.com

# Worker with certificates
sudo ./deploy.sh --worker \
  --master-url https://master.example.com:8443 \
  --api-key <key> \
  --generate-certs
```

### Interactive Mode

When deploying interactively, you'll be prompted:

```
Generate CA certificate for mTLS? [y/N]
```

Answer `y` to enable mutual TLS authentication.

## Certificate Management

### Verify Certificates

```bash
# Check certificate details
openssl x509 -in /etc/ffrtmp-master/certs/master.crt -noout -text

# Check expiration
openssl x509 -in /etc/ffrtmp-master/certs/master.crt -noout -dates

# Verify certificate matches key
openssl x509 -noout -modulus -in master.crt | openssl md5
openssl rsa -noout -modulus -in master.key | openssl md5
# (Hashes should match)

# Test TLS connection
openssl s_client -connect master.example.com:8443
```

### Renew Certificates

```bash
# Generate new certificates with same settings
./deployment/generate-certs.sh --master \
  --master-ip 10.0.0.1 \
  --master-host master.example.com \
  --days 730 \
  --output /tmp/new-certs

# Backup old certificates
sudo cp -r /etc/ffrtmp-master/certs /etc/ffrtmp-master/certs.backup

# Install new certificates
sudo cp /tmp/new-certs/master.{crt,key} /etc/ffrtmp-master/certs/
sudo chmod 600 /etc/ffrtmp-master/certs/master.key

# Restart service
sudo systemctl restart ffrtmp-master
```

### Certificate Rotation

For automated certificate rotation, consider:

1. **Let's Encrypt** with certbot (for public domains)
2. **Internal PKI** with HashiCorp Vault
3. **cert-manager** (Kubernetes deployments)
4. **Cron jobs** for periodic regeneration

## Security Best Practices

### Certificate Generation

-  Use 2048-bit RSA keys minimum (4096-bit for CA)
-  Include all relevant SANs (IPs and hostnames)
-  Use strong passwords for key encryption (if encrypting)
-  Limit certificate validity (1 year recommended)
-  Generate unique certificates per node

### Storage

-  Store private keys with `600` permissions
-  Store certificates with `644` permissions
-  Use dedicated directories (`/etc/ffrtmp*/certs`)
-  Set proper ownership (ffrtmp-master, ffrtmp users)
-  Backup certificates securely
-  Never commit certificates to version control

### Deployment

-  Use TLS 1.2 or higher (configured in code)
-  Use strong cipher suites (configured in code)
-  Enable mTLS for production environments
-  Verify certificates (don't skip verification)
-  Use HTTPS URLs (`https://` not `http://`)
-  Don't use `INSECURE_SKIP_VERIFY` in production

## Troubleshooting

### Certificate Not Trusted

**Problem:** Worker can't connect to master - certificate verification failed

**Solutions:**
```bash
# Option 1: Copy CA certificate to worker
sudo cp ca.crt /etc/ffrtmp/certs/
# Update worker.env: TLS_CA=/etc/ffrtmp/certs/ca.crt

# Option 2: Add to system trust store (Ubuntu/Debian)
sudo cp ca.crt /usr/local/share/ca-certificates/ffrtmp-ca.crt
sudo update-ca-certificates

# Restart worker
sudo systemctl restart ffrtmp-worker
```

### Wrong Hostname/IP

**Problem:** Certificate doesn't match hostname or IP

**Check SANs:**
```bash
openssl x509 -in master.crt -noout -text | grep -A1 "Subject Alternative Name"
```

**Fix:** Regenerate certificate with correct SANs:
```bash
./deployment/generate-certs.sh --master \
  --master-ip <correct-ip> \
  --master-host <correct-hostname>
```

### Permission Denied

**Problem:** Service can't read certificate files

**Fix permissions:**
```bash
# Master
sudo chown -R ffrtmp-master:ffrtmp-master /etc/ffrtmp-master/certs
sudo chmod 600 /etc/ffrtmp-master/certs/*.key
sudo chmod 644 /etc/ffrtmp-master/certs/*.crt

# Worker
sudo chown -R ffrtmp:ffrtmp /etc/ffrtmp/certs
sudo chmod 600 /etc/ffrtmp/certs/*.key
sudo chmod 644 /etc/ffrtmp/certs/*.crt
```

### Certificate Expired

**Check expiration:**
```bash
openssl x509 -in master.crt -noout -dates
```

**Renew:** Follow certificate renewal steps above

## Using External Certificates

If you have certificates from a trusted CA (e.g., Let's Encrypt):

### Master

```bash
# Copy your certificates
sudo cp /path/to/fullchain.pem /etc/ffrtmp-master/certs/master.crt
sudo cp /path/to/privkey.pem /etc/ffrtmp-master/certs/master.key

# Set permissions
sudo chmod 600 /etc/ffrtmp-master/certs/master.key
sudo chown ffrtmp-master:ffrtmp-master /etc/ffrtmp-master/certs/*

# Update config (/etc/ffrtmp-master/master.env)
TLS_ENABLED=true
TLS_CERT=/etc/ffrtmp-master/certs/master.crt
TLS_KEY=/etc/ffrtmp-master/certs/master.key

# Restart
sudo systemctl restart ffrtmp-master
```

### Worker

Workers don't need client certificates unless using mTLS. System CA bundle will verify master's certificate.

```bash
# Update worker.env
MASTER_URL=https://master.example.com:8443
API_KEY=<your-key>

# No TLS_CA needed if using publicly trusted certificate

# Restart
sudo systemctl restart ffrtmp-worker
```

## Generated Files

| File | Description | Permissions | Location |
|------|-------------|-------------|----------|
| `master.crt` | Master server certificate | 644 | `/etc/ffrtmp-master/certs/` |
| `master.key` | Master private key | 600 | `/etc/ffrtmp-master/certs/` |
| `agent.crt` | Worker client certificate | 644 | `/etc/ffrtmp/certs/` |
| `agent.key` | Worker private key | 600 | `/etc/ffrtmp/certs/` |
| `ca.crt` | CA certificate (mTLS) | 644 | Both locations |
| `ca.key` | CA private key (keep secure!) | 600 | Secure backup only |

## References

- OpenSSL documentation: https://www.openssl.org/docs/
- TLS Best Practices: https://wiki.mozilla.org/Security/Server_Side_TLS
- Let's Encrypt: https://letsencrypt.org/
- Internal PKI: https://smallstep.com/docs/step-ca/

---

**Next Steps:**
- Generate certificates: `./deployment/generate-certs.sh --both --ca`
- Deploy with TLS: `./deploy.sh --master --generate-certs`
- Verify setup: Check service logs and test connections
