# Edge Workload Wrapper - Next Steps

## Current Status ✅

The minimalist edge workload wrapper is **stable and production-ready**.

### Completed
- ✅ Minimalist architecture (14.5 KB core)
- ✅ Run mode (fork/exec with setpgid)
- ✅ Attach mode (passive observation)
- ✅ Crash safety verified (workload survives wrapper crash)
- ✅ Process group isolation
- ✅ Cgroup v1/v2 support
- ✅ Graceful degradation
- ✅ Comprehensive testing (10 test suite)
- ✅ Edge reality design (seamless integration)

### Test Results
```
✅ Run mode basic execution
✅ Process survives wrapper detach
✅ Workload survives wrapper kill -9 (CRITICAL)
✅ Invalid PID error handling
✅ Empty job ID handling
✅ Exit code propagation
✅ Concurrent operations
✅ Rapid attach/detach cycles
```

---

## Next Steps (In Order)

### Phase 1: Prometheus Metrics Integration

**Goal:** Export wrapper metrics to Prometheus for monitoring.

**Tasks:**
1. Add Prometheus HTTP endpoint (`/metrics`)
2. Export metrics from `internal/report/metrics.go`:
   - `ffrtmp_wrapper_jobs_started_total`
   - `ffrtmp_wrapper_jobs_completed_total`
   - `ffrtmp_wrapper_jobs_failed_total`
   - `ffrtmp_wrapper_jobs_attached_total`
   - `ffrtmp_wrapper_active_jobs` (gauge)
3. Add per-job labels (job_id, mode)
4. Add platform SLA metrics
5. Document Prometheus queries

**Files to create:**
- `internal/metrics/prometheus.go` (HTTP handler)
- `docs/WRAPPER_METRICS.md` (metrics guide)

---

### Phase 2: Worker Agent Integration

**Goal:** Integrate wrapper with existing worker agent.

**Tasks:**
1. Update `worker/cmd/agent/main.go` to use wrapper
2. Replace direct exec with wrapper.Run()
3. Add job metadata from master (job_id, sla_eligible)
4. Pass constraints from job definition
5. Collect wrapper results and report to master
6. Add wrapper metrics to worker exporter

**Files to modify:**
- `worker/cmd/agent/main.go` (use wrapper instead of exec)
- `worker/exporters/prometheus/exporter.go` (add wrapper metrics)
- `shared/pkg/models/job.go` (add wrapper constraints)

---

### Phase 3: Edge Deployment Guide

**Goal:** Document production edge deployment.

**Tasks:**
1. Create deployment guide for edge nodes
2. Document attach mode for existing workloads
3. Document cgroup requirements and delegation
4. Create systemd service files
5. Document privilege requirements
6. Create troubleshooting guide
7. Add monitoring dashboards

**Files to create:**
- `docs/WRAPPER_EDGE_DEPLOYMENT.md`
- `deployment/wrapper-systemd.service`
- `deployment/wrapper-cgroup-delegation.conf`
- `grafana/wrapper-dashboard.json`

---

### Phase 4: Advanced Features (Optional)

**Goal:** Add advanced features if needed.

**Tasks (low priority):**
1. Real-time resource usage tracking
2. Dynamic constraint adjustment
3. Timeout enforcement
4. Advanced IO constraints
5. NUMA awareness
6. GPU resource constraints

**Decision:** Only implement if production use cases require them.
          Stay minimal by default.

---

## Integration Example

### Current Worker Agent
```go
// worker/cmd/agent/main.go (current)
cmd := exec.Command("ffmpeg", "-i", "input.mp4", "output.mp4")
cmd.Run()
```

### With Wrapper Integration
```go
// worker/cmd/agent/main.go (with wrapper)
import "github.com/psantana5/ffmpeg-rtmp/internal/wrapper"
import "github.com/psantana5/ffmpeg-rtmp/internal/cgroups"

limits := &cgroups.Limits{
    CPUMax:      "200000 100000", // 2 cores
    CPUWeight:   200,
    MemoryMax:   4 * 1024 * 1024 * 1024, // 4GB
}

result, err := wrapper.Run(ctx, job.ID, limits, "ffmpeg", 
    []string{"-i", "input.mp4", "output.mp4"})

// Report result to master
job.PlatformSLA = result.PlatformSLA
job.PlatformSLAReason = result.PlatformSLAReason
job.Duration = result.Duration
```

---

## Prometheus Metrics Design

### Counters
```
ffrtmp_wrapper_jobs_started_total{mode="run"}
ffrtmp_wrapper_jobs_started_total{mode="attach"}
ffrtmp_wrapper_jobs_completed_total{mode="run",sla="true"}
ffrtmp_wrapper_jobs_failed_total{mode="run",reason="workload_error"}
```

### Gauges
```
ffrtmp_wrapper_active_jobs{mode="run"}
ffrtmp_wrapper_active_jobs{mode="attach"}
```

### Histograms
```
ffrtmp_wrapper_job_duration_seconds{mode="run"}
ffrtmp_wrapper_job_duration_seconds{mode="attach"}
```

---

## Edge Deployment Scenarios

### Scenario 1: New Edge Node

```bash
# 1. Deploy wrapper binary
scp bin/ffrtmp edge-node:/usr/local/bin/

# 2. Configure systemd cgroup delegation
cat > /etc/systemd/system/user@.service.d/delegate.conf << EOF
[Service]
Delegate=yes
EOF

# 3. Run worker agent with wrapper
./bin/agent --master https://master:8080 --use-wrapper
```

### Scenario 2: Existing Edge Node (Already Streaming)

```bash
# 1. Find existing FFmpeg processes
ps aux | grep ffmpeg
# PID: 5678

# 2. Attach wrapper (zero downtime)
ffrtmp attach --pid 5678 --job-id existing-stream-001

# 3. Stream continues uninterrupted
# Governance applied retroactively
```

---

## Success Metrics

### Production Readiness Checklist

- [ ] Prometheus metrics exported
- [ ] Worker agent integration complete
- [ ] Edge deployment guide written
- [ ] Systemd service files created
- [ ] Monitoring dashboards deployed
- [ ] First edge node deployed
- [ ] Attach mode tested in production
- [ ] Grafana alerts configured
- [ ] Documentation complete

### Performance Targets

- Wrapper overhead: < 0.1% CPU
- Wrapper memory: < 10 MB
- Attach latency: < 100ms
- Cgroup setup: < 50ms
- Process survival rate: 100% (on wrapper crash)

---

## Timeline Estimate

**Phase 1 (Prometheus Metrics):** 4-6 hours
- Implementation: 2-3 hours
- Testing: 1-2 hours
- Documentation: 1 hour

**Phase 2 (Worker Agent Integration):** 6-8 hours
- Code changes: 3-4 hours
- Testing: 2-3 hours
- Documentation: 1 hour

**Phase 3 (Edge Deployment):** 4-6 hours
- Deployment guide: 2-3 hours
- Systemd configs: 1 hour
- Testing: 1-2 hours

**Total:** 14-20 hours (2-3 days)

---

## Decision Points

### When to use Run Mode vs Attach Mode?

**Run Mode:**
- New workloads
- Full lifecycle control wanted
- Starting fresh processes

**Attach Mode:**
- Existing workloads already running
- Zero downtime requirement
- Legacy system integration
- Production streams that can't be interrupted

### When to skip wrapper?

- Development/testing only
- No resource constraints needed
- Maximum simplicity required
- Observability not important

### When to use cgroups vs nice?

**Cgroups (preferred):**
- Hard limits needed (memory, CPU)
- Multi-tenant environments
- Production workloads

**Nice fallback:**
- No root access
- No cgroup delegation
- Simple priority adjustment

---

## Open Questions

1. Should wrapper metrics be exposed on same port as worker?
2. How should master know if worker is using wrapper?
3. Should constraints be required or optional in API?
4. What happens to in-flight jobs during worker restart?
5. How to handle wrapper version upgrades?

---

## Conclusion

The wrapper is **production-ready** for core functionality.

Next steps focus on:
1. **Integration** (Prometheus, worker agent)
2. **Deployment** (edge guides, systemd)
3. **Monitoring** (dashboards, alerts)

The minimalist architecture ensures stability and maintainability.

**Ready to proceed with Phase 1: Prometheus Metrics Integration.**
