#!/bin/bash
set -e

# Ensure results directory exists and is writable
RESULTS_DIR=${RESULTS_DIR:-/results}

if [ ! -d "$RESULTS_DIR" ]; then
    echo "Creating results directory: $RESULTS_DIR"
    mkdir -p "$RESULTS_DIR"
fi

# Set proper permissions (readable by exporter)
chmod 755 "$RESULTS_DIR"

echo "Results directory configured: $RESULTS_DIR"
echo "Starting results exporter..."

# Execute the main application
exec python3 /app/results_exporter.py
