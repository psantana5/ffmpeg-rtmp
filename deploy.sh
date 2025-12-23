#!/bin/bash
# RTMP + HLS Streaming Setup Script med Prometheus & Grafana
# Plug-and-play version för lokal maskin

set -e

# Konfigurationsvariabler
HLS_DIR="$HOME/Documents/hls"
PROMETHEUS_DIR="$HOME/Documents/prometheus"
PROMETHEUS_DATA_DIR="$PROMETHEUS_DIR/data"
GRAFANA_DIR="$HOME/Documents/grafana"
GRAFANA_DATA_DIR="$GRAFANA_DIR/data"

CONTAINER_NGINX="nginx-rtmp"
CONTAINER_EXPORTER="nginx-rtmp-exporter"
CONTAINER_GRAFANA="grafana"

RTMP_PORT=1935
HTTP_PORT=8080
METRICS_PORT=9114
PROMETHEUS_PORT=9090
GRAFANA_PORT=3000

# Installera nödvändiga verktyg
sudo apt update
sudo apt install -y ffmpeg obs-studio curl wget tar

# Skapa mappar
mkdir -p "$HLS_DIR" "$PROMETHEUS_DATA_DIR" "$GRAFANA_DATA_DIR"
chmod 777 "$HLS_DIR" "$PROMETHEUS_DATA_DIR" "$GRAFANA_DATA_DIR"

# Stoppa och ta bort gamla containrar
for c in "$CONTAINER_NGINX" "$CONTAINER_EXPORTER" "$CONTAINER_GRAFANA"; do
  if docker ps -a | grep -q "$c"; then
    docker stop "$c" >/dev/null 2>&1 || true
    docker rm "$c" >/dev/null 2>&1 || true
  fi
done

# Skapa NGINX RTMP-konfiguration
NGINX_CONF=$(mktemp)
cat >"$NGINX_CONF" <<'EOF'
worker_processes auto;
rtmp_auto_push on;
events {}
rtmp {
    server {
        listen 1935;
        application live {
            live on;
            record off;
            hls on;
            hls_path /tmp/hls;
            hls_fragment 3s;
            hls_playlist_length 30s;
        }
    }
}
http {
    server {
        listen 8080;
        server_name localhost;
        location /hls {
            types {
                application/vnd.apple.mpegurl m3u8;
                video/mp2t ts;
            }
            root /tmp;
            add_header Cache-Control no-cache;
        }
        location /stat {
            rtmp_stat all;
            rtmp_stat_stylesheet stat.xsl;
        }
        location /stat.xsl {
            root /usr/local/nginx/html;
        }
        location / {
            return 200 "NGINX RTMP HLS Lab running\n";
        }
    }
}
EOF

# Starta NGINX RTMP-container
docker run -d \
  --name "$CONTAINER_NGINX" \
  -p $RTMP_PORT:1935 \
  -p $HTTP_PORT:8080 \
  -v "$HLS_DIR:/tmp/hls" \
  tiangolo/nginx-rtmp

# Kopiera och ladda konfiguration
sleep 2
docker cp "$NGINX_CONF" "$CONTAINER_NGINX:/etc/nginx/nginx.conf"
docker exec "$CONTAINER_NGINX" nginx -s reload
rm "$NGINX_CONF"

# Prometheus RTMP Exporter
docker run -d \
  --name "$CONTAINER_EXPORTER" \
  --network host \
  skyefuzz/nginx-rtmp-exporter:latest \
  --scrape-url "http://localhost:$HTTP_PORT/stat" \
  --host 0.0.0.0 \
  --port $METRICS_PORT

# Ladda ner Prometheus
PROMETHEUS_VERSION="2.48.1"
if [ ! -f "$PROMETHEUS_DIR/prometheus" ]; then
  cd /tmp
  wget -q "https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-amd64.tar.gz" -O prometheus.tar.gz
  tar -xzf prometheus.tar.gz
  mv prometheus-${PROMETHEUS_VERSION}.linux-amd64/* "$PROMETHEUS_DIR/"
  rm -rf prometheus.tar.gz prometheus-${PROMETHEUS_VERSION}.linux-amd64
fi

# Skapa Prometheus-konfiguration
cat >"$PROMETHEUS_DIR/prometheus.yml" <<EOF
global:
  scrape_interval: 5s
scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:$PROMETHEUS_PORT']
  - job_name: 'nginx-rtmp'
    static_configs:
      - targets: ['localhost:$METRICS_PORT']
EOF

# Starta Prometheus
cd "$PROMETHEUS_DIR"
nohup ./prometheus \
  --config.file=prometheus.yml \
  --storage.tsdb.path="$PROMETHEUS_DATA_DIR" \
  --web.listen-address="0.0.0.0:$PROMETHEUS_PORT" \
  --web.enable-lifecycle >prometheus.log 2>&1 &

PROMETHEUS_PID=$!

# Starta Grafana
docker run -d \
  --name "$CONTAINER_GRAFANA" \
  --network host \
  -v "$GRAFANA_DATA_DIR:/var/lib/grafana" \
  -e "GF_SECURITY_ADMIN_PASSWORD=admin" \
  -e "GF_USERS_ALLOW_SIGN_UP=false" \
  grafana/grafana:latest

sleep 5
echo "Setup klar!"
echo "RTMP: rtmp://localhost:$RTMP_PORT/live/stream"
echo "HLS: http://localhost:$HTTP_PORT/hls/stream.m3u8"
echo "Prometheus: http://localhost:$PROMETHEUS_PORT"
echo "Grafana: http://localhost:$GRAFANA_PORT (admin/admin)"
