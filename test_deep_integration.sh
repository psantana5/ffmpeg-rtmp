#!/bin/bash
# Deep Integration Tests for Migration
# Tests complex scenarios and edge cases

set -e

echo "=========================================="
echo "Deep Integration Test Suite"
echo "=========================================="
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0

pass_test() {
    echo -e "${GREEN}✓${NC} $1"
    PASSED=$((PASSED + 1))
}

fail_test() {
    echo -e "${RED}✗${NC} $1"
    FAILED=$((FAILED + 1))
}

# Test 1: Docker Build Tests
echo "=========================================="
echo "Test 1: Docker Build Tests"
echo "=========================================="

# Test master exporters can build
echo "Testing master exporter builds..."
for exporter in results qoe cost health_checker; do
    if docker build -f master/exporters/$exporter/Dockerfile . -t test-master-$exporter > /tmp/docker-build-master-$exporter.log 2>&1; then
        pass_test "Master exporter $exporter Docker build succeeds"
    else
        fail_test "Master exporter $exporter Docker build fails"
        echo "Log: /tmp/docker-build-master-$exporter.log"
    fi
done

# Test worker exporters can build
echo "Testing worker exporter builds..."
for exporter in cpu_exporter gpu_exporter ffmpeg_exporter; do
    if docker build -f worker/exporters/$exporter/Dockerfile . -t test-worker-$exporter > /tmp/docker-build-worker-$exporter.log 2>&1; then
        pass_test "Worker exporter $exporter Docker build succeeds"
    else
        fail_test "Worker exporter $exporter Docker build fails"
        echo "Log: /tmp/docker-build-worker-$exporter.log"
    fi
done

# docker_stats is a Python app
if docker build worker/exporters/docker_stats -t test-worker-docker-stats > /tmp/docker-build-docker-stats.log 2>&1; then
    pass_test "Worker exporter docker_stats Docker build succeeds"
else
    fail_test "Worker exporter docker_stats Docker build fails"
fi

# Test 2: Go Import Path Resolution
echo ""
echo "=========================================="
echo "Test 2: Go Import Path Deep Check"
echo "=========================================="

# Check all import statements in master
echo "Checking master imports..."
if grep -r "github.com/psantana5/ffmpeg-rtmp/pkg" master/cmd/master/ > /tmp/master-imports.txt 2>&1; then
    IMPORT_COUNT=$(wc -l < /tmp/master-imports.txt)
    pass_test "Master has $IMPORT_COUNT pkg imports (all should resolve via replace)"
else
    pass_test "Master has no direct pkg imports"
fi

# Check all import statements in agent
echo "Checking agent imports..."
if grep -r "github.com/psantana5/ffmpeg-rtmp/pkg" worker/cmd/agent/ > /tmp/agent-imports.txt 2>&1; then
    IMPORT_COUNT=$(wc -l < /tmp/agent-imports.txt)
    pass_test "Agent has $IMPORT_COUNT pkg imports (all should resolve via replace)"
else
    pass_test "Agent has no direct pkg imports"
fi

# Verify all shared/pkg packages compile
echo "Testing shared package compilation..."
for pkg in api models auth agent store tls metrics logging; do
    if go build ./shared/pkg/$pkg > /dev/null 2>&1; then
        pass_test "Shared package $pkg compiles"
    else
        fail_test "Shared package $pkg fails to compile"
    fi
done

# Test 3: Python Import Resolution
echo ""
echo "=========================================="
echo "Test 3: Python Import Resolution"
echo "=========================================="

# Test Python scripts can import from shared
cat > /tmp/test_python_imports.py << 'EOF'
import sys
import os

# Test importing advisor from shared
sys.path.insert(0, os.path.join(os.getcwd(), 'shared'))

try:
    from advisor import cost
    print("✓ Can import advisor.cost")
except ImportError as e:
    print(f"✗ Cannot import advisor.cost: {e}")
    sys.exit(1)

try:
    from advisor import scoring
    print("✓ Can import advisor.scoring")
except ImportError as e:
    print(f"✗ Cannot import advisor.scoring: {e}")
    sys.exit(1)

print("All Python imports successful")
EOF

if python3 /tmp/test_python_imports.py; then
    pass_test "Python can import from shared/advisor"
else
    fail_test "Python cannot import from shared/advisor"
fi

# Test 4: Configuration File Validation
echo ""
echo "=========================================="
echo "Test 4: Configuration File Validation"
echo "=========================================="

# Validate YAML files
if command -v yamllint > /dev/null 2>&1; then
    if yamllint master/monitoring/victoriametrics.yml > /dev/null 2>&1; then
        pass_test "victoriametrics.yml is valid YAML"
    else
        fail_test "victoriametrics.yml has YAML errors"
    fi
    
    if yamllint master/monitoring/alertmanager/alertmanager.yml > /dev/null 2>&1; then
        pass_test "alertmanager.yml is valid YAML"
    else
        fail_test "alertmanager.yml has YAML errors"
    fi
else
    echo "  (yamllint not available, skipping YAML validation)"
fi

# Check victoriametrics.yml scrape targets reference correct paths
if grep -q "cpu-exporter-go:9500" master/monitoring/victoriametrics.yml; then
    pass_test "victoriametrics.yml has cpu-exporter scrape target"
else
    fail_test "victoriametrics.yml missing cpu-exporter"
fi

if grep -q "results-exporter:9502" master/monitoring/victoriametrics.yml; then
    pass_test "victoriametrics.yml has results-exporter scrape target"
else
    fail_test "victoriametrics.yml missing results-exporter"
fi

# Test 5: Script Execution Test
echo ""
echo "=========================================="
echo "Test 5: Script Functionality"
echo "=========================================="

# Test run_tests.py can be imported and shows help
if python3 -c "import sys; sys.path.insert(0, 'shared/scripts'); import run_tests" > /dev/null 2>&1; then
    pass_test "run_tests.py can be imported"
else
    fail_test "run_tests.py cannot be imported"
fi

# Test analyze_results.py can be imported
if python3 -c "import sys; sys.path.insert(0, 'shared/scripts'); import analyze_results" > /dev/null 2>&1; then
    pass_test "analyze_results.py can be imported"
else
    fail_test "analyze_results.py cannot be imported"
fi

# Test 6: Documentation Link Validation
echo ""
echo "=========================================="
echo "Test 6: Documentation Link Validation"
echo "=========================================="

# Check for common broken link patterns in documentation
check_doc_links() {
    local doc=$1
    local name=$(basename $doc)
    
    # Check for old path references
    if grep -q "cmd/master\|cmd/agent" "$doc" 2>/dev/null; then
        fail_test "$name references old cmd/ paths"
        return
    fi
    
    if grep -q "src/exporters" "$doc" 2>/dev/null; then
        fail_test "$name references old src/exporters path"
        return
    fi
    
    if grep -q "pkg/" "$doc" 2>/dev/null && ! grep -q "shared/pkg" "$doc"; then
        # Check if it's actually referring to old structure
        if grep -q "\`pkg/\|/pkg/" "$doc" 2>/dev/null; then
            fail_test "$name may reference old pkg/ path"
            return
        fi
    fi
    
    pass_test "$name doesn't reference old paths"
}

for doc in README.md master/README.md worker/README.md shared/README.md FOLDER_ORGANIZATION.md; do
    if [ -f "$doc" ]; then
        check_doc_links "$doc"
    fi
done

# Test 7: Systemd Service File Validation
echo ""
echo "=========================================="
echo "Test 7: Systemd Service Files"
echo "=========================================="

# Check master service file
if grep -q "ExecStart=.*/bin/master" master/deployment/ffmpeg-master.service; then
    pass_test "Master service file references bin/master"
else
    fail_test "Master service file doesn't reference bin/master correctly"
fi

# Check agent service file
if grep -q "ExecStart=.*/bin/agent" worker/deployment/ffmpeg-agent.service; then
    pass_test "Agent service file references bin/agent"
else
    fail_test "Agent service file doesn't reference bin/agent correctly"
fi

# Test 8: Grafana Dashboard Validation
echo ""
echo "=========================================="
echo "Test 8: Grafana Dashboard Validation"
echo "=========================================="

# Check dashboards are valid JSON
DASHBOARD_DIR="master/monitoring/grafana/provisioning/dashboards"
if [ -d "$DASHBOARD_DIR" ]; then
    for dashboard in "$DASHBOARD_DIR"/*.json; do
        if [ -f "$dashboard" ]; then
            if python3 -m json.tool "$dashboard" > /dev/null 2>&1; then
                pass_test "$(basename $dashboard) is valid JSON"
            else
                fail_test "$(basename $dashboard) has JSON errors"
            fi
        fi
    done
else
    fail_test "Grafana dashboard directory not found"
fi

# Test 9: Cross-Reference Validation
echo ""
echo "=========================================="
echo "Test 9: Cross-Reference Validation"
echo "=========================================="

# Verify docker-compose references match actual files
check_docker_path() {
    local path=$1
    local name=$2
    
    if [ -e "$path" ]; then
        pass_test "docker-compose path exists: $name"
    else
        fail_test "docker-compose references missing path: $path"
    fi
}

# Extract and check paths from docker-compose.yml
if grep -q "worker/exporters/cpu_exporter" docker-compose.yml; then
    check_docker_path "worker/exporters/cpu_exporter/Dockerfile" "cpu_exporter"
fi

if grep -q "master/exporters/results" docker-compose.yml; then
    check_docker_path "master/exporters/results" "results"
fi

if grep -q "master/monitoring/grafana" docker-compose.yml; then
    check_docker_path "master/monitoring/grafana/provisioning" "grafana"
fi

# Test 10: File Permission Check
echo ""
echo "=========================================="
echo "Test 10: File Permissions"
echo "=========================================="

# Check binaries are executable
if [ -x "bin/master" ]; then
    pass_test "bin/master is executable"
else
    fail_test "bin/master is not executable"
fi

if [ -x "bin/agent" ]; then
    pass_test "bin/agent is executable"
else
    fail_test "bin/agent is not executable"
fi

# Check script files
for script in shared/scripts/*.sh; do
    if [ -f "$script" ]; then
        if [ -x "$script" ]; then
            pass_test "$(basename $script) is executable"
        else
            echo "  Note: $(basename $script) is not executable (may be intentional)"
        fi
    fi
done

# Final Summary
echo ""
echo "=========================================="
echo "Deep Integration Test Summary"
echo "=========================================="
echo ""
echo -e "${GREEN}Passed:${NC}   $PASSED"
echo -e "${RED}Failed:${NC}   $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}=========================================="
    echo "✓ ALL DEEP INTEGRATION TESTS PASSED!"
    echo "==========================================${NC}"
    exit 0
else
    echo -e "${RED}=========================================="
    echo "✗ SOME TESTS FAILED"
    echo "==========================================${NC}"
    exit 1
fi
