#!/bin/bash
set -e

echo "================================================================"
echo "  Streaming Energy Monitoring Stack - Setup"
echo "================================================================"
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check prerequisites
echo "Checking prerequisites..."

if ! command -v docker &> /dev/null; then
    echo -e "${RED}ERROR: Docker is not installed${NC}"
    exit 1
fi
echo -e "${GREEN}✓${NC} Docker found"

if ! docker compose version &> /dev/null; then
    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}ERROR: Docker Compose is not installed${NC}"
        exit 1
    fi
    COMPOSE_CMD="docker-compose"
else
    COMPOSE_CMD="docker compose"
fi
echo -e "${GREEN}✓${NC} Docker Compose found"

if ! command -v ffmpeg &> /dev/null; then
    echo -e "${YELLOW}WARNING: ffmpeg not found - install it to run streaming tests${NC}"
    echo "  Ubuntu/Debian: sudo apt-get install ffmpeg"
    echo "  macOS: brew install ffmpeg"
else
    echo -e "${GREEN}✓${NC} FFmpeg found"
fi

if ! command -v python3 &> /dev/null; then
    echo -e "${RED}ERROR: Python 3 is not installed${NC}"
    exit 1
fi
echo -e "${GREEN}✓${NC} Python 3 found"

# Check RAPL access
if [ ! -r /sys/class/powercap/intel-rapl:0/energy_uj ] 2>/dev/null; then
    echo -e "${YELLOW}WARNING: Cannot read RAPL interface${NC}"
    echo "  You may need to run this script with sudo for power monitoring"
    echo "  Or run: sudo chmod -R a+r /sys/class/powercap/"
fi

echo ""
echo "Creating directory structure..."

# Create directories
mkdir -p rapl-exporter
mkdir -p docker-stats-exporter
mkdir -p grafana/provisioning/datasources
mkdir -p grafana/provisioning/dashboards
mkdir -p streams
mkdir -p test_results

echo -e "${GREEN}✓${NC} Directories created"

# Create Grafana datasource
echo "Configuring Grafana..."

cat > grafana/provisioning/datasources/prometheus.yml << 'EOF'
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
    jsonData:
      timeInterval: 5s
EOF

# Create dashboard provisioning config
cat > grafana/provisioning/dashboards/default.yml << 'EOF'
apiVersion: 1

providers:
  - name: 'Default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
EOF

echo -e "${GREEN}✓${NC} Grafana configured"

# Copy exporter files
echo "Setting up exporters..."

if [ -f "rapl_exporter.py" ]; then
    cp rapl_exporter.py rapl-exporter/
    echo -e "${GREEN}✓${NC} RAPL exporter configured"
else
    echo -e "${YELLOW}WARNING: rapl_exporter.py not found${NC}"
fi

if [ -f "docker_stats_exporter.py" ]; then
    cp docker_stats_exporter.py docker-stats-exporter/
    echo -e "${GREEN}✓${NC} Docker stats exporter configured"
else
    echo -e "${YELLOW}WARNING: docker_stats_exporter.py not found${NC}"
fi

# Make scripts executable
chmod +x *.py 2>/dev/null || true
chmod +x *.sh 2>/dev/null || true

echo ""
echo "Building Docker images..."
$COMPOSE_CMD build

echo ""
echo "Starting services..."
$COMPOSE_CMD up -d

echo ""
echo "Waiting for services to start..."
sleep 10

# Check service health
echo ""
echo "Checking service health..."

check_service() {
    local name=$1
    local url=$2
    
    if curl -sf "$url" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} $name is healthy"
        return 0
    else
        echo -e "${RED}✗${NC} $name is not responding"
        return 1
    fi
}

all_healthy=true
check_service "Nginx RTMP" "http://localhost:8080/health" || all_healthy=false
check_service "Prometheus" "http://localhost:9090/-/healthy" || all_healthy=false
check_service "Grafana" "http://localhost:3000/api/health" || all_healthy=false
check_service "RAPL Exporter" "http://localhost:9500/health" || all_healthy=false
check_service "Docker Stats" "http://localhost:9501/health" || all_healthy=false

echo ""
if [ "$all_healthy" = true ]; then
    echo -e "${GREEN}================================================================"
    echo "  Setup Complete! All services are running"
    echo "================================================================${NC}"
else
    echo -e "${YELLOW}================================================================"
    echo "  Setup Complete with warnings"
    echo "================================================================${NC}"
    echo ""
    echo "Some services are not responding. Check logs with:"
    echo "  $COMPOSE_CMD logs [service_name]"
fi

echo ""
echo "Service URLs:"
echo "  Nginx RTMP:      rtmp://localhost:1935/live"
echo "  Nginx Stats:     http://localhost:8080/stat"
echo "  Prometheus:      http://localhost:9090"
echo "  Grafana:         http://localhost:3000 (admin/admin)"
echo "  RAPL Metrics:    http://localhost:9500/metrics"
echo "  Docker Metrics:  http://localhost:9501/metrics"
echo ""
echo "Quick Start:"
echo "  1. Run automated tests:"
echo "     python3 scripts/run_tests.py"
echo ""
echo "  2. Analyze results:"
echo "     python3 scripts/analyze_results.py"
echo ""
echo "  3. Manual streaming test:"
echo "     ./test_stream.sh -b 2500k"
echo ""
echo "Useful Commands:"
echo "  View logs:    $COMPOSE_CMD logs -f [service]"
echo "  Stop stack:   $COMPOSE_CMD down"
echo "  Restart:      $COMPOSE_CMD restart"
echo ""
