#!/bin/bash
set -e

BASE_DIR="/home/sanpau/Documents/projects/ffmpeg-rtmp/ansible"
cd "$BASE_DIR"

# Create role directories
for role in common dependencies ffrtmp-master ffrtmp-worker; do
    mkdir -p "roles/$role"/{tasks,handlers,templates,files,vars,defaults,meta}
done

echo "Ansible structure created successfully!"
