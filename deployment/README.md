# Deployment Templates

This directory contains production deployment templates for the FFmpeg RTMP Power Monitoring system.

## Contents

- `ffmpeg-master.service` - Systemd unit file for master node
- `ffmpeg-agent.service` - Systemd unit file for compute agent nodes

## Usage

### Master Node Deployment

1. **Create service user**
```bash
sudo useradd -r -s /bin/false -d /opt/ffmpeg-rtmp ffmpeg
```

2. **Clone repository**
```bash
sudo git clone https://github.com/psantana5/ffmpeg-rtmp.git /opt/ffmpeg-rtmp
sudo chown -R ffmpeg:ffmpeg /opt/ffmpeg-rtmp
```

3. **Build master binary**
```bash
cd /opt/ffmpeg-rtmp
sudo -u ffmpeg make build-master
```

4. **Install systemd service**
```bash
sudo cp deployment/ffmpeg-master.service /etc/systemd/system/
sudo systemctl daemon-reload
```

5. **Start and enable service**
```bash
sudo systemctl enable ffmpeg-master
sudo systemctl start ffmpeg-master
sudo systemctl status ffmpeg-master
```

6. **View logs**
```bash
sudo journalctl -u ffmpeg-master -f
```

### Agent Node Deployment

1. **Create service user**
```bash
sudo useradd -r -s /bin/false -d /opt/ffmpeg-rtmp ffmpeg
```

2. **Clone repository**
```bash
sudo git clone https://github.com/psantana5/ffmpeg-rtmp.git /opt/ffmpeg-rtmp
sudo chown -R ffmpeg:ffmpeg /opt/ffmpeg-rtmp
```

3. **Build agent binary**
```bash
cd /opt/ffmpeg-rtmp
sudo -u ffmpeg make build-agent
```

4. **Install and configure systemd service**
```bash
sudo cp deployment/ffmpeg-agent.service /etc/systemd/system/

# Edit to set your master URL
sudo nano /etc/systemd/system/ffmpeg-agent.service
# Change: Environment="MASTER_URL=http://YOUR_MASTER_IP:8080"

sudo systemctl daemon-reload
```

5. **Start and enable service**
```bash
sudo systemctl enable ffmpeg-agent
sudo systemctl start ffmpeg-agent
sudo systemctl status ffmpeg-agent
```

6. **View logs**
```bash
sudo journalctl -u ffmpeg-agent -f
```

## Configuration

### Master Service Configuration

Edit `/etc/systemd/system/ffmpeg-master.service`:

- **Port**: Change `--port 8080` in `ExecStart` to use different port
- **Resource Limits**: Adjust `MemoryLimit` and `CPUQuota` as needed
- **User**: Change `User=ffmpeg` if using different service user
- **Working Directory**: Update `WorkingDirectory` if installed elsewhere

### Agent Service Configuration

Edit `/etc/systemd/system/ffmpeg-agent.service`:

- **Master URL**: Set `Environment="MASTER_URL=http://your-master:8080"`
- **Poll Interval**: Adjust `POLL_INTERVAL` (default: 10s)
- **Heartbeat Interval**: Adjust `HEARTBEAT_INTERVAL` (default: 30s)
- **Resource Limits**: Adjust `MemoryLimit` and `CPUQuota` as needed
- **Timeout**: Adjust `TimeoutStopSec` (default: 600s for long-running jobs)

## Service Management

### Start/Stop/Restart
```bash
sudo systemctl start ffmpeg-master     # or ffmpeg-agent
sudo systemctl stop ffmpeg-master
sudo systemctl restart ffmpeg-master
```

### Enable/Disable (Start on Boot)
```bash
sudo systemctl enable ffmpeg-master
sudo systemctl disable ffmpeg-master
```

### Check Status
```bash
sudo systemctl status ffmpeg-master
sudo systemctl is-active ffmpeg-master
sudo systemctl is-enabled ffmpeg-master
```

### View Logs
```bash
# Follow logs
sudo journalctl -u ffmpeg-master -f

# Last 100 lines
sudo journalctl -u ffmpeg-master -n 100

# Since specific time
sudo journalctl -u ffmpeg-master --since "2025-12-30 10:00:00"

# With priority filter
sudo journalctl -u ffmpeg-master -p err
```

## Troubleshooting

### Service won't start

```bash
# Check status
sudo systemctl status ffmpeg-master

# Check recent logs
sudo journalctl -u ffmpeg-master -n 50

# Verify binary exists and is executable
ls -lh /opt/ffmpeg-rtmp/bin/master
file /opt/ffmpeg-rtmp/bin/master

# Verify permissions
sudo -u ffmpeg /opt/ffmpeg-rtmp/bin/master --version
```

### Port already in use

```bash
# Check what's using the port
sudo lsof -i :8080

# Kill the process or change port in service file
```

### Permission errors

```bash
# Verify ownership
sudo chown -R ffmpeg:ffmpeg /opt/ffmpeg-rtmp

# Verify ReadWritePaths in service file
# Add any additional paths the service needs access to
```

### Agent can't connect to master

```bash
# Test connectivity
curl http://master.example.com:8080/health

# Check DNS resolution
nslookup master.example.com

# Check firewall
sudo ufw status

# Check agent logs for errors
sudo journalctl -u ffmpeg-agent -n 50
```

## Security Considerations

### Firewall Rules

**Master Node:**
```bash
sudo ufw allow 8080/tcp comment 'FFmpeg Master API'
sudo ufw allow 3000/tcp comment 'Grafana'
sudo ufw allow 8428/tcp comment 'VictoriaMetrics'
sudo ufw enable
```

**Agent Nodes:**
```bash
# No inbound ports required (agents initiate connections)
sudo ufw enable
```

### TLS/HTTPS

The service files assume HTTP. For production:

1. **Deploy nginx reverse proxy** with TLS termination
2. **Use Let's Encrypt** for certificates
3. **Update MASTER_URL** in agent service to use `https://`

Example nginx config:
```nginx
server {
    listen 443 ssl http2;
    server_name master.example.com;
    
    ssl_certificate /etc/letsencrypt/live/master.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/master.example.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Resource Limits

The service files include resource limits:
- **Memory**: Prevents OOM issues
- **CPU**: Prevents CPU hogging

Adjust based on your hardware and workload.

### Security Hardening

Service files include security options:
- `NoNewPrivileges=true` - Prevents privilege escalation
- `PrivateTmp=yes` - Isolated /tmp
- `ProtectSystem=strict` - Read-only filesystem (except ReadWritePaths)
- `ProtectHome=yes` - No access to user home directories

## Monitoring Services

### View Resource Usage
```bash
systemctl status ffmpeg-master
# Shows memory and CPU usage
```

### Enable Detailed Accounting
```bash
# Already enabled in service files via:
# MemoryAccounting=yes
# CPUAccounting=yes

# View accounting data
systemd-cgtop
```

### Set Up Alerts

Use systemd with OnFailure hook:

```ini
[Unit]
OnFailure=failure-notification@%n.service
```

Create notification service:
```bash
# /etc/systemd/system/failure-notification@.service
[Unit]
Description=Send notification on %i failure

[Service]
Type=oneshot
ExecStart=/usr/local/bin/notify-admin.sh %i
```

## Backup and Recovery

### Backup Master Data

```bash
# Backup VictoriaMetrics volume
sudo systemctl stop ffmpeg-master
docker run --rm -v victoriametrics-data:/data -v /backup:/backup \
  ubuntu tar czf /backup/vm-backup-$(date +%Y%m%d).tar.gz /data
sudo systemctl start ffmpeg-master
```

### Restore Master Data

```bash
sudo systemctl stop ffmpeg-master
docker run --rm -v victoriametrics-data:/data -v /backup:/backup \
  ubuntu tar xzf /backup/vm-backup-YYYYMMDD.tar.gz -C /
sudo systemctl start ffmpeg-master
```

## Updates

### Update Master

```bash
# Stop service
sudo systemctl stop ffmpeg-master

# Pull latest code
cd /opt/ffmpeg-rtmp
sudo -u ffmpeg git pull

# Rebuild binary
sudo -u ffmpeg make build-master

# Start service
sudo systemctl start ffmpeg-master

# Verify
sudo systemctl status ffmpeg-master
curl http://localhost:8080/health
```

### Update Agent

```bash
# Stop service
sudo systemctl stop ffmpeg-agent

# Pull latest code
cd /opt/ffmpeg-rtmp
sudo -u ffmpeg git pull

# Rebuild binary
sudo -u ffmpeg make build-agent

# Start service
sudo systemctl start ffmpeg-agent

# Verify
sudo systemctl status ffmpeg-agent
```

### Rolling Updates (Multiple Agents)

Update agents one at a time to avoid downtime:

```bash
# For each agent:
# 1. Stop agent service
# 2. Wait for running job to complete (check logs)
# 3. Update and rebuild
# 4. Start agent service
# 5. Verify registration with master
```

## Related Documentation

- [DEPLOYMENT_MODES.md](../docs/DEPLOYMENT_MODES.md) - Deployment modes guide
- [INTERNAL_ARCHITECTURE.md](../docs/INTERNAL_ARCHITECTURE.md) - Architecture reference
- [distributed_architecture_v1.md](../docs/distributed_architecture_v1.md) - Distributed design

## Support

For issues or questions:
- Open issue on GitHub: https://github.com/psantana5/ffmpeg-rtmp/issues
- Check troubleshooting section above
- Review logs with `journalctl`
