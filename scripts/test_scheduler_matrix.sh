#!/bin/bash
#
# Comprehensive Scheduler Matrix Test
# Tests ALL possible combinations of scheduler parameters
# Total: 30+ unique job configurations
#

set -e

API_KEY="${MASTER_API_KEY}"
MASTER_URL="${MASTER_URL:-https://localhost:8080}"

if [ -z "$API_KEY" ]; then
  echo "ERROR: MASTER_API_KEY not set"
  exit 1
fi

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         SCHEDULER MATRIX TEST - 30+ CONFIGURATIONS           ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Testing every possible combination of:"
echo "  • Priorities: high, medium, low"
echo "  • Queues: live, default, batch"
echo "  • Engines: auto, ffmpeg, gstreamer"
echo "  • Confidence: auto, high, medium, low"
echo "  • Scenarios: Multiple resolutions & codecs"
echo "  • Parameters: bitrates, presets, CRF, encoders"
echo ""

# Job counter
JOB_NUM=0

# Helper function to submit job
submit_job() {
  local scenario=$1
  local priority=$2
  local queue=$3
  local engine=$4
  local confidence=$5
  shift 5
  local params="$@"
  
  JOB_NUM=$((JOB_NUM + 1))
  
  local json_body="{\"scenario\":\"$scenario\",\"priority\":\"$priority\",\"queue\":\"$queue\",\"engine\":\"$engine\",\"confidence\":\"$confidence\""
  
  # Parse additional parameters
  if [ -n "$params" ]; then
    json_body="$json_body,\"parameters\":{"
    local first=true
    for param in $params; do
      if [ "$first" = true ]; then
        first=false
      else
        json_body="$json_body,"
      fi
      
      # Parse key=value
      local key=$(echo "$param" | cut -d= -f1)
      local value=$(echo "$param" | cut -d= -f2)
      json_body="$json_body\"$key\":\"$value\""
    done
    json_body="$json_body}"
  fi
  
  json_body="$json_body}"
  
  local response=$(curl -sk -X POST \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "$json_body" \
    "$MASTER_URL/jobs" 2>/dev/null)
  
  local job_id=$(echo "$response" | jq -r '.id' 2>/dev/null)
  
  if [ "$job_id" != "null" ] && [ -n "$job_id" ]; then
    printf "  %3d. ✓ %s [%s/%s/%s/%s]\n" $JOB_NUM "$scenario" "$priority" "$queue" "$engine" "$confidence"
  else
    printf "  %3d. ✗ %s [FAILED]\n" $JOB_NUM "$scenario"
  fi
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 1: Resolution x Priority x Queue (27 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 3 resolutions × 3 priorities × 3 queues = 27 combinations
for resolution in "4K60-h264" "1080p60-h265" "720p30-h264"; do
  for priority in "high" "medium" "low"; do
    for queue in "live" "default" "batch"; do
      submit_job "$resolution" "$priority" "$queue" "auto" "high"
    done
  done
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 2: Engine Selection (9 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 3 engines × 3 scenarios = 9 combinations
for engine in "auto" "ffmpeg" "gstreamer"; do
  submit_job "1080p30-h264" "medium" "default" "$engine" "high"
  submit_job "720p60-h265" "medium" "default" "$engine" "medium"
  submit_job "480p30-h264" "low" "batch" "$engine" "low"
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 3: Confidence Levels (12 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 4 confidence levels × 3 priorities = 12 combinations
for confidence in "auto" "high" "medium" "low"; do
  submit_job "1080p60-h264" "high" "live" "auto" "$confidence"
  submit_job "720p30-h265" "medium" "default" "ffmpeg" "$confidence"
  submit_job "480p60-h264" "low" "batch" "gstreamer" "$confidence"
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 4: Bitrate Variations (10 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Different bitrates for different resolutions
submit_job "4K60-h264" "high" "default" "auto" "high" "bitrate=20000k"
submit_job "4K60-h264" "high" "default" "auto" "high" "bitrate=15000k"
submit_job "4K60-h264" "high" "default" "auto" "high" "bitrate=10000k"
submit_job "1080p60-h265" "medium" "default" "auto" "high" "bitrate=8000k"
submit_job "1080p60-h265" "medium" "default" "auto" "high" "bitrate=5000k"
submit_job "1080p60-h265" "medium" "default" "auto" "high" "bitrate=3000k"
submit_job "720p30-h264" "medium" "default" "auto" "medium" "bitrate=2500k"
submit_job "720p30-h264" "medium" "default" "auto" "medium" "bitrate=1500k"
submit_job "480p30-h264" "low" "batch" "auto" "low" "bitrate=1000k"
submit_job "480p30-h264" "low" "batch" "auto" "low" "bitrate=500k"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 5: Encoder Variations (8 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Different encoders
submit_job "1080p60-h264-nvenc" "high" "default" "ffmpeg" "high" "encoder=h264_nvenc"
submit_job "1080p60-h264-qsv" "high" "default" "ffmpeg" "high" "encoder=h264_qsv"
submit_job "1080p60-h264-vaapi" "high" "default" "ffmpeg" "high" "encoder=h264_vaapi"
submit_job "1080p60-x264" "medium" "default" "ffmpeg" "high" "encoder=libx264"
submit_job "1080p60-x265" "medium" "default" "ffmpeg" "high" "encoder=libx265"
submit_job "1080p60-hevc-nvenc" "high" "default" "ffmpeg" "high" "encoder=hevc_nvenc"
submit_job "720p30-vp9" "low" "batch" "ffmpeg" "medium" "encoder=libvpx-vp9"
submit_job "720p30-av1" "low" "batch" "ffmpeg" "low" "encoder=libaom-av1"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 6: Preset Variations (7 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Different presets
submit_job "1080p60-ultrafast" "high" "live" "ffmpeg" "high" "preset=ultrafast"
submit_job "1080p60-veryfast" "high" "default" "ffmpeg" "high" "preset=veryfast"
submit_job "1080p60-fast" "medium" "default" "ffmpeg" "high" "preset=fast"
submit_job "1080p60-medium" "medium" "default" "ffmpeg" "high" "preset=medium"
submit_job "1080p60-slow" "low" "batch" "ffmpeg" "medium" "preset=slow"
submit_job "1080p60-slower" "low" "batch" "ffmpeg" "low" "preset=slower"
submit_job "720p30-veryslow" "low" "batch" "ffmpeg" "low" "preset=veryslow"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 7: CRF Variations (6 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Different CRF values (quality)
submit_job "1080p60-crf18" "high" "default" "ffmpeg" "high" "crf=18"
submit_job "1080p60-crf20" "high" "default" "ffmpeg" "high" "crf=20"
submit_job "1080p60-crf23" "medium" "default" "ffmpeg" "medium" "crf=23"
submit_job "1080p60-crf26" "medium" "default" "ffmpeg" "medium" "crf=26"
submit_job "720p30-crf28" "low" "batch" "ffmpeg" "low" "crf=28"
submit_job "720p30-crf30" "low" "batch" "ffmpeg" "low" "crf=30"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 8: Complex Parameters (8 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Jobs with multiple parameters
submit_job "4K60-complex-1" "high" "default" "ffmpeg" "high" "bitrate=15000k" "preset=fast" "encoder=libx265"
submit_job "4K60-complex-2" "high" "default" "ffmpeg" "high" "bitrate=20000k" "preset=medium" "encoder=hevc_nvenc"
submit_job "1080p60-complex-1" "medium" "default" "ffmpeg" "high" "bitrate=5000k" "crf=23" "preset=fast"
submit_job "1080p60-complex-2" "medium" "default" "ffmpeg" "high" "bitrate=8000k" "preset=veryfast" "encoder=h264_nvenc"
submit_job "720p30-complex-1" "low" "batch" "ffmpeg" "medium" "bitrate=2000k" "crf=26" "preset=medium"
submit_job "720p30-complex-2" "low" "batch" "gstreamer" "medium" "bitrate=1500k" "preset=ultrafast"
submit_job "480p30-complex-1" "low" "batch" "ffmpeg" "low" "bitrate=800k" "crf=28" "encoder=libx264"
submit_job "480p30-complex-2" "low" "batch" "gstreamer" "low" "bitrate=500k" "preset=veryfast"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 9: HDR & Advanced Formats (5 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# HDR and advanced formats
submit_job "4K60-HDR-h265" "high" "default" "ffmpeg" "high" "bitrate=25000k" "encoder=libx265" "pixel_format=yuv420p10le"
submit_job "4K60-HDR-hevc-nvenc" "high" "default" "ffmpeg" "high" "bitrate=30000k" "encoder=hevc_nvenc" "pixel_format=p010le"
submit_job "1080p60-10bit" "medium" "default" "ffmpeg" "high" "encoder=libx264" "pixel_format=yuv420p10le"
submit_job "1080p60-444" "medium" "default" "ffmpeg" "medium" "encoder=libx264" "pixel_format=yuv444p"
submit_job "720p30-422" "low" "default" "ffmpeg" "medium" "encoder=libx264" "pixel_format=yuv422p"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "MATRIX 10: Extreme Edge Cases (8 jobs)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Edge cases
submit_job "8K60-extreme" "high" "default" "ffmpeg" "high" "bitrate=50000k" "encoder=libx265" "preset=ultrafast"
submit_job "360p15-minimal" "low" "batch" "ffmpeg" "low" "bitrate=250k" "encoder=libx264" "preset=ultrafast"
submit_job "4K120-highfps" "high" "live" "ffmpeg" "high" "bitrate=40000k" "encoder=h264_nvenc" "preset=p1"
submit_job "1080p240-esports" "high" "live" "ffmpeg" "high" "bitrate=15000k" "encoder=h264_nvenc" "preset=p1"
submit_job "720p60-lowlatency" "high" "live" "gstreamer" "high" "bitrate=3000k" "preset=ultrafast"
submit_job "4K24-cinema" "medium" "batch" "ffmpeg" "high" "bitrate=25000k" "encoder=libx265" "preset=slow"
submit_job "1080p30-streaming" "medium" "live" "ffmpeg" "high" "bitrate=4500k" "encoder=libx264" "preset=veryfast"
submit_job "480p60-retro" "low" "batch" "gstreamer" "low" "bitrate=800k" "preset=ultrafast"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ MATRIX TEST COMPLETE"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Total Jobs Submitted: $JOB_NUM"
echo ""
echo "Waiting 10 seconds before status check..."
sleep 10

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "INITIAL STATUS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
curl -sk -H "Authorization: Bearer $API_KEY" "$MASTER_URL/jobs" | \
  jq '{
    total: .count,
    by_status: [.jobs | group_by(.status)[] | {status: .[0].status, count: length}],
    by_priority: [.jobs | group_by(.priority)[] | {priority: .[0].priority, count: length}],
    by_queue: [.jobs | group_by(.queue)[] | {queue: .[0].queue, count: length}],
    by_engine: [.jobs | group_by(.engine)[] | {engine: .[0].engine, count: length}],
    by_confidence: [.jobs | group_by(.confidence)[] | {confidence: .[0].confidence, count: length}]
  }'

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 DETAILED BREAKDOWN"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Matrix Sizes:"
echo "  1. Resolution x Priority x Queue: 27 jobs"
echo "  2. Engine Selection: 9 jobs"
echo "  3. Confidence Levels: 12 jobs"
echo "  4. Bitrate Variations: 10 jobs"
echo "  5. Encoder Variations: 8 jobs"
echo "  6. Preset Variations: 7 jobs"
echo "  7. CRF Variations: 6 jobs"
echo "  8. Complex Parameters: 8 jobs"
echo "  9. HDR & Advanced: 5 jobs"
echo " 10. Edge Cases: 8 jobs"
echo ""
echo "Expected Total: 100 jobs"
echo "Actual Submitted: $JOB_NUM jobs"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "⏳ Monitor with:"
echo "    watch -n 2 'curl -sk -H \"Authorization: Bearer \$MASTER_API_KEY\" $MASTER_URL/jobs | jq \".count, .jobs | group_by(.status) | map({status: .[0].status, count: length})\"'"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
