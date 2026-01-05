# 🚀 Quick Start - FFmpeg-RTMP Production Stack

## One Command Deployment

```bash
./full-stack-deploy.sh
```

**That's it!** 30 seconds later you have:
- ✅ Master + Worker (job processing)
- ✅ NGINX RTMP (streaming)
- ✅ Grafana (dashboards) 
- ✅ 12+ exporters (monitoring)
- ✅ 17 services total

## Access

- **Grafana**: http://localhost:3000 (admin/admin)
- **Master API**: https://localhost:8080
- **RTMP Stream**: `rtmp://localhost:1935/live/<key>`

## Commands

```bash
./full-stack-deploy.sh start    # Start all
./full-stack-deploy.sh stop     # Stop all
./full-stack-deploy.sh status   # Check status
```

## Test It

```bash
# Submit a job
./bin/ffrtmp jobs submit --scenario test

# Check status
./bin/ffrtmp jobs list

# View in Grafana
open http://localhost:3000
```

**Complete docs**: See `FULL_STACK_DEPLOY.md`
