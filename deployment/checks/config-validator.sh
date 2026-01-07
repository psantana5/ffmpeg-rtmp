#!/bin/bash
# Configuration Validator for FFmpeg-RTMP
# Validates configuration files before deployment
# Usage: ./config-validator.sh <config-file>

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[⚠]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }

CONFIG_FILE=""
ERRORS=0
WARNINGS=0

# Parse arguments
if [ $# -lt 1 ]; then
    log_error "Usage: $0 <config-file>"
    exit 1
fi

CONFIG_FILE="$1"

if [ ! -f "$CONFIG_FILE" ]; then
    log_error "Config file not found: $CONFIG_FILE"
    exit 1
fi

log_info "Validating configuration: $CONFIG_FILE"
echo ""

# Detect config type
if basename "$CONFIG_FILE" | grep -q "master"; then
    CONFIG_TYPE="master"
elif basename "$CONFIG_FILE" | grep -q "worker"; then
    CONFIG_TYPE="worker"
elif basename "$CONFIG_FILE" | grep -q "watch"; then
    CONFIG_TYPE="watch"
else
    log_warn "Cannot determine config type from filename"
    CONFIG_TYPE="unknown"
fi

# Check file syntax based on extension
EXT="${CONFIG_FILE##*.}"
case $EXT in
    yaml|yml)
        log_info "Checking YAML syntax..."
        if command -v python3 >/dev/null 2>&1; then
            if python3 -c "import yaml; yaml.safe_load(open('$CONFIG_FILE'))" 2>/dev/null; then
                log_success "YAML syntax is valid"
            else
                log_error "YAML syntax error"
                python3 -c "import yaml; yaml.safe_load(open('$CONFIG_FILE'))" 2>&1
                ((ERRORS++))
            fi
        else
            log_warn "Python3 not available, skipping YAML validation"
            ((WARNINGS++))
        fi
        ;;
    env)
        log_info "Checking ENV file syntax..."
        if grep -q "^[^#=]*=[^=]*$" "$CONFIG_FILE"; then
            log_success "ENV file syntax appears valid"
        else
            log_warn "ENV file may have syntax issues"
            ((WARNINGS++))
        fi
        ;;
    *)
        log_warn "Unknown config file format: $EXT"
        ((WARNINGS++))
        ;;
esac

# Master config validation
if [ "$CONFIG_TYPE" = "master" ]; then
    log_info "Validating master configuration..."
    
    # Check required fields
    if grep -q "^server:" "$CONFIG_FILE" 2>/dev/null; then
        log_success "Server section found"
        
        # Check port
        if grep -A5 "^server:" "$CONFIG_FILE" | grep -q "port:"; then
            PORT=$(grep -A5 "^server:" "$CONFIG_FILE" | grep "port:" | awk '{print $2}')
            if [ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ]; then
                log_success "Valid port: $PORT"
            else
                log_error "Invalid port: $PORT"
                ((ERRORS++))
            fi
        else
            log_warn "No port specified (will use default)"
            ((WARNINGS++))
        fi
    else
        log_warn "No server section found"
        ((WARNINGS++))
    fi
    
    # Check database config
    if grep -q "^database:" "$CONFIG_FILE" 2>/dev/null; then
        log_success "Database section found"
        
        DB_TYPE=$(grep -A3 "^database:" "$CONFIG_FILE" | grep "type:" | awk '{print $2}')
        case $DB_TYPE in
            sqlite|postgres)
                log_success "Valid database type: $DB_TYPE"
                ;;
            *)
                log_warn "Unknown database type: $DB_TYPE"
                ((WARNINGS++))
                ;;
        esac
    else
        log_warn "No database section (will use default)"
        ((WARNINGS++))
    fi
fi

# Worker config validation
if [ "$CONFIG_TYPE" = "worker" ]; then
    log_info "Validating worker configuration..."
    
    # Check for required environment variables
    REQUIRED_VARS=(MASTER_URL API_KEY)
    for var in "${REQUIRED_VARS[@]}"; do
        if grep -q "^${var}=" "$CONFIG_FILE" 2>/dev/null; then
            VALUE=$(grep "^${var}=" "$CONFIG_FILE" | cut -d= -f2- | tr -d '"' | tr -d "'")
            if [ -n "$VALUE" ]; then
                log_success "$var is set"
            else
                log_error "$var is empty"
                ((ERRORS++))
            fi
        else
            log_error "$var is missing"
            ((ERRORS++))
        fi
    done
    
    # Check optional variables
    OPTIONAL_VARS=(WORKER_ID MAX_CONCURRENT_JOBS HEARTBEAT_INTERVAL)
    for var in "${OPTIONAL_VARS[@]}"; do
        if grep -q "^${var}=" "$CONFIG_FILE" 2>/dev/null; then
            log_success "$var is configured"
        else
            log_info "$var not set (will use default)"
        fi
    done
fi

# Watch config validation  
if [ "$CONFIG_TYPE" = "watch" ]; then
    log_info "Validating watch configuration..."
    
    if grep -q "^watch:" "$CONFIG_FILE" 2>/dev/null; then
        log_success "Watch section found"
        
        # Check watch directory
        if grep -A10 "^watch:" "$CONFIG_FILE" | grep -q "directory:"; then
            WATCH_DIR=$(grep -A10 "^watch:" "$CONFIG_FILE" | grep "directory:" | awk '{print $2}' | tr -d '"')
            if [ -d "$WATCH_DIR" ]; then
                log_success "Watch directory exists: $WATCH_DIR"
            else
                log_warn "Watch directory does not exist: $WATCH_DIR"
                ((WARNINGS++))
            fi
        else
            log_error "No watch directory specified"
            ((ERRORS++))
        fi
    else
        log_error "No watch section found"
        ((ERRORS++))
    fi
fi

# Check for sensitive data
log_info "Checking for sensitive data..."
SENSITIVE_PATTERNS=("password:" "secret:" "key:" "token:")
for pattern in "${SENSITIVE_PATTERNS[@]}"; do
    if grep -i "$pattern" "$CONFIG_FILE" | grep -qv "CHANGE_ME\|example\|your-"; then
        VALUE=$(grep -i "$pattern" "$CONFIG_FILE" | head -1 | cut -d: -f2 | tr -d ' "' | cut -c1-10)
        if [ ${#VALUE} -gt 5 ]; then
            log_success "Sensitive field '$pattern' appears to be configured"
        else
            log_warn "Sensitive field '$pattern' may need configuration"
            ((WARNINGS++))
        fi
    fi
done

# Check file permissions
log_info "Checking file permissions..."
PERMS=$(stat -c %a "$CONFIG_FILE")
if [ "$PERMS" = "600" ] || [ "$PERMS" = "640" ] || [ "$PERMS" = "644" ]; then
    log_success "File permissions are secure: $PERMS"
else
    log_warn "File permissions may be too permissive: $PERMS (recommend 600 or 640)"
    ((WARNINGS++))
fi

# Summary
echo ""
echo "═══════════════════════════════════════"
echo "  Validation Summary"
echo "═══════════════════════════════════════"
echo -e "${GREEN}Config Type:${NC} $CONFIG_TYPE"
echo -e "${RED}Errors:${NC}      $ERRORS"
echo -e "${YELLOW}Warnings:${NC}    $WARNINGS"
echo ""

if [ $ERRORS -gt 0 ]; then
    log_error "Configuration validation failed!"
    exit 1
elif [ $WARNINGS -gt 0 ]; then
    log_warn "Configuration validation passed with warnings"
    exit 0
else
    log_success "Configuration validation passed!"
    exit 0
fi
