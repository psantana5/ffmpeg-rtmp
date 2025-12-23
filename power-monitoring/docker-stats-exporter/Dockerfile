FROM python:3.11-slim

RUN apt-get update && \
    apt-get install -y docker.io procps curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY docker_stats_exporter.py /app/

RUN chmod +x docker_stats_exporter.py

EXPOSE 9501

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:9501/health || exit 1

CMD ["python3", "docker_stats_exporter.py"]
