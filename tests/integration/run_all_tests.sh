#!/bin/bash
# Master Test Runner - Runs all integration tests
# Usage: ./run_all_tests.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}╔════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  FFmpeg-RTMP Integration Test Suite          ║${NC}"
echo -e "${BLUE}║  Production-Grade Scheduling & Control Tests  ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════╝${NC}"

# Configuration
export MASTER_URL="http://localhost:8080"
export MASTER_API_KEY="test-api-key-$(date +%s)"

TESTS=(
    "test_priority_scheduling.sh:Priority Scheduling System"
    "test_job_control.sh:Job Control (Pause/Resume/Cancel)"
    "test_gpu_filtering.sh:GPU Filtering & Hardware Awareness"
    "test_user_workflows.sh:Complete User Workflows"
)

PASSED=0
FAILED=0
TOTAL=${#TESTS[@]}

echo -e "\n${YELLOW}Running $TOTAL test suites...${NC}\n"

for test_entry in "${TESTS[@]}"; do
    IFS=':' read -r test_file test_name <<< "$test_entry"
    test_path="$SCRIPT_DIR/$test_file"
    
    if [ ! -f "$test_path" ]; then
        echo -e "${RED}✗ Test file not found: $test_file${NC}"
        ((FAILED++))
        continue
    fi
    
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Running: $test_name${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    if bash "$test_path"; then
        echo -e "${GREEN}✓ PASSED: $test_name${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ FAILED: $test_name${NC}"
        ((FAILED++))
    fi
    
    echo ""
    sleep 1  # Brief pause between tests
done

# Summary
echo -e "${BLUE}╔════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  TEST RESULTS SUMMARY                          ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════╝${NC}"
echo -e "\nTotal Tests: $TOTAL"
echo -e "${GREEN}Passed: $PASSED${NC}"
if [ $FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $FAILED${NC}"
fi

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ ALL TESTS PASSED!${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 0
else
    echo -e "${RED}✗ SOME TESTS FAILED${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 1
fi
