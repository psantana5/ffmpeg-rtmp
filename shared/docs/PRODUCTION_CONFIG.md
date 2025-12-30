# Production Configuration for Distributed Compute

This guide shows how to configure and deploy the distributed compute system for production use with mTLS, persistence, and authentication.

## Features

- ✅ **SQLite Persistence**: Jobs and nodes persist across restarts
- ✅ **TLS/mTLS**: Encrypted communication with mutual authentication
- ✅ **API Key Authentication**: Token-based access control
- ✅ **Graceful Shutdown**: Clean shutdown with connection draining
- ✅ **Production Timeouts**: Increased timeouts for stability

## Quick Start (Production Mode)

### 1. Generate Certificates (Self-Signed for Testing)

```bash
# Generate master certificate
./bin/master --generate-cert --cert certs/master.crt --key certs/master.key

# Generate agent certificate (use same command on agent machine)
./bin/master --generate-cert --cert certs/agent.crt --key certs/agent.key
```

For production, use proper CA-signed certificates.

### 2. Start Master with Full Production Features

```bash
./bin/master \
  --port 8443 \
  --db data/master.db \
  --tls \
  --cert certs/master.crt \
  --key certs/master.key \
  --ca certs/ca.crt \
  --mtls \
  --api-key "your-secret-api-key-here"
```

Options explained:
- `--db data/master.db`: SQLite database for persistence
- `--tls`: Enable TLS encryption
- `--cert/--key`: Server certificate and key
- `--ca`: CA certificate for verifying client certificates
- `--mtls`: Require client certificates (mutual TLS)
- `--api-key`: API key for authentication

### 3. Start Agent with TLS and Authentication

```bash
./bin/agent \
  --register \
  --master https://master-host:8443 \
  --cert certs/agent.crt \
  --key certs/agent.key \
  --ca certs/ca.crt \
  --api-key "your-secret-api-key-here"
```

## Configuration Options

### Master Node Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8080` | HTTP/HTTPS port |
| `--db` | `""` | SQLite database path (empty = in-memory) |
| `--tls` | `false` | Enable TLS |
| `--cert` | `certs/master.crt` | TLS certificate file |
| `--key` | `certs/master.key` | TLS key file |
| `--ca` | `""` | CA certificate for mTLS |
| `--mtls` | `false` | Require client certificates |
| `--generate-cert` | `false` | Generate self-signed certificate |
| `--api-key` | `""` | API key for authentication |

### Agent Node Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--master` | `http://localhost:8080` | Master node URL |
| `--register` | `false` | Register with master |
| `--cert` | `""` | Client certificate (for mTLS) |
| `--key` | `""` | Client key (for mTLS) |
| `--ca` | `""` | CA certificate to verify server |
| `--api-key` | `""` | API key for authentication |
| `--poll-interval` | `10s` | Job polling interval |
| `--heartbeat-interval` | `30s` | Heartbeat interval |

## Production Deployment Scenarios

### Scenario 1: HTTP + SQLite (Basic Persistence)

Good for: Development, internal networks

```bash
# Master
./bin/master --db data/master.db

# Agent
./bin/agent --register --master http://master:8080
```

### Scenario 2: HTTPS + API Key (Encrypted + Authenticated)

Good for: Trusted networks with basic security

```bash
# Master
./bin/master \
  --tls \
  --cert certs/master.crt \
  --key certs/master.key \
  --api-key "secret-key-123" \
  --db data/master.db

# Agent
./bin/agent \
  --register \
  --master https://master:8443 \
  --ca certs/ca.crt \
  --api-key "secret-key-123"
```

### Scenario 3: mTLS + SQLite + API Key (Full Production)

Good for: Production environments, untrusted networks

```bash
# Master
./bin/master \
  --port 8443 \
  --tls \
  --mtls \
  --cert certs/master.crt \
  --key certs/master.key \
  --ca certs/ca.crt \
  --api-key "production-api-key" \
  --db data/master.db

# Agent
./bin/agent \
  --register \
  --master https://master:8443 \
  --cert certs/agent.crt \
  --key certs/agent.key \
  --ca certs/ca.crt \
  --api-key "production-api-key"
```

## Certificate Management

### Generate CA and Certificates (Production)

Use a proper PKI tool like `cfssl`, `easy-rsa`, or `openssl`:

```bash
# Generate CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt

# Generate master certificate
openssl genrsa -out master.key 2048
openssl req -new -key master.key -out master.csr
openssl x509 -req -in master.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out master.crt -days 365

# Generate agent certificate
openssl genrsa -out agent.key 2048
openssl req -new -key agent.key -out agent.csr
openssl x509 -req -in agent.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out agent.crt -days 365
```

### Certificate Rotation

1. Generate new certificates with same CA
2. Deploy new certificates to nodes
3. Restart nodes with new certificates
4. Old certificates remain valid until expiry

## Database Persistence

### SQLite Database Location

```bash
# Recommended structure
data/
  master.db        # Master database
  master.db-wal    # Write-ahead log
  master.db-shm    # Shared memory
```

### Backup Strategy

```bash
# Backup SQLite database
sqlite3 data/master.db ".backup data/master-backup.db"

# Or use file copy (stop master first)
cp data/master.db data/master-$(date +%Y%m%d).db
```

### Migration from In-Memory to SQLite

```bash
# Old command (in-memory)
./bin/master --port 8080

# New command (persistent)
./bin/master --port 8080 --db data/master.db
```

Data from in-memory mode cannot be migrated. Start fresh with SQLite.

## API Authentication

### Using API Keys

Master validates API keys in the `Authorization` header:

```bash
# Create job with authentication
curl -X POST https://master:8443/jobs \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"scenario": "4K60-h264"}'
```

### Best Practices

1. Use long, random API keys (32+ characters)
2. Store API keys in environment variables or secrets management
3. Rotate API keys regularly
4. Use different keys for different environments (dev/staging/prod)

## Monitoring Production Deployment

### Health Checks

```bash
# Check master health (no auth required)
curl https://master:8443/health

# Check node count
curl -H "Authorization: Bearer api-key" https://master:8443/nodes
```

### Logs

```bash
# Master logs
./bin/master --db data/master.db 2>&1 | tee logs/master.log

# Agent logs
./bin/agent --register --master https://master:8443 2>&1 | tee logs/agent.log
```

### Systemd Service (Linux)

```ini
# /etc/systemd/system/ffmpeg-master.service
[Unit]
Description=FFmpeg RTMP Distributed Master
After=network.target

[Service]
Type=simple
User=ffmpeg
WorkingDirectory=/opt/ffmpeg-rtmp
ExecStart=/opt/ffmpeg-rtmp/bin/master \
  --port 8443 \
  --db /var/lib/ffmpeg-rtmp/master.db \
  --tls \
  --cert /etc/ffmpeg-rtmp/certs/master.crt \
  --key /etc/ffmpeg-rtmp/certs/master.key \
  --api-key-file /etc/ffmpeg-rtmp/api-key
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

## Security Best Practices

1. **Always use TLS in production** - Never HTTP for sensitive data
2. **Enable mTLS for untrusted networks** - Verify client identity
3. **Use API keys** - Add authentication layer
4. **Limit network access** - Firewall rules, VPN
5. **Regular certificate rotation** - Don't let certs expire
6. **Monitor logs** - Watch for authentication failures
7. **Backup database** - Regular SQLite backups
8. **Use strong TLS ciphers** - Default config is secure

## Performance Tuning

### Database

```bash
# For high-throughput scenarios, consider PostgreSQL
# SQLite is good for up to ~1000 jobs/sec

# Optimize SQLite
echo "PRAGMA journal_mode=WAL;" | sqlite3 data/master.db
echo "PRAGMA synchronous=NORMAL;" | sqlite3 data/master.db
```

### Connection Timeouts

Defaults are set for production:
- ReadTimeout: 30s
- WriteTimeout: 30s
- IdleTimeout: 120s

Adjust in code if needed for your workload.

## Troubleshooting

### Certificate Errors

```bash
# Verify certificate
openssl x509 -in certs/master.crt -text -noout

# Test TLS connection
openssl s_client -connect master:8443 -CAfile certs/ca.crt
```

### Database Locked

SQLite may lock under high concurrency. Solutions:
1. Enable WAL mode (write-ahead logging)
2. Reduce concurrent writes
3. Consider PostgreSQL for high-scale

### API Key Rejected

Check:
1. API key matches between master and agent
2. Authorization header format: `Bearer <key>`
3. Master logs for authentication errors

## Comparison: Development vs Production

| Feature | Development | Production |
|---------|------------|------------|
| Storage | In-memory | SQLite |
| Encryption | HTTP | HTTPS + mTLS |
| Authentication | None | API Keys |
| Certificates | Self-signed | CA-signed |
| Shutdown | Immediate | Graceful (30s) |
| Timeouts | 15s | 30s |
| Monitoring | Logs only | Logs + Health checks |

## Next Steps

- Set up log aggregation (ELK, Loki)
- Add Prometheus metrics export
- Implement certificate auto-renewal
- Deploy with container orchestration (Kubernetes)
- Add distributed tracing (Jaeger)

## Support

For issues or questions:
- Check [IMPLEMENTATION_NOTES.md](IMPLEMENTATION_NOTES.md) for technical details
- Review [distributed_architecture_v1.md](distributed_architecture_v1.md) for architecture
- File an issue on GitHub
