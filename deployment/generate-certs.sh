#!/bin/bash
# TLS Certificate Generation Script
# Generates self-signed certificates for FFmpeg-RTMP deployment
# Supports: Master server, Worker/Agent nodes, CA certificate
# Usage: ./deployment/generate-certs.sh [--master|--worker|--both] [OPTIONS]

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Logging
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[⚠]${NC} $1"; }
log_error() { echo -e "${RED}[✗]${NC} $1"; }

error_exit() {
    log_error "$1"
    exit 1
}

# Default configuration
MODE=""  # master, worker, both, ca
OUTPUT_DIR="certs"
MASTER_CN="ffrtmp-master"
WORKER_CN="ffrtmp-worker"
CA_CN="ffrtmp-ca"
DAYS=365
MASTER_IPS=()
MASTER_HOSTS=()
WORKER_IPS=()
WORKER_HOSTS=()
GENERATE_CA=false
USE_SYSTEM_CA=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --master)
            MODE="master"
            shift
            ;;
        --worker|--agent)
            MODE="worker"
            shift
            ;;
        --both)
            MODE="both"
            shift
            ;;
        --ca)
            GENERATE_CA=true
            shift
            ;;
        --output|-o)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --master-cn)
            MASTER_CN="$2"
            shift 2
            ;;
        --worker-cn)
            WORKER_CN="$2"
            shift 2
            ;;
        --master-ip)
            MASTER_IPS+=("$2")
            shift 2
            ;;
        --master-host)
            MASTER_HOSTS+=("$2")
            shift 2
            ;;
        --worker-ip)
            WORKER_IPS+=("$2")
            shift 2
            ;;
        --worker-host)
            WORKER_HOSTS+=("$2")
            shift 2
            ;;
        --days)
            DAYS="$2"
            shift 2
            ;;
        --help|-h)
            cat << 'EOF'
TLS Certificate Generation Script

Usage: ./deployment/generate-certs.sh [MODE] [OPTIONS]

Modes:
  --master              Generate master server certificates
  --worker, --agent     Generate worker/agent certificates
  --both                Generate both master and worker certificates
  --ca                  Generate CA certificate (for mTLS)

Options:
  --output DIR          Output directory for certificates (default: certs)
  --master-cn NAME      Master common name (default: ffrtmp-master)
  --worker-cn NAME      Worker common name (default: ffrtmp-worker)
  --master-ip IP        Master IP address (can specify multiple)
  --master-host HOST    Master hostname (can specify multiple)
  --worker-ip IP        Worker IP address (can specify multiple)
  --worker-host HOST    Worker hostname (can specify multiple)
  --days DAYS           Certificate validity in days (default: 365)
  --help, -h            Show this help

Examples:
  # Generate master certificates with custom IPs
  ./deployment/generate-certs.sh --master \
    --master-ip 10.0.0.1 \
    --master-host master.example.com

  # Generate worker certificates
  ./deployment/generate-certs.sh --worker \
    --worker-ip 10.0.0.10 \
    --worker-host worker01.example.com

  # Generate both with CA (for mTLS)
  ./deployment/generate-certs.sh --both --ca

  # Custom output directory
  ./deployment/generate-certs.sh --master --output /etc/ffrtmp-master/certs

Generated files:
  Master:  master.crt, master.key
  Worker:  agent.crt, agent.key
  CA:      ca.crt, ca.key

EOF
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Validate
if [ -z "$MODE" ] && [ "$GENERATE_CA" = false ]; then
    log_error "Mode required: --master, --worker, --both, or --ca"
    exit 1
fi

# Check OpenSSL
if ! command -v openssl &> /dev/null; then
    error_exit "OpenSSL not found. Please install OpenSSL first."
fi

# Create output directory
mkdir -p "$OUTPUT_DIR"
log_info "Output directory: $OUTPUT_DIR"

# ============================================
# CA Certificate Generation
# ============================================
generate_ca() {
    log_info "Generating CA certificate..."
    
    local ca_cert="$OUTPUT_DIR/ca.crt"
    local ca_key="$OUTPUT_DIR/ca.key"
    
    if [ -f "$ca_cert" ] && [ -f "$ca_key" ]; then
        log_warn "CA certificate already exists, skipping"
        return 0
    fi
    
    # Generate CA private key
    openssl genrsa -out "$ca_key" 4096 2>/dev/null || error_exit "Failed to generate CA key"
    chmod 600 "$ca_key"
    
    # Generate CA certificate
    openssl req -new -x509 -days "$DAYS" -key "$ca_key" -out "$ca_cert" \
        -subj "/CN=$CA_CN/O=FFmpeg-RTMP/OU=Certificate Authority" \
        2>/dev/null || error_exit "Failed to generate CA certificate"
    
    chmod 644 "$ca_cert"
    
    log_success "CA certificate generated:"
    log_success "  Certificate: $ca_cert"
    log_success "  Private Key: $ca_key"
}

# ============================================
# Master Certificate Generation
# ============================================
generate_master_cert() {
    log_info "Generating master server certificate..."
    
    local cert="$OUTPUT_DIR/master.crt"
    local key="$OUTPUT_DIR/master.key"
    local csr="$OUTPUT_DIR/master.csr"
    
    if [ -f "$cert" ] && [ -f "$key" ]; then
        log_warn "Master certificate already exists"
        read -p "Overwrite? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Skipping master certificate generation"
            return 0
        fi
    fi
    
    # Build SAN list
    local san_list="DNS:localhost,DNS:$MASTER_CN,IP:127.0.0.1,IP:::1"
    
    for host in "${MASTER_HOSTS[@]}"; do
        san_list="${san_list},DNS:${host}"
    done
    
    for ip in "${MASTER_IPS[@]}"; do
        san_list="${san_list},IP:${ip}"
    done
    
    log_info "Subject Alternative Names: $san_list"
    
    # Generate private key
    openssl genrsa -out "$key" 2048 2>/dev/null || error_exit "Failed to generate master key"
    chmod 600 "$key"
    
    # Create OpenSSL config for SAN
    local config="$OUTPUT_DIR/master.cnf"
    cat > "$config" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = $MASTER_CN
O = FFmpeg-RTMP
OU = Master Server

[v3_req]
keyUsage = keyEncipherment, digitalSignature
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = $san_list
EOF
    
    # Generate CSR
    openssl req -new -key "$key" -out "$csr" -config "$config" \
        2>/dev/null || error_exit "Failed to generate CSR"
    
    # Generate certificate (self-signed or CA-signed)
    if [ -f "$OUTPUT_DIR/ca.crt" ] && [ -f "$OUTPUT_DIR/ca.key" ]; then
        log_info "Signing with CA certificate..."
        openssl x509 -req -in "$csr" -CA "$OUTPUT_DIR/ca.crt" -CAkey "$OUTPUT_DIR/ca.key" \
            -CAcreateserial -out "$cert" -days "$DAYS" -extensions v3_req -extfile "$config" \
            2>/dev/null || error_exit "Failed to sign certificate"
    else
        log_info "Generating self-signed certificate..."
        openssl x509 -req -in "$csr" -signkey "$key" -out "$cert" -days "$DAYS" \
            -extensions v3_req -extfile "$config" \
            2>/dev/null || error_exit "Failed to generate certificate"
    fi
    
    chmod 644 "$cert"
    
    # Cleanup
    rm -f "$csr" "$config"
    
    log_success "Master certificate generated:"
    log_success "  Certificate: $cert"
    log_success "  Private Key: $key"
    
    # Show certificate info
    log_info "Certificate details:"
    openssl x509 -in "$cert" -noout -subject -dates -ext subjectAltName
}

# ============================================
# Worker Certificate Generation
# ============================================
generate_worker_cert() {
    log_info "Generating worker/agent certificate..."
    
    local cert="$OUTPUT_DIR/agent.crt"
    local key="$OUTPUT_DIR/agent.key"
    local csr="$OUTPUT_DIR/agent.csr"
    
    if [ -f "$cert" ] && [ -f "$key" ]; then
        log_warn "Worker certificate already exists"
        read -p "Overwrite? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Skipping worker certificate generation"
            return 0
        fi
    fi
    
    # Build SAN list
    local san_list="DNS:localhost,DNS:$WORKER_CN,IP:127.0.0.1,IP:::1"
    
    for host in "${WORKER_HOSTS[@]}"; do
        san_list="${san_list},DNS:${host}"
    done
    
    for ip in "${WORKER_IPS[@]}"; do
        san_list="${san_list},IP:${ip}"
    done
    
    log_info "Subject Alternative Names: $san_list"
    
    # Generate private key
    openssl genrsa -out "$key" 2048 2>/dev/null || error_exit "Failed to generate worker key"
    chmod 600 "$key"
    
    # Create OpenSSL config for SAN
    local config="$OUTPUT_DIR/agent.cnf"
    cat > "$config" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = $WORKER_CN
O = FFmpeg-RTMP
OU = Worker Agent

[v3_req]
keyUsage = keyEncipherment, digitalSignature
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = $san_list
EOF
    
    # Generate CSR
    openssl req -new -key "$key" -out "$csr" -config "$config" \
        2>/dev/null || error_exit "Failed to generate CSR"
    
    # Generate certificate (self-signed or CA-signed)
    if [ -f "$OUTPUT_DIR/ca.crt" ] && [ -f "$OUTPUT_DIR/ca.key" ]; then
        log_info "Signing with CA certificate..."
        openssl x509 -req -in "$csr" -CA "$OUTPUT_DIR/ca.crt" -CAkey "$OUTPUT_DIR/ca.key" \
            -CAcreateserial -out "$cert" -days "$DAYS" -extensions v3_req -extfile "$config" \
            2>/dev/null || error_exit "Failed to sign certificate"
    else
        log_info "Generating self-signed certificate..."
        openssl x509 -req -in "$csr" -signkey "$key" -out "$cert" -days "$DAYS" \
            -extensions v3_req -extfile "$config" \
            2>/dev/null || error_exit "Failed to generate certificate"
    fi
    
    chmod 644 "$cert"
    
    # Cleanup
    rm -f "$csr" "$config"
    
    log_success "Worker certificate generated:"
    log_success "  Certificate: $cert"
    log_success "  Private Key: $key"
    
    # Show certificate info
    log_info "Certificate details:"
    openssl x509 -in "$cert" -noout -subject -dates -ext subjectAltName
}

# ============================================
# Main Execution
# ============================================

echo ""
echo "╔════════════════════════════════════════╗"
echo "║   TLS Certificate Generation           ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Generate CA first if requested
if [ "$GENERATE_CA" = true ]; then
    generate_ca
    echo ""
fi

# Generate certificates based on mode
case $MODE in
    master)
        generate_master_cert
        ;;
    worker)
        generate_worker_cert
        ;;
    both)
        generate_master_cert
        echo ""
        generate_worker_cert
        ;;
    "")
        # Only CA generation requested
        if [ "$GENERATE_CA" = false ]; then
            error_exit "No mode specified"
        fi
        ;;
    *)
        error_exit "Invalid mode: $MODE"
        ;;
esac

echo ""
log_success "Certificate generation complete!"
echo ""
log_info "Next steps:"
echo ""

if [ "$MODE" = "master" ] || [ "$MODE" = "both" ]; then
    echo "  Master deployment:"
    echo "    sudo mkdir -p /etc/ffrtmp-master/certs"
    echo "    sudo cp $OUTPUT_DIR/master.{crt,key} /etc/ffrtmp-master/certs/"
    echo "    sudo chmod 600 /etc/ffrtmp-master/certs/master.key"
    echo ""
    echo "  Enable TLS in /etc/ffrtmp-master/master.env:"
    echo "    TLS_ENABLED=true"
    echo "    TLS_CERT=/etc/ffrtmp-master/certs/master.crt"
    echo "    TLS_KEY=/etc/ffrtmp-master/certs/master.key"
    echo ""
fi

if [ "$MODE" = "worker" ] || [ "$MODE" = "both" ]; then
    echo "  Worker deployment:"
    echo "    sudo mkdir -p /etc/ffrtmp/certs"
    echo "    sudo cp $OUTPUT_DIR/agent.{crt,key} /etc/ffrtmp/certs/"
    echo "    sudo chmod 600 /etc/ffrtmp/certs/agent.key"
    echo ""
    if [ -f "$OUTPUT_DIR/ca.crt" ]; then
        echo "    sudo cp $OUTPUT_DIR/ca.crt /etc/ffrtmp/certs/"
        echo ""
    fi
    echo "  Enable TLS in /etc/ffrtmp/worker.env:"
    echo "    MASTER_URL=https://master.example.com:8443"
    if [ -f "$OUTPUT_DIR/ca.crt" ]; then
        echo "    TLS_CA=/etc/ffrtmp/certs/ca.crt"
    fi
    echo ""
fi

if [ -f "$OUTPUT_DIR/ca.crt" ]; then
    echo "  For mTLS (mutual TLS authentication):"
    echo "    Distribute ca.crt to all nodes"
    echo "    Configure both master and workers to use client certificates"
    echo ""
fi

echo "  Certificate verification:"
echo "    openssl x509 -in $OUTPUT_DIR/master.crt -noout -text"
echo "    openssl x509 -in $OUTPUT_DIR/agent.crt -noout -text"
echo ""
