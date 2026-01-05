#!/bin/bash
#
# GPU-aware job submission script
# Detects GPU availability and submits appropriate codec jobs
#

set -euo pipefail

COUNT=${1:-1000}
MASTER_URL="https://localhost:8080"

echo "=================================================="
echo "  GPU-Aware Job Submission"
echo "=================================================="
echo ""

# Detect GPU availability
HAS_GPU=false
if ./bin/ffrtmp nodes list 2>&1 | grep -q "│ Yes │"; then
    HAS_GPU=true
    echo "✓ GPU detected - including h264/h265 scenarios"
else
    echo "✗ No GPU detected - using CPU-optimized codecs only"
fi
echo ""

# Define scenarios based on GPU availability
if [ "$HAS_GPU" = true ]; then
    # GPU available - use hardware-accelerated codecs
    SCENARIOS=(
        "4K60-h264" "4K60-h265" "4K30-h264" "4K30-h265"
        "1080p60-h264" "1080p60-h265" "1080p30-h264" "1080p30-h265"
        "720p60-h264" "720p60-h265" "720p30-h264" "720p30-h265"
        "480p60-h264" "480p30-h264" "480p30-h265"
    )
else
    # No GPU - use CPU-friendly codecs (VP8, VP9, Theora, or lower res h264)
    SCENARIOS=(
        "1080p30-vp9" "1080p60-vp9"
        "720p60-vp9" "720p30-vp9" "720p60-vp8" "720p30-vp8"
        "480p60-vp9" "480p30-vp9" "480p60-vp8" "480p30-vp8"
        "480p30-av1" "360p30-vp9" "360p30-vp8"
        "720p30-h264" "480p30-h264" "360p30-h264"  # Low res h264 is OK on CPU
    )
fi

PRIORITIES=("high" "medium" "low")
QUEUES=("live" "default" "batch")

SUCCESS=0
FAILED=0

echo "Submitting $COUNT jobs..."
echo ""

for ((i=1; i<=$COUNT; i++)); do
    # Random parameters
    SCENARIO="${SCENARIOS[$((RANDOM % ${#SCENARIOS[@]}))]}"
    PRIORITY="${PRIORITIES[$((RANDOM % ${#PRIORITIES[@]}))]}"
    QUEUE="${QUEUES[$((RANDOM % ${#QUEUES[@]}))]}"
    DURATION=$((30 + RANDOM % 271))
    
    # Bitrate based on resolution
    if [[ "$SCENARIO" =~ 4K ]]; then
        BITRATE="$((10 + RANDOM % 15))M"
    elif [[ "$SCENARIO" =~ 1080p ]]; then
        BITRATE="$((3 + RANDOM % 5))M"
    elif [[ "$SCENARIO" =~ 720p ]]; then
        BITRATE="$((1 + RANDOM % 3))M"
    elif [[ "$SCENARIO" =~ 480p ]]; then
        BITRATE="$((500 + RANDOM % 1000))K"
    else
        BITRATE="$((300 + RANDOM % 500))K"
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
    
    # Small delay
    sleep 0.01
done

echo ""
echo "=================================================="
echo "  Submission Complete"
echo "=================================================="
echo "Total: $COUNT"
echo "Success: $SUCCESS"
echo "Failed: $FAILED"
echo "GPU Mode: $HAS_GPU"
echo ""
