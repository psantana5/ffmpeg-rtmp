#!/bin/bash
# demo_watch_discovery.sh
#
# Interactive demo showing ffrtmp watch discovering processes

set -e

cd "$(dirname "$0")/.."

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          FFmpeg Watch - Auto-Discovery Demo                    ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""
echo "This demo shows how 'ffrtmp watch' automatically discovers"
echo "and governs FFmpeg processes as they start."
echo ""

# Check if ffmpeg is available
if ! command -v ffmpeg &> /dev/null; then
    echo "⚠️  FFmpeg not found - this demo requires FFmpeg"
    echo "   Install with: sudo apt install ffmpeg"
    exit 1
fi

echo "Step 1: Starting watch daemon..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
./bin/ffrtmp watch --scan-interval 5s --cpu-quota 100 > /tmp/watch_demo.log 2>&1 &
WATCH_PID=$!
echo "Watch daemon started (PID: $WATCH_PID)"
echo "Scan interval: 5 seconds"
echo ""

sleep 3

echo "Step 2: Watch daemon is now monitoring for FFmpeg processes..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Current status:"
tail -5 /tmp/watch_demo.log
echo ""

echo "Step 3: Starting FFmpeg process (externally, not via wrapper)..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
ffmpeg -f lavfi -i testsrc=duration=60:size=640x480:rate=30 \
       -c:v libx264 -preset ultrafast /tmp/demo_output.mp4 \
       > /dev/null 2>&1 &
FFMPEG_PID=$!
echo "FFmpeg started (PID: $FFMPEG_PID)"
echo "→ This is an EXTERNAL process (not started by wrapper)"
echo "→ Duration: 60 seconds (long enough to be discovered)"
echo ""

echo "Step 4: Waiting for watch daemon to discover it..."
echo "(Scan interval is 5s, so max wait time is ~5 seconds)"
echo ""

for i in {1..10}; do
    sleep 1
    if grep -q "Attaching to PID $FFMPEG_PID" /tmp/watch_demo.log 2>/dev/null; then
        echo "✓ Discovery happened after $i second(s)!"
        break
    fi
    echo -n "."
done
echo ""
echo ""

echo "Step 5: Check what watch daemon did..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if grep -q "Attaching to PID $FFMPEG_PID" /tmp/watch_demo.log; then
    echo "✓ SUCCESS: Watch daemon discovered and attached!"
    grep "Attaching to PID $FFMPEG_PID" /tmp/watch_demo.log
    echo ""
    echo "Resource governance is now active on PID $FFMPEG_PID"
else
    echo "⚠️  Process may not have been discovered yet"
    echo "   (scan interval is 5s, timing may vary)"
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 6: FFmpeg is still running independently..."
if ps -p $FFMPEG_PID > /dev/null 2>&1; then
    echo "✓ FFmpeg PID $FFMPEG_PID is still running"
    echo "✓ Wrapper is observing, not controlling"
else
    echo "✓ FFmpeg completed (transcode finished)"
fi
echo ""

echo "Step 7: Cleaning up..."
kill $WATCH_PID 2>/dev/null || true
kill $FFMPEG_PID 2>/dev/null || true
rm -f /tmp/demo_output.mp4
echo "✓ Demo complete"
echo ""

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║                      How It Works                              ║"
echo "╠════════════════════════════════════════════════════════════════╣"
echo "║ 1. Watch daemon scans /proc every 5 seconds                    ║"
echo "║ 2. Finds processes matching: ffmpeg, gst-launch-1.0            ║"
echo "║ 3. Automatically attaches governance (cgroups)                 ║"
echo "║ 4. Process continues running independently                     ║"
echo "║ 5. If watch daemon crashes, processes are unaffected           ║"
echo "║                                                                 ║"
echo "║ This is AUTOMATIC GOVERNANCE without manual intervention!      ║"
echo "╚════════════════════════════════════════════════════════════════╝"
