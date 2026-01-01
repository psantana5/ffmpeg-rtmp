#!/bin/bash
# Comprehensive Test Suite for Directory Migration
# Tests all components to ensure migration didn't break functionality

set -e  # Exit on error

echo "=========================================="
echo "Comprehensive Migration Test Suite"
echo "=========================================="
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASSED=0
FAILED=0
WARNINGS=0

pass_test() {
    echo -e "${GREEN}✓${NC} $1"
    PASSED=$((PASSED + 1))
}

fail_test() {
    echo -e "${RED}✗${NC} $1"
    FAILED=$((FAILED + 1))
}

warn_test() {
    echo -e "${YELLOW}⚠${NC} $1"
    WARNINGS=$((WARNINGS + 1))
}

# Test 1: Directory Structure
echo "=========================================="
echo "Test 1: Verify New Directory Structure"
echo "=========================================="

if [ -d "master/cmd/master" ]; then
    pass_test "master/cmd/master exists"
else
    fail_test "master/cmd/master missing"
fi

if [ -d "worker/cmd/agent" ]; then
    pass_test "worker/cmd/agent exists"
else
    fail_test "worker/cmd/agent missing"
fi

if [ -d "shared/pkg" ]; then
    pass_test "shared/pkg exists"
else
    fail_test "shared/pkg missing"
fi

if [ -d "master/monitoring" ]; then
    pass_test "master/monitoring exists"
else
    fail_test "master/monitoring missing"
fi

if [ -d "worker/exporters" ]; then
    pass_test "worker/exporters exists"
else
    fail_test "worker/exporters missing"
fi

# Test 2: Old directories should be removed
echo ""
echo "=========================================="
echo "Test 2: Verify Old Structure Removed"
echo "=========================================="

if [ ! -d "cmd" ]; then
    pass_test "Old cmd/ directory removed"
else
    fail_test "Old cmd/ directory still exists"
fi

if [ ! -d "pkg" ]; then
    pass_test "Old pkg/ directory removed"
else
    fail_test "Old pkg/ directory still exists"
fi

if [ ! -d "src/exporters" ]; then
    pass_test "Old src/exporters/ removed"
else
    fail_test "Old src/exporters/ still exists"
fi

# Test 3: Build System
echo ""
echo "=========================================="
echo "Test 3: Build System Tests"
echo "=========================================="

# Clean builds
rm -rf bin/
if make build-master > /tmp/build-master.log 2>&1; then
    pass_test "Master builds successfully"
    if [ -f "bin/master" ]; then
        pass_test "Master binary created at bin/master"
        SIZE=$(stat -f%z bin/master 2>/dev/null || stat -c%s bin/master 2>/dev/null)
        if [ "$SIZE" -gt 1000000 ]; then
            pass_test "Master binary size reasonable ($SIZE bytes)"
        else
            warn_test "Master binary seems small ($SIZE bytes)"
        fi
    else
        fail_test "Master binary not created"
    fi
else
    fail_test "Master build failed - see /tmp/build-master.log"
    cat /tmp/build-master.log
fi

if make build-agent > /tmp/build-agent.log 2>&1; then
    pass_test "Agent builds successfully"
    if [ -f "bin/agent" ]; then
        pass_test "Agent binary created at bin/agent"
        SIZE=$(stat -f%z bin/agent 2>/dev/null || stat -c%s bin/agent 2>/dev/null)
        if [ "$SIZE" -gt 1000000 ]; then
            pass_test "Agent binary size reasonable ($SIZE bytes)"
        else
            warn_test "Agent binary seems small ($SIZE bytes)"
        fi
    else
        fail_test "Agent binary not created"
    fi
else
    fail_test "Agent build failed - see /tmp/build-agent.log"
    cat /tmp/build-agent.log
fi

# Test 4: Binary Functionality
echo ""
echo "=========================================="
echo "Test 4: Binary Functionality Tests"
echo "=========================================="

if ./bin/master --help > /tmp/master-help.txt 2>&1; then
    pass_test "Master binary runs (--help works)"
    if grep -q "api-key" /tmp/master-help.txt; then
        pass_test "Master has expected flags"
    else
        fail_test "Master missing expected flags"
    fi
else
    fail_test "Master binary doesn't run"
fi

if ./bin/agent --help > /tmp/agent-help.txt 2>&1; then
    pass_test "Agent binary runs (--help works)"
    if grep -q "master" /tmp/agent-help.txt; then
        pass_test "Agent has expected flags"
    else
        fail_test "Agent missing expected flags"
    fi
else
    fail_test "Agent binary doesn't run"
fi

# Test 5: Go Module Resolution
echo ""
echo "=========================================="
echo "Test 5: Go Module Resolution"
echo "=========================================="

if [ -f "go.mod" ]; then
    pass_test "Root go.mod exists"
    if grep -q "replace.*shared/pkg" go.mod; then
        pass_test "go.mod has replace directive for shared/pkg"
    else
        fail_test "go.mod missing replace directive"
    fi
else
    fail_test "Root go.mod missing"
fi

if [ -f "shared/pkg/go.mod" ]; then
    pass_test "shared/pkg/go.mod exists"
else
    fail_test "shared/pkg/go.mod missing"
fi

# Test Go imports resolve
if go list ./master/cmd/master > /dev/null 2>&1; then
    pass_test "Master imports resolve correctly"
else
    fail_test "Master imports don't resolve"
fi

if go list ./worker/cmd/agent > /dev/null 2>&1; then
    pass_test "Agent imports resolve correctly"
else
    fail_test "Agent imports don't resolve"
fi

# Test 6: Docker Compose Configuration
echo ""
echo "=========================================="
echo "Test 6: Docker Compose Configuration"
echo "=========================================="

if [ -f "docker-compose.yml" ]; then
    pass_test "docker-compose.yml exists"
    
    # Check for updated paths
    if grep -q "master/exporters" docker-compose.yml; then
        pass_test "docker-compose.yml references master/exporters"
    else
        fail_test "docker-compose.yml doesn't reference master/exporters"
    fi
    
    if grep -q "worker/exporters" docker-compose.yml; then
        pass_test "docker-compose.yml references worker/exporters"
    else
        fail_test "docker-compose.yml doesn't reference worker/exporters"
    fi
    
    if grep -q "master/monitoring" docker-compose.yml; then
        pass_test "docker-compose.yml references master/monitoring"
    else
        fail_test "docker-compose.yml doesn't reference master/monitoring"
    fi
    
    if grep -q "shared/advisor" docker-compose.yml; then
        pass_test "docker-compose.yml references shared/advisor"
    else
        fail_test "docker-compose.yml doesn't reference shared/advisor"
    fi
    
    # Validate docker-compose syntax
    if docker compose config > /dev/null 2>&1; then
        pass_test "docker-compose.yml syntax valid"
    else
        warn_test "docker-compose validation failed (docker may not be available)"
    fi
else
    fail_test "docker-compose.yml missing"
fi

# Test 7: Configuration Files
echo ""
echo "=========================================="
echo "Test 7: Configuration Files"
echo "=========================================="

if [ -f "master/monitoring/victoriametrics.yml" ]; then
    pass_test "victoriametrics.yml in correct location"
else
    fail_test "victoriametrics.yml missing from master/monitoring/"
fi

if [ -f "master/monitoring/alertmanager/alertmanager.yml" ]; then
    pass_test "alertmanager.yml in correct location"
else
    fail_test "alertmanager.yml missing from master/monitoring/alertmanager/"
fi

if [ -f "master/monitoring/alert-rules.yml" ]; then
    pass_test "alert-rules.yml in correct location"
else
    fail_test "alert-rules.yml missing from master/monitoring/"
fi

# Test 8: Exporters
echo ""
echo "=========================================="
echo "Test 8: Exporter Files"
echo "=========================================="

# Master exporters
MASTER_EXPORTERS=("results" "qoe" "cost" "health_checker")
for exp in "${MASTER_EXPORTERS[@]}"; do
    if [ -d "master/exporters/$exp" ]; then
        pass_test "Master exporter $exp exists"
        if [ -f "master/exporters/$exp/Dockerfile" ]; then
            pass_test "Master exporter $exp has Dockerfile"
        else
            fail_test "Master exporter $exp missing Dockerfile"
        fi
    else
        fail_test "Master exporter $exp missing"
    fi
done

# Worker exporters
WORKER_EXPORTERS=("cpu_exporter" "gpu_exporter" "ffmpeg_exporter" "docker_stats")
for exp in "${WORKER_EXPORTERS[@]}"; do
    if [ -d "worker/exporters/$exp" ]; then
        pass_test "Worker exporter $exp exists"
        if [ -f "worker/exporters/$exp/Dockerfile" ]; then
            pass_test "Worker exporter $exp has Dockerfile"
        else
            fail_test "Worker exporter $exp missing Dockerfile"
        fi
    else
        fail_test "Worker exporter $exp missing"
    fi
done

# Test 9: Shared Components
echo ""
echo "=========================================="
echo "Test 9: Shared Components"
echo "=========================================="

# Check shared packages
SHARED_PKGS=("api" "models" "auth" "agent" "store" "tls" "metrics" "logging")
for pkg in "${SHARED_PKGS[@]}"; do
    if [ -d "shared/pkg/$pkg" ]; then
        pass_test "Shared package $pkg exists"
    else
        fail_test "Shared package $pkg missing"
    fi
done

# Check shared scripts
if [ -d "shared/scripts" ]; then
    pass_test "shared/scripts directory exists"
    SCRIPT_COUNT=$(find shared/scripts -name "*.py" -o -name "*.sh" | wc -l)
    if [ "$SCRIPT_COUNT" -gt 0 ]; then
        pass_test "Found $SCRIPT_COUNT scripts in shared/scripts"
    else
        warn_test "No scripts found in shared/scripts"
    fi
else
    fail_test "shared/scripts missing"
fi

# Check shared docs
if [ -d "shared/docs" ]; then
    pass_test "shared/docs directory exists"
    DOC_COUNT=$(find shared/docs -name "*.md" | wc -l)
    if [ "$DOC_COUNT" -gt 0 ]; then
        pass_test "Found $DOC_COUNT docs in shared/docs"
    else
        warn_test "No docs found in shared/docs"
    fi
else
    fail_test "shared/docs missing"
fi

# Test 10: Deployment Files
echo ""
echo "=========================================="
echo "Test 10: Deployment Files"
echo "=========================================="

if [ -f "master/deployment/ffmpeg-master.service" ]; then
    pass_test "Master systemd service file exists"
else
    fail_test "Master systemd service file missing"
fi

if [ -f "worker/deployment/ffmpeg-agent.service" ]; then
    pass_test "Worker systemd service file exists"
else
    fail_test "Worker systemd service file missing"
fi

# Test 11: Python Scripts Functionality
echo ""
echo "=========================================="
echo "Test 11: Python Scripts"
echo "=========================================="

if [ -f "shared/scripts/run_tests.py" ]; then
    pass_test "run_tests.py exists in shared/scripts"
    if python3 shared/scripts/run_tests.py --help > /dev/null 2>&1; then
        pass_test "run_tests.py runs successfully"
    else
        warn_test "run_tests.py may have import issues"
    fi
else
    fail_test "run_tests.py missing"
fi

if [ -f "shared/scripts/analyze_results.py" ]; then
    pass_test "analyze_results.py exists in shared/scripts"
else
    fail_test "analyze_results.py missing"
fi

# Test 12: Documentation
echo ""
echo "=========================================="
echo "Test 12: Documentation"
echo "=========================================="

README_FILES=("README.md" "master/README.md" "worker/README.md" "shared/README.md")
for readme in "${README_FILES[@]}"; do
    if [ -f "$readme" ]; then
        pass_test "$readme exists"
    else
        fail_test "$readme missing"
    fi
done

# Check documentation references
DOC_FILES=("FOLDER_ORGANIZATION.md" "ARCHITECTURE_DIAGRAM.md" "MIGRATION_GUIDE.md" "PROJECT_ORGANIZATION_SUMMARY.md")
for doc in "${DOC_FILES[@]}"; do
    if [ -f "$doc" ]; then
        pass_test "$doc exists"
    else
        fail_test "$doc missing"
    fi
done

# Test 13: Makefile Targets
echo ""
echo "=========================================="
echo "Test 13: Makefile Targets"
echo "=========================================="

if make -n build-master > /dev/null 2>&1; then
    pass_test "Makefile target 'build-master' exists"
else
    fail_test "Makefile target 'build-master' missing"
fi

if make -n build-agent > /dev/null 2>&1; then
    pass_test "Makefile target 'build-agent' exists"
else
    fail_test "Makefile target 'build-agent' missing"
fi

if make -n build-distributed > /dev/null 2>&1; then
    pass_test "Makefile target 'build-distributed' exists"
else
    fail_test "Makefile target 'build-distributed' missing"
fi

# Test 14: Grafana Dashboards
echo ""
echo "=========================================="
echo "Test 14: Grafana Dashboards"
echo "=========================================="

if [ -d "master/monitoring/grafana/provisioning/dashboards" ]; then
    pass_test "Grafana dashboards directory exists"
    DASHBOARD_COUNT=$(find master/monitoring/grafana/provisioning/dashboards -name "*.json" | wc -l)
    if [ "$DASHBOARD_COUNT" -gt 0 ]; then
        pass_test "Found $DASHBOARD_COUNT Grafana dashboards"
    else
        warn_test "No Grafana dashboards found"
    fi
else
    fail_test "Grafana dashboards directory missing"
fi

if [ -d "master/monitoring/grafana/provisioning/datasources" ]; then
    pass_test "Grafana datasources directory exists"
else
    fail_test "Grafana datasources directory missing"
fi

# Test 15: Check for Broken Symlinks
echo ""
echo "=========================================="
echo "Test 15: Check for Broken Symlinks"
echo "=========================================="

BROKEN_LINKS=$(find . -type l ! -exec test -e {} \; -print 2>/dev/null | wc -l)
if [ "$BROKEN_LINKS" -eq 0 ]; then
    pass_test "No broken symlinks found"
else
    warn_test "Found $BROKEN_LINKS broken symlinks"
fi

# Final Summary
echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""
echo -e "${GREEN}Passed:${NC}   $PASSED"
echo -e "${YELLOW}Warnings:${NC} $WARNINGS"
echo -e "${RED}Failed:${NC}   $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}=========================================="
    echo "✓ ALL TESTS PASSED!"
    echo "==========================================${NC}"
    exit 0
else
    echo -e "${RED}=========================================="
    echo "✗ SOME TESTS FAILED"
    echo "==========================================${NC}"
    exit 1
fi
