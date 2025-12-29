FROM python:3.14-slim

RUN apt-get update && \
    apt-get install -y curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY rapl_exporter.py /app/

RUN chmod +x rapl_exporter.py

EXPOSE 9500

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:9500/health || exit 1

CMD ["python3", "rapl_exporter.py"]
