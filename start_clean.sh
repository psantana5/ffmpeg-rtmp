#!/bin/bash
# Start deployment in clean environment without API key
cd /home/sanpau/Documents/projects/ffmpeg-rtmp
unset FFMPEG_RTMP_API_KEY
rm -f master.db
NUM_WORKERS=1 ./deploy_production.sh start
