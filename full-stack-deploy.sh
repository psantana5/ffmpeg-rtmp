#!/bin/bash
#
# FFmpeg-RTMP Complete Production Stack Deployment
# Deploys master, worker, RTMP server, monitoring, and all exporters
#
# Usage: ./full-stack-deploy.sh [start|stop|restart|status]
#

# Colors
G='\033[0;32m' Y='\033[1;33m' R='\033[0;31m' B='\033[0;34m' C='\033[0;36m' NC='\033[0m'

# Configuration
MASTER_PORT="${MASTER_PORT:-8080}"
AGENT_METRICS_PORT="${AGENT_METRICS_PORT:-9091}"
DB_PATH="${DB_PATH:-master.db}"

# Load/generate API key
[ -f .env ] && source .env
MASTER_API_KEY="${MASTER_API_KEY:-$(openssl rand -hex 16)}"
[ ! -f .env ] && echo "MASTER_API_KEY=$MASTER_API_KEY" > .env

# Create directories
mkdir -p logs pids certs streams test_results

# Helper functions
info() { echo -e "${G}✓${NC} $1"; }
warn() { echo -e "${Y}⚠${NC} $1"; }
error() { echo -e "${R}✗${NC} $1"; exit 1; }
section() { echo ""; echo -e "${B}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"; echo -e "${B}  $1${NC}"; echo -e "${B}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"; }

is_running() { [ -f "pids/$1.pid" ] && kill -0 $(cat "pids/$1.pid") 2>/dev/null; }
wait_http() { local url=$1 max=30 i=0; while [ $i -lt $max ]; do curl -sk "$url" >/dev/null 2>&1 && return 0; sleep 1; i=$((i + 1)); done; return 1; }

check_docker() {
    if ! command -v docker &> /dev/null; then
        error "Docker not found. Please install Docker first."
    fi
    if ! docker info &> /dev/null; then
        error "Docker daemon not running. Please start Docker."
    fi
}

# Build binaries
build() {
    info "Building Go binaries..."
    [ ! -f bin/master ] && go build -o bin/master ./master/cmd/master 2>/dev/null
    [ ! -f bin/agent ] && go build -o bin/agent ./worker/cmd/agent 2>/dev/null
    [ ! -f bin/ffrtmp ] && go build -o bin/ffrtmp ./cmd/ffrtmp 2>/dev/null
}

generate_certs() {
    if [ -f certs/master.crt ]; then return 0; fi
    info "Generating TLS certificates..."
    ./bin/master --generate-cert --cert certs/master.crt --key certs/master.key >/dev/null 2>&1 || true
}

# START FUNCTIONS

start_docker_stack() {
    section "Starting Docker Stack (RTMP + Monitoring + Exporters)"
    
    check_docker
    
    info "Starting Docker services..."
    docker-compose up -d \
        nginx-rtmp \
        victoriametrics \
        grafana \
        alertmanager \
        nginx-exporter \
        cpu-exporter-go \
        docker-stats-exporter \
        node-exporter \
        cadvisor \
        results-exporter \
        qoe-exporter \
        cost-exporter \
        ml-predictions-exporter \
        exporter-health-checker \
        ffmpeg-exporter \
        2>&1 | grep -E "Creating|Starting|started" || true
    
    sleep 3
    
    # Check critical services
    local services=(nginx-rtmp victoriametrics grafana)
    for svc in "${services[@]}"; do
        if docker ps --format "{{.Names}}" | grep -q "^${svc}$"; then
            info "$svc started"
        else
            warn "$svc failed to start (check: docker logs $svc)"
        fi
    done
    
    info "Docker stack ready"
    echo ""
    echo -e "${C}RTMP Server:${NC}     rtmp://localhost:1935/live"
    echo -e "${C}Grafana:${NC}         http://localhost:3000 (admin/admin)"
    echo -e "${C}VictoriaMetrics:${NC} http://localhost:8428"
    echo -e "${C}Alertmanager:${NC}    http://localhost:9093"
}

start_master() {
    section "Starting Master Node"
    
    if is_running master; then
        warn "Master already running"
        return 0
    fi
    
    build
    generate_certs
    
    info "Starting master on port $MASTER_PORT..."
    MASTER_API_KEY="$MASTER_API_KEY" nohup ./bin/master \
        --port "$MASTER_PORT" --db "$DB_PATH" \
        --tls --cert certs/master.crt --key certs/master.key \
        --metrics --metrics-port 9090 \
        >>logs/master.log 2>&1 &
    echo $! > pids/master.pid
    
    if wait_http "https://localhost:$MASTER_PORT/health" 30; then
        info "Master ready (PID: $(cat pids/master.pid))"
    else
        error "Master failed to start (check logs/master.log)"
    fi
}

start_agent() {
    section "Starting Worker Agent"
    
    if is_running agent; then
        warn "Agent already running"
        return 0
    fi
    
    info "Starting agent..."
    FFMPEG_RTMP_API_KEY="$MASTER_API_KEY" nohup ./bin/agent \
        --master "https://localhost:$MASTER_PORT" \
        --register --insecure-skip-verify \
        --metrics-port "$AGENT_METRICS_PORT" \
        --allow-master-as-worker --skip-confirmation \
        >>logs/agent.log 2>&1 &
    echo $! > pids/agent.pid
    
    sleep 3
    info "Agent ready (PID: $(cat pids/agent.pid))"
}

start_all() {
    section "🚀 FFmpeg-RTMP Complete Production Stack"
    
    start_docker_stack
    start_master
    start_agent
    
    section "✅ Complete Stack Started Successfully!"
    
    echo ""
    echo -e "${G}Core Services:${NC}"
    echo "  Master API:      https://localhost:$MASTER_PORT"
    echo "  Worker Agent:    Running (metrics: http://localhost:$AGENT_METRICS_PORT)"
    echo "  API Key:         $MASTER_API_KEY"
    echo ""
    echo -e "${C}RTMP Streaming:${NC}"
    echo "  RTMP Ingest:     rtmp://localhost:1935/live/<stream_key>"
    echo "  HLS Playback:    http://localhost:80/hls/<stream_key>.m3u8"
    echo ""
    echo -e "${C}Monitoring:${NC}"
    echo "  Grafana:         http://localhost:3000 (admin/admin)"
    echo "  VictoriaMetrics: http://localhost:8428"
    echo "  Alertmanager:    http://localhost:9093"
    echo ""
    echo -e "${C}Metrics Endpoints:${NC}"
    echo "  Master:          http://localhost:9090/metrics"
    echo "  Agent:           http://localhost:9091/metrics"
    echo "  NGINX RTMP:      http://localhost:9728/metrics"
    echo "  Node Exporter:   http://localhost:9100/metrics"
    echo "  cAdvisor:        http://localhost:8081/metrics"
    echo "  Results:         http://localhost:9502/metrics"
    echo "  QoE:             http://localhost:9503/metrics"
    echo "  Cost:            http://localhost:9504/metrics"
    echo "  FFmpeg:          http://localhost:9506/metrics"
    echo ""
    echo -e "${G}Quick Start:${NC}"
    echo "  1. Submit job:   ./bin/ffrtmp jobs submit --scenario test"
    echo "  2. Check status: ./full-stack-deploy.sh status"
    echo "  3. View Grafana: http://localhost:3000"
    echo ""
}

# STOP FUNCTIONS

stop_agent() {
    if is_running agent; then
        info "Stopping agent..."
        kill $(cat pids/agent.pid) 2>/dev/null || true
        rm -f pids/agent.pid
    fi
}

stop_master() {
    if is_running master; then
        info "Stopping master..."
        kill $(cat pids/master.pid) 2>/dev/null || true
        rm -f pids/master.pid
    fi
}

stop_docker_stack() {
    info "Stopping Docker stack..."
    docker-compose down 2>&1 | grep -E "Stopping|Removing" || true
}

stop_all() {
    section "🛑 Stopping Complete Stack"
    
    stop_agent
    stop_master
    stop_docker_stack
    
    sleep 2
    info "Complete stack stopped"
}

# STATUS FUNCTION

show_status() {
    section "📊 Complete Stack Status"
    
    # Master & Agent
    echo -e "${B}Core Services:${NC}"
    echo -n "  Master:  "
    if is_running master; then
        echo -e "${G}RUNNING${NC} (PID: $(cat pids/master.pid))"
        if curl -sk "https://localhost:$MASTER_PORT/health" >/dev/null 2>&1; then
            local jobs=$(curl -sk -H "Authorization: Bearer $MASTER_API_KEY" "https://localhost:$MASTER_PORT/jobs" 2>/dev/null | jq -r '.jobs | length' 2>/dev/null || echo "?")
            local nodes=$(curl -sk -H "Authorization: Bearer $MASTER_API_KEY" "https://localhost:$MASTER_PORT/nodes" 2>/dev/null | jq -r '.count' 2>/dev/null || echo "?")
            echo "    └─ Jobs: $jobs | Nodes: $nodes"
        fi
    else
        echo -e "${R}STOPPED${NC}"
    fi
    
    echo -n "  Agent:   "
    if is_running agent; then
        echo -e "${G}RUNNING${NC} (PID: $(cat pids/agent.pid))"
    else
        echo -e "${R}STOPPED${NC}"
    fi
    
    echo ""
    echo -e "${B}Docker Services:${NC}"
    
    # Check Docker services
    if command -v docker &> /dev/null && docker info &> /dev/null 2>&1; then
        local critical_services=("nginx-rtmp" "victoriametrics" "grafana" "results-exporter" "qoe-exporter")
        for svc in "${critical_services[@]}"; do
            echo -n "  $svc: "
            if docker ps --format "{{.Names}}" | grep -q "^${svc}$"; then
                echo -e "${G}RUNNING${NC}"
            else
                echo -e "${R}STOPPED${NC}"
            fi
        done
        
        # Count all running exporters
        local exporter_count=$(docker ps --format "{{.Names}}" | grep -E "exporter|cadvisor" | wc -l)
        echo "  Total exporters: $exporter_count running"
    else
        echo -e "  ${Y}Docker not available${NC}"
    fi
    
    echo ""
}

# LOGS FUNCTION

show_logs() {
    section "📜 Recent Logs"
    
    echo ""
    echo "=== Master (last 15 lines) ==="
    tail -15 logs/master.log 2>/dev/null || echo "No logs"
    
    echo ""
    echo "=== Agent (last 15 lines) ==="
    tail -15 logs/agent.log 2>/dev/null || echo "No logs"
    
    echo ""
    echo "=== Docker Services ==="
    echo "nginx-rtmp:"
    docker logs nginx-rtmp --tail 5 2>/dev/null || echo "  Not running"
    echo ""
    echo "grafana:"
    docker logs grafana --tail 5 2>/dev/null || echo "  Not running"
    
    echo ""
    info "Live logs:"
    echo "  Master:  tail -f logs/master.log"
    echo "  Agent:   tail -f logs/agent.log"
    echo "  Docker:  docker logs -f nginx-rtmp"
}

# HEALTH CHECK

health_check() {
    section "🏥 Health Check"
    
    local healthy=0 total=0
    
    # Master
    echo -n "Master API:        "
    total=$((total + 1))
    if curl -sk "https://localhost:$MASTER_PORT/health" >/dev/null 2>&1; then
        echo -e "${G}✓ HEALTHY${NC}"
        healthy=$((healthy + 1))
    else
        echo -e "${R}✗ UNHEALTHY${NC}"
    fi
    
    # RTMP
    echo -n "RTMP Server:       "
    total=$((total + 1))
    if curl -s "http://localhost:80/stat" >/dev/null 2>&1; then
        echo -e "${G}✓ HEALTHY${NC}"
        healthy=$((healthy + 1))
    else
        echo -e "${R}✗ UNHEALTHY${NC}"
    fi
    
    # Grafana
    echo -n "Grafana:           "
    total=$((total + 1))
    if curl -s "http://localhost:3000/api/health" >/dev/null 2>&1; then
        echo -e "${G}✓ HEALTHY${NC}"
        healthy=$((healthy + 1))
    else
        echo -e "${R}✗ UNHEALTHY${NC}"
    fi
    
    # VictoriaMetrics
    echo -n "VictoriaMetrics:   "
    total=$((total + 1))
    if curl -s "http://localhost:8428/health" >/dev/null 2>&1; then
        echo -e "${G}✓ HEALTHY${NC}"
        healthy=$((healthy + 1))
    else
        echo -e "${R}✗ UNHEALTHY${NC}"
    fi
    
    echo ""
    echo "Overall Health: $healthy/$total services healthy"
    
    [ $healthy -eq $total ] && return 0 || return 1
}

# MAIN

case "${1:-start}" in
    start)
        start_all
        ;;
    stop)
        stop_all
        ;;
    restart)
        stop_all
        sleep 3
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
    *)
        echo "Usage: $0 {start|stop|restart|status|logs|health}"
        echo ""
        echo "Complete FFmpeg-RTMP Production Stack"
        echo ""
        echo "Includes:"
        echo "  • Master node & Worker agent"
        echo "  • NGINX RTMP server"
        echo "  • Grafana + VictoriaMetrics"
        echo "  • Alertmanager"
        echo "  • 12+ Prometheus exporters"
        echo ""
        echo "Commands:"
        echo "  start    - Start complete stack"
        echo "  stop     - Stop everything"
        echo "  restart  - Restart all services"
        echo "  status   - Show service status"
        echo "  logs     - View recent logs"
        echo "  health   - Run health check"
        echo ""
        exit 1
        ;;
esac
