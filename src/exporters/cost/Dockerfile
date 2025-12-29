FROM python:3.14-slim

RUN apt-get update && \
    apt-get install -y curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Python dependencies from root
COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt

# Copy application file
COPY src/exporters/cost/cost_exporter.py /app/

RUN chmod +x /app/cost_exporter.py

EXPOSE 9504

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:${COST_EXPORTER_PORT:-9504}/health || exit 1

ENV COST_EXPORTER_PORT=9504
ENV RESULTS_DIR=/results
ENV ENERGY_COST_PER_KWH=0.12
ENV CPU_COST_PER_HOUR=0.50
ENV CURRENCY=USD
ENV PROMETHEUS_URL=http://prometheus:9090

# Note: advisor module is mounted as volume at runtime

# Use shell form to allow environment variable expansion
CMD python3 /app/cost_exporter.py \
    --port ${COST_EXPORTER_PORT} \
    --results-dir ${RESULTS_DIR} \
    --energy-cost ${ENERGY_COST_PER_KWH} \
    --cpu-cost ${CPU_COST_PER_HOUR} \
    --currency ${CURRENCY} \
    --prometheus-url ${PROMETHEUS_URL}
