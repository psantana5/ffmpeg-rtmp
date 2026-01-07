# Worker Re-Registration Fix

## Problem Statement

When a worker/agent restarts or reconnects to the master, it would fail with:
```
Failed to register with master: registration failed with status 409: Node with address <hostname> is already registered
```

This happened because:
1. Agent crashed or was stopped without proper deregistration
2. Network issues caused registration to succeed server-side but fail client-side
3. Master and agent were both restarted, but database persisted the old registration

## Solution Implemented

**Smart Re-Registration Logic** - The master now handles re-registration gracefully instead of rejecting it.

### How It Works

#### Master Side (`shared/pkg/api/master.go`)

When a node tries to register with an address that already exists:

1. **Before (409 Conflict)**:
   ```go
   if existingNode != nil {
       return Error("Node already registered", 409)
   }
   ```

2. **After (200 OK + Update)**:
   ```go
   if existingNode != nil {
       // Update existing node with new capabilities
       existingNode.CPUThreads = reg.CPUThreads
       existingNode.Status = "available"
       existingNode.LastHeartbeat = time.Now()
       existingNode.CurrentJobID = ""  // Clear stale assignments
       
       // Update in database
       store.UpdateNode(existingNode)
       
       return OK(existingNode, 200)  // Return existing node
   }
   ```

**Benefits**:
-  Existing node is reused (keeps Node ID, registration timestamp)
-  Hardware capabilities are updated (in case hardware changed)
-  Status reset to "available"
-  Heartbeat updated
-  Stale job assignments cleared
-  No manual database cleanup needed

#### Agent Side (`shared/pkg/agent/client.go`)

Agent now accepts both response codes:

```go
// Before: Only accepted 201 Created
if resp.StatusCode != http.StatusCreated {
    return error
}

// After: Accepts 201 (new) and 200 (re-registration)
if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
    return error
}
```

#### Agent Main (`worker/cmd/agent/main.go`)

Better logging to show what happened:

```go
log.Printf("✓ Registered successfully!")
log.Printf("  Node ID: %s", node.ID)
log.Printf("  Node Name: %s", node.Name)
log.Printf("  Status: %s", node.Status)
if node.RegisteredAt.Before(time.Now().Add(-1 * time.Minute)) {
    log.Printf("  Note: Reconnected to existing registration")
}
```

---

## Response Codes

| Status | Meaning | When |
|--------|---------|------|
| **201 Created** | New registration | First time this address registers |
| **200 OK** | Re-registration | Address already exists, node updated |
| **409 Conflict** |  Removed | No longer returned |

---

## Example Scenarios

### Scenario 1: Normal Restart
```bash
# Start agent
./bin/agent --register --master https://master:8080
# Output: ✓ Registered successfully! Node ID: abc-123

# Stop agent (Ctrl+C)

# Start agent again
./bin/agent --register --master https://master:8080
# Output: ✓ Registered successfully! Node ID: abc-123 (same ID!)
#         Note: Reconnected to existing registration
```

### Scenario 2: Master and Agent Both Restart
```bash
# Both stopped, database preserved
# Start master
./bin/master --port 8080

# Start agent
./bin/agent --register --master https://master:8080
# Output: ✓ Registered successfully! Node ID: abc-123
#         Note: Reconnected to existing registration
```

### Scenario 3: Hardware Change
```bash
# Agent with 8 CPU cores registers
./bin/agent --register

# Hardware upgraded to 16 CPU cores
# Agent restarted
./bin/agent --register
# Master updates: CPUThreads: 8 → 16
# Same Node ID preserved
```

### Scenario 4: Network Issues
```bash
# Registration request sent
# Server responds with 201, but network drops before agent receives it
# Server has node in database, agent thinks it failed

# Agent retries
./bin/agent --register
# Server returns 200 with existing node
# Agent succeeds with same Node ID
```

---

## Backward Compatibility

 **Fully Backward Compatible**
- Existing agents work without changes
- First registration still returns 201 Created
- Only re-registrations return 200 OK
- Agent accepts both 200 and 201

---

## Database Impact

**No schema changes required** - Uses existing fields:
- `status` - Reset to "available"
- `last_heartbeat` - Updated to now
- `current_job_id` - Cleared (empty string)
- Hardware fields - Updated with latest values

---

## Testing

### Manual Test
```bash
# Terminal 1: Start master
./bin/master --port 8080

# Terminal 2: Register agent
./bin/agent --register --master http://localhost:8080
# Note the Node ID

# Terminal 2: Stop agent (Ctrl+C)

# Terminal 2: Register again
./bin/agent --register --master http://localhost:8080
# Should succeed with same Node ID

# Terminal 3: Check nodes
curl http://localhost:8080/nodes
# Should show single node, not duplicate
```

### Integration Test
```bash
# Run multiple register cycles
for i in {1..5}; do
    timeout 5s ./bin/agent --register --master http://localhost:8080 &
    wait
done

# Check database
curl http://localhost:8080/nodes | jq '.count'
# Should be 1, not 5
```

---

## Logs Examples

### First Registration (201 Created)
```
2026/01/05 09:20:00 Registering with master node...
2026/01/05 09:20:00 ✓ Registered successfully!
2026/01/05 09:20:00   Node ID: 550e8400-e29b-41d4-a716-446655440000
2026/01/05 09:20:00   Node Name: worker-1
2026/01/05 09:20:00   Status: available
```

### Re-Registration (200 OK)
```
2026/01/05 09:25:00 Registering with master node...
2026/01/05 09:25:00 ✓ Registered successfully!
2026/01/05 09:25:00   Node ID: 550e8400-e29b-41d4-a716-446655440000
2026/01/05 09:25:00   Node Name: worker-1
2026/01/05 09:25:00   Status: available
2026/01/05 09:25:00   Note: Reconnected to existing registration (registered at: 2026-01-05T09:20:00Z)
```

### Master Logs
```
2026/01/05 09:25:00 Node with address worker-1 already exists (ID: 550e8400...), handling re-registration...
2026/01/05 09:25:00 Node re-registered: worker-1 [550e8400...] (server, 16 threads, Intel Core i7)
```

---

## Benefits

1. **No Manual Cleanup** - Workers can restart freely
2. **Idempotent** - Safe to retry registration
3. **Zero Downtime** - Workers reconnect automatically
4. **Hardware Updates** - Capabilities refreshed on restart
5. **Stale Job Clearing** - Old assignments automatically cleared
6. **Better UX** - Clear error messages → Automatic resolution

---

## Future Enhancements

Possible future improvements:

1. **TTL-Based Expiration**
   - Automatically remove nodes inactive for X days
   - `DELETE /nodes/{id}` for manual cleanup

2. **Force Re-register Flag**
   ```bash
   ./bin/agent --register --force  # Always create new registration
   ```

3. **Registration Token**
   - Prevent unauthorized re-registrations
   - Match secret token on re-registration

4. **Audit Trail**
   - Log all registrations with timestamps
   - Track re-registration frequency

---

## Files Changed

1. **`shared/pkg/api/master.go`**
   - Modified `RegisterNode()` function
   - Added re-registration logic
   - Returns 200 OK instead of 409 Conflict

2. **`shared/pkg/agent/client.go`**
   - Modified `Register()` function
   - Accepts both 200 and 201 status codes

3. **`worker/cmd/agent/main.go`**
   - Enhanced registration success logging
   - Shows re-registration notification

---

## Summary

**Problem**: Worker couldn't reconnect after restart (409 Conflict)  
**Solution**: Smart re-registration - update existing node instead of rejecting  
**Result**: Workers can restart freely, no manual cleanup needed

**Status**:  Implemented and tested  
**Version**: 2.3.0+  
**Date**: 2026-01-05
