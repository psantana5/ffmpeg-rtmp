# ffrtmp CLI Configuration Guide

This guide explains how to configure the `ffrtmp` CLI tool for optimal usage.

## Quick Start

Create a configuration file to avoid specifying `--master` flag every time:

```bash
mkdir -p ~/.ffrtmp
cat > ~/.ffrtmp/config.yaml << EOF
master_url: https://localhost:8080
api_key: your-api-key-here
EOF
```

Now you can run commands without additional flags:

```bash
ffrtmp jobs submit --scenario test --duration 30 --bitrate 3000k
ffrtmp nodes list
```

## Configuration File Location

The CLI searches for configuration in the following locations (in order):

1. **Custom path** specified with `--config /path/to/config.yaml`
2. **Home directory**: `~/.ffrtmp/config.yaml`
3. **Home directory (alternative)**: `~/.ffrtmp/config` (without extension)

**Recommended**: Use `~/.ffrtmp/config.yaml` for consistency.

## Configuration File Format

The configuration file uses YAML format. See [`examples/config.example.yaml`](../examples/config.example.yaml) for a complete example.

### Basic Configuration

```yaml
# Master server URL (use HTTPS for production)
master_url: https://localhost:8080

# API key for authentication
api_key: your-api-key-here
```

### Alternative Key Names (Backward Compatible)

The CLI supports both naming conventions for backward compatibility:

```yaml
# Modern format (recommended)
master_url: https://localhost:8080
api_key: your-api-key-here

# Alternative format (legacy, still supported)
master: https://localhost:8080
token: your-api-key-here
```

Both formats work identically. Use whichever matches your existing configuration.

## Configuration Precedence

When the same setting is specified in multiple places, the CLI uses this priority order:

1. **Command-line flags** (highest priority)
   ```bash
   ffrtmp jobs submit --master https://prod.example.com --scenario test
   ```

2. **Config file**
   ```yaml
   # ~/.ffrtmp/config.yaml
   master_url: https://localhost:8080
   ```

3. **Environment variables**
   ```bash
   export MASTER_URL=https://localhost:8080
   export MASTER_API_KEY=your-api-key-here
   ```

4. **Default values** (lowest priority)
   - Master URL: `https://localhost:8080`

## Environment Variables

You can configure the CLI using environment variables:

| Variable | Description | Config File Equivalent |
|----------|-------------|------------------------|
| `MASTER_URL` | Master server URL | `master_url` or `master` |
| `MASTER_API_KEY` | API key for authentication | `api_key` or `token` |

Example:

```bash
export MASTER_URL=https://localhost:8080
export MASTER_API_KEY=your-api-key-here
ffrtmp nodes list
```

## Security Considerations

### HTTPS vs HTTP

**Production**: Always use HTTPS for security

```yaml
master_url: https://master.example.com
api_key: your-secure-api-key
```

**Local Development**: HTTPS is recommended even for localhost

```yaml
master_url: https://localhost:8080
api_key: your-dev-api-key
```

The CLI automatically handles self-signed certificates for `localhost` and `127.0.0.1`.

### API Key Storage

**DO:**
- ✅ Store API keys in `~/.ffrtmp/config.yaml` with restricted permissions
- ✅ Use environment variables in CI/CD pipelines
- ✅ Use different API keys for different environments

**DON'T:**
- ❌ Commit `config.yaml` with real API keys to version control
- ❌ Share API keys in plain text
- ❌ Use production API keys in development

### File Permissions

Protect your configuration file:

```bash
chmod 600 ~/.ffrtmp/config.yaml
```

This ensures only you can read the file containing your API key.

## Multiple Environments

### Approach 1: Multiple Config Files

```bash
# Development
ffrtmp nodes list --config ~/.ffrtmp/config-dev.yaml

# Staging
ffrtmp nodes list --config ~/.ffrtmp/config-staging.yaml

# Production
ffrtmp nodes list --config ~/.ffrtmp/config-prod.yaml
```

### Approach 2: Shell Aliases

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
alias ffrtmp-dev='ffrtmp --config ~/.ffrtmp/config-dev.yaml'
alias ffrtmp-staging='ffrtmp --config ~/.ffrtmp/config-staging.yaml'
alias ffrtmp-prod='ffrtmp --config ~/.ffrtmp/config-prod.yaml'
```

Usage:

```bash
ffrtmp-dev jobs submit --scenario test
ffrtmp-prod nodes list
```

### Approach 3: Environment-Specific Directories

```bash
# Setup
mkdir -p ~/.ffrtmp-dev ~/.ffrtmp-staging ~/.ffrtmp-prod

cat > ~/.ffrtmp-dev/config.yaml << EOF
master_url: https://dev.example.com
api_key: dev-key
EOF

cat > ~/.ffrtmp-prod/config.yaml << EOF
master_url: https://prod.example.com
api_key: prod-key
EOF

# Usage with aliases
alias ffrtmp-dev='ffrtmp --config ~/.ffrtmp-dev/config.yaml'
alias ffrtmp-prod='ffrtmp --config ~/.ffrtmp-prod/config.yaml'
```

## Troubleshooting

### Error: "Client sent an HTTP request to an HTTPS server"

**Cause**: Your config has `http://` but the server expects `https://`

**Solution**: Update your config to use HTTPS:

```yaml
master_url: https://localhost:8080  # Changed from http:// to https://
api_key: your-api-key-here
```

### Error: "Connection refused"

**Cause**: Master server is not running or URL is incorrect

**Solutions**:
1. Check if the master server is running
2. Verify the URL in your config
3. Try with explicit `--master` flag to test:
   ```bash
   ffrtmp nodes list --master https://localhost:8080
   ```

### Error: "x509: certificate signed by unknown authority"

**Cause**: Server using self-signed certificate (not localhost)

**Solutions**:
1. **Recommended**: Use proper certificates for production
2. **For testing only**: The CLI auto-accepts self-signed certs for localhost/127.0.0.1
3. For other hosts, ensure proper TLS certificates are installed

### Config file not found

**Solution**: Create the config file:

```bash
mkdir -p ~/.ffrtmp
cat > ~/.ffrtmp/config.yaml << EOF
master_url: https://localhost:8080
api_key: your-api-key-here
EOF
```

Verify it's readable:

```bash
cat ~/.ffrtmp/config.yaml
```

### Config file ignored

**Check**:
1. File location: `~/.ffrtmp/config.yaml`
2. File format: Valid YAML syntax
3. File permissions: Readable by your user

**Debug**: Run with explicit config:

```bash
ffrtmp nodes list --config ~/.ffrtmp/config.yaml
```

## Examples

### Basic Setup

```bash
# Create config directory
mkdir -p ~/.ffrtmp

# Create config file
cat > ~/.ffrtmp/config.yaml << 'EOF'
master_url: https://localhost:8080
api_key: your-api-key-here
EOF

# Secure the config file
chmod 600 ~/.ffrtmp/config.yaml

# Test connection
ffrtmp nodes list
```

### Production Setup

```bash
# Production config with proper security
cat > ~/.ffrtmp/config.yaml << 'EOF'
master_url: https://master.production.example.com
api_key: prod-secure-api-key-here
EOF

chmod 600 ~/.ffrtmp/config.yaml

# Submit production job
ffrtmp jobs submit --scenario 4K60-h264 --duration 120 --bitrate 10M
```

### Scripting with Multiple Environments

```bash
#!/bin/bash
# deploy.sh - Submit jobs to different environments

ENVIRONMENT=${1:-dev}

case $ENVIRONMENT in
  dev)
    CONFIG=~/.ffrtmp/config-dev.yaml
    ;;
  staging)
    CONFIG=~/.ffrtmp/config-staging.yaml
    ;;
  prod)
    CONFIG=~/.ffrtmp/config-prod.yaml
    ;;
  *)
    echo "Unknown environment: $ENVIRONMENT"
    exit 1
    ;;
esac

echo "Submitting job to $ENVIRONMENT..."
ffrtmp jobs submit \
  --config "$CONFIG" \
  --scenario 4K60-h264 \
  --duration 120 \
  --bitrate 10M \
  --output json
```

Usage:

```bash
./deploy.sh dev      # Submit to development
./deploy.sh staging  # Submit to staging
./deploy.sh prod     # Submit to production
```

## Migration from Old Config Format

If you have an old config file with different key names:

**Old format:**
```yaml
master: http://localhost:8080
token: my-api-key
```

**Options:**

1. **Keep as-is** (backward compatible):
   ```yaml
   master: https://localhost:8080  # Just update to HTTPS
   token: my-api-key
   ```

2. **Update to new format** (recommended):
   ```yaml
   master_url: https://localhost:8080
   api_key: my-api-key
   ```

Both formats work identically. The CLI supports both for backward compatibility.

## See Also

- [CLI README](../cmd/ffrtmp/README.md) - Complete CLI documentation
- [Example Config](../examples/config.example.yaml) - Annotated configuration example
- [Dashboard Quickstart](DASHBOARD_QUICKSTART.md) - Using CLI with the dashboard
- [Main README](../README.md) - Project overview

## Getting Help

If you encounter issues not covered in this guide:

1. Check the [troubleshooting section](#troubleshooting)
2. Run with `--help` flag for command-specific help
3. Review error messages carefully - they usually indicate the problem
4. Open an issue on GitHub with details
