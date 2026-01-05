# Cleanup and Maintenance Guide

Complete guide to automatic cleanup and database maintenance in the FFmpeg-RTMP distributed transcoding system.

## Overview

The CleanupManager automatically handles:
- Old job deletion based on retention policies
- Database vacuum/optimization
- Statistics tracking for cleanup operations
- Manual cleanup triggers

**Default Configuration**:
- **Job Retention**: 7 days
- **Cleanup Interval**: 24 hours (daily)
- **Vacuum Interval**: 7 days (weekly)
- **Delete Batch Size**: 100 jobs per batch

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                 Master Node                          │
│                                                       │
│  ┌──────────────────────────────────────────────┐  │
│  │         CleanupManager                        │  │
│  │                                                │  │
│  │  ┌──────────────┐      ┌───────────────┐    │  │
│  │  │ Cleanup Loop │      │  Vacuum Loop   │    │  │
│  │  │ (24h ticker) │      │  (7d ticker)   │    │  │
│  │  └──────┬───────┘      └───────┬───────┘    │  │
│  │         │                      │              │  │
│  │         v                      v              │  │
│  │  ┌─────────────────────────────────────┐    │  │
│  │  │      Store Interface                 │    │  │
│  │  │  - GetJobs(status)                   │    │  │
│  │  │  - DeleteJob(id)                     │    │  │
│  │  │  - Vacuum()                          │    │  │
│  │  └─────────────────────────────────────┘    │  │
│  └──────────────────────────────────────────────┘  │
│                      │                               │
│                      v                               │
│         ┌────────────────────────┐                  │
│         │  Database (SQLite/PG)  │                  │
│         └────────────────────────┘                  │
└─────────────────────────────────────────────────────┘
```

## Configuration

### Master Node Flags

```bash
# Enable cleanup (default: true)
--cleanup=true

# Job retention period in days (default: 7)
--cleanup-retention=7
```

### Example Usage

```bash
# Default configuration (7 day retention)
./bin/master --cleanup=true

# Extended retention (30 days)
./bin/master --cleanup-retention=30

# Disable cleanup
./bin/master --cleanup=false
```

### Programmatic Configuration

```go
import "github.com/psantana5/ffmpeg-rtmp/pkg/cleanup"

config := cleanup.CleanupConfig{
    Enabled:          true,
    JobRetentionDays: 7,                // Delete jobs older than 7 days
    CleanupInterval:  24 * time.Hour,   // Run cleanup daily
    VacuumInterval:   7 * 24 * time.Hour, // Vacuum weekly
    DeleteBatchSize:  100,              // Delete 100 jobs at a time
}

mgr := cleanup.NewCleanupManager(config, store)
mgr.Start()
defer mgr.Stop()
```

## Cleanup Behavior

### Job Deletion

**Jobs Eligible for Deletion**:
- Status: `completed`, `failed`, or `canceled`
- Age: Older than retention period (CompletedAt or CreatedAt)
- Batch processing: 100 jobs per batch to avoid DB overload

**Jobs NOT Deleted**:
- Status: `pending`, `queued`, `assigned`, `running`
- Jobs newer than retention period
- Jobs with no completion time (still active)

### Database Vacuum

**SQLite**:
- Runs `VACUUM` to reclaim space and defragment
- Typically takes seconds to minutes depending on DB size
- Reduces file size by removing deleted rows

**PostgreSQL**:
- Runs `VACUUM ANALYZE` to reclaim space and update statistics
- Improves query performance
- Safe to run on production databases

## Cleanup Schedule

### Initial Startup
- Waits 5 minutes after master startup
- Runs first cleanup pass
- Then follows regular schedule

### Regular Schedule
```
Day 1 00:00 - Cleanup + Vacuum (if weekly schedule)
Day 2 00:00 - Cleanup only
Day 3 00:00 - Cleanup only
Day 4 00:00 - Cleanup only
Day 5 00:00 - Cleanup only
Day 6 00:00 - Cleanup only
Day 7 00:00 - Cleanup only
Day 8 00:00 - Cleanup + Vacuum
```

## Manual Operations

### Trigger Immediate Cleanup

```bash
# Via API (if exposed)
curl -X POST http://master:8080/admin/cleanup

# Via code
cleanupMgr.CleanupNow()
```

### Trigger Immediate Vacuum

```bash
# Via API (if exposed)
curl -X POST http://master:8080/admin/vacuum

# Via code
cleanupMgr.VacuumNow()
```

### Get Cleanup Statistics

```go
stats := cleanupMgr.GetStats()
fmt.Printf("Last cleanup: %v\n", stats.LastCleanupTime)
fmt.Printf("Total deleted: %d jobs\n", stats.TotalJobsDeleted)
fmt.Printf("Last vacuum: %v\n", stats.LastVacuumTime)
fmt.Printf("Cleanup duration: %v\n", stats.LastCleanupDuration)
```

## Monitoring

### Log Messages

**Startup**:
```
[Cleanup] Starting cleanup manager (retention: 7 days, interval: 24h0m0s)
✓ Cleanup manager started (retention: 7 days)
```

**Cleanup Operation**:
```
[Cleanup] Starting job cleanup...
[Cleanup] Job cleanup complete: deleted 150 jobs in 2.3s
```

**Vacuum Operation**:
```
[Cleanup] Starting database vacuum...
[Cleanup] Database vacuum complete in 5.7s
```

**Errors**:
```
[Cleanup] Error cleaning completed jobs: database locked
[Cleanup] Failed to delete job abc123: permission denied
[Cleanup] Database vacuum failed: connection refused
```

### Performance Impact

**Cleanup Operation**:
- CPU: Low (mostly I/O bound)
- Memory: Minimal (<10MB)
- I/O: Moderate (database reads/deletes)
- Duration: ~1-5 seconds per 100 jobs

**Vacuum Operation**:
- CPU: Moderate during vacuum
- Memory: Moderate (temporary working memory)
- I/O: High (rewrites database file/pages)
- Duration: Varies (seconds to minutes)

## Best Practices

### Retention Policies

**Development**:
```bash
# Short retention for testing
--cleanup-retention=1  # 1 day
```

**Staging**:
```bash
# Medium retention for debugging
--cleanup-retention=7  # 7 days (default)
```

**Production**:
```bash
# Extended retention for compliance
--cleanup-retention=30  # 30 days
```

### Backup Before Vacuum

For critical production systems:

```bash
# Backup before vacuum (SQLite)
cp master.db master.db.backup

# Or use SQLite backup command
sqlite3 master.db ".backup master.db.backup"

# PostgreSQL
pg_dump ffmpeg_rtmp > backup.sql
```

### Disk Space Monitoring

Monitor database size growth:

```bash
# SQLite
ls -lh master.db

# PostgreSQL
psql -c "SELECT pg_size_pretty(pg_database_size('ffmpeg_rtmp'));"
```

## Troubleshooting

### Cleanup Not Running

**Check if enabled**:
```bash
# Look for startup message
grep "Cleanup manager" /var/log/ffmpeg-rtmp/master.log
```

**Verify retention**:
```bash
# Check job ages
sqlite3 master.db "SELECT id, status, completed_at FROM jobs WHERE status='completed' ORDER BY completed_at DESC LIMIT 10;"
```

### Database Growing Too Large

**Check job counts**:
```sql
SELECT status, COUNT(*) FROM jobs GROUP BY status;
```

**Force cleanup**:
```go
// Trigger immediate cleanup
cleanupMgr.CleanupNow()
cleanupMgr.VacuumNow()
```

**Reduce retention**:
```bash
# Restart with shorter retention
./bin/master --cleanup-retention=3
```

### Vacuum Taking Too Long

**Check database size**:
```bash
ls -lh master.db
```

**Run vacuum during maintenance window**:
```bash
# Disable automatic vacuum
--cleanup=false

# Run manual vacuum during off-hours
sqlite3 master.db "VACUUM;"
```

### Permission Errors

**Check file permissions**:
```bash
ls -l master.db
chown ffmpeg-rtmp:ffmpeg-rtmp master.db
chmod 644 master.db
```

**Check directory permissions**:
```bash
ls -ld /var/lib/ffmpeg-rtmp/
chmod 755 /var/lib/ffmpeg-rtmp/
```

## Performance Tuning

### Adjust Batch Size

Larger batches = faster cleanup but higher load:

```go
config.DeleteBatchSize = 500  // Delete 500 jobs per batch
```

Smaller batches = slower cleanup but lower load:

```go
config.DeleteBatchSize = 50   // Delete 50 jobs per batch
```

### Adjust Intervals

More frequent cleanup (smaller batches):

```go
config.CleanupInterval = 12 * time.Hour  // Twice daily
```

Less frequent vacuum (reduce I/O):

```go
config.VacuumInterval = 30 * 24 * time.Hour  // Monthly
```

### Rate Limiting

Default: 100ms sleep every 100 deletions

Adjust in code:

```go
// In cleanupJobsByStatus()
if *deletedCount%cm.config.DeleteBatchSize == 0 {
    time.Sleep(200 * time.Millisecond)  // Slower
}
```

## Integration with Log Rotation

See `deployment/logrotate/ffmpeg-rtmp` for log rotation configuration.

Combined cleanup strategy:
- **Jobs**: Automatic deletion via CleanupManager (7 days)
- **Logs**: Logrotate handles master/worker logs (14 days)
- **Job Logs**: Separate rotation (7 days)

## API Endpoints (Future Enhancement)

Potential REST API endpoints:

```
GET  /admin/cleanup/stats       - Get cleanup statistics
POST /admin/cleanup/trigger     - Trigger immediate cleanup
POST /admin/cleanup/vacuum      - Trigger immediate vacuum
GET  /admin/cleanup/config      - Get current configuration
PUT  /admin/cleanup/config      - Update configuration
```

## References

- **Implementation**: `shared/pkg/cleanup/cleanup.go`
- **Store Interface**: `shared/pkg/store/interface.go`
- **Master Integration**: `master/cmd/master/main.go`
- **Logrotate Config**: `deployment/logrotate/ffmpeg-rtmp`
