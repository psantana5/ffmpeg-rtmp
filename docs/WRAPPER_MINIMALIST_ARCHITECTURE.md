# Minimalist Edge Wrapper - Architecture

## Golden Rules (Code Comments Everywhere)

```go
// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.
```

## Module Structure (STENHÅRD SEPARATION)

```
internal/
├── cgroups/
│   ├── manager.go    // Create. Join. Delete. Nothing else.
│   └── limits.go     // Write cpu.max, cpu.weight, memory.max, io.max ONLY
│
├── wrapper/
│   ├── run.go        // Fork/exec path ONLY
│   ├── attach.go     // Attach-only path ONLY
│   └── lifecycle.go  // Intentionally minimal (observe happens elsewhere)
│
├── observe/
│   ├── watcher.go    // PID exit, signals ONLY
│   └── timing.go     // Start/end timestamps ONLY
│
└── report/
    ├── result.go     // Immutable job result
    └── metrics.go    // Counters ONLY
```

**Rule:** If wrapper/ starts containing policy → STOP.

## Process Lifecycle (CRITICAL BIT)

### Run Mode (spawn, non-owning)

```
fork()
  ↓
child:
  apply limits
  execve()
  ↓
parent:
  register PID
  never hold process alive
  just waitpid()
```

**IMPORTANT:** Use process group (`setpgid`) so you can:
- Observe entire workload
- But never kill it "by mistake"

### Attach Mode (WHERE TRUST IS WON)

```
validate PID exists
  ↓
move PID to cgroup
  ↓
start observing
  ↓
NO signaling
NO exec
NO retry
```

**CRITICAL TEST:**
```bash
ffrtmp attach --pid 1234
kill -9 ffrtmp
# workload continues ✓
```

## Cgroup Handling (BORING = GOOD)

### 1 workload = 1 cgroup

**Name:** `ffrtmp/<job_id>/`

**Write ONLY:**
- `cpu.max`
- `cpu.weight`
- `memory.max`
- `io.max`

**❌ NO:**
- kernel params
- sysctl
- magic detection
- "smart defaults"

Everything must be reversible.

## SLA Freeze (VERY IMPORTANT)

At job completion:
1. Calculate SLA status **EXACTLY ONCE**
2. Save the result
3. **NEVER CHANGE** afterwards

```json
{
  "platform_sla_compliant": true,
  "platform_sla_reason": "completed_within_limits"
}
```

Grafana should **NEVER** need to interpret this.

## Edge Integration (SEAMLESS)

Edge nodes are **already receiving signals/streams from clients**.

**Attach mode is THE critical feature:**
```bash
# Existing ffmpeg receiving RTMP stream (PID 5678)
ffrtmp attach --pid 5678 --job-id stream-001
```

**Result:**
- ✅ Stream continues uninterrupted
- ✅ Governance applied retroactively
- ✅ Wrapper can detach anytime
- ✅ Zero downtime adoption

## Minimal "Are We Doing It Right?" Check

You're right if:

✅ Wrapper can be killed anytime
✅ Workload survives
✅ SLA assessed before execution
✅ Attach mode feels "boring"
✅ No retries exist in code
✅ You're ashamed of how little code exists 😄

## Code Metrics

```
internal/cgroups/   ~5.5 KB  (manager + limits primitives)
internal/wrapper/   ~5.0 KB  (run + attach + lifecycle stub)
internal/observe/   ~1.9 KB  (watcher + timing)
internal/report/    ~2.1 KB  (result + metrics counters)
---
Total:             ~14.5 KB  (minimal core)
```

**If this grows beyond 20KB → something is wrong.**

## What Changed From Original

### Removed (Too Complex)
- ❌ `shared/pkg/wrapper/` (35KB) - Too much policy
- ❌ Constraint presets (high/low priority) - Policy
- ❌ "Smart" defaults - Magic
- ❌ OOM score adjustment - Not reversible enough
- ❌ Nice priority - Can be done outside wrapper
- ❌ Complex lifecycle events - Overengineered

### Kept (Essential)
- ✅ Run mode (fork/exec with setpgid)
- ✅ Attach mode (passive observation)
- ✅ Cgroup v1/v2 support (primitives only)
- ✅ Immutable results
- ✅ Platform SLA (calculated once)

### Added (Simplifications)
- ✅ Golden rules as code comments
- ✅ Strict module separation
- ✅ No policy in wrapper/
- ✅ Everything best-effort
- ✅ Ashamed of how little code 😄

## Usage

### Run Mode
```bash
ffrtmp run --job-id job-001 -- ffmpeg -i input.mp4 output.mp4
```

### Attach Mode (CRITICAL FOR EDGE)
```bash
# Find existing process
ps aux | grep ffmpeg

# Attach wrapper (no disruption)
ffrtmp attach --pid 12345 --job-id job-001
```

### With Constraints
```bash
ffrtmp run \
  --job-id job-002 \
  --cpu-max "200000 100000" \
  --cpu-weight 200 \
  --memory-max $((4*1024*1024*1024)) \
  -- python train.py
```

## Testing

```bash
# Test run mode
ffrtmp run --job-id test-1 -- sleep 2

# Test attach mode
sleep 10 &
ffrtmp attach --pid $! --job-id test-2

# Test crash safety (CRITICAL)
ffrtmp run --job-id test-3 -- sleep 60 &
WRAPPER_PID=$!
sleep 1
kill -9 $WRAPPER_PID
# Workload still running ✓
```

## Next Steps (IN ORDER)

1. ✅ CLI + run/attach skeleton
2. ✅ PID lifecycle observer (without limits)
3. ✅ Cgroup join/create (without policy)
4. ✅ SLA metadata wiring
5. [ ] Metrics export (Prometheus)
6. [ ] Integration with worker agent

**STOP** – first thereafter: improvements

## Production Checklist

Before edge deployment:

- [ ] Test: `kill -9 wrapper` → workload continues
- [ ] Test: Attach to existing RTMP stream (no dropped frames)
- [ ] Verify: No policy in `internal/wrapper/`
- [ ] Verify: Total code < 20KB
- [ ] Verify: Golden rules in every file
- [ ] Test: Graceful degradation (no cgroups)
- [ ] Document: Why so little code (it's intentional)

## Philosophy

This wrapper is **boring on purpose**.

- No magic
- No retries
- No "smart" behavior
- No policy
- No orchestration

It does ONE thing: Move PIDs into cgroups.

Everything else is someone else's problem.

**If you're not ashamed of how little code this is, you're doing it wrong.** 😄

---

## Edge Reality Check

Edge nodes are **production systems receiving real traffic**.

They **cannot tolerate**:
- Restarts
- Dropped connections
- Service interruptions
- "Smart" behavior
- Surprises

They **need**:
- Attach mode (zero downtime)
- Boring behavior (predictable)
- Graceful degradation (no hard deps)
- Crash safety (workload survives)
- Minimal code (easy to audit)

This wrapper was designed for **edge reality**, not cloud perfection.
