#!/bin/bash
#
# Production Stack Deployment Script
# Brings up the entire ffmpeg-rtmp distributed system with production scheduler
#
# Usage: ./deploy_production.sh [start|stop|restart|status|logs]
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
MASTER_PORT="${MASTER_PORT:-8080}"
WORKER_BASE_PORT="${WORKER_BASE_PORT:-9000}"
NUM_WORKERS="${NUM_WORKERS:-3}"
LOG_DIR="${LOG_DIR:-./logs}"
PID_DIR="${PID_DIR:-./pids}"
DB_PATH="${DB_PATH:-./master.db}"
MASTER_BINARY="${MASTER_BINARY:-./bin/master}"
WORKER_BINARY="${WORKER_BINARY:-./bin/agent}"

# Ensure directories exist
mkdir -p "$LOG_DIR" "$PID_DIR"

#############################################
# Helper Functions
#############################################

log_info() {
  echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

log_section() {
  echo ""
  echo -e "${BLUE}========================================${NC}"
  echo -e "${BLUE}$1${NC}"
  echo -e "${BLUE}========================================${NC}"
}

check_binary() {
  if [ ! -f "$1" ]; then
    log_error "Binary not found: $1"
    log_info "Please build first: make build"
    exit 1
  fi
}

is_running() {
  local pid_file=$1
  if [ -f "$pid_file" ]; then
    local pid=$(cat "$pid_file")
    if kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
  fi
  return 1
}

wait_for_service() {
  local name=$1
  local port=$2
  local max_wait=30
  local waited=0

  log_info "Waiting for $name to be ready on port $port..."
  while [ $waited -lt $max_wait ]; do
    if curl -s "http://localhost:$port/health" >/dev/null 2>&1; then
      log_info "$name is ready!"
      return 0
    fi
    sleep 1
    waited=$((waited + 1))
    echo -n "."
  done
  echo ""
  log_error "$name failed to start within ${max_wait}s"
  return 1
}

#############################################
# Start Functions
#############################################

start_master() {
  log_section "Starting Master Node"

  check_binary "$MASTER_BINARY"

  local pid_file="$PID_DIR/master.pid"

  if is_running "$pid_file"; then
    log_warn "Master already running (PID: $(cat $pid_file))"
    return 0
  fi

  log_info "Starting master on port $MASTER_PORT..."
  log_info "Database: $DB_PATH"
  log_info "Logs: $LOG_DIR/master.log"

  # Start master with production scheduler enabled (disable TLS for local dev)
  PORT="$MASTER_PORT" \
    DB_PATH="$DB_PATH" \
    DISABLE_TLS="${DISABLE_TLS:-true}" \
    nohup "$MASTER_BINARY" \
    >>"$LOG_DIR/master.log" 2>&1 &

  local pid=$!
  echo $pid >"$pid_file"

  log_info "Master started (PID: $pid)"

  # Wait for master to be ready
  if wait_for_service "Master" "$MASTER_PORT"; then
    log_info "✓ Master node operational"

    # Show scheduler status
    log_info "Production scheduler active:"
    tail -5 "$LOG_DIR/master.log" | grep -E "\[Scheduler\]|\[FSM\]|\[Health\]" || true
  else
    log_error "✗ Master failed to start"
    return 1
  fi
}

start_workers() {
  log_section "Starting Worker Nodes"

  check_binary "$WORKER_BINARY"

  local master_url="http://localhost:$MASTER_PORT"

  for i in $(seq 1 $NUM_WORKERS); do
    local worker_port=$((WORKER_BASE_PORT + i - 1))
    local worker_name="worker-$i"
    local pid_file="$PID_DIR/${worker_name}.pid"

    if is_running "$pid_file"; then
      log_warn "$worker_name already running (PID: $(cat $pid_file))"
      continue
    fi

    log_info "Starting $worker_name on port $worker_port..."

    # Start worker with unique port and registration flag
    PORT="$worker_port" \
      MASTER_URL="$master_url" \
      WORKER_NAME="$worker_name" \
      nohup "$WORKER_BINARY" --register \
      >>"$LOG_DIR/${worker_name}.log" 2>&1 &

    local pid=$!
    echo $pid >"$pid_file"

    log_info "  ✓ $worker_name started (PID: $pid, Port: $worker_port)"

    # Brief pause between workers
    sleep 1
  done

  log_info "Waiting for workers to register..."
  sleep 3

  # Verify workers registered
  local registered=$(curl -s "http://localhost:$MASTER_PORT/nodes" | grep -c '"status":"available"' || echo 0)
  log_info "✓ $registered/$NUM_WORKERS workers registered and available"
}

start_monitoring() {
  log_section "Starting Monitoring Stack (Optional)"

  # Check if Prometheus/Grafana configs exist
  if [ -f "prometheus.yml" ] && [ -f "docker-compose.yml" ]; then
    log_info "Starting Prometheus and Grafana..."
    docker-compose up -d prometheus grafana 2>/dev/null || {
      log_warn "Docker Compose not available or monitoring stack not configured"
      log_info "Skipping monitoring stack"
      return 0
    }
    log_info "✓ Monitoring available at http://localhost:3000 (Grafana)"
  else
    log_info "No monitoring configuration found, skipping"
  fi
}

start_all() {
  log_section "🚀 Starting Production Stack"

  start_master
  start_workers
  start_monitoring

  log_section "✅ Stack Started Successfully"

  show_status

  echo ""
  log_info "Dashboard URLs:"
  log_info "  Master API:  http://localhost:$MASTER_PORT"
  log_info "  Health:      http://localhost:$MASTER_PORT/health"
  log_info "  Jobs:        http://localhost:$MASTER_PORT/jobs"
  log_info "  Workers:     http://localhost:$MASTER_PORT/nodes"
  echo ""
  log_info "Logs:"
  log_info "  Master:      tail -f $LOG_DIR/master.log"
  log_info "  All Workers: tail -f $LOG_DIR/worker-*.log"
  log_info "  Scheduler:   tail -f $LOG_DIR/master.log | grep -E '\\[Scheduler\\]|\\[FSM\\]|\\[Health\\]|\\[Cleanup\\]'"
  echo ""
  log_info "Quick commands:"
  log_info "  Submit job:  curl -X POST http://localhost:$MASTER_PORT/jobs -d '{\"scenario\":\"test\",\"confidence\":\"high\"}' -H 'Content-Type: application/json'"
  log_info "  List jobs:   curl http://localhost:$MASTER_PORT/jobs"
  log_info "  View status: ./deploy_production.sh status"
  log_info "  View logs:   ./deploy_production.sh logs"
}

#############################################
# Stop Functions
#############################################

stop_master() {
  log_info "Stopping master..."
  local pid_file="$PID_DIR/master.pid"

  if is_running "$pid_file"; then
    local pid=$(cat "$pid_file")
    log_info "Stopping master (PID: $pid)..."
    kill "$pid" 2>/dev/null || true

    # Wait for graceful shutdown
    local waited=0
    while kill -0 "$pid" 2>/dev/null && [ $waited -lt 10 ]; do
      sleep 1
      waited=$((waited + 1))
    done

    # Force kill if still running
    if kill -0 "$pid" 2>/dev/null; then
      log_warn "Force killing master..."
      kill -9 "$pid" 2>/dev/null || true
    fi

    rm -f "$pid_file"
    log_info "✓ Master stopped"
  else
    log_info "Master not running"
  fi
}

stop_workers() {
  log_info "Stopping workers..."

  for i in $(seq 1 $NUM_WORKERS); do
    local worker_name="worker-$i"
    local pid_file="$PID_DIR/${worker_name}.pid"

    if is_running "$pid_file"; then
      local pid=$(cat "$pid_file")
      log_info "Stopping $worker_name (PID: $pid)..."
      kill "$pid" 2>/dev/null || true
      rm -f "$pid_file"
    fi
  done

  # Wait for all workers to stop
  sleep 2
  log_info "✓ Workers stopped"
}

stop_monitoring() {
  if command -v docker-compose &>/dev/null && [ -f "docker-compose.yml" ]; then
    log_info "Stopping monitoring stack..."
    docker-compose down 2>/dev/null || true
  fi
}

stop_all() {
  log_section "🛑 Stopping Production Stack"

  stop_workers
  stop_master
  stop_monitoring

  log_info "✓ Stack stopped"
}

#############################################
# Status Functions
#############################################

show_status() {
  log_section "📊 Production Stack Status"

  # Master status
  local master_pid_file="$PID_DIR/master.pid"
  echo -n "Master:    "
  if is_running "$master_pid_file"; then
    local pid=$(cat "$master_pid_file")
    echo -e "${GREEN}RUNNING${NC} (PID: $pid, Port: $MASTER_PORT)"

    # Show scheduler metrics
    if curl -s "http://localhost:$MASTER_PORT/health" >/dev/null 2>&1; then
      echo "  └─ Health: ✓ OK"

      # Try to get job count
      local jobs=$(curl -s "http://localhost:$MASTER_PORT/jobs" 2>/dev/null | grep -o '"id"' | wc -l || echo "?")
      local nodes=$(curl -s "http://localhost:$MASTER_PORT/nodes" 2>/dev/null | grep -o '"id"' | wc -l || echo "?")
      echo "  └─ Jobs: $jobs | Workers: $nodes"
    else
      echo "  └─ Health: ✗ NOT RESPONDING"
    fi
  else
    echo -e "${RED}STOPPED${NC}"
  fi

  # Worker status
  echo ""
  echo "Workers:"
  local running_count=0
  for i in $(seq 1 $NUM_WORKERS); do
    local worker_name="worker-$i"
    local pid_file="$PID_DIR/${worker_name}.pid"
    local worker_port=$((WORKER_BASE_PORT + i - 1))

    echo -n "  $worker_name: "
    if is_running "$pid_file"; then
      local pid=$(cat "$pid_file")
      echo -e "${GREEN}RUNNING${NC} (PID: $pid, Port: $worker_port)"
      running_count=$((running_count + 1))
    else
      echo -e "${RED}STOPPED${NC}"
    fi
  done

  echo ""
  echo "Summary: $running_count/$NUM_WORKERS workers running"

  # Recent scheduler activity
  if [ -f "$LOG_DIR/master.log" ]; then
    echo ""
    echo "Recent Scheduler Activity:"
    tail -10 "$LOG_DIR/master.log" | grep -E "\[Scheduler\]|\[Health\]|\[Cleanup\]" | tail -5 || echo "  (No recent activity)"
  fi
}

#############################################
# Log Functions
#############################################

show_logs() {
  log_section "📜 Recent Logs"

  if [ -f "$LOG_DIR/master.log" ]; then
    echo ""
    echo "=== Master (last 20 lines) ==="
    tail -20 "$LOG_DIR/master.log"
  fi

  echo ""
  echo "=== Scheduler Activity (last 15 lines) ==="
  if [ -f "$LOG_DIR/master.log" ]; then
    grep -E "\[Scheduler\]|\[FSM\]|\[Health\]|\[Cleanup\]" "$LOG_DIR/master.log" | tail -15 || echo "(No scheduler activity yet)"
  fi

  echo ""
  echo "=== Workers (last 5 lines each) ==="
  for i in $(seq 1 $NUM_WORKERS); do
    local worker_name="worker-$i"
    if [ -f "$LOG_DIR/${worker_name}.log" ]; then
      echo "--- $worker_name ---"
      tail -5 "$LOG_DIR/${worker_name}.log"
    fi
  done

  echo ""
  log_info "To follow logs in real-time:"
  log_info "  Master:    tail -f $LOG_DIR/master.log"
  log_info "  Scheduler: tail -f $LOG_DIR/master.log | grep -E '\\[Scheduler\\]|\\[FSM\\]|\\[Health\\]|\\[Cleanup\\]'"
  log_info "  Workers:   tail -f $LOG_DIR/worker-*.log"
}

#############################################
# Health Check
#############################################

health_check() {
  log_section "🏥 Health Check"

  # Check master
  echo -n "Master API: "
  if curl -s -f "http://localhost:$MASTER_PORT/health" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ HEALTHY${NC}"
  else
    echo -e "${RED}✗ UNHEALTHY${NC}"
    return 1
  fi

  # Check database
  echo -n "Database:   "
  if [ -f "$DB_PATH" ]; then
    echo -e "${GREEN}✓ EXISTS${NC} ($DB_PATH)"
  else
    echo -e "${YELLOW}! NOT FOUND${NC} (will be created on first run)"
  fi

  # Check workers
  echo ""
  echo "Workers:"
  local healthy=0
  for i in $(seq 1 $NUM_WORKERS); do
    local worker_port=$((WORKER_BASE_PORT + i - 1))
    echo -n "  worker-$i (port $worker_port): "
    if curl -s -f "http://localhost:$worker_port/health" >/dev/null 2>&1; then
      echo -e "${GREEN}✓ HEALTHY${NC}"
      healthy=$((healthy + 1))
    else
      echo -e "${RED}✗ UNREACHABLE${NC}"
    fi
  done

  echo ""
  echo "Summary: $healthy/$NUM_WORKERS workers healthy"

  # Scheduler metrics
  echo ""
  echo "Scheduler Metrics:"
  if [ -f "$LOG_DIR/master.log" ]; then
    echo -n "  Last scheduling run: "
    grep "\[Scheduler\].*Scheduling:" "$LOG_DIR/master.log" | tail -1 | awk '{print $1, $2}' || echo "Never"

    echo -n "  Last health check:   "
    grep "\[Health\]" "$LOG_DIR/master.log" | tail -1 | awk '{print $1, $2}' || echo "Never"

    echo -n "  Last cleanup cycle:  "
    grep "\[Cleanup\]" "$LOG_DIR/master.log" | tail -1 | awk '{print $1, $2}' || echo "Never"
  else
    echo "  (No log file yet)"
  fi
}

#############################################
# Demo
#############################################

run_demo() {
  log_section "🎬 Running Demo"

  log_info "Submitting 5 test jobs with different priorities..."

  # High priority job
  curl -s -X POST "http://localhost:$MASTER_PORT/jobs" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"4K60-h264","priority":"high","queue":"default"}' | jq -r '.id' || true
  log_info "✓ Submitted high priority job"

  # Medium priority jobs
  for i in {1..3}; do
    curl -s -X POST "http://localhost:$MASTER_PORT/jobs" \
      -H "Content-Type: application/json" \
      -d '{"scenario":"1080p30-h264","priority":"medium","queue":"default"}' | jq -r '.id' || true
  done
  log_info "✓ Submitted 3 medium priority jobs"

  # Low priority job
  curl -s -X POST "http://localhost:$MASTER_PORT/jobs" \
    -H "Content-Type: application/json" \
    -d '{"scenario":"720p30-h264","priority":"low","queue":"batch"}' | jq -r '.id' || true
  log_info "✓ Submitted low priority job"

  sleep 2

  log_info "Current job status:"
  curl -s "http://localhost:$MASTER_PORT/jobs" | jq -r '.[] | "\(.sequence_number). \(.scenario) - \(.status) (priority: \(.priority))"' | head -10 || true

  echo ""
  log_info "Watch scheduler in action:"
  log_info "  tail -f $LOG_DIR/master.log | grep -E '\\[Scheduler\\]|\\[FSM\\]'"
}

#############################################
# Main
#############################################

case "${1:-}" in
start)
  start_all
  ;;
stop)
  stop_all
  ;;
restart)
  stop_all
  sleep 2
  start_all
  ;;
status)
  show_status
  ;;
logs)
  show_logs
  ;;
health)
  health_check
  ;;
demo)
  run_demo
  ;;
*)
  echo "Usage: $0 {start|stop|restart|status|logs|health|demo}"
  echo ""
  echo "Commands:"
  echo "  start    - Start master and workers with production scheduler"
  echo "  stop     - Stop all services"
  echo "  restart  - Stop and start all services"
  echo "  status   - Show service status and metrics"
  echo "  logs     - Show recent logs"
  echo "  health   - Run health check"
  echo "  demo     - Submit test jobs to demonstrate scheduler"
  echo ""
  echo "Environment Variables:"
  echo "  MASTER_PORT       - Master API port (default: 8080)"
  echo "  NUM_WORKERS       - Number of workers (default: 3)"
  echo "  WORKER_BASE_PORT  - Starting port for workers (default: 9000)"
  echo "  LOG_DIR           - Log directory (default: ./logs)"
  echo "  DB_PATH           - Database path (default: ./master.db)"
  echo ""
  echo "Examples:"
  echo "  # Start with 5 workers"
  echo "  NUM_WORKERS=5 $0 start"
  echo ""
  echo "  # Use custom ports"
  echo "  MASTER_PORT=9090 WORKER_BASE_PORT=10000 $0 start"
  echo ""
  echo "  # Watch scheduler logs in real-time"
  echo "  tail -f ./logs/master.log | grep -E '\\[Scheduler\\]|\\[FSM\\]|\\[Health\\]|\\[Cleanup\\]'"
  exit 1
  ;;
esac
