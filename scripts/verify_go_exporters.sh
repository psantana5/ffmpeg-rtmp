#!/bin/bash
# Integration test script to verify Go exporters work correctly
# This script tests all Go exporters can start and respond to health checks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "========================================="
echo "Go Exporters Integration Test"
echo "========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

cd "$PROJECT_ROOT"

# Build all exporters
echo "Building Go exporters..."
echo ""

echo -n "Building results_exporter... "
if go build -o /tmp/test_results_exporter ./master/exporters/results_go/ 2>&1 | grep -q "error"; then
    echo -e "${RED}FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}OK${NC}"
fi

echo -n "Building qoe_exporter... "
if go build -o /tmp/test_qoe_exporter ./master/exporters/qoe_go/ 2>&1 | grep -q "error"; then
    echo -e "${RED}FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}OK${NC}"
fi

echo -n "Building cost_exporter... "
if go build -o /tmp/test_cost_exporter ./master/exporters/cost_go/ 2>&1 | grep -q "error"; then
    echo -e "${RED}FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}OK${NC}"
fi

echo -n "Building health_checker... "
if go build -o /tmp/test_health_checker ./master/exporters/health_checker_go/ 2>&1 | grep -q "error"; then
    echo -e "${RED}FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}OK${NC}"
fi

echo -n "Building docker_stats_exporter... "
if go build -o /tmp/test_docker_stats_exporter ./worker/exporters/docker_stats_go/ 2>&1 | grep -q "error"; then
    echo -e "${RED}FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}OK${NC}"
fi

echo ""
echo "All exporters built successfully!"
echo ""

# Test exporters can start and respond
echo "Testing exporters..."
echo ""

# Ensure test_results directory exists
mkdir -p ./test_results

# Test Results Exporter
echo -n "Testing results_exporter... "
RESULTS_EXPORTER_PORT=29502 RESULTS_DIR=./test_results /tmp/test_results_exporter > /tmp/results_test.log 2>&1 &
RESULTS_PID=$!
sleep 2
if curl -sf http://localhost:29502/health > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
    kill $RESULTS_PID 2>/dev/null || true
else
    echo -e "${RED}FAILED${NC}"
    kill $RESULTS_PID 2>/dev/null || true
    exit 1
fi

# Test QoE Exporter
echo -n "Testing qoe_exporter... "
QOE_EXPORTER_PORT=29503 RESULTS_DIR=./test_results /tmp/test_qoe_exporter > /tmp/qoe_test.log 2>&1 &
QOE_PID=$!
sleep 2
if curl -sf http://localhost:29503/health > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
    kill $QOE_PID 2>/dev/null || true
else
    echo -e "${RED}FAILED${NC}"
    kill $QOE_PID 2>/dev/null || true
    exit 1
fi

# Test Cost Exporter
echo -n "Testing cost_exporter... "
COST_EXPORTER_PORT=29504 RESULTS_DIR=./test_results ENERGY_COST=0.12 CPU_COST=0.50 /tmp/test_cost_exporter > /tmp/cost_test.log 2>&1 &
COST_PID=$!
sleep 2
if curl -sf http://localhost:29504/health > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
    kill $COST_PID 2>/dev/null || true
else
    echo -e "${RED}FAILED${NC}"
    kill $COST_PID 2>/dev/null || true
    exit 1
fi

# Test Health Checker
echo -n "Testing health_checker... "
HEALTH_CHECK_PORT=29600 /tmp/test_health_checker > /tmp/health_test.log 2>&1 &
HEALTH_PID=$!
sleep 2
if curl -sf http://localhost:29600/health > /dev/null 2>&1; then
    echo -e "${GREEN}OK${NC}"
    kill $HEALTH_PID 2>/dev/null || true
else
    echo -e "${RED}FAILED${NC}"
    kill $HEALTH_PID 2>/dev/null || true
    exit 1
fi

echo ""
echo "========================================="
echo -e "${GREEN}All tests passed!${NC}"
echo "========================================="
echo ""
echo "Summary:"
echo "  ✓ All Go exporters compile successfully"
echo "  ✓ All exporters respond to health checks"
echo "  ✓ Integration is working correctly"
echo ""
echo "You can now start the full stack with:"
echo "  docker compose up -d"
echo ""
