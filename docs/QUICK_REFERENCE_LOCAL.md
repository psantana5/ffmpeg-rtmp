# FFmpeg RTMP Stack - Quick Reference Card

## 🚀 One-Command Local Setup

```bash
./scripts/run_local_stack.sh
```

## 📋 Essential Commands

### Start the Stack
```bash
./scripts/run_local_stack.sh                    # Full setup with test job
SKIP_TEST_JOB=true ./scripts/run_local_stack.sh # Skip test job
MASTER_PORT=8888 ./scripts/run_local_stack.sh   # Custom port
```

### Check Status
```bash
# List nodes (get API key from script output)
curl -s -k -H "Authorization: Bearer $MASTER_API_KEY" \
  https://localhost:8080/nodes | python3 -m json.tool

# List jobs
curl -s -k -H "Authorization: Bearer $MASTER_API_KEY" \
  https://localhost:8080/jobs | python3 -m json.tool

# Health check
curl -s -k https://localhost:8080/health
```

### Submit Jobs
```bash
# Simple test
curl -X POST -k -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  https://localhost:8080/jobs \
  -d '{"scenario":"test-720p","confidence":"auto"}'

# With parameters
curl -X POST -k -H "Authorization: Bearer $MASTER_API_KEY" \
  -H "Content-Type: application/json" \
  https://localhost:8080/jobs \
  -d '{
    "scenario": "live-stream-1080p",
    "queue": "live",
    "priority": "high",
    "parameters": {"duration": 60, "bitrate": "5000k"}
  }'
```

### View Logs
```bash
tail -f logs/master.log   # Master logs
tail -f logs/agent.log    # Agent logs
```

### View Metrics
```bash
curl -s http://localhost:9090/metrics | grep ffmpeg_  # Master
curl -s http://localhost:9091/metrics | grep ffmpeg_  # Agent
```

## 🔨 Manual Build Commands

```bash
make build-master      # Build master only
make build-agent       # Build agent only
make build-cli         # Build CLI only
make build-distributed # Build all
```

## 🌐 Default URLs

| Service | URL | Notes |
|---------|-----|-------|
| Master API | https://localhost:8080 | Requires API key |
| Master Health | https://localhost:8080/health | No auth required |
| Master Metrics | http://localhost:9090/metrics | Prometheus format |
| Agent Metrics | http://localhost:9091/metrics | Prometheus format |

## 🔑 API Authentication

All API requests (except `/health`) require authentication:

```bash
# Header format
Authorization: Bearer <your-api-key>

# Get API key from environment
echo $MASTER_API_KEY

# Or from script output
```

## 📊 Job Priorities

| Priority | Use Case | Queue |
|----------|----------|-------|
| `high` | Live streams, urgent VOD | `live` or `default` |
| `medium` | Standard VOD, webinars | `default` |
| `low` | Batch processing, archives | `default` |

## 🎯 Common Scenarios

| Scenario | Resolution | Use Case |
|----------|------------|----------|
| `live-stream-4k` | 3840x2160 | 4K live streaming |
| `live-stream-1080p` | 1920x1080 | HD live streaming |
| `live-stream-720p` | 1280x720 | HD live streaming |
| `standard-vod-1080p` | 1920x1080 | On-demand video |
| `standard-vod-720p` | 1280x720 | On-demand video |
| `test-720p` | 1280x720 | Testing |

## 🛠️ Troubleshooting

| Problem | Solution |
|---------|----------|
| Port in use | `lsof -ti:8080 \| xargs kill -9` |
| Build fails | `go mod tidy && make build-distributed` |
| Agent won't register | Check `logs/agent.log` for errors |
| Jobs not processing | Verify agent is running: `ps aux \| grep agent` |

## 📖 Documentation

- **Full Guide**: [docs/LOCAL_STACK_GUIDE.md](LOCAL_STACK_GUIDE.md)
- **Architecture**: [docs/ARCHITECTURE_DIAGRAM.md](ARCHITECTURE_DIAGRAM.md)
- **Production**: [deployment/README.md](../deployment/README.md)
- **API Docs**: [docs/API.md](API.md)

## 💡 Tips

- Use `SKIP_TEST_JOB=true` for faster startup
- Set `MASTER_API_KEY` to use consistent key across restarts
- Check logs in `logs/` directory for debugging
- Press Ctrl+C to cleanly stop the stack
- Master uses SQLite (`master.db`) for persistence

## 🎓 Next Steps

1. ✅ Run `./scripts/run_local_stack.sh`
2. 📝 Submit test jobs
3. 📊 View metrics
4. 🚀 Deploy to production (see [deployment/README.md](../deployment/README.md))
