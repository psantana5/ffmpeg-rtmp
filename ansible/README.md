# FFmpeg-RTMP Ansible Deployment

Ansible playbooks for deploying FFmpeg-RTMP across RockyLinux and Debian-based systems.

## Directory Structure

```
ansible/
├── inventory/
│   ├── production.ini       # Production inventory
│   ├── staging.ini          # Staging inventory
│   └── development.ini      # Development inventory
├── group_vars/
│   ├── all.yml              # Common variables
│   ├── master.yml           # Master node variables
│   └── workers.yml          # Worker node variables
├── host_vars/               # Host-specific variables
├── roles/
│   ├── common/              # Base system setup
│   ├── dependencies/        # Go, FFmpeg, GStreamer
│   ├── ffrtmp-master/       # Master node deployment
│   └── ffrtmp-worker/       # Worker node deployment
└── playbooks/
    ├── site.yml             # Complete deployment
    ├── master.yml           # Master-only deployment
    ├── workers.yml          # Workers-only deployment
    └── certificates.yml     # TLS certificate deployment
```

## Quick Start

### 1. Configure Inventory

Edit `inventory/production.ini` with your hosts:

```ini
[master]
master.example.com ansible_host=10.0.0.1

[workers]
worker01.example.com ansible_host=10.0.0.10
worker02.example.com ansible_host=10.0.0.11
```

### 2. Configure Variables

Edit `group_vars/all.yml`:

```yaml
ffrtmp_version: "latest"
ffrtmp_master_url: "https://master.example.com:8443"
enable_tls: true
```

### 3. Run Playbook

```bash
# Deploy everything
ansible-playbook -i inventory/production.ini playbooks/site.yml

# Deploy master only
ansible-playbook -i inventory/production.ini playbooks/master.yml

# Deploy workers only
ansible-playbook -i inventory/production.ini playbooks/workers.yml
```

## See full documentation in this directory's README files.
