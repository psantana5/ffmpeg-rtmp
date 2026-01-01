# ffrtmp CLI

Command-line interface for the ffmpeg-rtmp distributed transcoding system.

## Installation

Build the CLI:

```bash
go build -o bin/ffrtmp ./cmd/ffrtmp
```

Or add to your PATH:

```bash
go install ./cmd/ffrtmp
```

## Configuration

The CLI reads configuration from `~/.ffrtmp/config.yaml`:

```yaml
master_url: https://localhost:8080
api_key: your-api-key-here
```

### Configuration Options

- `master_url` (or `master`) - The URL of the master server (default: https://localhost:8080)
- `api_key` (or `token`) - API key for authentication (required if master has authentication enabled)

**Note:** The CLI supports both `master_url`/`master` and `api_key`/`token` keys for backward compatibility.

### Configuration Precedence

1. Command-line flags (`--master`, `--config`)
2. Config file (`~/.ffrtmp/config.yaml` or `~/.ffrtmp/config`)
3. Environment variables (`MASTER_URL`, `MASTER_API_KEY`)
4. Default values (https://localhost:8080)

**Security Note:** Use HTTPS for production deployments. For local development with self-signed certificates, the CLI automatically skips TLS verification for localhost/127.0.0.1.

## Usage

### Global Flags

- `--master <url>` - Master API URL (default: https://localhost:8080 or from config)
- `--output <format>` - Output format: `table` (default) or `json`
- `--config <file>` - Config file path (default: `~/.ffrtmp/config.yaml` or `~/.ffrtmp/config`)

### Commands

#### List Nodes

Display all registered compute nodes:

```bash
# Table output (default)
ffrtmp nodes list

# JSON output for scripting
ffrtmp nodes list --output json
```

**Example output (table):**
```
┌──────────────────────────────────────┬──────────────────────────┬───────────┬────────┬─────────────────────────┬─────────────────┐
│                  ID                  │           HOST           │  STATUS   │  TYPE  │           CPU           │       GPU       │
├──────────────────────────────────────┼──────────────────────────┼───────────┼────────┼─────────────────────────┼─────────────────┤
│ d8455638-456f-4bf4-9131-b56cc1b7ecb6 │ worker1.example.com:8081 │ available │ server │ Intel Xeon E5-2680 (16) │ NVIDIA RTX 4090 │
└──────────────────────────────────────┴──────────────────────────┴───────────┴────────┴─────────────────────────┴─────────────────┘

Total nodes: 1
```

#### Submit Job

Create a new transcoding job:

```bash
# Submit with all parameters
ffrtmp jobs submit --scenario 4K60-h264 --duration 120 --bitrate 10M

# Minimal submission
ffrtmp jobs submit --scenario simple-test

# With custom confidence level
ffrtmp jobs submit --scenario 1080p60-h265 --confidence high --duration 60 --bitrate 5M

# JSON output
ffrtmp jobs submit --scenario test --output json
```

**Flags:**
- `--scenario <name>` - Scenario name (required, e.g., "4K60-h264")
- `--duration <seconds>` - Duration in seconds (optional)
- `--bitrate <rate>` - Target bitrate (optional, e.g., "10M")
- `--confidence <level>` - Confidence level: "auto" (default), "high", "medium", "low"

**Example output:**
```
┌────────────┬──────────────────────────────────────┐
│   FIELD    │                VALUE                 │
├────────────┼──────────────────────────────────────┤
│ Job ID     │ c5fe10ab-7629-4118-a851-b315228925f5 │
│ Scenario   │ 4K60-h264                            │
│ Confidence │ auto                                 │
│ Status     │ pending                              │
│ Created At │ 2025-12-30T13:34:41Z                 │
└────────────┴──────────────────────────────────────┘

Job submitted successfully! Job ID: c5fe10ab-7629-4118-a851-b315228925f5
```

#### Get Job Status

Retrieve the status of a specific job:

```bash
# Table output
ffrtmp jobs status <job-id>

# JSON output for scripting
ffrtmp jobs status <job-id> --output json
```

**Example:**
```bash
ffrtmp jobs status c5fe10ab-7629-4118-a851-b315228925f5
```

**Example output:**
```
┌─────────────┬──────────────────────────────────────┐
│    FIELD    │                VALUE                 │
├─────────────┼──────────────────────────────────────┤
│ Job ID      │ c5fe10ab-7629-4118-a851-b315228925f5 │
│ Scenario    │ 4K60-h264                            │
│ Confidence  │ auto                                 │
│ Status      │ pending                              │
│ Retry Count │ 0                                    │
│ Created At  │ 2025-12-30T13:34:41Z                 │
│ Parameters  │ {                                    │
│             │   "bitrate": "10M",                  │
│             │   "duration": 120                    │
│             │ }                                    │
└─────────────┴──────────────────────────────────────┘
```

## Examples

### Using with different master servers

```bash
# Connect to local development server (with default HTTPS)
ffrtmp nodes list --master https://localhost:8080

# Or use HTTP if your local setup requires it
ffrtmp nodes list --master http://localhost:8080

# Connect to production server
ffrtmp nodes list --master https://master.example.com

# Use config file (recommended for regular use)
cat > ~/.ffrtmp/config.yaml << EOF
master_url: https://localhost:8080
api_key: your-secure-api-key
EOF
ffrtmp nodes list  # No --master flag needed!
```

### Configuration File Example

See [`examples/config.example.yaml`](../../examples/config.example.yaml) for a complete configuration example with comments.

### Scripting with JSON output

```bash
# Get job ID from submission
JOB_ID=$(ffrtmp jobs submit --scenario test --output json | jq -r '.id')

# Check job status
ffrtmp jobs status $JOB_ID --output json | jq '.status'

# List all nodes and extract IDs
ffrtmp nodes list --output json | jq -r '.nodes[].id'
```

### Monitoring job progress

```bash
#!/bin/bash
JOB_ID=$1

while true; do
  STATUS=$(ffrtmp jobs status $JOB_ID --output json | jq -r '.status')
  echo "Job status: $STATUS"
  
  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
    break
  fi
  
  sleep 5
done
```

## Error Handling

The CLI provides clear error messages for common issues:

- **Connection refused**: Master server is not running or URL is incorrect
- **Job not found**: Invalid job ID
- **Missing required flags**: Required parameters not provided
- **Invalid output format**: Unsupported output format (defaults to table)

Exit codes:
- `0`: Success
- `1`: Error occurred (details in error message)

## Development

Run tests:
```bash
go test ./cmd/ffrtmp/...
```

Build for all platforms:
```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o bin/ffrtmp-linux ./cmd/ffrtmp

# macOS
GOOS=darwin GOARCH=amd64 go build -o bin/ffrtmp-macos ./cmd/ffrtmp

# Windows
GOOS=windows GOARCH=amd64 go build -o bin/ffrtmp.exe ./cmd/ffrtmp
```
