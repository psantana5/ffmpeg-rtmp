# FFmpeg-RTMP Ansible Deployment Guide

Complete guide for deploying FFmpeg-RTMP using Ansible on RockyLinux and Debian-based systems.

## Prerequisites

### Control Machine
- Ansible 2.12 or higher
- Python 3.8+
- SSH key authentication configured

### Target Hosts
- **Debian**: Ubuntu 20.04+, Debian 11+
- **RHEL**: RockyLinux 8+, AlmaLinux 8+
- Systemd installed
- Python 3.6+
- SSH access with sudo privileges

## Quick Start

### 1. Setup Inventory

Copy and edit the production inventory:

\`\`\`bash
cp inventory/production.ini inventory/my-prod.ini
vi inventory/my-prod.ini
\`\`\`

Example:
\`\`\`ini
[master]
master.prod.example.com ansible_host=10.0.0.1 ansible_user=ubuntu

[workers]
worker01.prod.example.com ansible_host=10.0.0.10 ansible_user=rocky
worker02.prod.example.com ansible_host=10.0.0.11 ansible_user=ubuntu

[debian_hosts]
worker02.prod.example.com

[rocky_hosts]
worker01.prod.example.com
\`\`\`

### 2. Test Connection

\`\`\`bash
ansible -i inventory/my-prod.ini all -m ping
\`\`\`

### 3. Deploy

\`\`\`bash
# Full deployment
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml

# Or deploy in stages
ansible-playbook -i inventory/my-prod.ini playbooks/master.yml
ansible-playbook -i inventory/my-prod.ini playbooks/workers.yml
\`\`\`

## Configuration

### Global Variables (group_vars/all.yml)

Key variables to customize:

\`\`\`yaml
# Build options
build_from_source: true          # Build from source or use pre-built binaries
go_version: "1.24.0"             # Go version to install

# TLS
enable_tls: true                 # Enable TLS/HTTPS
tls_generate_certs: true         # Auto-generate certificates

# FFmpeg
ffmpeg_enable_nvenc: true        # Enable NVIDIA hardware encoding
ffmpeg_enable_qsv: false         # Enable Intel Quick Sync

# GStreamer
install_gstreamer: true          # Install GStreamer
\`\`\`

### Master Variables (group_vars/master.yml)

\`\`\`yaml
ffrtmp_master_port: 8080         # HTTP port
ffrtmp_master_tls_port: 8443     # HTTPS port (if TLS enabled)
\`\`\`

### Worker Variables (group_vars/workers.yml)

\`\`\`yaml
ffrtmp_worker_max_jobs: 4                # Concurrent jobs per worker
ffrtmp_watch_enabled: true               # Enable auto-discovery
ffrtmp_watch_cpu_quota: 150              # CPU limit (150% = 1.5 cores)
ffrtmp_watch_memory_limit: 2048          # Memory limit in MB
\`\`\`

### Host-Specific Variables

Create `host_vars/hostname.yml`:

\`\`\`yaml
# host_vars/worker01.prod.example.com.yml
ffrtmp_worker_max_jobs: 8
ffrtmp_watch_cpu_quota: 400
ffrtmp_watch_memory_limit: 4096
\`\`\`

## Playbooks

### site.yml - Full Stack

Deploys everything:
- Base system setup
- Dependencies (Go, FFmpeg, GStreamer)
- Master node
- Worker nodes

\`\`\`bash
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml
\`\`\`

### master.yml - Master Only

Deploys only the master orchestration server:

\`\`\`bash
ansible-playbook -i inventory/my-prod.ini playbooks/master.yml
\`\`\`

### workers.yml - Workers Only

Deploys only worker nodes:

\`\`\`bash
ansible-playbook -i inventory/my-prod.ini playbooks/workers.yml
\`\`\`

## Advanced Usage

### Tags

Use tags to run specific parts:

\`\`\`bash
# Only install dependencies
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml --tags dependencies

# Only configure services
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml --tags services

# Skip TLS setup
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml --skip-tags tls
\`\`\`

Available tags:
- `common` - Base system
- `dependencies` - Go, FFmpeg, GStreamer
- `build` - Compile binaries
- `services` - Systemd services
- `tls` - TLS certificates
- `firewall` - Firewall rules
- `go` - Go installation
- `ffmpeg` - FFmpeg installation
- `gstreamer` - GStreamer installation

### Limit to Specific Hosts

\`\`\`bash
# Deploy to single host
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml \
  --limit worker01.prod.example.com

# Deploy to multiple hosts
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml \
  --limit "worker01,worker02"
\`\`\`

### Check Mode (Dry Run)

\`\`\`bash
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml --check
\`\`\`

### Verbose Output

\`\`\`bash
ansible-playbook -vvv -i inventory/my-prod.ini playbooks/site.yml
\`\`\`

## Post-Deployment

### Verify Services

\`\`\`bash
# Check all services
ansible -i inventory/my-prod.ini all -b -m shell \
  -a "systemctl status ffrtmp-master ffrtmp-worker ffrtmp-watch"

# Check master API key
ansible -i inventory/my-prod.ini master -b -m shell \
  -a "cat /etc/ffrtmp-master/api-key"
\`\`\`

### Test Connectivity

\`\`\`bash
# From control machine
curl http://master.prod.example.com:8080/health

# With TLS
curl https://master.prod.example.com:8443/health
\`\`\`

## Troubleshooting

### Connection Issues

\`\`\`bash
# Test SSH
ansible -i inventory/my-prod.ini all -m ping

# Check Python
ansible -i inventory/my-prod.ini all -m setup -a "filter=ansible_python_version"
\`\`\`

### Service Issues

\`\`\`bash
# Check logs
ansible -i inventory/my-prod.ini workers -b -m shell \
  -a "journalctl -u ffrtmp-worker -n 50"

# Restart services
ansible -i inventory/my-prod.ini workers -b -m systemd \
  -a "name=ffrtmp-worker state=restarted"
\`\`\`

### Firewall Issues

\`\`\`bash
# Check firewall (Debian)
ansible -i inventory/my-prod.ini debian_hosts -b -m shell \
  -a "ufw status"

# Check firewall (RedHat)
ansible -i inventory/my-prod.ini rocky_hosts -b -m shell \
  -a "firewall-cmd --list-all"
\`\`\`

## OS-Specific Notes

### RockyLinux / RHEL

- Uses `yum` for package management
- Firewalld for firewall
- Requires EPEL and RPM Fusion for FFmpeg
- SELinux may need configuration

### Debian / Ubuntu

- Uses `apt` for package management
- UFW for firewall
- FFmpeg available in default repos
- AppArmor may need configuration

## Security Best Practices

1. **Use Ansible Vault for sensitive data:**
   \`\`\`bash
   ansible-vault encrypt group_vars/all.yml
   ansible-playbook -i inventory/my-prod.ini playbooks/site.yml --ask-vault-pass
   \`\`\`

2. **Enable TLS in production:**
   \`\`\`yaml
   enable_tls: true
   tls_generate_certs: true
   \`\`\`

3. **Restrict firewall rules:**
   \`\`\`yaml
   firewall_allowed_networks:
     - "10.0.0.0/8"
   \`\`\`

4. **Use SSH keys (not passwords)**

5. **Regularly update systems:**
   \`\`\`bash
   ansible -i inventory/my-prod.ini all -b -m apt -a "upgrade=dist update_cache=yes"  # Debian
   ansible -i inventory/my-prod.ini all -b -m yum -a "name=* state=latest"            # RHEL
   \`\`\`

## Maintenance

### Update Binaries

\`\`\`bash
ansible-playbook -i inventory/my-prod.ini playbooks/site.yml \
  -e "ffrtmp_version=v1.2.3" \
  --tags build,services
\`\`\`

### Scale Workers

Add new workers to inventory and run:

\`\`\`bash
ansible-playbook -i inventory/my-prod.ini playbooks/workers.yml \
  --limit new-worker.prod.example.com
\`\`\`

### Rotate API Keys

1. Generate new key on master
2. Update workers with new key
3. Restart services

## Examples

### Production with TLS

\`\`\`bash
ansible-playbook -i inventory/prod.ini playbooks/site.yml \
  -e "enable_tls=true" \
  -e "tls_generate_certs=true" \
  -e "ffrtmp_master_url=https://master.prod.example.com:8443"
\`\`\`

### Development (Local)

\`\`\`bash
ansible-playbook -i inventory/development.ini playbooks/site.yml \
  -e "enable_tls=false" \
  -e "build_from_source=true"
\`\`\`

### High-Performance Workers

\`\`\`bash
ansible-playbook -i inventory/prod.ini playbooks/workers.yml \
  -e "ffrtmp_worker_max_jobs=16" \
  -e "ffrtmp_watch_cpu_quota=800" \
  -e "ffrtmp_watch_memory_limit=8192"
\`\`\`

## Support

- Documentation: ../docs/
- Issues: https://github.com/psantana5/ffmpeg-rtmp/issues
- Ansible Docs: https://docs.ansible.com/
