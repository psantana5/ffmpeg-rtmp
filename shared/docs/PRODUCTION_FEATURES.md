# Production-Ready Features Guide

This document explains the production-ready features that have been implemented in v2.2+ of the FFmpeg RTMP distributed system.

## Overview

As of v2.2, the master node now defaults to production-ready configuration with:
- ✅ **TLS/HTTPS encryption** enabled by default
- ✅ **SQLite persistence** instead of in-memory storage
- ✅ **Automatic job retry** on failure (up to 3 attempts)
- ✅ **Prometheus metrics** endpoint for monitoring
- ✅ **Required API authentication** via environment variable

## Feature Details

### 1. TLS/HTTPS Security

**Status**: ✅ Enabled by default

**Configuration**:
```bash
# TLS is ON by default
./bin/master --port 8080

# Auto-generates self-signed cert if missing at:
#   certs/master.crt
#   certs/master.key

# Use custom certificates
./bin/master --tls --cert /path/to/cert.crt --key /path/to/key.key

# Disable TLS for development (NOT recommended)
./bin/master --tls=false
```

**mTLS Support** (client certificates):
```bash
# Require client certificates for authentication
./bin/master --mtls --ca /path/to/ca.crt
```

**Generate Self-Signed Certificate**:
```bash
# Generate and exit
./bin/master --generate-cert

# Or let master auto-generate on first run
```

**Production Recommendations**:
- Use Let's Encrypt or your organization's CA for production certificates
- Deploy nginx reverse proxy for TLS termination
- Use mTLS for agent-to-master authentication

### 2. API Authentication

**Status**: ✅ Required by default

**Configuration**:
```bash
# Set via environment variable (RECOMMENDED)
export MASTER_API_KEY=$(openssl rand -base64 32)
./bin/master

# Or via command-line flag
./bin/master --api-key "your-secure-api-key-here"
```

**Agent Configuration**:
```bash
# Agent must provide matching API key
export MASTER_API_KEY="same-key-as-master"
./bin/agent --register --master https://master:8080 --api-key "$MASTER_API_KEY"
```

**Security**:
- API key transmitted in `Authorization: Bearer <key>` header
- Uses constant-time comparison to prevent timing attacks
- Health endpoint (`/health`) excluded from authentication

**Production Recommendations**:
- Store API key in secrets manager (AWS Secrets Manager, HashiCorp Vault, etc.)
- Rotate API key periodically
- Use different keys for different environments (dev/staging/prod)

### 3. SQLite Persistence

**Status**: ✅ Enabled by default

**Configuration**:
```bash
# Default: Uses master.db in current directory
./bin/master

# Custom database path
./bin/master --db /var/lib/ffmpeg-rtmp/master.db

# Use in-memory (development only)
./bin/master --db ""
```

**What's Persisted**:
- Node registrations and status
- Job queue and history
- Job retry counts
- Timestamps and metadata

**Benefits**:
- Survives master restart
- No data loss on crash
- Query history for auditing
- Support for analytics

**Production Recommendations**:
- Use dedicated database directory: `/var/lib/ffmpeg-rtmp/`
- Regular backups (SQLite backup command or file copy)
- Monitor database size and implement rotation if needed
- Consider PostgreSQL for multi-master setups (future)

### 4. Automatic Job Retry

**Status**: ✅ Enabled by default (3 attempts)

**Configuration**:
```bash
# Default: 3 retries
./bin/master

# Custom retry count
./bin/master --max-retries 5

# Disable retries
./bin/master --max-retries 0
```

**How It Works**:
1. Agent executes job and reports failure
2. Master checks retry count < max_retries
3. If eligible, master re-queues job with incremented retry count
4. Another agent picks up the retried job
5. Process repeats until success or max retries reached

**Retry Behavior**:
- Original job marked as "failed" (for auditing)
- New job created with same parameters + retry counter
- Exponential backoff: automatic via queue depth
- Retry count tracked in database

**Logging**:
```
Job 550e8400-... failed on node abc123 (attempt 1/3) - re-queued for retry
Job 550e8400-... failed on node def456 (attempt 2/3) - re-queued for retry
Job 550e8400-... failed after 3 attempts - max retries reached
```

**Production Recommendations**:
- Set retry count based on workload reliability
- Monitor retry metrics (see Prometheus metrics below)
- Investigate patterns in retried jobs
- Consider transient failures (network, resource contention)

### 5. Prometheus Metrics

**Status**: ✅ Enabled by default on port 9090

**Configuration**:
```bash
# Default: Enabled on port 9090
./bin/master

# Custom metrics port
./bin/master --metrics-port 9091

# Disable metrics
./bin/master --metrics=false
```

**Metrics Endpoint**:
```bash
# Query metrics
curl http://localhost:9090/metrics

# Health check
curl http://localhost:9090/health
```

**Available Metrics**:
```prometheus
# Master uptime
ffmpeg_master_uptime_seconds 3600

# Node metrics
ffmpeg_master_nodes_total 5
ffmpeg_master_nodes_by_status{status="available"} 3
ffmpeg_master_nodes_by_status{status="busy"} 2
ffmpeg_master_nodes_by_status{status="offline"} 0

# Job metrics
ffmpeg_master_jobs_total 150
ffmpeg_master_jobs_by_status{status="pending"} 5
ffmpeg_master_jobs_by_status{status="running"} 2
ffmpeg_master_jobs_by_status{status="completed"} 140
ffmpeg_master_jobs_by_status{status="failed"} 3

# Retry metrics
ffmpeg_master_job_retries_total 8

# Cluster capacity
ffmpeg_master_cluster_cpu_threads 128
ffmpeg_master_cluster_ram_bytes 274877906944
ffmpeg_master_cluster_gpu_nodes 2
```

**Prometheus Scrape Config**:
```yaml
scrape_configs:
  - job_name: 'ffmpeg-master'
    static_configs:
      - targets: ['master:9090']
    scrape_interval: 15s
```

**Grafana Queries**:
```promql
# Job completion rate
rate(ffmpeg_master_jobs_by_status{status="completed"}[5m])

# Retry rate
rate(ffmpeg_master_job_retries_total[5m])

# Node availability
sum(ffmpeg_master_nodes_by_status{status="available"})

# Cluster utilization
sum(ffmpeg_master_nodes_by_status{status="busy"}) / sum(ffmpeg_master_nodes_total)
```

**Production Recommendations**:
- Integrate with existing Prometheus instance
- Create alerts for:
  - High retry rate
  - Low node availability
  - Job queue depth
  - Master uptime/health
- Set up Grafana dashboards
- Consider VictoriaMetrics for long-term storage

### 6. Structured Logging

**Status**: ✅ JSON logging available

**Configuration**:
```bash
# Standard text logging (default)
./bin/master

# JSON structured logging (future)
# Currently outputs to stdout/stderr
# Captured by systemd journald
```

**Log Format** (systemd journal):
```
[2025-12-30 08:30:15] INFO: Starting FFmpeg RTMP Distributed Master Node
[2025-12-30 08:30:15] INFO: Port: 8080
[2025-12-30 08:30:15] INFO: Max Retries: 3
[2025-12-30 08:30:15] INFO: ✓ Persistent storage enabled
[2025-12-30 08:30:15] INFO: ✓ API authentication enabled
[2025-12-30 08:30:15] INFO: ✓ TLS enabled
[2025-12-30 08:30:15] INFO: ✓ Metrics endpoint enabled
[2025-12-30 08:30:15] INFO: Master node listening on :8080
```

**View Logs**:
```bash
# Systemd journal
sudo journalctl -u ffmpeg-master -f

# Filter by priority
sudo journalctl -u ffmpeg-master -p err

# Since specific time
sudo journalctl -u ffmpeg-master --since "2025-12-30 08:00:00"
```

**Production Recommendations**:
- Ship logs to centralized logging (ELK, Loki, Splunk)
- Set up log rotation (systemd handles this automatically)
- Create alerts on ERROR log entries
- Implement log sampling for high-volume deployments

## Systemd Integration

**Updated Service File**:
```ini
[Service]
Environment="MASTER_API_KEY=your-secure-api-key-here-change-me"

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

**Setup**:
```bash
# Create directories
sudo mkdir -p /var/lib/ffmpeg-rtmp
sudo mkdir -p /etc/ffmpeg-rtmp/certs
sudo chown -R ffmpeg:ffmpeg /var/lib/ffmpeg-rtmp /etc/ffmpeg-rtmp

# Generate certificates
sudo -u ffmpeg /opt/ffmpeg-rtmp/bin/master --generate-cert \
    --cert /etc/ffmpeg-rtmp/certs/master.crt \
    --key /etc/ffmpeg-rtmp/certs/master.key

# Set API key
echo 'MASTER_API_KEY='$(openssl rand -base64 32) | \
    sudo tee -a /etc/systemd/system/ffmpeg-master.service.d/override.conf

# Start service
sudo systemctl enable ffmpeg-master
sudo systemctl start ffmpeg-master

# Verify
sudo systemctl status ffmpeg-master
curl -k https://localhost:8080/health
curl http://localhost:9090/metrics
```

## Migration from v2.1

**Breaking Changes**:
- TLS now enabled by default (was opt-in)
- API key now required (was optional)
- SQLite now default (was in-memory)

**Migration Steps**:

1. **Generate API Key**:
```bash
export MASTER_API_KEY=$(openssl rand -base64 32)
echo "API Key: $MASTER_API_KEY" > /secure/location/api-key.txt
```

2. **Update Master Startup**:
```bash
# Old (v2.1)
./bin/master --port 8080

# New (v2.2) - minimal changes
export MASTER_API_KEY="<your-key>"
./bin/master --port 8080
# TLS auto-generates cert, SQLite uses master.db, retries enabled
```

3. **Update Agent Startup**:
```bash
# Agents need API key and HTTPS
export MASTER_API_KEY="<same-key-as-master>"
./bin/agent --register --master https://master:8080 --api-key "$MASTER_API_KEY"
```

4. **Update Monitoring**:
```bash
# Add Prometheus scrape config
# Target: master:9090
```

5. **Test**:
```bash
# Verify TLS
curl -k https://master:8080/health

# Verify auth (should fail without key)
curl https://master:8080/nodes
# 401 Unauthorized

# Verify auth (with key)
curl -H "Authorization: Bearer $MASTER_API_KEY" https://master:8080/nodes

# Verify metrics
curl http://master:9090/metrics
```

## Backward Compatibility

**Disable Features for Development**:
```bash
# Development mode (like v2.1)
./bin/master --tls=false --db="" --max-retries=0 --metrics=false

# WARNING: Not recommended for production
```

**Environment Variable Override**:
```bash
# If MASTER_API_KEY not set, master will exit with error
# To test without auth (dev only), set empty string:
MASTER_API_KEY="" ./bin/master --tls=false --db=""
# Still requires explicit --tls=false flag
```

## Monitoring Checklist

Set up monitoring for:
- [ ] Master health endpoint (`/health`)
- [ ] Metrics endpoint (`/metrics`)
- [ ] Node availability
- [ ] Job success rate
- [ ] Retry rate
- [ ] Queue depth
- [ ] Database size
- [ ] Certificate expiration
- [ ] Log errors
- [ ] Resource usage (CPU/RAM)

## Security Checklist

- [ ] Change default API key
- [ ] Use production TLS certificates
- [ ] Enable mTLS if possible
- [ ] Restrict network access (firewall)
- [ ] Regular security updates
- [ ] Audit access logs
- [ ] Rotate API keys periodically
- [ ] Monitor for anomalies
- [ ] Backup encryption keys
- [ ] Document incident response

## Performance Tuning

**Database**:
```bash
# Vacuum database periodically
sqlite3 master.db 'VACUUM;'

# Check database size
ls -lh master.db
```

**Metrics**:
```bash
# If metrics impact performance, disable or use separate port
./bin/master --metrics-port 9091
```

**Retries**:
```bash
# Reduce retry count if jobs are consistently failing
./bin/master --max-retries 1
```

**TLS**:
```bash
# Use TLS 1.3 only (modern clients)
# Configured in pkg/tls/tls.go - update MinVersion
```

## Troubleshooting

**TLS Issues**:
```bash
# Check certificate
openssl x509 -in certs/master.crt -text -noout

# Test TLS connection
openssl s_client -connect localhost:8080

# Regenerate certificate
rm certs/master.*
./bin/master --generate-cert
```

**Auth Issues**:
```bash
# Verify API key is set
echo $MASTER_API_KEY

# Test with curl
curl -v -H "Authorization: Bearer $MASTER_API_KEY" https://master:8080/nodes
```

**Database Issues**:
```bash
# Check database integrity
sqlite3 master.db 'PRAGMA integrity_check;'

# Reset database (WARNING: deletes all data)
rm master.db
./bin/master
```

**Metrics Issues**:
```bash
# Check metrics port is not in use
lsof -i :9090

# Test metrics endpoint
curl http://localhost:9090/metrics
```

## Future Enhancements

Planned for v2.3+:
- OpenTelemetry tracing
- Distributed tracing across master-agent
- Advanced retry strategies (exponential backoff, jitter)
- PostgreSQL support for multi-master
- Kubernetes operators
- Health check improvements (deep health vs shallow)
- Metrics aggregation across multiple masters

## References

- [Internal Architecture](INTERNAL_ARCHITECTURE.md)
- [Deployment Modes](DEPLOYMENT_MODES.md)
- [Systemd Templates](../deployment/)
- [TLS Package](../pkg/tls/)
- [Metrics Package](../pkg/metrics/)

---

**Version**: 2.2  
**Last Updated**: 2025-12-30  
**Status**: Production-Ready ✅
