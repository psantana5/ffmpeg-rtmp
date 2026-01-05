#!/bin/bash
#
# Submit 1000 jobs using the ffrtmp CLI (with authentication support)
#

set -euo pipefail

COUNT=${1:-1000}
MASTER_URL="https://localhost:8080"

echo "=================================================="
echo "  Submitting $COUNT Jobs via CLI"
echo "=================================================="
echo ""

# Scenarios to cycle through (CPU-friendly mix)
SCENARIOS=(
    "4K60-h264" "4K60-h265" "4K30-h264" "4K30-h265"
    "1080p60-h264" "1080p60-h265" "1080p30-h264" "1080p30-h265"
    "720p60-h264" "720p60-h265" "720p30-h264" "720p30-h265"
    "480p30-h264" "480p30-h265" "480p60-h264"
    "1080p60-vp9" "720p30-vp9" "480p30-av1"
)
PRIORITIES=("high" "medium" "low")
QUEUES=("live" "default" "batch")

SUCCESS=0
FAILED=0

for ((i=1; i<=$COUNT; i++)); do
    # Random parameters
    SCENARIO="${SCENARIOS[$((RANDOM % ${#SCENARIOS[@]}))]}"
    PRIORITY="${PRIORITIES[$((RANDOM % ${#PRIORITIES[@]}))]}"
    QUEUE="${QUEUES[$((RANDOM % ${#QUEUES[@]}))]}"
    DURATION=$((30 + RANDOM % 271))
    
    # Bitrate based on scenario
    if [[ "$SCENARIO" =~ 4K ]]; then
        BITRATE="$((10 + RANDOM % 15))M"
    elif [[ "$SCENARIO" =~ 1080p ]]; then
        BITRATE="$((4 + RANDOM % 6))M"
    elif [[ "$SCENARIO" =~ 720p ]]; then
        BITRATE="$((2 + RANDOM % 3))M"
    else
        BITRATE="$((1 + RANDOM % 2))M"
    fi
    
    # Submit job
    if ./bin/ffrtmp jobs submit \
        --scenario "$SCENARIO" \
        --priority "$PRIORITY" \
        --queue "$QUEUE" \
        --duration "$DURATION" \
        --bitrate "$BITRATE" \
        --output json \
        > /dev/null 2>&1; then
        ((SUCCESS++))
    else
        ((FAILED++))
    fi
    
    # Progress
    if [[ $((i % 50)) -eq 0 ]]; then
        PERCENT=$((i * 100 / COUNT))
        echo "Progress: $i/$COUNT ($PERCENT%) - Success: $SUCCESS, Failed: $FAILED"
    fi
    
    # Small delay to avoid overwhelming
    sleep 0.01
done

echo ""
echo "=================================================="
echo "  Submission Complete"
echo "=================================================="
echo "Total: $COUNT"
echo "Success: $SUCCESS"
echo "Failed: $FAILED"
echo ""
