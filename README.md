FFmpeg + Nginx RTMP Streaming Energy Monitoring

Overview
Detta projekt är en komplett test- och övervakningsstack för att mäta energiförbrukning vid videostreaming med Nginx RTMP och FFmpeg. Projektet inkluderar:

Nginx RTMP-server med HLS-streaming

FFmpeg-baserade automatiserade testscenarion

RAPL-baserad CPU/DRAM power monitoring

Docker-overhead monitoring

Prometheus & Grafana integration för metrics

Automatisk analys och CSV-export av resultat

Requirements

Docker & Docker Compose

Python 3.11+

Intel CPU med RAPL-stöd (valfritt)

Linux-baserad miljö (Ubuntu, Debian, CentOS, etc.)

Setup

Klona repo:
git clone https://github.com/ditt-anvandarnamn/ffmpeg-nginx-rtmp.git
cd ffmpeg-nginx-rtmp

Bygg och starta stacken:
docker-compose up --build -d

Kontrollera att alla tjänster är igång:

Nginx RTMP HLS: http://localhost:8080/hls/

Prometheus: http://localhost:9090

Grafana: http://localhost:3000
 (admin/admin)

RAPL-exporter: http://localhost:9500/metrics

Docker stats exporter: http://localhost:9501/metrics

Running Tests

Baslinjetest (ingen streaming):
python3 tests/test_runner.py --baseline

Kör fördefinierade scenarion:
python3 tests/test_runner.py

Flera samtidiga streams:
python3 tests/test_runner.py --multi 3 2500k

Resultat sparas i tests/test_results/ som JSON-filer

Analyzing Results
Generera analyser och export till CSV:
python3 tests/analyze_results.py tests/test_results/test_results_YYYYMMDD_HHMMSS.json
Resultatet skrivs ut i terminalen och sparas som CSV i samma mapp.

Grafana Dashboards

Förkonfigurerade dashboards finns i power-monitoring/grafana/provisioning/dashboards

Datasources är redan kopplade till Prometheus-metrics

Project Structure

exporters/

rapl/

docker-stats/

tests/

test_runner.py

analyze_results.py

docker/

docker-compose.yml

nginx/

nginx.conf

video_samples/

README.md

Troubleshooting

Kontrollera att inga andra tjänster använder portarna 1935, 8080, 9090, 3000, 9500, 9501

RAPL-exporter kräver root eller privilegierad Docker-container

Om metrics inte visas, kolla loggar:
docker-compose logs -f
