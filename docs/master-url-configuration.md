# Master URL Configuration for RTMP Streaming

## Overview

Workers need to know the master node's address to stream video via RTMP. This document explains how the master URL is obtained and used.

## How Master URL is Obtained

### Method: Command-Line Flag (Current Implementation)

**The master URL is configured via the `--master` flag when starting the worker agent.**

```bash
# Start worker with master URL
./bin/agent --register --master https://10.0.1.5:8080
```

**Flow**:
1. Worker starts with `--master` flag specifying master's API endpoint
2. URL is stored in the `Client` object during initialization
3. When RTMP streaming is needed, worker calls `client.GetMasterURL()`
4. Master hostname is extracted from the API URL
5. RTMP URL is constructed: `rtmp://{master_host}:1935/live/{stream_key}`

### Code Flow

```go
// 1. Worker startup - main.go
masterURL := flag.String("master", "http://localhost:8080", "Master node URL")
// User provides: --master https://10.0.1.5:8080

// 2. Client initialization
client := agent.NewClient(*masterURL)
// Client stores masterURL internally

// 3. Job execution - executeFFmpegJob()
masterURL := client.GetMasterURL()  // Returns "https://10.0.1.5:8080"

// 4. Parse URL to extract hostname
parsedURL, _ := url.Parse(masterURL)
host := parsedURL.Host  // "10.0.1.5:8080"
// Remove port to get just hostname
masterHost := host[:strings.Index(host, ":")]  // "10.0.1.5"

// 5. Construct RTMP URL
rtmpURL := fmt.Sprintf("rtmp://%s:1935/live/%s", masterHost, streamKey)
// Result: "rtmp://10.0.1.5:1935/live/abc123"
```

## Configuration Examples

### Example 1: IP Address

```bash
./bin/agent --register --master https://10.0.1.5:8080
```
**Result**: Streams to `rtmp://10.0.1.5:1935/live/{stream_key}`

### Example 2: Hostname

```bash
./bin/agent --register --master https://master.example.com:8080
```
**Result**: Streams to `rtmp://master.example.com:1935/live/{stream_key}`

### Example 3: Local Development

```bash
./bin/agent --register --master http://localhost:8080
```
**Result**: Streams to `rtmp://localhost:1935/live/{stream_key}`

### Example 4: Custom Port

```bash
./bin/agent --register --master https://master.local:9090
```
**Result**: Streams to `rtmp://master.local:1935/live/{stream_key}`

**Note**: RTMP server always runs on port **1935** regardless of API port.

## Overriding RTMP URL

If you need to override the default RTMP URL (e.g., RTMP server on different host), specify it in job parameters:

```json
{
  "scenario": "custom-rtmp-test",
  "parameters": {
    "output_mode": "rtmp",
    "rtmp_url": "rtmp://rtmp-server.example.com:1935/live/custom-stream",
    "bitrate": "5000k"
  }
}
```

## Architecture

```
┌──────────────────────────────────┐
│  Master Node (10.0.1.5)          │
│                                  │
│  ┌────────────────────────────┐ │
│  │  API Server :8080          │ │ ◄── Worker registers here
│  │  (REST API for jobs)       │ │     (--master flag points here)
│  └────────────────────────────┘ │
│                                  │
│  ┌────────────────────────────┐ │
│  │  RTMP Server :1935         │ │ ◄── Worker streams video here
│  │  (nginx-rtmp or similar)   │ │     (auto-constructed URL)
│  └────────────────────────────┘ │
└──────────────────────────────────┘
           ▲                ▲
           │                │
           │ Jobs           │ RTMP stream
           │                │
┌──────────┴────────────────┴──────┐
│  Worker Node                     │
│                                  │
│  Started with:                   │
│  --master https://10.0.1.5:8080  │
│                                  │
│  ┌────────────────────────────┐ │
│  │  Agent                     │ │
│  │  - Polls for jobs         │ │
│  │  - Executes FFmpeg        │ │
│  │  - Streams to master RTMP │ │
│  └────────────────────────────┘ │
└──────────────────────────────────┘
```

## Logging and Verification

When a worker executes an RTMP streaming job, it logs the master URL source:

```
2025/12/30 15:00:00 RTMP streaming mode enabled: rtmp://10.0.1.5:1935/live/abc123
2025/12/30 15:00:00   Master URL source: https://10.0.1.5:8080 (from --master flag)
2025/12/30 15:00:00   Streaming to master node RTMP server
```

## Troubleshooting

### Problem: Worker streams to localhost instead of master

**Symptom**:
```
RTMP streaming mode enabled: rtmp://localhost:1935/live/abc123
```

**Cause**: Worker not started with `--master` flag, using default.

**Solution**:
```bash
# Correct - specify master URL
./bin/agent --register --master https://MASTER_IP:8080

# Wrong - uses default localhost
./bin/agent --register
```

### Problem: Cannot resolve master hostname

**Symptom**:
```
ffmpeg error: rtmp://master.local:1935/live/stream - Connection failed
```

**Cause**: Hostname not resolvable from worker node.

**Solution**:
```bash
# Option 1: Use IP address instead
./bin/agent --register --master https://10.0.1.5:8080

# Option 2: Add to /etc/hosts
echo "10.0.1.5 master.local" | sudo tee -a /etc/hosts
```

### Problem: Wrong port extracted

**Symptom**:
```
RTMP streaming mode enabled: rtmp://10.0.1.5:8080:1935/live/abc123
```

**Cause**: Bug in hostname extraction (should not happen with current code).

**Debug**: Check logs for "Master URL source" to see what was configured.

## Alternative Approaches (Not Currently Implemented)

### Option 1: Service Discovery

Use DNS SRV records or Consul/etcd for automatic master discovery:

```bash
# Worker discovers master automatically
./bin/agent --register --service-name ffmpeg-master
```

**Pros**: No manual configuration
**Cons**: Requires additional infrastructure

### Option 2: Master Returns RTMP URL in Registration

Master could return its RTMP endpoint in the registration response:

```json
{
  "id": "node-123",
  "status": "registered",
  "master_rtmp_url": "rtmp://10.0.1.5:1935/live"
}
```

**Pros**: More flexible, master controls RTMP location
**Cons**: Requires API changes

### Option 3: Environment Variable

```bash
export MASTER_RTMP_URL="rtmp://10.0.1.5:1935/live"
./bin/agent --register --master https://10.0.1.5:8080
```

**Pros**: Separates API and RTMP configuration
**Cons**: More configuration needed

## Current Design Benefits

1. **Simple**: Single configuration point (`--master` flag)
2. **Consistent**: Same URL for both API and RTMP (just different ports)
3. **Explicit**: User explicitly specifies master location
4. **Flexible**: Can override with `rtmp_url` parameter if needed
5. **Logged**: Clear logging of master URL source and RTMP destination

## Security Considerations

### TLS for API, No TLS for RTMP

```bash
./bin/agent --register --master https://10.0.1.5:8080
# API uses: https://10.0.1.5:8080 (encrypted)
# RTMP uses: rtmp://10.0.1.5:1935 (not encrypted)
```

**Note**: RTMP doesn't support TLS by default. For secure streaming, consider:
1. RTMPS (RTMP over TLS) - requires nginx-rtmp with SSL
2. VPN/Wireguard between nodes
3. RTMP with SRT (Secure Reliable Transport)

### API Authentication vs RTMP Authentication

```bash
# API requires authentication
curl -H "Authorization: Bearer $API_KEY" https://master:8080/jobs

# RTMP typically doesn't require authentication
# Use stream keys as lightweight auth:
rtmp://master:1935/live/{unique-stream-key}
```

## Summary

- **Master URL**: Configured via `--master` flag when starting worker
- **Storage**: Stored in `Client` object, accessed via `GetMasterURL()`
- **RTMP Construction**: Master hostname extracted, RTMP URL built as `rtmp://{host}:1935/live/{key}`
- **Override**: Job can specify custom `rtmp_url` parameter
- **Logging**: Clear indication of master URL source and RTMP destination

**Example**:
```bash
# Worker command
./bin/agent --register --master https://10.0.1.5:8080

# Results in RTMP streaming to
rtmp://10.0.1.5:1935/live/{job_id}
```
