# Logging System Documentation

## Overview

The FFmpeg-RTMP system now uses centralized file-based logging with automatic rotation.

## Log Directory Structure

```
/var/log/ffrtmp/
├── master/
│   ├── master.log
│   ├── scheduler.log
│   └── api.log
├── worker/
│   ├── agent.log
│   └── executor.log
└── wrapper/
    └── wrapper.log
```

If `/var/log/ffrtmp` is not writable (e.g., development mode), logs fall back to `./logs/` in the project directory.

## Usage

### In Application Code

```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/logging"

// Initialize logger
logger, err := logging.NewFileLogger("worker", "agent", logging.INFO, false)
if err != nil {
    log.Fatalf("Failed to initialize logger: %v", err)
}
defer logger.Close()

// Use logger
logger.Info("Server started on port 8080")
logger.Warn("Job queue is full")
logger.Error("Failed to connect to database")
logger.Debug("Processing job ID: %s", jobID)

// With fields (structured logging)
logger.WithField("job_id", "123").Info("Job started")
```

### Log Levels

- **DEBUG**: Detailed diagnostic information
- **INFO**: General informational messages
- **WARN**: Warning messages (non-critical issues)
- **ERROR**: Error messages (failures that don't stop the app)
- **FATAL**: Critical errors (causes application exit)

### Component Names

| Component | SubComponent | Log Path |
|-----------|--------------|----------|
| master | master | /var/log/ffrtmp/master/master.log |
| master | scheduler | /var/log/ffrtmp/master/scheduler.log |
| master | api | /var/log/ffrtmp/master/api.log |
| worker | agent | /var/log/ffrtmp/worker/agent.log |
| worker | executor | /var/log/ffrtmp/worker/executor.log |
| wrapper | wrapper | /var/log/ffrtmp/wrapper/wrapper.log |

## Log Rotation

Logs are automatically rotated daily by logrotate.

### Install Logrotate Configuration

```bash
# For master
sudo cp deployment/logrotate/ffrtmp-master /etc/logrotate.d/

# For worker
sudo cp deployment/logrotate/ffrtmp-worker /etc/logrotate.d/

# For wrapper
sudo cp deployment/logrotate/ffrtmp-wrapper /etc/logrotate.d/

# Test configuration
sudo logrotate -d /etc/logrotate.d/ffrtmp-master
```

### Logrotate Settings

- **Rotation**: Daily
- **Retention**: 14 days
- **Compression**: Yes (gzip)
- **Permissions**: 0644 ffrtmp:ffrtmp

## Manual Rotation

To manually rotate logs:

```go
// Rotate if log exceeds 100MB
err := logger.RotateIfNeeded(100 * 1024 * 1024)
if err != nil {
    logger.Error("Failed to rotate log: %v", err)
}
```

## Viewing Logs

### Real-time Monitoring

```bash
# Watch all logs
sudo tail -f /var/log/ffrtmp/worker/agent.log

# Watch with filtering
sudo tail -f /var/log/ffrtmp/worker/agent.log | grep ERROR

# Multiple logs
sudo tail -f /var/log/ffrtmp/master/*.log
```

### Search Logs

```bash
# Find errors in last hour
sudo grep ERROR /var/log/ffrtmp/worker/agent.log | grep "$(date +%Y-%m-%d\ %H)"

# Search all master logs
sudo grep "connection refused" /var/log/ffrtmp/master/*.log

# Count error frequency
sudo grep ERROR /var/log/ffrtmp/worker/agent.log | wc -l
```

### Using journalctl (Systemd Services)

```bash
# Worker agent logs (includes stdout)
sudo journalctl -u ffrtmp-worker -f

# Master logs
sudo journalctl -u ffrtmp-master -f

# Show last 100 lines
sudo journalctl -u ffrtmp-worker -n 100

# Filter by time
sudo journalctl -u ffrtmp-worker --since "1 hour ago"
```

## Directory Permissions

Ensure proper permissions for log directories:

```bash
# Create directories
sudo mkdir -p /var/log/ffrtmp/{master,worker,wrapper}

# Set ownership
sudo chown -R ffrtmp:ffrtmp /var/log/ffrtmp

# Set permissions
sudo chmod 755 /var/log/ffrtmp
sudo chmod 755 /var/log/ffrtmp/{master,worker,wrapper}
```

## Integration with Systemd

Systemd services automatically capture stdout/stderr and send to journald.  
File logs provide persistent, rotated storage independent of journal.

Both are available:
- **File logs**: `/var/log/ffrtmp/` (persistent, rotated)
- **Journal logs**: `journalctl -u ffrtmp-worker` (systemd managed)

## Migration from Old Logging

### Before (standard log package)

```go
log.Println("Server started")
log.Printf("Processing job: %s", jobID)
```

### After (centralized logging)

```go
logger.Info("Server started")
logger.Info(fmt.Sprintf("Processing job: %s", jobID))
```

### Quick Migration Pattern

1. Add logger initialization at main():
   ```go
   logger, err := logging.NewFileLogger("component", "subcomponent", logging.INFO, false)
   if err != nil {
       log.Fatalf("Failed to initialize logger: %v", err)
   }
   defer logger.Close()
   ```

2. Replace log calls:
   - `log.Println(...)` → `logger.Info(...)`
   - `log.Printf(...)` → `logger.Info(fmt.Sprintf(...))`
   - `log.Fatalf(...)` → `logger.Fatal(...)`

3. Add log level flag:
   ```go
   logLevel := flag.String("log-level", "info", "Log level")
   flag.Parse()
   ```

## Troubleshooting

### Logs Not Appearing

**Issue**: No log files created

**Solution**:
```bash
# Check permissions
ls -la /var/log/ffrtmp/

# Check if directory exists
sudo mkdir -p /var/log/ffrtmp/worker

# Fix ownership
sudo chown -R ffrtmp:ffrtmp /var/log/ffrtmp
```

### Permission Denied

**Issue**: Cannot write to /var/log/ffrtmp

**Solution**: Logger automatically falls back to `./logs/`. Check:
```bash
ls -la ./logs/worker/
```

### Logs Too Large

**Issue**: Log files consuming too much disk space

**Solution**:
```bash
# Check log sizes
du -sh /var/log/ffrtmp/*

# Manually compress old logs
sudo gzip /var/log/ffrtmp/worker/agent.log.*

# Force rotation
sudo logrotate -f /etc/logrotate.d/ffrtmp-worker
```

### Logrotate Not Working

**Issue**: Logs not rotating daily

**Solution**:
```bash
# Test logrotate config
sudo logrotate -d /etc/logrotate.d/ffrtmp-worker

# Check logrotate status
sudo cat /var/lib/logrotate/status | grep ffrtmp

# Force rotation
sudo logrotate -f /etc/logrotate.d/ffrtmp-worker
```

## Best Practices

1. **Use appropriate log levels**
   - DEBUG: Only for development
   - INFO: Normal operations
   - WARN: Potential issues
   - ERROR: Failures (but application continues)
   - FATAL: Critical failures (application exits)

2. **Add context to logs**
   ```go
   logger.WithField("job_id", jobID).Info("Job started")
   ```

3. **Don't log sensitive information**
   - No passwords, API keys, or tokens
   - Sanitize URLs (remove query params)

4. **Use structured logging for important events**
   ```go
   logger.WithField("duration", duration).
          WithField("status", status).
          Info("Job completed")
   ```

5. **Monitor disk space**
   ```bash
   df -h /var/log
   ```

6. **Set up log monitoring**
   - Configure alerts for ERROR/FATAL messages
   - Monitor log growth rate
   - Track critical patterns

## Example: Complete Application Setup

```go
package main

import (
    "flag"
    "fmt"
    "github.com/psantana5/ffmpeg-rtmp/pkg/logging"
)

func main() {
    component := flag.String("component", "worker", "Component name")
    subComponent := flag.String("subcomponent", "agent", "Subcomponent name")
    logLevel := flag.String("log-level", "info", "Log level")
    flag.Parse()

    // Initialize logger
    logger, err := logging.NewFileLogger(*component, *subComponent, logging.ParseLevel(*logLevel), false)
    if err != nil {
        panic(fmt.Sprintf("Failed to initialize logger: %v", err))
    }
    defer logger.Close()

    // Rotate if > 100MB
    defer logger.RotateIfNeeded(100 * 1024 * 1024)

    // Application code
    logger.Info("Application started")
    // ...
}
```

## Summary

- ✅ Centralized logging to `/var/log/ffrtmp/<component>/`
- ✅ Automatic daily rotation (14 days retention)
- ✅ Graceful fallback to `./logs/` in development
- ✅ Structured logging with fields
- ✅ Multiple log levels (debug, info, warn, error, fatal)
- ✅ Integration with systemd journald
- ✅ Logrotate configuration included
