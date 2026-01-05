#!/bin/bash
#
# Grafana Dashboard Automated Deployment Script
# Deploys production monitoring dashboards via Grafana provisioning
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Grafana Dashboard Automated Deployment"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# Check if running in docker-compose environment
check_grafana_container() {
    if docker ps --format '{{.Names}}' | grep -q '^grafana$'; then
        echo -e "${GREEN}✓${NC} Grafana container is running"
        return 0
    else
        echo -e "${RED}✗${NC} Grafana container not running"
        echo "  Start with: docker-compose up -d grafana"
        return 1
    fi
}

# Verify provisioning directory is mounted
check_provisioning_mount() {
    local mount_check=$(docker inspect grafana --format '{{range .Mounts}}{{if eq .Destination "/etc/grafana/provisioning"}}{{.Source}}{{end}}{{end}}')
    
    if [ -n "$mount_check" ]; then
        echo -e "${GREEN}✓${NC} Provisioning directory mounted: $mount_check"
        return 0
    else
        echo -e "${RED}✗${NC} Provisioning directory not mounted"
        echo "  Check docker-compose.yml volumes configuration"
        return 1
    fi
}

# List available dashboards
list_dashboards() {
    local dashboard_dir="$PROJECT_ROOT/master/monitoring/grafana/provisioning/dashboards"
    echo
    echo "Available dashboards:"
    echo
    ls -1 "$dashboard_dir"/*.json 2>/dev/null | while read -r file; do
        local name=$(basename "$file" .json)
        local size=$(du -h "$file" | cut -f1)
        echo "  • $name ($size)"
    done
}

# Restart Grafana to reload dashboards
reload_grafana() {
    echo
    echo "Reloading Grafana to apply changes..."
    docker restart grafana > /dev/null
    
    # Wait for Grafana to be ready
    echo -n "Waiting for Grafana to start"
    for i in {1..30}; do
        if curl -s http://localhost:3000/api/health > /dev/null 2>&1; then
            echo " ${GREEN}✓${NC}"
            return 0
        fi
        echo -n "."
        sleep 1
    done
    
    echo " ${RED}✗${NC}"
    echo "Grafana did not respond in time"
    return 1
}

# Verify dashboard loaded via API
verify_dashboard() {
    local dashboard_uid="$1"
    local response=$(curl -s -u admin:admin "http://localhost:3000/api/dashboards/uid/$dashboard_uid")
    
    if echo "$response" | grep -q '"dashboard"'; then
        echo -e "${GREEN}✓${NC} Dashboard verified: $dashboard_uid"
        return 0
    else
        echo -e "${YELLOW}⚠${NC} Dashboard not found yet: $dashboard_uid"
        return 1
    fi
}

# Main deployment
main() {
    echo "Step 1: Checking prerequisites..."
    echo
    
    if ! check_grafana_container; then
        exit 1
    fi
    
    if ! check_provisioning_mount; then
        exit 1
    fi
    
    echo
    echo "Step 2: Dashboard inventory"
    list_dashboards
    
    echo
    echo "Step 3: Validating dashboard JSON..."
    
    local dashboard_dir="$PROJECT_ROOT/master/monitoring/grafana/provisioning/dashboards"
    local valid_count=0
    local total_count=0
    
    for file in "$dashboard_dir"/*.json; do
        [ -f "$file" ] || continue
        total_count=$((total_count + 1))
        
        local name=$(basename "$file")
        if python3 -m json.tool "$file" > /dev/null 2>&1; then
            echo -e "  ${GREEN}✓${NC} $name"
            valid_count=$((valid_count + 1))
        else
            echo -e "  ${RED}✗${NC} $name (invalid JSON)"
        fi
    done
    
    echo
    echo "  Valid: $valid_count/$total_count dashboards"
    
    if [ $valid_count -eq 0 ]; then
        echo -e "${RED}No valid dashboards found${NC}"
        exit 1
    fi
    
    echo
    echo "Step 4: Reloading Grafana..."
    
    if ! reload_grafana; then
        exit 1
    fi
    
    echo
    echo "Step 5: Verifying dashboards loaded..."
    echo
    
    sleep 3  # Give Grafana time to load dashboards
    
    # Verify production monitoring dashboard
    verify_dashboard "ffmpeg-rtmp-prod"
    
    echo
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${GREEN}Deployment Complete!${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo
    echo "Access Grafana:"
    echo "  URL: http://localhost:3000"
    echo "  User: admin"
    echo "  Pass: admin"
    echo
    echo "Production Dashboard:"
    echo "  http://localhost:3000/d/ffmpeg-rtmp-prod/ffmpeg-rtmp-production-monitoring"
    echo
}

# Run deployment
main "$@"
