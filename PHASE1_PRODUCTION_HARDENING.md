# Phase 1: Production Hardening - Progress Tracker

**Timeline**: 2-4 weeks  
**Status**: 🟢 In Progress  
**Started**: 2026-01-05

## Overview

Systematic hardening of the FFmpeg-RTMP distributed system for production deployment. Focus on performance validation, resource management, monitoring improvements, and operational reliability.

## Goals

1. ✅ Understand system limits through load testing
2. ⏳ Prevent resource exhaustion with proper limits
3. ⏳ Improve observability with alerts and metrics
4. ⏳ Ensure reliable job lifecycle management

---

## Week 1: Load Testing & Benchmarking

### Day 1-2: Load Test Infrastructure ✅

**Status**: ✅ Complete  
**Completed**: 2026-01-05

- [x] Create comprehensive load testing script
- [x] Support multiple test scenarios (quick, standard, stress, scale)
- [x] Real-time progress monitoring
- [x] Automatic report generation
- [x] Metrics collection (submission rate, completion rate, latency)

**Deliverables**:
- `scripts/load_test.sh` - Full-featured load testing tool
- Supports 10-1000+ jobs with configurable parameters
- Generates markdown reports and JSON data
- Tracks system resources during test

**Files**:
- `scripts/load_test.sh` (605 lines)

**Next Steps**:
- [ ] Run baseline test with current setup (1 worker, 4 concurrent jobs)
- [ ] Test with increased concurrency (8, 16 concurrent jobs)
- [ ] Multi-worker test (3-5 workers)
- [ ] Identify performance bottlenecks

### Day 3: Database Performance Testing ⏳

**Status**: ⏳ Not Started  
**Priority**: HIGH

**Tasks**:
- [ ] Set up PostgreSQL test instance
- [ ] Configure master with PostgreSQL backend
- [ ] Run identical load tests (SQLite vs PostgreSQL)
- [ ] Compare write throughput at scale
- [ ] Test with concurrent workers (contention scenarios)
- [ ] Document when to use each database

**Expected Deliverables**:
- PostgreSQL docker-compose configuration
- Performance comparison report
- Database selection guide in docs

**Success Criteria**:
- Understand SQLite write limits (target: document at what scale it becomes a bottleneck)
- PostgreSQL performs better at 100+ jobs/min write rate
- Clear recommendation for production

### Day 4-5: Benchmarking & Documentation ⏳

**Status**: ⏳ Not Started  
**Priority**: HIGH

**Tasks**:
- [ ] Run comprehensive test matrix
  - [ ] 1 worker: 1, 2, 4, 8 concurrent jobs
  - [ ] 3 workers: 4 concurrent jobs each
  - [ ] 5 workers: 4 concurrent jobs each (if hardware available)
- [ ] Document performance characteristics
  - [ ] Jobs/second throughput
  - [ ] CPU saturation points
  - [ ] Memory usage patterns
  - [ ] Network bandwidth usage
- [ ] Identify bottlenecks
  - [ ] Database write contention
  - [ ] CPU limits
  - [ ] Network bandwidth
  - [ ] Job duration variance
- [ ] Update README with:
  - [ ] Tested configurations
  - [ ] Performance benchmarks
  - [ ] Scaling recommendations
  - [ ] Hardware requirements

**Expected Deliverables**:
- `docs/PERFORMANCE.md` - Complete performance guide
- Updated README with "System Requirements" section
- Test results in `test_results/load_tests/`
- Scaling guidelines

**Success Criteria**:
- Document max throughput for 1, 3, 5 worker configurations
- Identify CPU vs I/O bottlenecks
- Provide clear capacity planning guidelines

---

## Week 2: Resource Management

### Resource Limits Per Job ⏳

**Status**: ⏳ Not Started  
**Priority**: HIGH  
**Estimated Effort**: 2-3 days

**Problem**: Jobs can exhaust system resources (CPU, memory) without limits.

**Solution**: Implement cgroup-based resource limits per job.

**Tasks**:
- [ ] Research Go cgroup integration
  - [ ] Evaluate libraries (containerd, runc, or manual)
  - [ ] Design API for resource limits in job parameters
- [ ] Implement CPU limits
  - [ ] Add `max_cpu_percent` to job parameters
  - [ ] Create cgroup for each job
  - [ ] Enforce CPU quota
- [ ] Implement memory limits
  - [ ] Add `max_memory_mb` to job parameters
  - [ ] Enforce memory limits via cgroup
  - [ ] Handle OOM scenarios gracefully
- [ ] Add disk space monitoring
  - [ ] Check available disk space before starting job
  - [ ] Cleanup temporary files after job
  - [ ] Alert if disk usage > 90%
- [ ] Worker resource reservation
  - [ ] Track total reserved resources
  - [ ] Reject jobs if resources unavailable
  - [ ] Consider memory + CPU together

**API Changes**:
```json
{
  "scenario": "1080p-h264",
  "resource_limits": {
    "max_cpu_percent": 200,  // 2 cores
    "max_memory_mb": 2048,    // 2GB
    "max_disk_mb": 5000       // 5GB temp space
  }
}
```

**Deliverables**:
- Resource limit implementation in worker
- Documentation in `docs/RESOURCE_LIMITS.md`
- Example configurations
- Tests for resource enforcement

**Success Criteria**:
- Jobs respect CPU limits (verified with top/htop)
- Jobs killed if exceeding memory limit
- Worker rejects jobs when resources unavailable

### Job Timeout Enforcement ⏳

**Status**: ⏳ Not Started  
**Priority**: MEDIUM  
**Estimated Effort**: 1 day

**Tasks**:
- [ ] Add `timeout` parameter to job schema
- [ ] Implement timeout in worker job execution
- [ ] Kill job process if timeout exceeded
- [ ] Update job status to "failed" with timeout reason
- [ ] Add timeout metrics (jobs_timed_out_total)

**Deliverables**:
- Timeout implementation
- Prometheus metric for timeouts
- Documentation update

**Success Criteria**:
- Job killed after timeout
- Proper cleanup of resources
- Timeout recorded in metrics

---

## Week 3: Enhanced Monitoring & Alerting

### Prometheus Alerting Rules ⏳

**Status**: ⏳ Not Started  
**Priority**: HIGH  
**Estimated Effort**: 1-2 days

**Tasks**:
- [ ] Create `prometheus/alerts.yml`
- [ ] Define critical alerts:
  - [ ] Master down (no heartbeat)
  - [ ] All workers down
  - [ ] Job failure rate > 10%
  - [ ] Queue depth > 1000 jobs
  - [ ] Job completion rate dropped significantly
  - [ ] Database write latency > 1s
- [ ] Define warning alerts:
  - [ ] Worker down (specific node)
  - [ ] High job latency (> 5min queue time)
  - [ ] Low completion rate (< expected throughput)
  - [ ] Disk space < 20%
- [ ] Set up Alertmanager configuration
- [ ] Test alert firing and resolution

**Deliverable**:
- `deployment/prometheus/alerts.yml`
- `deployment/prometheus/alertmanager.yml`
- Documentation in `docs/ALERTING.md`

**Success Criteria**:
- Alerts fire when thresholds exceeded
- Alertmanager routes to appropriate channels
- Runbook links in alert annotations

### Enhanced Bandwidth Metrics ⏳

**Status**: ⏳ Not Started  
**Priority**: MEDIUM  
**Estimated Effort**: 1 day

**Tasks**:
- [ ] Add per-job bandwidth tracking
- [ ] Track input/output bytes
- [ ] Calculate bandwidth utilization per worker
- [ ] Add to Prometheus metrics
- [ ] Create Grafana dashboard panel

**Metrics to Add**:
- `job_input_bytes_total{job_id}`
- `job_output_bytes_total{job_id}`
- `job_bandwidth_mbps{job_id}`
- `worker_bandwidth_utilization{worker_id}`

**Deliverable**:
- Bandwidth metrics implementation
- Updated Grafana dashboard
- Documentation

### SLA Tracking ⏳

**Status**: ⏳ Not Started  
**Priority**: LOW  
**Estimated Effort**: 1 day

**Tasks**:
- [ ] Define SLA targets (e.g., 95% jobs complete in < 10min)
- [ ] Track SLA compliance metrics
- [ ] Add to Prometheus
- [ ] Create SLA dashboard in Grafana

**Deliverable**:
- SLA metrics and dashboard
- Documentation

---

## Week 4: Job Cancellation & Cleanup

### Improved Job Cancellation ⏳

**Status**: ⏳ Not Started  
**Priority**: MEDIUM  
**Estimated Effort**: 2 days

**Current State**: CLI has cancel command, needs testing with concurrent jobs.

**Tasks**:
- [ ] Test cancellation with concurrent jobs
- [ ] Implement graceful termination
  - [ ] Send SIGTERM to job process
  - [ ] Wait 30s for graceful shutdown
  - [ ] Send SIGKILL if still running
- [ ] Cleanup partial outputs
  - [ ] Delete incomplete video files
  - [ ] Clean temporary directories
  - [ ] Update job status appropriately
- [ ] Add cancellation metrics
  - [ ] `jobs_cancelled_total`
  - [ ] `jobs_cancelled_graceful_total`
  - [ ] `jobs_cancelled_forceful_total`
- [ ] Handle concurrent job cancellation race conditions

**Deliverable**:
- Improved cancellation logic
- Integration tests for cancellation
- Metrics and documentation

**Success Criteria**:
- Jobs terminate within 35s (30s graceful + 5s force)
- No orphan processes left
- Proper cleanup of temporary files
- Cancellation works with 4+ concurrent jobs

### Cleanup and Maintenance Tasks ⏳

**Status**: ⏳ Not Started  
**Priority**: LOW  
**Estimated Effort**: 1 day

**Tasks**:
- [ ] Automatic cleanup of old completed jobs
  - [ ] Add job retention policy (default: 7 days)
  - [ ] Background cleanup task in master
- [ ] Disk space monitoring
  - [ ] Alert if < 20% free
  - [ ] Auto-cleanup old logs if critical
- [ ] Database maintenance
  - [ ] SQLite VACUUM on schedule
  - [ ] PostgreSQL ANALYZE
- [ ] Log rotation
  - [ ] Configure logrotate for master/worker logs

**Deliverable**:
- Cleanup tasks implementation
- Configuration options
- Documentation

---

## Additional Tasks (As Time Permits)

### Integration Tests ⏳

**Status**: ⏳ Not Started  
**Priority**: MEDIUM

**Tasks**:
- [ ] End-to-end test: submit job → completion
- [ ] Test job retry logic
- [ ] Test worker failure and job reassignment
- [ ] Test concurrent job execution
- [ ] Test job cancellation during execution

### Runbooks ⏳

**Status**: ⏳ Not Started  
**Priority**: MEDIUM

**Tasks**:
- [ ] Create `docs/RUNBOOKS.md`
- [ ] Common issues and solutions
- [ ] Performance troubleshooting
- [ ] Database issues
- [ ] Worker connectivity problems

---

## Success Metrics

### Week 1 Goals
- [ ] Comprehensive load test results documented
- [ ] Database performance comparison complete
- [ ] System capacity limits known and documented

### Week 2 Goals
- [ ] Resource limits enforced per job
- [ ] Job timeouts implemented
- [ ] No resource exhaustion possible

### Week 3 Goals
- [ ] Alerting rules defined and tested
- [ ] Enhanced metrics available
- [ ] Runbooks created

### Week 4 Goals
- [ ] Job cancellation reliable with concurrent jobs
- [ ] Automatic cleanup tasks running
- [ ] Integration tests passing

### Overall Phase 1 Success
- [ ] System can handle 1000+ jobs without failure
- [ ] Performance limits documented
- [ ] Alerts fire for critical issues
- [ ] Jobs respect resource limits
- [ ] Cancellation works reliably
- [ ] Ready for production use

---

## Notes & Decisions

### 2026-01-05
- Created comprehensive load testing framework
- Script supports multiple test scenarios
- Generates markdown reports and JSON data
- Ready to start baseline performance testing

### Next Session
- Run baseline load tests
- Document initial performance characteristics
- Identify first bottlenecks

---

## Related Documents

- [CONCURRENT_JOBS_IMPLEMENTATION.md](CONCURRENT_JOBS_IMPLEMENTATION.md) - Concurrent processing implementation
- [docs/README.md](docs/README.md) - Comprehensive system documentation
- [DEPLOYMENT.md](DEPLOYMENT.md) - Production deployment guide
- [scripts/load_test.sh](scripts/load_test.sh) - Load testing tool

---

**Last Updated**: 2026-01-05  
**Next Review**: After Week 1 completion
