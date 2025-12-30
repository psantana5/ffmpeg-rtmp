# Deployment Modes

This document explains the two deployment modes available in the FFmpeg RTMP Power Monitoring system and helps you choose the right one for your use case.

## Overview

The system supports two distinct deployment modes:

1. **Local Testing Mode** (Docker Compose) - For development and local testing
2. **Distributed Compute Mode** (Master + Agents) - For production workloads

## Quick Comparison

| Feature | Local Testing (Docker Compose) | Distributed Compute (Master + Agents) |
|---------|-------------------------------|--------------------------------------|
| **Primary Use Case** | Development, testing, demos | Production workloads, scaling |
| **Deployment Complexity** | Simple (one command) | Moderate (multiple nodes) |
| **Resource Requirements** | Single machine (4+ GB RAM) | Master + multiple compute nodes |
| **Scalability** | Limited to single machine | Horizontal scaling across nodes |
| **Setup Time** | 2-5 minutes | 10-15 minutes |
| **Production Ready** | ❌ No | ✅ Yes |
| **Best For** | Quick tests, feature development | Long-running benchmarks, multi-node workloads |

---

## Mode 1: Local Testing (Docker Compose)

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│              Single Host Machine (Development)          │
│                                                         │
│  ┌───────────────────────────────────────────────────┐ │
│  │         Docker Compose Stack                      │ │
│  │                                                   │ │
│  │  ┌─────────────┐  ┌──────────────┐  ┌─────────┐ │ │
│  │  │ Victoria    │  │   Grafana    │  │ Nginx   │ │ │
│  │  │ Metrics     │  │              │  │ RTMP    │ │ │
│  │  └─────────────┘  └──────────────┘  └─────────┘ │ │
│  │                                                   │ │
│  │  ┌─────────────────────────────────────────────┐ │ │
│  │  │     12+ Exporters (CPU, GPU, Docker, etc)   │ │ │
│  │  └─────────────────────────────────────────────┘ │ │
│  │                                                   │ │
│  │  ┌─────────────┐  ┌──────────────┐              │ │
│  │  │ FFmpeg      │  │ Alertmanager │              │ │
│  │  │ (Host)      │  │              │              │ │
│  │  └─────────────┘  └──────────────┘              │ │
│  └───────────────────────────────────────────────────┘ │
│                                                         │
│  All components run on localhost                        │
│  Network: Docker bridge (streaming-net)                 │
└─────────────────────────────────────────────────────────┘
```

### Description

Local Testing mode deploys all components on a single machine using Docker Compose. This mode is **optimized for development** and is **NOT recommended for production** workloads.

### When to Use

✅ **Use Local Testing Mode when:**
- Developing new features or exporters
- Testing configuration changes
- Running quick performance tests
- Learning the system
- Creating demos or tutorials
- Debugging issues locally

❌ **Do NOT use Local Testing Mode for:**
- Production workloads
- Long-running benchmarks (>1 hour)
- Multi-node scaling requirements
- High-availability deployments
- Resource-intensive transcoding jobs

### Prerequisites

- Docker 20.10+ and Docker Compose 2.0+
- Linux host with kernel 4.15+ (for RAPL power monitoring)
- 4+ GB RAM, 10+ GB free disk space
- Python 3.10+
- FFmpeg installed on host

### Setup Steps

**1. Clone Repository**
```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp
```

**2. Start Stack**
```bash
# Start all services with one command
make up-build

# Or with GPU support (requires nvidia-container-toolkit)
make nvidia-up-build
```

**3. Verify Deployment**
```bash
# Check all containers are running
docker compose ps

# Should see 12+ containers in "healthy" state
```

**4. Access Dashboards**
- Grafana: http://localhost:3000 (admin/admin)
- VictoriaMetrics: http://localhost:8428
- Alertmanager: http://localhost:9093

**5. Run Test**
```bash
# Run a simple 60-second test
python3 scripts/run_tests.py single \
  --name "test1" \
  --bitrate 2000k \
  --duration 60
```

**6. View Results**
- Open Grafana dashboard at http://localhost:3000
- Navigate to "Power Monitoring" dashboard
- View real-time metrics

### Stopping the Stack

```bash
# Stop all containers
make down

# Stop and remove volumes (clean slate)
docker compose down -v
```

### Limitations

- **Single machine only**: Cannot distribute workloads across multiple nodes
- **Resource contention**: All services compete for CPU/RAM on one machine
- **No high availability**: If the machine fails, entire system goes down
- **Limited scalability**: Constrained by single machine's resources
- **Not production-grade**: No redundancy, no automatic failover

### Resource Usage

- **Memory**: ~1-2 GB (idle), ~2-4 GB (active test)
- **CPU**: ~5-10% (idle), ~60-420% (active test)
- **Disk**: ~3-6 GB initial, +50-110 MB per day
- **Network**: ~10-550 KB/s

---

## Mode 2: Distributed Compute (Master + Agents)

### Architecture Diagram

```
┌────────────────────────────────────────────────────────────────┐
│                      MASTER NODE                               │
│                  (Coordination & Monitoring)                    │
│                                                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │   Master     │  │ Victoria     │  │   Grafana    │       │
│  │   Service    │  │ Metrics      │  │              │       │
│  │  (Go HTTP)   │  │   (TSDB)     │  │ (Dashboards) │       │
│  │              │  │              │  │              │       │
│  │  Port 8080   │  │  Port 8428   │  │  Port 3000   │       │
│  └──────┬───────┘  └──────────────┘  └──────────────┘       │
│         │                                                      │
│         │ REST API (HTTP/JSON)                                │
└─────────┼──────────────────────────────────────────────────────┘
          │
          │ Network (LAN/WAN)
          │
┌─────────┴──────────────────────────────────────────────────────┐
│                                                                │
│  ┌──────────────────┐         ┌──────────────────┐           │
│  │  COMPUTE NODE 1  │         │  COMPUTE NODE N  │           │
│  │                  │         │                  │           │
│  │  ┌────────────┐  │         │  ┌────────────┐  │           │
│  │  │   Agent    │  │   ...   │  │   Agent    │  │           │
│  │  │ (Go Binary)│  │         │  │ (Go Binary)│  │           │
│  │  └────────────┘  │         │  └────────────┘  │           │
│  │                  │         │                  │           │
│  │  When job runs:  │         │  When job runs:  │           │
│  │  ┌────────────┐  │         │  ┌────────────┐  │           │
│  │  │  FFmpeg    │  │         │  │  FFmpeg    │  │           │
│  │  │  Exporters │  │         │  │  Exporters │  │           │
│  │  │  Analyzer  │  │         │  │  Analyzer  │  │           │
│  │  └────────────┘  │         │  └────────────┘  │           │
│  └──────────────────┘         └──────────────────┘           │
│                                                                │
│  Agents poll master for jobs (HTTP GET /jobs/next)            │
│  Agents send heartbeats (HTTP POST /nodes/{id}/heartbeat)     │
│  Agents submit results (HTTP POST /results)                   │
└────────────────────────────────────────────────────────────────┘
```

### Description

Distributed Compute mode separates the control plane (master) from the data plane (agents). The master node coordinates job distribution and aggregates results, while compute nodes execute workloads. This is the **recommended mode for production** deployments.

### When to Use

✅ **Use Distributed Compute Mode when:**
- Running production workloads
- Scaling across multiple machines
- Executing long-running benchmarks (hours to days)
- Utilizing heterogeneous hardware (CPUs, GPUs, different specs)
- Requiring high availability (multiple agents)
- Optimizing resource utilization across fleet
- Running automated CI/CD pipelines
- Need to isolate compute from coordination

❌ **Do NOT use Distributed Compute Mode when:**
- Quick local testing (use Local Testing instead)
- Single machine is sufficient
- Setup complexity is a concern

### Prerequisites

**Master Node:**
- Go 1.21+ (for building binaries)
- Docker 20.10+ and Docker Compose 2.0+ (for monitoring stack)
- 2+ GB RAM, 10+ GB disk
- Public IP or accessible hostname
- Open ports: 8080 (master API), 3000 (Grafana), 8428 (VictoriaMetrics)

**Compute Nodes:**
- Go 1.21+ (for building agent binary)
- Python 3.10+ (for analyzer scripts)
- FFmpeg with codec support
- 4+ GB RAM, 10+ GB disk
- Network connectivity to master node
- No inbound ports required (outbound only)

### Setup Steps

#### Master Node Setup

**1. Clone and Build**
```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# Build master binary
make build-master
# Creates ./bin/master
```

**2. Start Master Service**

**Option A: Manual Start (Development)**
```bash
# Start in foreground
./bin/master --port 8080

# Or background with logging
./bin/master --port 8080 > /var/log/ffmpeg-master.log 2>&1 &
```

**Option B: Systemd Service (Production)**
```bash
# Copy systemd service file
sudo cp deployment/ffmpeg-master.service /etc/systemd/system/

# Edit service file to set correct paths
sudo nano /etc/systemd/system/ffmpeg-master.service

# Enable and start service
sudo systemctl enable ffmpeg-master
sudo systemctl start ffmpeg-master

# Check status
sudo systemctl status ffmpeg-master
```

**3. Start Monitoring Stack**
```bash
# Start VictoriaMetrics and Grafana
make vm-up-build
```

**4. Verify Master is Running**
```bash
# Health check
curl http://localhost:8080/health

# Should return: {"status":"healthy"}

# Check registered nodes (initially empty)
curl http://localhost:8080/nodes
```

#### Compute Node Setup

**1. Clone and Build**
```bash
git clone https://github.com/psantana5/ffmpeg-rtmp.git
cd ffmpeg-rtmp

# Build agent binary
make build-agent
# Creates ./bin/agent
```

**2. Start Agent Service**

**Option A: Manual Start (Development)**
```bash
# Replace MASTER_IP with your master node's IP or hostname
./bin/agent --register --master http://MASTER_IP:8080

# Agent will:
# - Auto-detect hardware (CPU, GPU, RAM)
# - Register with master
# - Start heartbeat loop (every 30s)
# - Start job polling loop (every 10s)
```

**Option B: Systemd Service (Production)**
```bash
# Copy systemd service file
sudo cp deployment/ffmpeg-agent.service /etc/systemd/system/

# Edit service file to set correct paths and master URL
sudo nano /etc/systemd/system/ffmpeg-agent.service

# Enable and start service
sudo systemctl enable ffmpeg-agent
sudo systemctl start ffmpeg-agent

# Check status
sudo systemctl status ffmpeg-agent
```

**3. Verify Agent Registration**
```bash
# On master node, check registered nodes
curl http://MASTER_IP:8080/nodes | jq

# Should see your agent with hardware info
```

#### Submit and Run Jobs

**1. Submit Job to Master**
```bash
curl -X POST http://MASTER_IP:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "scenario": "1080p-h264",
    "confidence": "auto",
    "parameters": {
      "duration": 300,
      "bitrate": "5000k",
      "resolution": "1920x1080"
    }
  }'

# Returns job_id
```

**2. Monitor Job Execution**
```bash
# Check job status
curl http://MASTER_IP:8080/jobs | jq

# Agent will automatically pick up job and execute
```

**3. View Results**
- Open Grafana: http://MASTER_IP:3000
- Check master logs for completed job results
- Query master API for job results (future feature)

### Production Deployment Recommendations

#### 1. Use Systemd Services

Systemd provides:
- Automatic restart on failure
- Logging to journald
- Start on boot
- Resource limits
- Dependency management

See `deployment/` directory for service templates.

#### 2. Configure TLS/HTTPS

**IMPORTANT**: The current implementation uses HTTP. For production:
- Deploy nginx reverse proxy with TLS termination
- Use Let's Encrypt for certificates
- Configure mTLS between master and agents (future)

Example nginx config:
```nginx
server {
    listen 443 ssl;
    server_name master.example.com;
    
    ssl_certificate /etc/letsencrypt/live/master.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/master.example.com/privkey.pem;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

#### 3. Set Up Monitoring and Alerting

- Configure Alertmanager for critical alerts
- Set up alerts for:
  - Master service down
  - Agent heartbeat failures
  - Job failures
  - Resource exhaustion
- Integrate with PagerDuty, Slack, or email

#### 4. Backup and Disaster Recovery

**Master Node**:
```bash
# Backup VictoriaMetrics data
docker run --rm -v victoriametrics-data:/data -v $(pwd):/backup \
  ubuntu tar czf /backup/vm-backup-$(date +%Y%m%d).tar.gz /data

# Backup Grafana dashboards
curl -u admin:admin http://localhost:3000/api/dashboards/uid/power_monitoring \
  > dashboard-backup-$(date +%Y%m%d).json
```

**Scheduled Backups**:
```bash
# Add to cron
0 2 * * * /opt/ffmpeg-rtmp/scripts/backup.sh
```

#### 5. Resource Limits

Set systemd resource limits in service files:

```ini
[Service]
# Limit memory
MemoryLimit=4G
MemoryAccounting=yes

# Limit CPU
CPUQuota=400%  # 4 cores max

# Restart policy
Restart=always
RestartSec=10s
```

#### 6. Firewall Configuration

**Master Node**:
```bash
# Allow master API, Grafana, VictoriaMetrics
sudo ufw allow 8080/tcp comment 'FFmpeg Master API'
sudo ufw allow 3000/tcp comment 'Grafana'
sudo ufw allow 8428/tcp comment 'VictoriaMetrics'

# If using SSH for management
sudo ufw allow 22/tcp

# Enable firewall
sudo ufw enable
```

**Compute Nodes**:
```bash
# No inbound ports required (agents initiate connections)
# Only allow SSH if needed for management
sudo ufw allow 22/tcp
sudo ufw enable
```

#### 7. Log Management

**Systemd Journal**:
```bash
# View master logs
sudo journalctl -u ffmpeg-master -f

# View agent logs
sudo journalctl -u ffmpeg-agent -f

# Set log retention
sudo journalctl --vacuum-time=7d
```

**Centralized Logging** (Optional):
- Ship logs to ELK stack, Splunk, or Loki
- Use fluentd or filebeat for log forwarding

### Scaling Compute Nodes

**Adding More Agents**:
1. Deploy agent to new machine
2. Start agent with `--register` flag
3. Agent automatically joins pool
4. Master distributes jobs across all available agents

**Horizontal Scaling Benefits**:
- Increased throughput (more parallel jobs)
- Hardware diversity (mix CPUs, GPUs)
- Fault tolerance (one agent down doesn't affect others)
- Load distribution (master balances work)

**Example: 10-Node Cluster**
- 1 Master node (2 GB RAM)
- 10 Compute nodes (4 GB RAM each)
- Total capacity: 10 concurrent jobs
- Total cost: ~$50-100/month (cloud VMs)

### Network Requirements

- **Latency**: <100ms between agents and master recommended
- **Bandwidth**: ~1-10 KB/s per agent (heartbeats + job metadata)
- **Connectivity**: Agents need outbound HTTP access to master
- **Firewall**: No inbound ports needed on agents

### Maintenance Operations

**Update Master**:
```bash
# Pull latest code
cd /opt/ffmpeg-rtmp
git pull

# Rebuild master
make build-master

# Restart service
sudo systemctl restart ffmpeg-master
```

**Update Agent**:
```bash
# Pull latest code
cd /opt/ffmpeg-rtmp
git pull

# Rebuild agent
make build-agent

# Restart service
sudo systemctl restart ffmpeg-agent
```

**Zero-Downtime Updates** (Future):
- Update agents one at a time
- Running jobs complete before restart
- Master continues serving other agents

### Troubleshooting

**Master Issues**:
```bash
# Check if master is running
curl http://localhost:8080/health

# Check logs
sudo journalctl -u ffmpeg-master -n 100

# Check port binding
sudo lsof -i :8080

# Restart master
sudo systemctl restart ffmpeg-master
```

**Agent Issues**:
```bash
# Check agent logs
sudo journalctl -u ffmpeg-agent -n 100

# Test connectivity to master
curl http://MASTER_IP:8080/health

# Check agent process
ps aux | grep agent

# Restart agent
sudo systemctl restart ffmpeg-agent
```

**Network Issues**:
```bash
# Test connectivity
ping MASTER_IP

# Test port reachability
telnet MASTER_IP 8080

# Check firewall rules
sudo ufw status

# Check DNS resolution
nslookup master.example.com
```

---

## Comparison Summary

### Use Local Testing Mode if:
- ✅ You're developing features
- ✅ You need quick tests
- ✅ Single machine is enough
- ✅ Minimal setup complexity desired

### Use Distributed Compute Mode if:
- ✅ Running production workloads
- ✅ Need to scale horizontally
- ✅ Long-running benchmarks
- ✅ High availability required
- ✅ Resource optimization needed

---

## Migration Path

### From Local Testing to Distributed

If you've been using Local Testing mode and want to migrate to Distributed:

**1. Extract Configuration**
- Export Grafana dashboards
- Save custom pricing_config.json
- Backup test_results/ directory
- Document any custom modifications

**2. Deploy Master Node**
- Follow master setup steps above
- Import Grafana dashboards
- Copy pricing_config.json

**3. Deploy Compute Nodes**
- Follow agent setup steps above
- Register agents with master
- Verify registration

**4. Migrate Workloads**
- Convert `run_tests.py` commands to job submissions
- Submit jobs via master API instead of running locally
- Monitor via master Grafana dashboards

**5. Deprecate Local Stack**
```bash
# Stop local Docker Compose stack
make down

# Optionally remove volumes
docker compose down -v
```

### Maintaining Both Modes

You can keep both modes available:
- Use Local Testing for development
- Use Distributed for production benchmarks
- Same codebase, different deployment methods

---

## Systemd Service Templates

### Master Service

**File**: `/etc/systemd/system/ffmpeg-master.service`

```ini
[Unit]
Description=FFmpeg RTMP Master Service
After=network.target

[Service]
Type=simple
User=ffmpeg
Group=ffmpeg
WorkingDirectory=/opt/ffmpeg-rtmp
ExecStart=/opt/ffmpeg-rtmp/bin/master --port 8080
Restart=always
RestartSec=10s

# Resource limits
MemoryLimit=2G
CPUQuota=200%

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=ffmpeg-master

[Install]
WantedBy=multi-user.target
```

### Agent Service

**File**: `/etc/systemd/system/ffmpeg-agent.service`

```ini
[Unit]
Description=FFmpeg RTMP Agent Service
After=network.target

[Service]
Type=simple
User=ffmpeg
Group=ffmpeg
WorkingDirectory=/opt/ffmpeg-rtmp
Environment="MASTER_URL=http://master.example.com:8080"
ExecStart=/opt/ffmpeg-rtmp/bin/agent --register --master ${MASTER_URL}
Restart=always
RestartSec=10s

# Resource limits
MemoryLimit=4G
CPUQuota=800%

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=ffmpeg-agent

[Install]
WantedBy=multi-user.target
```

### Setup Instructions

```bash
# Create service user
sudo useradd -r -s /bin/false ffmpeg

# Clone repository to /opt
sudo git clone https://github.com/psantana5/ffmpeg-rtmp.git /opt/ffmpeg-rtmp
sudo chown -R ffmpeg:ffmpeg /opt/ffmpeg-rtmp

# Build binaries
cd /opt/ffmpeg-rtmp
sudo -u ffmpeg make build-master  # or build-agent

# Install service file
sudo cp deployment/ffmpeg-master.service /etc/systemd/system/

# Customize service file
sudo nano /etc/systemd/system/ffmpeg-master.service

# Reload systemd
sudo systemctl daemon-reload

# Enable and start
sudo systemctl enable ffmpeg-master
sudo systemctl start ffmpeg-master

# Check status
sudo systemctl status ffmpeg-master
```

---

## FAQ

### Q: Can I run multiple agents on the same machine?

**A**: Technically yes, but not recommended. Each agent should have dedicated resources. If you need to maximize single-machine utilization, use Local Testing mode instead.

### Q: What happens if the master fails?

**A**: In v1.0, there's no automatic failover. Agents will log connection errors and continue retrying. Jobs in the queue are lost. For high availability, consider:
- Running master in a container with restart policy
- Using systemd auto-restart
- Planning for multi-master support in v1.5+

### Q: Can agents have different hardware specs?

**A**: Yes! That's a key benefit. The master tracks each agent's hardware capabilities (CPU, GPU, RAM). Future versions will use this for intelligent job scheduling.

### Q: How do I update agents without downtime?

**A**: In v1.0, updates require restart. Best practice:
1. Update agents one at a time
2. Wait for running jobs to complete
3. Restart agent service
4. Verify reconnection to master

### Q: Can I mix Docker Compose and distributed mode?

**A**: Not recommended. Choose one mode per environment. However, you can run distributed mode with the master using Docker Compose for its monitoring stack (VictoriaMetrics + Grafana).

### Q: What's the maximum number of agents supported?

**A**: Not formally tested. The in-memory data structures should handle hundreds of agents. Bottlenecks:
- Master CPU for job dispatch
- Master memory for node registry
- Network bandwidth for heartbeats

Optimize by increasing heartbeat interval if >100 agents.

---

## Related Documentation

- [Internal Architecture Reference](INTERNAL_ARCHITECTURE.md) - Complete runtime model
- [Distributed Architecture v1](distributed_architecture_v1.md) - Detailed distributed design
- [Getting Started Guide](getting-started.md) - Initial setup
- [Contributing Guide](../CONTRIBUTING.md) - Development workflow

---

## Document Version

**Version**: 1.0  
**Last Updated**: 2025-12-30  
**Maintainers**: See CONTRIBUTING.md
