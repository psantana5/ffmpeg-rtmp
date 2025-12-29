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
COPY src/exporters/qoe/qoe_exporter.py /app/

RUN chmod +x /app/qoe_exporter.py

EXPOSE 9503

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:${QOE_EXPORTER_PORT:-9503}/health || exit 1

ENV QOE_EXPORTER_PORT=9503
ENV RESULTS_DIR=/results

# Note: advisor module is mounted as volume at runtime

# Use shell form to allow environment variable expansion
CMD python3 /app/qoe_exporter.py --port ${QOE_EXPORTER_PORT} --results-dir ${RESULTS_DIR}
