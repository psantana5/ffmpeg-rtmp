# Cleanup Manager Quick Start

## TL;DR

The CleanupManager automatically deletes old jobs and maintains your database. Set it and forget it.

```bash
# Default: 7 day retention, daily cleanup
./bin/master --cleanup=true

# Custom retention: 30 days
./bin/master --cleanup-retention=30

# Disable cleanup
./bin/master --cleanup=false
```

## What Gets Cleaned Up?

**Jobs deleted** (when older than retention period):
- ✅ completed
- ✅ failed
- ✅ canceled

**Jobs kept** (never deleted):
- ❌ pending
- ❌ queued
- ❌ assigned
- ❌ running

## Schedule

| Time | Action | Frequency |
|------|--------|-----------|
| Startup + 5min | Initial cleanup | Once |
| Daily 00:00 | Job cleanup | Every 24h |
| Weekly (Day 7) | Database vacuum | Every 7 days |

## Monitoring

**Check logs**:
```bash
grep Cleanup /var/log/ffmpeg-rtmp/master.log
```

**Typical output**:
```
[Cleanup] Starting cleanup manager (retention: 7 days, interval: 24h0m0s)
✓ Cleanup manager started (retention: 7 days)
[Cleanup] Starting job cleanup...
[Cleanup] Job cleanup complete: deleted 150 jobs in 2.3s
[Cleanup] Starting database vacuum...
[Cleanup] Database vacuum complete in 5.7s
```

## Quick Troubleshooting

**Database growing too large?**
```bash
# Reduce retention to 3 days
./bin/master --cleanup-retention=3
```

**Need more history?**
```bash
# Extend to 30 days
./bin/master --cleanup-retention=30
```

**Vacuum taking too long?**
```bash
# Disable auto-vacuum, run manually during maintenance
./bin/master --cleanup=false
sqlite3 master.db "VACUUM;"
```

## Log Rotation

Install logrotate config:
```bash
sudo cp deployment/logrotate/ffmpeg-rtmp /etc/logrotate.d/
sudo logrotate -d /etc/logrotate.d/ffmpeg-rtmp  # test
```

Retention:
- Master/worker logs: 14 days
- Job logs: 7 days
- Error logs: 30 days

## Performance Impact

**Cleanup**: Minimal (1-5 seconds per 100 jobs)  
**Vacuum**: Moderate (seconds to minutes, depends on DB size)  
**Recommended**: Run during off-peak hours (default: midnight)

## Full Documentation

See `docs/CLEANUP_MAINTENANCE.md` for complete guide.
