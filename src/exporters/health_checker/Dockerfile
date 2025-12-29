FROM python:3.14-slim

WORKDIR /app

# Copy the health check script
COPY check_exporters_health.py /app/

# Make it executable
RUN chmod +x /app/check_exporters_health.py

# Run the health checker
ENTRYPOINT ["python3", "/app/check_exporters_health.py"]

# Default command: expose metrics on port 9600
CMD ["--port", "9600"]
