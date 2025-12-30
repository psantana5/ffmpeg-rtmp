# Migration Guide: Adopting the Master/Worker Organization

This guide explains how to gradually adopt the new master/worker folder organization in your deployments.

## Overview

The new organization separates components by their role in the distributed system:
- **`master/`** - Components that run on the master node
- **`worker/`** - Components that run on worker nodes
- **`shared/`** - Components used by both master and workers

## Current Status: Dual Structure

✅ **Good news**: You don't need to change anything immediately!

The repository now maintains **both** structures:
- **Original structure** (at root level) - Still works, fully supported
- **New organized structure** (in `master/`, `worker/`, `shared/`) - Available for adoption

Both structures contain the same code, so you can use whichever works best for your deployment.

## When to Migrate

### Stick with Original Structure If:
- ✅ You're using Docker Compose for local development
- ✅ Your deployment scripts reference existing paths
- ✅ You don't want to update documentation right now
- ✅ Your team is comfortable with current structure

### Consider New Structure If:
- ✅ You're deploying production distributed mode (master + workers)
- ✅ You want clearer separation of master vs worker components
- ✅ You're documenting deployment procedures
- ✅ You're onboarding new team members
- ✅ You want to deploy master-only or worker-only nodes

## Migration Strategies

### Strategy 1: Hybrid Approach (Recommended)

Use the new structure for **documentation** while keeping original structure for **builds**:

```bash
# Build using original paths (no changes needed)
make build-master
make build-agent

# But reference new structure in documentation
# "Master components are in master/ directory"
# "Worker exporters are in worker/exporters/"
```

**Benefits**: 
- No code changes required
- Better documentation clarity
- Gradual team education

### Strategy 2: New Deployments Only

Use new structure for **new deployments**, keep original for **existing**:

```bash
# Existing deployments: Use original paths
cd /opt/ffmpeg-rtmp
git pull
make build-master

# New deployments: Use new organization
cd /opt/ffmpeg-rtmp-v2
git pull
# Reference master/ and worker/ directories in deployment docs
make build-master  # Still uses original cmd/master
```

**Benefits**:
- Zero risk to existing deployments
- Test new structure in parallel
- Easy rollback

### Strategy 3: Full Migration (Advanced)

Gradually update all references to use new structure:

**Phase 1**: Update Makefile
```makefile
# Change build targets to reference new structure
build-master:
    go build -o bin/master ./master/cmd/master

build-agent:
    go build -o bin/agent ./worker/cmd/agent
```

**Phase 2**: Update Go imports
```go
// If you later want to use new paths in go.mod
replace github.com/psantana5/ffmpeg-rtmp/pkg => ./shared/pkg
```

**Phase 3**: Update docker-compose.yml
```yaml
# Update build contexts
services:
  cpu-exporter-go:
    build:
      context: ./worker/exporters/cpu_exporter
```

**Phase 4**: Update deployment scripts
```bash
# Update systemd service files to reference new paths
ExecStart=/opt/ffmpeg-rtmp/bin/master
# (No change - binary location same)
```

## Docker Compose Users

### No Action Required

Docker Compose uses **original paths** and continues to work:

```yaml
# docker-compose.yml at root still references:
services:
  cpu-exporter-go:
    build:
      context: .
      dockerfile: src/exporters/cpu_exporter/Dockerfile
```

The new `master/`, `worker/`, `shared/` directories are copies for organizational reference.

### Optional: Update for Clarity

If desired, update docker-compose.yml to use new paths:

```yaml
services:
  # Master-side exporters
  results-exporter:
    build: ./master/exporters/results
    
  # Worker-side exporters
  cpu-exporter-go:
    build: ./worker/exporters/cpu_exporter
```

## Production Deployments

### Master Node

**Current approach** (still works):
```bash
cd /opt/ffmpeg-rtmp
make build-master
./bin/master --port 8080
```

**With new organization** (same result):
```bash
cd /opt/ffmpeg-rtmp
# Build still uses cmd/master (no change)
make build-master

# But documentation now references:
# "Master monitoring configs are in master/monitoring/"
# "Master exporters are in master/exporters/"
```

### Worker Node

**Current approach** (still works):
```bash
cd /opt/ffmpeg-rtmp
make build-agent
./bin/agent --register --master https://master:8080
```

**With new organization** (same result):
```bash
cd /opt/ffmpeg-rtmp
# Build still uses cmd/agent (no change)
make build-agent

# But documentation now references:
# "Worker exporters are in worker/exporters/"
# "Worker deployment config is worker/deployment/ffmpeg-agent.service"
```

## Documentation Migration

### Update Your Deployment Docs

Instead of:
```markdown
The master binary is in `cmd/master/`
Grafana dashboards are in `grafana/provisioning/`
```

Write:
```markdown
The master components are organized in `master/`:
- Binary: `master/cmd/master/`
- Monitoring: `master/monitoring/grafana/`
- Exporters: `master/exporters/`
```

### Reference New READMEs

Link to component-specific docs:
- [Master Setup](master/README.md) - Master node deployment
- [Worker Setup](worker/README.md) - Worker node deployment
- [Shared Components](shared/README.md) - Common libraries

## Testing the New Structure

### Verify Nothing Broke

```bash
# Test master build
make build-master
./bin/master --help

# Test agent build
make build-agent
./bin/agent --help

# Test docker-compose
make up-build
make down
```

All should work exactly as before!

### Explore New Organization

```bash
# Browse new structure
ls -la master/
ls -la worker/
ls -la shared/

# Read new documentation
cat master/README.md
cat worker/README.md
cat FOLDER_ORGANIZATION.md
```

## Rollback Plan

### If Something Goes Wrong

The original structure is unchanged:

```bash
# Continue using original paths
cd /opt/ffmpeg-rtmp
make build-master  # Uses cmd/master
make build-agent   # Uses cmd/agent

# Ignore master/, worker/, shared/ directories
# They're organizational copies only
```

### Remove New Structure

If you want to remove the new directories:

```bash
# NOT RECOMMENDED - but possible
rm -rf master/ worker/ shared/
git checkout HEAD -- master/ worker/ shared/

# Original structure still intact
ls cmd/
ls pkg/
ls src/exporters/
```

## FAQ

### Q: Do I need to change my build scripts?
**A:** No. The Makefile and build commands work exactly as before.

### Q: Will this break my Docker Compose setup?
**A:** No. Docker Compose uses the original `docker-compose.yml` which references original paths.

### Q: Do I need to update Go imports?
**A:** No. The Go module still uses `github.com/psantana5/ffmpeg-rtmp/pkg` paths.

### Q: What about my production deployment?
**A:** Continue using current deployment. The new organization is primarily for documentation clarity.

### Q: Should I delete the original files?
**A:** No. Keep both structures. The new structure is for reference and future deployments.

### Q: How do I know which structure I'm using?
**A:** If you're building with `make build-master`, you're using original structure (which still works perfectly).

### Q: Can I mix and match?
**A:** Yes! Reference new structure in docs while using original structure for builds.

### Q: When will the original structure be removed?
**A:** No current plans. Both structures will coexist for the foreseeable future.

## Recommended Approach

### For Most Users: Do Nothing

The new structure is **documentation and organization**. Your builds still work!

### For New Deployments: Use New Docs

When deploying new master/worker nodes:
1. Read `master/README.md` for master setup
2. Read `worker/README.md` for worker setup
3. Reference organized structure in your documentation
4. Continue using original build commands

### For Advanced Users: Gradual Adoption

1. **Week 1**: Update documentation to reference new structure
2. **Week 2**: Update internal docs to use new READMEs
3. **Month 1**: Train team on new organization
4. **Month 3**: Consider updating Makefile if beneficial
5. **Future**: Evaluate full migration if/when needed

## Support

Questions about migration?
- Check [FOLDER_ORGANIZATION.md](FOLDER_ORGANIZATION.md) for detailed structure
- Check [ARCHITECTURE_DIAGRAM.md](ARCHITECTURE_DIAGRAM.md) for visual diagrams
- Open an issue: https://github.com/psantana5/ffmpeg-rtmp/issues

## Summary

✅ **No breaking changes** - Everything still works
✅ **New structure available** - Use it if helpful
✅ **Backward compatible** - Original paths still supported
✅ **Flexible adoption** - Migrate at your own pace
✅ **Better docs** - Clearer organization for new users

The new structure is a **benefit, not a burden**. Adopt it when it makes sense for your team!
