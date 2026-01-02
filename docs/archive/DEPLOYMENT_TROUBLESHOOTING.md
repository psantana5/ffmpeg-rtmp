# Deployment Script - Troubleshooting

## Issue: Workers fail with "401: Missing Authorization header"

**Symptom:**
```
Failed to register with master: registration failed with status 401: Missing Authorization header
```

**Root Cause:**
The `FFMPEG_RTMP_API_KEY` environment variable is set in your shell, causing the master to enable authentication.

**Solution:**

Before running the deployment script, unset the API key:

```bash
unset FFMPEG_RTMP_API_KEY
./deploy_production.sh start
```

**Permanent Fix:**

Add to your `~/.bashrc` or `~/.zshrc`:
```bash
# Disable FFMPEG RTMP API key for local development
unset FFMPEG_RTMP_API_KEY
```

Then reload your shell:
```bash
source ~/.bashrc
```

**Alternative: Use the Same API Key**

If you want to keep authentication enabled, set the same API key for workers:

```bash
export FFMPEG_RTMP_API_KEY="your-secret-key"
./deploy_production.sh start
```

The script will automatically pass it to both master and workers.

##  Test It's Working

After fixing:
```bash
./deploy_production.sh restart
sleep 5
curl http://localhost:8080/nodes | jq
```

You should see 3 registered workers!

## Workaround Script

Create `start_local.sh`:
```bash
#!/bin/bash
unset FFMPEG_RTMP_API_KEY
./deploy_production.sh start
```

Then use:
```bash
chmod +x start_local.sh
./start_local.sh
```
