#!/bin/bash
# Deployment Scripts Test Suite
# Tests deploy.sh, install-master.sh, and install-edge.sh
# Validates syntax, structure, and critical functionality

# Don't use set -e, we handle errors explicitly
set +e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Logging
log_test() { echo -e "${CYAN}[TEST]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((TESTS_PASSED++)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; ((TESTS_FAILED++)); }
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# Test function wrapper
run_test() {
    local test_name="$1"
    shift
    ((TESTS_RUN++))
    log_test "$test_name"
    if "$@"; then
        log_pass "$test_name"
        return 0
    else
        log_fail "$test_name"
        return 1
    fi
}

# ============================================
# TEST 1: Script Existence
# ============================================
test_scripts_exist() {
    [ -f "deploy.sh" ] || { echo "deploy.sh not found"; return 1; }
    [ -f "deployment/install-edge.sh" ] || { echo "install-edge.sh not found"; return 1; }
    [ -f "master/deployment/install-master.sh" ] || { echo "install-master.sh not found"; return 1; }
    return 0
}

# ============================================
# TEST 2: Script Permissions
# ============================================
test_scripts_executable() {
    [ -x "deploy.sh" ] || { echo "deploy.sh not executable"; return 1; }
    [ -x "deployment/install-edge.sh" ] || { echo "install-edge.sh not executable"; return 1; }
    [ -x "master/deployment/install-master.sh" ] || { echo "install-master.sh not executable"; return 1; }
    return 0
}

# ============================================
# TEST 3: Bash Syntax Validation
# ============================================
test_bash_syntax() {
    local script="$1"
    bash -n "$script" 2>&1 || return 1
    return 0
}

# ============================================
# TEST 4: Shebang Validation
# ============================================
test_shebang() {
    local script="$1"
    local shebang=$(head -n1 "$script")
    [[ "$shebang" == "#!/bin/bash" ]] || { echo "Invalid shebang: $shebang"; return 1; }
    return 0
}

# ============================================
# TEST 5: Required Functions Present
# ============================================
test_required_functions() {
    local script="$1"
    shift
    local functions=("$@")
    
    for func in "${functions[@]}"; do
        if ! grep -q "^${func}()" "$script" && ! grep -q "^${func} ()" "$script"; then
            echo "Function '$func' not found in $script"
            return 1
        fi
    done
    return 0
}

# ============================================
# TEST 6: Error Handling Present
# ============================================
test_error_handling() {
    local script="$1"
    
    # Check for set -e
    if ! grep -q "set -e" "$script"; then
        echo "Missing 'set -e' in $script"
        return 1
    fi
    
    # Check for error_exit function
    if ! grep -q "error_exit" "$script"; then
        echo "Missing error_exit function in $script"
        return 1
    fi
    
    return 0
}

# ============================================
# TEST 7: Help Text Present
# ============================================
test_help_available() {
    local script="$1"
    
    # Check for --help or -h handling
    if ! grep -q "\-\-help\|\-h" "$script"; then
        echo "No help option in $script"
        return 1
    fi
    
    return 0
}

# ============================================
# TEST 8: Deploy.sh Mode Handling
# ============================================
test_deploy_modes() {
    # Check if all deployment modes are handled
    if ! grep -q "\-\-master" "deploy.sh"; then
        echo "Missing --master mode"
        return 1
    fi
    
    if ! grep -q "\-\-worker" "deploy.sh"; then
        echo "Missing --worker mode"
        return 1
    fi
    
    if ! grep -q "\-\-both" "deploy.sh"; then
        echo "Missing --both mode"
        return 1
    fi
    
    return 0
}

# ============================================
# TEST 9: Systemd Service References
# ============================================
test_systemd_services() {
    # Check master script references correct service
    if ! grep -q "ffrtmp-master.service" "master/deployment/install-master.sh"; then
        echo "Master service not referenced"
        return 1
    fi
    
    # Check edge script references correct services
    if ! grep -q "ffrtmp-worker.service" "deployment/install-edge.sh"; then
        echo "Worker service not referenced"
        return 1
    fi
    
    if ! grep -q "ffrtmp-watch.service" "deployment/install-edge.sh"; then
        echo "Watch service not referenced"
        return 1
    fi
    
    return 0
}

# ============================================
# TEST 10: Directory Creation
# ============================================
test_directory_handling() {
    local script="$1"
    
    # deploy.sh delegates directory creation to install scripts
    if [[ "$script" == "deploy.sh" ]]; then
        return 0
    fi
    
    # Check if script creates necessary directories
    if ! grep -q "mkdir -p" "$script"; then
        echo "No directory creation in $script"
        return 1
    fi
    
    return 0
}

# ============================================
# TEST 11: User Creation Safety
# ============================================
test_user_creation_safety() {
    local script="$1"
    
    # Check if script checks for existing user before creation
    if grep -q "useradd" "$script"; then
        if ! grep -B5 "useradd" "$script" | grep -q "id.*&>/dev/null"; then
            echo "User creation not checking for existing user in $script"
            return 1
        fi
    fi
    
    return 0
}

# ============================================
# TEST 12: Binary Installation Paths
# ============================================
test_binary_paths() {
    # Master should use /opt/ffrtmp-master/bin
    if ! grep -q "/opt/ffrtmp-master/bin" "master/deployment/install-master.sh"; then
        echo "Master binary path incorrect"
        return 1
    fi
    
    # Edge should use /opt/ffrtmp/bin
    if ! grep -q "/opt/ffrtmp/bin" "deployment/install-edge.sh"; then
        echo "Edge binary path incorrect"
        return 1
    fi
    
    return 0
}

# ============================================
# TEST 13: Configuration Files
# ============================================
test_config_handling() {
    # Check if systemd service files exist
    [ -f "deployment/systemd/ffrtmp-worker.service" ] || { echo "Worker service file missing"; return 1; }
    [ -f "deployment/systemd/ffrtmp-watch.service" ] || { echo "Watch service file missing"; return 1; }
    
    # Check if config templates exist
    [ -f "deployment/config/watch-config.production.yaml" ] || { echo "Watch config template missing"; return 1; }
    
    return 0
}

# ============================================
# TEST 14: Root Check Present
# ============================================
test_root_check() {
    local script="$1"
    
    # Installation scripts should check for root
    if [[ "$script" == *"install"* ]]; then
        if ! grep -q "EUID.*-ne 0" "$script"; then
            echo "No root check in $script"
            return 1
        fi
    fi
    
    return 0
}

# ============================================
# TEST 15: Build Capability
# ============================================
test_build_capability() {
    local script="$1"
    
    # Check if script can build binaries if they don't exist
    if ! grep -q "make build" "$script" && ! grep -q "go build" "$script"; then
        echo "No build capability in $script"
        log_warn "Script may require pre-built binaries"
        return 0  # Warning, not failure
    fi
    
    return 0
}

# ============================================
# TEST 16: Deploy.sh Script Integration
# ============================================
test_deploy_integration() {
    # Check if deploy.sh calls the underlying scripts
    if ! grep -q "install-master.sh" "deploy.sh"; then
        echo "deploy.sh doesn't call install-master.sh"
        return 1
    fi
    
    if ! grep -q "install-edge.sh" "deploy.sh"; then
        echo "deploy.sh doesn't call install-edge.sh"
        return 1
    fi
    
    return 0
}

# ============================================
# TEST 17: Interactive Mode Support
# ============================================
test_interactive_support() {
    # Check if deploy.sh supports interactive mode
    if ! grep -q "NON_INTERACTIVE" "deploy.sh"; then
        echo "No interactive mode support"
        return 1
    fi
    
    return 0
}

# ============================================
# TEST 18: Validation Steps
# ============================================
test_validation_present() {
    local script="$1"
    
    # Check if script has validation steps
    if ! grep -q "validation\|validate\|verify" "$script" -i; then
        echo "No validation in $script"
        log_warn "Script may lack post-install validation"
        return 0  # Warning, not failure
    fi
    
    return 0
}

# ============================================
# MAIN TEST EXECUTION
# ============================================
echo ""
echo "=========================================="
echo "  Deployment Scripts Test Suite"
echo "=========================================="
echo ""

# Check if running from project root
if [ ! -f "deploy.sh" ]; then
    echo "Error: Must run from project root directory"
    exit 1
fi

log_info "Starting test suite..."
echo ""

# Test all scripts
run_test "Scripts exist" test_scripts_exist
run_test "Scripts executable" test_scripts_executable

# Test each script individually
for script in "deploy.sh" "deployment/install-edge.sh" "master/deployment/install-master.sh"; do
    echo ""
    log_info "Testing: $script"
    run_test "  Bash syntax valid" test_bash_syntax "$script"
    run_test "  Shebang correct" test_shebang "$script"
    run_test "  Error handling present" test_error_handling "$script"
    run_test "  Directory handling" test_directory_handling "$script"
    run_test "  User creation safe" test_user_creation_safety "$script"
    run_test "  Build capability" test_build_capability "$script"
    run_test "  Validation present" test_validation_present "$script"
    
    if [[ "$script" == *"install"* ]]; then
        run_test "  Root check present" test_root_check "$script"
    fi
done

# Integration tests
echo ""
log_info "Running integration tests..."
run_test "Deploy.sh mode handling" test_deploy_modes
run_test "Deploy.sh integration" test_deploy_integration
run_test "Interactive mode support" test_interactive_support
run_test "Systemd service references" test_systemd_services
run_test "Binary path correctness" test_binary_paths
run_test "Configuration files exist" test_config_handling

# ============================================
# SUMMARY
# ============================================
echo ""
echo "=========================================="
echo "  Test Summary"
echo "=========================================="
echo ""
echo "Total tests run:    $TESTS_RUN"
echo -e "Tests passed:       ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests failed:       ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    log_info "Scripts are ready for deployment"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    echo ""
    log_warn "Please fix the issues before deploying"
    exit 1
fi
