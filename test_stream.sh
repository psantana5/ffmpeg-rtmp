#!/bin/bash
# Test streaming script - generates RTMP streams at various bitrates

set -e

if ! command -v ffmpeg &> /dev/null; then
    echo "ERROR: ffmpeg is not installed"
    echo "Install with: sudo apt-get install ffmpeg (Ubuntu/Debian)"
    echo "Or: brew install ffmpeg (macOS)"
    exit 1
fi

RTMP_URL="rtmp://localhost:1935/live"
STREAM_KEY="test"

show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -b, --bitrate BITRATE    Video bitrate (e.g., 1000k, 2500k, 5000k)"
    echo "  -r, --resolution WxH     Video resolution (default: 1280x720)"
    echo "  -f, --fps FPS           Frame rate (default: 30)"
    echo "  -k, --key KEY           Stream key (default: test)"
    echo "  -i, --input FILE        Input video file (if not provided, generates test pattern)"
    echo "  -d, --duration SECONDS  Duration to stream (default: continuous)"
    echo "  -h, --help              Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -b 2500k                           # Stream at 2.5 Mbps, 720p, 30fps"
    echo "  $0 -b 5000k -r 1920x1080 -f 60        # Stream at 5 Mbps, 1080p, 60fps"
    echo "  $0 -i video.mp4 -b 3000k              # Stream existing video at 3 Mbps"
    echo ""
}

# Default values
BITRATE="2500k"
RESOLUTION="1280x720"
FPS="30"
INPUT_FILE=""
DURATION=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--bitrate)
            BITRATE="$2"
            shift 2
            ;;
        -r|--resolution)
            RESOLUTION="$2"
            shift 2
            ;;
        -f|--fps)
            FPS="$2"
            shift 2
            ;;
        -k|--key)
            STREAM_KEY="$2"
            shift 2
            ;;
        -i|--input)
            INPUT_FILE="$2"
            shift 2
            ;;
        -d|--duration)
            DURATION="-t $2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

echo "=== Starting RTMP Stream ==="
echo "Target: ${RTMP_URL}/${STREAM_KEY}"
echo "Bitrate: ${BITRATE}"
echo "Resolution: ${RESOLUTION}"
echo "FPS: ${FPS}"
echo ""

if [ -z "$INPUT_FILE" ]; then
    echo "Generating synthetic test pattern (colorful moving bars)..."
    echo "Press Ctrl+C to stop streaming"
    echo ""
    
    # Generate synthetic video with moving color bars
    ffmpeg -re \
        -f lavfi -i "testsrc=size=${RESOLUTION}:rate=${FPS}" \
        -f lavfi -i "sine=frequency=1000:sample_rate=48000" \
        -c:v libx264 -preset veryfast -tune zerolatency \
        -b:v "${BITRATE}" -maxrate "${BITRATE}" -bufsize $((2 * ${BITRATE%k}))k \
        -pix_fmt yuv420p -g $((FPS * 2)) \
        -c:a aac -b:a 128k -ar 48000 \
        -f flv "${RTMP_URL}/${STREAM_KEY}" \
        $DURATION
else
    if [ ! -f "$INPUT_FILE" ]; then
        echo "ERROR: Input file not found: $INPUT_FILE"
        exit 1
    fi
    
    echo "Streaming from file: ${INPUT_FILE}"
    echo "Press Ctrl+C to stop streaming"
    echo ""
    
    # Stream from existing video file (looped)
    ffmpeg -re -stream_loop -1 -i "${INPUT_FILE}" \
        -c:v libx264 -preset veryfast -tune zerolatency \
        -b:v "${BITRATE}" -maxrate "${BITRATE}" -bufsize $((2 * ${BITRATE%k}))k \
        -s "${RESOLUTION}" -r "${FPS}" \
        -pix_fmt yuv420p -g $((FPS * 2)) \
        -c:a aac -b:a 128k -ar 48000 \
        -f flv "${RTMP_URL}/${STREAM_KEY}" \
        $DURATION
fi
