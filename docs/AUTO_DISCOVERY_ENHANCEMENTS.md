# Auto-Discovery System Enhancement Plan

## Current State Analysis

### What Works ✓
1. **Process Discovery**: Scanner correctly identifies processes from `/proc`
2. **Multiple Process Handling**: Can discover and attach to 5+ concurrent processes
3. **Duplicate Prevention**: Processes are not re-discovered in subsequent scans
4. **Graceful Shutdown**: Proper detachment on daemon termination
5. **Non-Owning Governance**: Processes in run mode survive wrapper crashes
6. **Resource Limits**: CPU/memory limits applied via cgroups

### Issues Identified

#### 1. Process Group Independence (Test 4 Warning)
**Status**: False alarm - working correctly
- Attach mode attaches to *already running* processes with their own PGID
- Test spawns child process in same session, creating shared PGID
- **Resolution**: This is expected behavior, not a bug

#### 2. Self-Discovery
**Status**: Minor issue
- Watch daemon discovers its own subprocess (sleep command from watch itself)
- Visible in logs: "Discovered 1 new process" on initial scan (watch's own child)
- **Impact**: Low - doesn't break anything, just noise

#### 3. Visibility Gaps
**Status**: Enhancement opportunity
- No explicit "already tracked" messages in logs
- Duplicate prevention works but isn't visible
- No statistics on discovery rate, attachment count, or scan efficiency

#### 4. Process Metadata
**Status**: Missing feature
- Only captures PID and command name
- No start time, parent PID, user ownership, or working directory
- Limits ability to create smart filters

## Enhancement Priorities

### P0 - Critical Fixes
None identified - system is functional

### P1 - High Value Improvements

#### 1.1 Enhanced Logging & Visibility
**Goal**: Make discovery behavior transparent to operators

Changes:
```go
// In auto_attach.go
if alreadyTracked {
    log.Printf("[watch] PID %d already tracked, skipping", pid)
    continue
}

// Add stats logging
log.Printf("[watch] Scan complete: %d discovered, %d new, %d tracked, duration: %v",
    totalFound, newCount, len(tracked), scanDuration)
```

**Benefits**:
- Operators can see duplicate prevention working
- Scan efficiency visible in real-time
- Easier troubleshooting

#### 1.2 Self-Process Filtering
**Goal**: Don't discover watch daemon's own children

Changes:
```go
// In scanner.go
type Scanner struct {
    targetCommands []string
    excludePPIDs   []int  // Parent PIDs to exclude
    ownPID         int    // Scanner's own PID
}

func (s *Scanner) ScanRunningProcesses() ([]ProcessInfo, error) {
    for _, pid := range pids {
        // Skip if parent is watch daemon
        ppid := getParentPID(pid)
        if ppid == s.ownPID {
            continue
        }
    }
}
```

**Benefits**:
- Cleaner logs (no self-discovery noise)
- More accurate discovery counts
- Better performance (fewer spurious attachments)

#### 1.3 Metrics Dashboard
**Goal**: Real-time visibility into discovery system health

New metrics:
```go
// Prometheus metrics
ffrtmp_discovery_scans_total           // Counter: total scans performed
ffrtmp_discovery_processes_found       // Gauge: processes found in last scan
ffrtmp_discovery_new_attachments       // Counter: new attachments created
ffrtmp_discovery_scan_duration_seconds // Histogram: scan latency
ffrtmp_discovery_tracked_processes     // Gauge: currently tracked PIDs
```

**Benefits**:
- Grafana dashboards showing discovery health
- Alerting on scan failures or long latencies
- Capacity planning data

### P2 - Nice to Have

#### 2.1 Process Metadata Collection
**Goal**: Capture rich process information for filtering

```go
type ProcessInfo struct {
    PID         int
    Command     string
    StartTime   time.Time
    ParentPID   int
    UserID      int
    WorkingDir  string
    CommandLine []string  // Full argv
}
```

**Benefits**:
- Filter by user: "only discover processes owned by 'ffmpeg' user"
- Filter by parent: "only discover processes spawned by systemd"
- Filter by start time: "ignore processes older than 1 hour"

#### 2.2 Advanced Filtering Rules
**Goal**: Configurable discovery policies

```yaml
# config.yaml
auto_discovery:
  scan_interval: 10s
  targets:
    - command: ffmpeg
      filters:
        user: [ffmpeg, video]
        exclude_parent: [watch-daemon]
        min_runtime: 5s
        max_runtime: 24h
```

**Benefits**:
- Prevent discovering test processes
- Focus on long-running production workloads
- Environment-specific policies

#### 2.3 State Persistence
**Goal**: Survive daemon restarts without rediscovering

```go
// Store tracked PIDs to disk
type PersistentState struct {
    TrackedPIDs map[int]time.Time  // PID -> discovery time
    LastScan    time.Time
}

func (s *AutoAttachService) saveState() error {
    return json.Marshal(s.trackedPIDs, "/var/lib/ffrtmp/watch-state.json")
}
```

**Benefits**:
- Restart daemon without disrupting governance
- Audit trail of all discovered processes
- Recovery from crashes

### P3 - Future Exploration

#### 3.1 inotify-Based Discovery
**Goal**: Sub-second discovery latency

Replace polling with event-driven detection:
```go
// Watch /proc for new directories (new PIDs)
watcher, _ := fsnotify.NewWatcher()
watcher.Add("/proc")

for event := range watcher.Events {
    if event.Op&fsnotify.Create == fsnotify.Create {
        // New PID directory created
        checkAndAttach(extractPID(event.Name))
    }
}
```

**Benefits**:
- Instant discovery (no scan interval delay)
- Lower CPU usage (no polling)
- Better for short-lived processes

#### 3.2 Process Tree Analysis
**Goal**: Discover workload hierarchies

```go
// Discover parent FFmpeg and all child processes
type ProcessTree struct {
    Root     ProcessInfo
    Children []ProcessInfo
}

// Attach to entire tree with shared cgroup
```

**Benefits**:
- Handle multi-process workloads (FFmpeg + hwaccel)
- Aggregate resource limits across process family
- Better visibility into complex jobs

#### 3.3 Network Bandwidth Limits
**Goal**: Complete resource governance

```go
// Add TC (traffic control) integration
type Limits struct {
    CPUMax      string
    MemoryMax   int64
    NetworkRx   int64  // bytes/sec ingress
    NetworkTx   int64  // bytes/sec egress
}
```

**Benefits**:
- Prevent network-intensive jobs from saturating links
- QoS for multi-tenant environments
- Complete resource isolation

## Implementation Plan

### Phase 1: Visibility (1-2 days)
1. ✅ Enhanced logging with stats
2. ✅ Self-process filtering
3. ✅ Prometheus metrics

### Phase 2: Intelligence (2-3 days)
4. Process metadata collection
5. Advanced filtering rules
6. Configuration file support

### Phase 3: Reliability (2-3 days)
7. State persistence
8. Improved error handling
9. Recovery mechanisms

### Phase 4: Performance (future)
10. inotify-based discovery
11. Process tree analysis
12. Network limits integration

## Testing Strategy

Each enhancement will include:
1. Unit tests for new functions
2. Integration test in test_discovery_comprehensive.sh
3. Load test (1000+ concurrent processes)
4. Failure injection (kill scanner mid-scan)
5. Documentation updates

## Success Metrics

- **Reliability**: 0 missed discoveries in 24h test
- **Performance**: <100ms scan time for 1000 processes
- **Visibility**: All discoveries logged with full context
- **Efficiency**: No duplicate attachments over 1000 scans
- **Resilience**: Recovery from crash within scan interval

## Timeline

- Week 1: Phase 1 (Visibility)
- Week 2: Phase 2 (Intelligence)  
- Week 3: Phase 3 (Reliability)
- Week 4+: Phase 4 (Performance) - future work
