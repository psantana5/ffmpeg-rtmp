FROM python:3.11-slim

RUN apt-get update && \
    apt-get install -y curl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the health check script
COPY check_exporters_health.py /app/

# Make it executable
RUN chmod +x /app/check_exporters_health.py

# Run the health checker
ENTRYPOINT ["python3", "/app/check_exporters_health.py"]

# Default command: expose metrics on port 9600
CMD ["--port", "9600"]
