# Quick Fix for Existing Edge Node Installation

If you've already run the old install script and are seeing systemd errors, follow these steps to fix the installation:

## Symptoms
```
systemd[1]: ffrtmp-worker.service: Failed to set up mount namespacing: /opt/ffrtmp/streams: No such file or directory
systemd[1]: ffrtmp-worker.service: Failed to set up mount namespacing: /var/log/ffrtmp: No such file or directory
systemd[1]: ffrtmp-worker.service: Main process exited, code=exited, status=203/EXEC
```

## Quick Fix (5 minutes)

### 1. Stop Services
```bash
sudo systemctl stop ffrtmp-worker
sudo systemctl stop ffrtmp-watch
```

### 2. Create Missing Directories
```bash
# Create all required directories
sudo mkdir -p /opt/ffrtmp/bin
sudo mkdir -p /opt/ffrtmp/streams
sudo mkdir -p /opt/ffrtmp/logs
sudo mkdir -p /var/log/ffrtmp
sudo mkdir -p /var/lib/ffrtmp

# Set ownership
sudo chown -R ffrtmp:ffrtmp /opt/ffrtmp
sudo chown -R ffrtmp:ffrtmp /var/lib/ffrtmp
sudo chown -R ffrtmp:ffrtmp /var/log/ffrtmp
```

### 3. Move Binaries (if in wrong location)
```bash
# If binaries are in /usr/local/bin, move them
if [ -f /usr/local/bin/agent ]; then
    sudo mv /usr/local/bin/agent /opt/ffrtmp/bin/ffrtmp-worker
    sudo ln -sf /opt/ffrtmp/bin/ffrtmp-worker /usr/local/bin/ffrtmp-worker
fi

if [ -f /usr/local/bin/ffrtmp ]; then
    sudo mv /usr/local/bin/ffrtmp /opt/ffrtmp/bin/ffrtmp
    sudo ln -sf /opt/ffrtmp/bin/ffrtmp /usr/local/bin/ffrtmp
fi
```

### 4. Update Systemd Services
```bash
cd /path/to/ffmpeg-rtmp

# Copy latest systemd files
sudo cp deployment/systemd/ffrtmp-worker.service /etc/systemd/system/
sudo cp deployment/systemd/ffrtmp-watch.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload
```

### 5. Start Services
```bash
# Start worker
sudo systemctl start ffrtmp-worker
sudo systemctl status ffrtmp-worker

# Start watch daemon
sudo systemctl start ffrtmp-watch
sudo systemctl status ffrtmp-watch
```

### 6. Verify
```bash
# Check logs - should be clean
sudo journalctl -u ffrtmp-worker -n 20
sudo journalctl -u ffrtmp-watch -n 20

# Run validation
cd /path/to/ffmpeg-rtmp
sudo ./deployment/validate-watch.sh
```

## Full Reinstall (Recommended)

For a clean installation with all fixes:

```bash
# Stop and disable services
sudo systemctl stop ffrtmp-worker ffrtmp-watch
sudo systemctl disable ffrtmp-worker ffrtmp-watch

# Clean up old files
sudo rm -rf /opt/ffrtmp/*
sudo rm -rf /var/lib/ffrtmp/*

# Pull latest code
cd /path/to/ffmpeg-rtmp
git pull

# Rebuild binaries
make build-agent
make build-cli

# Run fixed install script
sudo ./deployment/install-edge.sh

# Configure
sudo nano /etc/ffrtmp/worker.env
sudo nano /etc/ffrtmp/watch-config.yaml

# Start services
sudo systemctl enable ffrtmp-worker ffrtmp-watch
sudo systemctl start ffrtmp-worker ffrtmp-watch
```

## Verification Checklist

After fix, verify:
- [ ] `/opt/ffrtmp/bin/` contains binaries
- [ ] `/opt/ffrtmp/streams/` exists
- [ ] `/opt/ffrtmp/logs/` exists
- [ ] `/var/log/ffrtmp/` exists
- [ ] All directories owned by `ffrtmp:ffrtmp`
- [ ] Symlinks in `/usr/local/bin/` work
- [ ] Services start without errors
- [ ] Logs show successful initialization

## Troubleshooting

### Still seeing NAMESPACE errors?
```bash
# Check which directories are missing
ls -la /opt/ffrtmp/
ls -la /var/log/ffrtmp

# Create any missing directories
sudo mkdir -p /opt/ffrtmp/{bin,streams,logs}
sudo mkdir -p /var/log/ffrtmp
sudo chown -R ffrtmp:ffrtmp /opt/ffrtmp /var/log/ffrtmp
```

### Still seeing EXEC errors?
```bash
# Verify binary exists and is executable
ls -la /opt/ffrtmp/bin/ffrtmp-worker
ls -la /opt/ffrtmp/bin/ffrtmp

# Fix permissions if needed
sudo chmod +x /opt/ffrtmp/bin/*
```

### Permission denied errors?
```bash
# Ensure ffrtmp user owns everything
sudo chown -R ffrtmp:ffrtmp /opt/ffrtmp
sudo chown -R ffrtmp:ffrtmp /var/lib/ffrtmp
sudo chown -R ffrtmp:ffrtmp /var/log/ffrtmp

# Restart services
sudo systemctl restart ffrtmp-worker ffrtmp-watch
```

## Notes

- The fix changes installation from `/usr/local/bin` to `/opt/ffrtmp/bin`
- Symlinks in `/usr/local/bin` preserve convenience for command line use
- All systemd services now use `/opt/ffrtmp/bin/` paths
- This matches the expected directory structure in the systemd service files

## Support

If issues persist:
1. Run validation: `sudo ./deployment/validate-watch.sh`
2. Check full logs: `sudo journalctl -u ffrtmp-worker -u ffrtmp-watch -n 100`
3. Review service files: `cat /etc/systemd/system/ffrtmp-*.service`
4. Check GitHub issues
