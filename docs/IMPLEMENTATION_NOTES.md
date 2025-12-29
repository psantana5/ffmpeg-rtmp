# Implementation Notes - Distributed Compute v1.0

## What Was Implemented

This document provides technical implementation details for the distributed compute feature.

## Architecture Decisions

### 1. Push Model vs Pull Model
**Decision**: Use push model (nodes push results to master)  
**Rationale**: 
- Reduces master complexity (no need to track scraping endpoints)
- Minimizes network connections (nodes initiate all connections)
- Better for firewall/NAT scenarios (only outbound connections from workers)
- Aligns with existing VictoriaMetrics architecture

### 2. In-Memory Storage
**Decision**: Use in-memory storage for v1.0  
**Rationale**:
- Simple, no external database dependencies
- Fast for development and testing
- Sufficient for initial deployment scale
- Easy to migrate to persistent storage in v1.5+

**Trade-offs**:
- Jobs/nodes lost on master restart
- No horizontal scaling of master (single instance)
- Memory constrained by available RAM

### 3. FIFO Job Scheduling
**Decision**: Simple FIFO queue for v1.0  
**Rationale**:
- Minimal complexity
- Predictable behavior
- Good baseline for future optimization

**Future Enhancements** (v1.5+):
- Priority queues
- Resource-aware scheduling
- Affinity rules (GPU jobs → GPU nodes)
- Load balancing

### 4. HTTP/JSON Communication
**Decision**: Use HTTP with JSON payloads  
**Rationale**:
- Universal protocol, works everywhere
- Easy to debug (curl, browser, etc.)
- Well-understood by ops teams
- Simple client implementation

**Security Note**: For production:
- Add HTTPS/TLS
- Implement mTLS for mutual authentication
- Add API tokens/JWT

### 5. Master-as-Worker Safety
**Decision**: Require explicit flag + confirmation  
**Rationale**:
- Prevents accidental resource contention
- Clear UX with warnings and risks
- Allows development without multiple machines
- Forces conscious decision for production

## Code Structure

```
cmd/
  master/main.go        - Master node entry point
  agent/main.go         - Agent node entry point

pkg/
  models/
    node.go             - Node data structures
    job.go              - Job data structures
  api/
    master.go           - Master HTTP handlers
  store/
    memory.go           - In-memory storage implementation
  agent/
    client.go           - HTTP client for master communication
    hardware.go         - Hardware detection utilities
```

## Concurrency & Thread Safety

### Store Concurrency
- `MemoryStore` uses separate mutexes for nodes, jobs, and queue
- Read-write locks (RWMutex) allow concurrent reads
- Critical section in `GetNextJob` uses full lock to prevent race

### API Handlers
- Each HTTP request runs in its own goroutine (Go standard)
- Store methods handle all synchronization internally
- No shared state between handlers

### Agent Concurrency
- Main goroutine: job polling loop
- Background goroutine: heartbeat loop
- Both communicate via same HTTP client (which is thread-safe)

## Testing Strategy

### Unit Tests
Not implemented in v1.0 (minimal scope)

### Integration Tests
- `test_distributed.sh`: End-to-end workflow validation
- Tests all API endpoints
- Validates state transitions
- Verifies error handling

### Manual Testing
- Master-as-worker warning/confirmation
- Agent registration and hardware detection
- Job execution simulation
- Existing Python workflow compatibility

## Performance Considerations

### Master Node
- **Memory**: O(N) for nodes + O(M) for jobs
  - Typical: ~1KB per node, ~2KB per job
  - Can handle 10K nodes + 100K jobs in <1GB RAM
- **CPU**: Minimal (HTTP request handling only)
  - API is I/O bound, not CPU bound
  - JSON encoding/decoding is fast

### Compute Nodes
- **Polling**: Default 10s interval, configurable
  - At 100 nodes, master sees 10 req/sec
  - Easily scaled with load balancer
- **Heartbeat**: Default 30s interval
  - Lightweight, just updates timestamp

### Network
- **Bandwidth**: Results batched in single JSON payload
  - Typical payload: 1-10KB for metrics + analyzer output
  - Not suitable for raw video transfer (not intended)

## Limitations & Known Issues

### v1.0 Limitations
1. **No job persistence**: Jobs lost on master restart
2. **No retry logic**: Failed jobs require manual requeue
3. **No authentication**: Trust-on-first-register only
4. **No multi-master**: Single point of failure
5. **Simulated execution**: Agent doesn't actually run FFmpeg yet

### Not Bugs, By Design
1. Nodes can have same hostname (UUID differentiates)
2. Jobs don't auto-retry (intentional for v1.0)
3. No live log streaming (results are batch uploaded)
4. No job cancellation API (can add if needed)

## Migration from Single Node

The distributed system is **fully backward compatible** with single-node workflows:

- Python scripts continue to work
- Docker stack unchanged
- VictoriaMetrics/Grafana unaffected
- Master/Agent are opt-in additions

## Security Considerations

### v1.0 Security Posture
- HTTP (no encryption)
- No authentication
- Trust-on-first-register
- UUID-based node identity

**Risk Level**: Development/Testing only

### Production Hardening (Future)
1. **Transport Security**
   - HTTPS/TLS for all endpoints
   - Certificate validation
   
2. **Authentication**
   - API tokens per node
   - JWT for requests
   - Shared secrets
   
3. **Authorization**
   - Role-based access control
   - Node capabilities → job restrictions
   
4. **Network Isolation**
   - Private network for master-worker communication
   - Firewall rules
   - VPN/Wireguard mesh

## Future Enhancements

### v1.1 - Reliability
- Job retry with exponential backoff
- Dead node detection and cleanup
- Job timeout and recovery

### v1.2 - Smart Scheduling
- Resource-aware placement (CPU/GPU/RAM matching)
- Affinity and anti-affinity rules
- Priority queues

### v1.3 - Security
- mTLS
- API authentication
- Audit logging

### v1.4 - Persistence
- PostgreSQL or SQLite for job/node storage
- Results archival
- Historical analytics

### v1.5 - Multi-Master
- Master election (Raft/etcd)
- Distributed job queue
- Horizontal scaling

### v2.0 - Integration
- Real FFmpeg execution in agent
- Kubernetes operator
- Helm charts
- Prometheus metrics from master

## Monitoring & Observability

### Available Metrics (via logs)
- Node registrations
- Job dispatches
- Job completions
- Heartbeat failures

### Future: Prometheus Metrics
- Master: job queue depth, node count, job completion rate
- Agent: job execution time, resource utilization

### Recommended Alerts
1. Master down (health check failure)
2. No available nodes
3. Job queue growing (backlog)
4. High job failure rate

## Developer Notes

### Building
```bash
make build-distributed
```

### Running Tests
```bash
./test_distributed.sh
```

### Debugging
```bash
# Master logs
tail -f /tmp/master.log

# Agent logs
tail -f /tmp/agent.log

# Check state
curl http://localhost:8080/nodes | jq
curl http://localhost:8080/jobs | jq
```

### Adding Features
1. Update models in `pkg/models/`
2. Add API handlers in `pkg/api/`
3. Update store in `pkg/store/`
4. Test with integration script
5. Update documentation

## Lessons Learned

1. **Keep it simple**: In-memory storage was right choice for v1.0
2. **Safety first**: Master-as-worker warnings prevented confusion
3. **Test early**: Integration tests caught issues before manual testing
4. **Document risks**: Clear docs on limitations prevent surprises
5. **Race conditions**: Code review caught subtle concurrency bug

## Credits

Implemented following the requirements in the original issue, with additional safety features (master-as-worker warnings) based on common sense and development best practices.
