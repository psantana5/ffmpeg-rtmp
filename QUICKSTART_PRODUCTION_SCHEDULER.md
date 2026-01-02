# Production Scheduler - Quick Start Guide

## 🚀 Quick Start (5 minutes)

### 1. Run Tests (Verify Installation)

```bash
# Test FSM
cd shared/pkg/models
go test -v

# Test Scheduler
cd ../scheduler
go test -v -run TestProductionScheduler

# Expected: All tests pass ✅
```

### 2. Use in Your Code

```go
package main

import (
    "log"
    "time"
    "github.com/psantana5/ffmpeg-rtmp/pkg/scheduler"
    "github.com/psantana5/ffmpeg-rtmp/pkg/store"
)

func main() {
    // Create store
    st, err := store.NewSQLiteStore("master.db")
    if err != nil {
        log.Fatal(err)
    }
    
    // Create production scheduler (uses defaults)
    sched := scheduler.NewProductionScheduler(st, nil)
    
    // Start scheduler
    sched.Start()
    defer sched.Stop()
    
    // Your application continues...
    log.Println("Production scheduler running")
    select {}
}
```

### 3. Monitor Metrics

```go
// Get metrics every 30 seconds
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        m := sched.GetMetrics()
        log.Printf("Queue: %d | Assigned: %d/%d | Retries: %d | Orphans: %d",
            m.QueueDepth,
            m.AssignmentSuccesses,
            m.AssignmentAttempts,
            m.RetryCount,
            m.OrphanedJobsFound)
    }
}()
```

## 📖 Common Use Cases

### Customize Retry Policy

```go
config := scheduler.DefaultSchedulerConfig()
config.RetryPolicy.MaxRetries = 5
config.RetryPolicy.InitialBackoff = 10 * time.Second

sched := scheduler.NewProductionScheduler(st, config)
```

### Customize Timeouts

```go
config := scheduler.DefaultSchedulerConfig()
config.WorkerTimeout = 5 * time.Minute
config.JobTimeout.DefaultTimeout = 1 * time.Hour

sched := scheduler.NewProductionScheduler(st, config)
```

### Customize Loop Intervals

```go
config := scheduler.DefaultSchedulerConfig()
config.SchedulingInterval = 1 * time.Second    // More aggressive
config.HealthCheckInterval = 10 * time.Second  // Less frequent
config.CleanupInterval = 30 * time.Second      // Less frequent

sched := scheduler.NewProductionScheduler(st, config)
```

## 🔍 Observability

### Structured Logs

Enable structured logging to see scheduler activity:

```bash
tail -f logs/master.log | grep -E "\[FSM\]|\[Health\]|\[Cleanup\]"
```

Expected output:
```
[FSM] Job job-abc: QUEUED → ASSIGNED (reason: Assigned to worker-1)
[Health] Worker worker-1 (node-1) dead - no heartbeat for 2m30s
[Cleanup] Recovering orphaned job 5 from dead worker worker-2
```

### Monitor Metrics

```go
// Log metrics periodically
m := sched.GetMetrics()

// Calculate success rate
successRate := float64(m.AssignmentSuccesses) / float64(m.AssignmentAttempts) * 100

// Alert if success rate drops below 90%
if successRate < 90.0 {
    log.Printf("WARNING: Assignment success rate: %.1f%%", successRate)
}

// Alert on high orphan rate
orphanRate := float64(m.OrphanedJobsFound) / float64(m.AssignmentSuccesses) * 100
if orphanRate > 5.0 {
    log.Printf("WARNING: High orphan rate: %.1f%%", orphanRate)
}
```

## 🧪 Testing Your Integration

### Test Worker Death

```bash
# Terminal 1: Start master with production scheduler
./master

# Terminal 2: Start worker
./worker

# Terminal 3: Submit job
ffrtmp jobs submit test-scenario

# Terminal 4: Kill worker (simulate crash)
pkill -9 worker

# Check logs: Job should be recovered
tail -f logs/master.log | grep Cleanup
```

Expected:
```
[Cleanup] Found 1 orphaned jobs
[Cleanup] Recovering orphaned job 1 from dead worker worker-1
[Cleanup] Job 1 re-queued for retry
```

### Test Scheduler Restart

```bash
# Start scheduler with active jobs
./master &

# Submit several jobs
for i in {1..5}; do ffrtmp jobs submit test-$i; done

# Kill master
pkill -9 master

# Restart master
./master

# Check logs: Orphaned jobs should be recovered
tail logs/master.log | grep Cleanup
```

## 📊 Dashboard Example

Create a simple monitoring dashboard:

```go
package main

import (
    "fmt"
    "time"
    "github.com/psantana5/ffmpeg-rtmp/pkg/scheduler"
)

func monitorScheduler(sched *scheduler.ProductionScheduler) {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        m := sched.GetMetrics()
        
        fmt.Println("\n=== Scheduler Status ===")
        fmt.Printf("Queue Depth:       %d jobs\n", m.QueueDepth)
        fmt.Printf("Assignment Rate:   %.1f%%\n", 
            float64(m.AssignmentSuccesses)/float64(m.AssignmentAttempts)*100)
        fmt.Printf("Total Retries:     %d\n", m.RetryCount)
        fmt.Printf("Timeouts:          %d\n", m.TimeoutCount)
        fmt.Printf("Orphans Found:     %d\n", m.OrphanedJobsFound)
        fmt.Printf("Worker Failures:   %d\n", m.WorkerFailures)
        fmt.Printf("Last Scheduling:   %s ago\n", 
            time.Since(m.LastSchedulingRun).Round(time.Second))
        fmt.Printf("Last Health Check: %s ago\n", 
            time.Since(m.LastHealthCheck).Round(time.Second))
    }
}
```

## 🔧 Troubleshooting

### High Orphan Rate

**Symptom:** Many orphaned jobs detected

**Diagnosis:**
```bash
grep "Orphan" logs/master.log | wc -l
```

**Solutions:**
1. Check worker health: Are workers crashing?
2. Increase worker timeout: `config.WorkerTimeout = 5 * time.Minute`
3. Check network: Are heartbeats being lost?

### Low Assignment Success Rate

**Symptom:** Assignment success rate < 90%

**Diagnosis:**
```bash
grep "Failed to assign" logs/master.log
```

**Solutions:**
1. Check worker availability
2. Check database contention
3. Increase retry backoff

### Jobs Stuck in Queue

**Symptom:** Queue depth growing, no assignments

**Diagnosis:**
```bash
# Check for available workers
ffrtmp nodes list

# Check scheduler loops are running
grep "Scheduling:" logs/master.log | tail
```

**Solutions:**
1. Ensure workers are registered and online
2. Check worker heartbeats
3. Verify scheduling loop is running

## 📚 Next Steps

- **Full Documentation:** See `PRODUCTION_SCHEDULER.md`
- **Implementation Details:** See `IMPLEMENTATION_SUMMARY.md`
- **Test Results:** See `VERIFICATION.md`
- **API Reference:** Run `godoc -http=:6060`

## 💡 Best Practices

1. **Monitor Metrics:** Track success rates and orphan rates
2. **Tune Timeouts:** Adjust based on job characteristics
3. **Scale Workers:** Add workers before queue depth grows
4. **Log Analysis:** Use structured logs for debugging
5. **Test Failure:** Regularly test worker crashes in staging
6. **Graceful Shutdown:** Always call `sched.Stop()` on exit

## 🎯 Production Checklist

Before deploying to production:

- [ ] Tests passing locally
- [ ] Timeouts tuned for workload
- [ ] Retry policy configured
- [ ] Monitoring/alerting setup
- [ ] Log rotation configured
- [ ] Database backups enabled
- [ ] Worker health checks working
- [ ] Tested worker crash recovery
- [ ] Tested scheduler restart
- [ ] Load testing completed

---

**Ready to go!** 🚀

For questions or issues, see:
- `PRODUCTION_SCHEDULER.md` - Full documentation
- `IMPLEMENTATION_SUMMARY.md` - Design decisions
- `VERIFICATION.md` - Test results
