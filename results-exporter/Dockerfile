FROM python:3.11-slim

RUN apt-get update && \
    apt-get install -y curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY results_exporter.py /app/
COPY entrypoint.sh /app/

RUN chmod +x /app/results_exporter.py /app/entrypoint.sh

EXPOSE 9502

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:9502/health || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]
