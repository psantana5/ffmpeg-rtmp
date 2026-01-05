#!/bin/bash
#
# FFmpeg-RTMP Worker Configuration Recommender
#
# Analyzes the system environment and recommends optimal configuration
# parameters for the worker agent based on available hardware and
# deployment context.
#
# Usage: ./scripts/recommend_config.sh [OPTIONS]
#
# Options:
#   --environment ENV    Deployment environment: development, staging, production
#   --output FORMAT      Output format: bash, json, yaml (default: bash)
#   --help              Show this help message
#

set -euo pipefail

# Default values
ENVIRONMENT="production"
OUTPUT_FORMAT="bash"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output (disabled in non-interactive mode)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m'
else
    RED='' GREEN='' YELLOW='' BLUE='' NC=''
fi

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Show help
show_help() {
    sed -n '2,14p' "$0" | sed 's/^# //'
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        --output)
            OUTPUT_FORMAT="$2"
            shift 2
            ;;
        --help)
            show_help
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information" >&2
            exit 1
            ;;
    esac
done

# Validate environment
case "$ENVIRONMENT" in
    development|staging|production) ;;
    *)
        log_error "Invalid environment: $ENVIRONMENT"
        echo "Must be one of: development, staging, production" >&2
        exit 1
        ;;
esac

# Validate output format
case "$OUTPUT_FORMAT" in
    bash|json|yaml) ;;
    *)
        log_error "Invalid output format: $OUTPUT_FORMAT"
        echo "Must be one of: bash, json, yaml" >&2
        exit 1
        ;;
esac

log_info "Analyzing system configuration..."
log_info "Environment: $ENVIRONMENT"

# Detect hardware
CPU_CORES=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo "1")
CPU_MODEL=$(grep -m1 "model name" /proc/cpuinfo 2>/dev/null | cut -d: -f2 | xargs || uname -p)
TOTAL_RAM_KB=$(grep MemTotal /proc/meminfo 2>/dev/null | awk '{print $2}' || echo "4194304")
TOTAL_RAM_GB=$((TOTAL_RAM_KB / 1024 / 1024))

# Detect GPU
HAS_GPU=false
GPU_TYPE="none"
if command -v nvidia-smi &> /dev/null; then
    if nvidia-smi &> /dev/null; then
        HAS_GPU=true
        GPU_TYPE=$(nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | head -1 || echo "NVIDIA GPU")
    fi
elif lspci 2>/dev/null | grep -i "vga.*nvidia" &> /dev/null; then
    GPU_TYPE="NVIDIA (driver not loaded)"
elif lspci 2>/dev/null | grep -i "vga.*amd" &> /dev/null; then
    GPU_TYPE="AMD"
elif lspci 2>/dev/null | grep -i "vga.*intel" &> /dev/null; then
    GPU_TYPE="Intel"
fi

# Detect node type
if [ "$CPU_CORES" -le 4 ]; then
    NODE_TYPE="laptop"
elif [ "$CPU_CORES" -le 8 ]; then
    NODE_TYPE="desktop"
elif [ "$CPU_CORES" -le 32 ]; then
    NODE_TYPE="server"
else
    NODE_TYPE="hpc"
fi

# Detect if running in container
IN_CONTAINER=false
if [ -f /.dockerenv ] || grep -q docker /proc/1/cgroup 2>/dev/null; then
    IN_CONTAINER=true
fi

# Detect if master is on localhost
MASTER_LOCAL=false
if [ -f "$PROJECT_ROOT/master.db" ] || pgrep -f "bin/master" &> /dev/null; then
    MASTER_LOCAL=true
fi

log_info "Hardware Detection:"
log_info "  CPU: $CPU_MODEL"
log_info "  Cores: $CPU_CORES"
log_info "  RAM: ${TOTAL_RAM_GB}GB"
log_info "  GPU: $GPU_TYPE"
log_info "  Node Type: $NODE_TYPE"
log_info "  Container: $IN_CONTAINER"
log_info "  Master Local: $MASTER_LOCAL"

# Calculate recommendations
log_info "Calculating optimal configuration..."

# Concurrent jobs recommendation
if [ "$HAS_GPU" = true ]; then
    # GPU systems can handle more concurrent jobs
    case "$NODE_TYPE" in
        laptop)
            MAX_CONCURRENT_JOBS=4
            ;;
        desktop)
            MAX_CONCURRENT_JOBS=6
            ;;
        server)
            MAX_CONCURRENT_JOBS=12
            ;;
        hpc)
            MAX_CONCURRENT_JOBS=24
            ;;
    esac
else
    # CPU-only systems - more conservative
    if [ "$CPU_CORES" -le 4 ]; then
        MAX_CONCURRENT_JOBS=2
    elif [ "$CPU_CORES" -le 8 ]; then
        MAX_CONCURRENT_JOBS=3
    elif [ "$CPU_CORES" -le 16 ]; then
        MAX_CONCURRENT_JOBS=4
    else
        MAX_CONCURRENT_JOBS=$((CPU_CORES / 4))
    fi
fi

# Adjust for environment
case "$ENVIRONMENT" in
    development)
        MAX_CONCURRENT_JOBS=$((MAX_CONCURRENT_JOBS < 2 ? 1 : MAX_CONCURRENT_JOBS / 2))
        POLL_INTERVAL="5s"
        HEARTBEAT_INTERVAL="30s"
        ;;
    staging)
        POLL_INTERVAL="5s"
        HEARTBEAT_INTERVAL="30s"
        ;;
    production)
        POLL_INTERVAL="3s"
        HEARTBEAT_INTERVAL="15s"
        ;;
esac

# Master URL recommendation
if [ "$MASTER_LOCAL" = true ] && [ "$ENVIRONMENT" = "development" ]; then
    MASTER_URL="https://localhost:8080"
else
    MASTER_URL="https://master.example.com:8080"
fi

# TLS recommendations
if [ "$ENVIRONMENT" = "development" ]; then
    USE_TLS=true
    INSECURE_SKIP_VERIFY=true
    USE_MTLS=false
else
    USE_TLS=true
    INSECURE_SKIP_VERIFY=false
    USE_MTLS=true
fi

# Metrics port
METRICS_PORT=9091

# Generate input recommendation
if [ "$ENVIRONMENT" = "production" ]; then
    GENERATE_INPUT=false  # Use real input files in production
else
    GENERATE_INPUT=true   # Auto-generate for testing
fi

# Master as worker
if [ "$MASTER_LOCAL" = true ] && [ "$ENVIRONMENT" != "production" ]; then
    ALLOW_MASTER_AS_WORKER=true
else
    ALLOW_MASTER_AS_WORKER=false
fi

# Build command line
CMD_FLAGS=(
    "--master $MASTER_URL"
    "--register"
    "--max-concurrent-jobs $MAX_CONCURRENT_JOBS"
    "--poll-interval $POLL_INTERVAL"
    "--heartbeat-interval $HEARTBEAT_INTERVAL"
    "--metrics-port $METRICS_PORT"
)

if [ "$INSECURE_SKIP_VERIFY" = true ]; then
    CMD_FLAGS+=("--insecure-skip-verify")
fi

if [ "$GENERATE_INPUT" = true ]; then
    CMD_FLAGS+=("--generate-input")
else
    CMD_FLAGS+=("--generate-input=false")
fi

if [ "$ALLOW_MASTER_AS_WORKER" = true ]; then
    CMD_FLAGS+=("--allow-master-as-worker")
    CMD_FLAGS+=("--skip-confirmation")
fi

if [ "$USE_MTLS" = true ]; then
    CMD_FLAGS+=("--cert /path/to/worker.crt")
    CMD_FLAGS+=("--key /path/to/worker.key")
    CMD_FLAGS+=("--ca /path/to/ca.crt")
fi

if [ "$ENVIRONMENT" = "production" ]; then
    CMD_FLAGS+=("--api-key \${MASTER_API_KEY}")
fi

# Output recommendations
case "$OUTPUT_FORMAT" in
    bash)
        echo "#!/bin/bash"
        echo "#"
        echo "# FFmpeg-RTMP Worker Configuration"
        echo "# Generated: $(date)"
        echo "# Environment: $ENVIRONMENT"
        echo "# Node Type: $NODE_TYPE ($CPU_CORES cores, ${TOTAL_RAM_GB}GB RAM)"
        echo "# GPU: $GPU_TYPE"
        echo "#"
        echo ""
        echo "# Recommended configuration"
        echo "MAX_CONCURRENT_JOBS=$MAX_CONCURRENT_JOBS"
        echo "POLL_INTERVAL=\"$POLL_INTERVAL\""
        echo "HEARTBEAT_INTERVAL=\"$HEARTBEAT_INTERVAL\""
        echo "METRICS_PORT=$METRICS_PORT"
        echo "MASTER_URL=\"$MASTER_URL\""
        echo ""
        echo "# Start worker command"
        echo "./bin/agent \\"
        for i in "${!CMD_FLAGS[@]}"; do
            if [ "$i" -eq $((${#CMD_FLAGS[@]} - 1)) ]; then
                echo "  ${CMD_FLAGS[$i]}"
            else
                echo "  ${CMD_FLAGS[$i]} \\"
            fi
        done
        ;;
        
    json)
        cat <<EOF
{
  "environment": "$ENVIRONMENT",
  "hardware": {
    "cpu_cores": $CPU_CORES,
    "cpu_model": "$CPU_MODEL",
    "ram_gb": $TOTAL_RAM_GB,
    "gpu": "$GPU_TYPE",
    "has_gpu": $HAS_GPU,
    "node_type": "$NODE_TYPE",
    "in_container": $IN_CONTAINER
  },
  "recommendations": {
    "max_concurrent_jobs": $MAX_CONCURRENT_JOBS,
    "poll_interval": "$POLL_INTERVAL",
    "heartbeat_interval": "$HEARTBEAT_INTERVAL",
    "metrics_port": $METRICS_PORT,
    "master_url": "$MASTER_URL",
    "use_tls": $USE_TLS,
    "insecure_skip_verify": $INSECURE_SKIP_VERIFY,
    "use_mtls": $USE_MTLS,
    "generate_input": $GENERATE_INPUT,
    "allow_master_as_worker": $ALLOW_MASTER_AS_WORKER
  },
  "command": "./bin/agent ${CMD_FLAGS[*]}"
}
EOF
        ;;
        
    yaml)
        cat <<EOF
environment: $ENVIRONMENT

hardware:
  cpu_cores: $CPU_CORES
  cpu_model: "$CPU_MODEL"
  ram_gb: $TOTAL_RAM_GB
  gpu: "$GPU_TYPE"
  has_gpu: $HAS_GPU
  node_type: $NODE_TYPE
  in_container: $IN_CONTAINER

recommendations:
  max_concurrent_jobs: $MAX_CONCURRENT_JOBS
  poll_interval: $POLL_INTERVAL
  heartbeat_interval: $HEARTBEAT_INTERVAL
  metrics_port: $METRICS_PORT
  master_url: $MASTER_URL
  use_tls: $USE_TLS
  insecure_skip_verify: $INSECURE_SKIP_VERIFY
  use_mtls: $USE_MTLS
  generate_input: $GENERATE_INPUT
  allow_master_as_worker: $ALLOW_MASTER_AS_WORKER

command: |
  ./bin/agent ${CMD_FLAGS[*]}
EOF
        ;;
esac

log_success "Configuration generated successfully!" >&2

if [ "$OUTPUT_FORMAT" = "bash" ]; then
    echo "" >&2
    log_info "To use this configuration:" >&2
    echo "  1. Save output to a file: $0 > worker-config.sh" >&2
    echo "  2. Review and adjust paths/URLs as needed" >&2
    echo "  3. Execute: bash worker-config.sh" >&2
fi
