-- PostgreSQL Initialization Script
-- This script is run automatically when the Docker container starts

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create indexes for better performance
-- (Tables are created by the Go application)

-- Add a trigger to automatically update last_activity_at
CREATE OR REPLACE FUNCTION update_last_activity()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_activity_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (if needed for additional users)
GRANT ALL PRIVILEGES ON DATABASE ffmpeg_rtmp TO ffmpeg;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ffmpeg;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO ffmpeg;

-- Create a read-only user for metrics/monitoring
CREATE USER ffmpeg_readonly WITH PASSWORD 'readonly_password';
GRANT CONNECT ON DATABASE ffmpeg_rtmp TO ffmpeg_readonly;
GRANT USAGE ON SCHEMA public TO ffmpeg_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO ffmpeg_readonly;

-- Ensure future tables are also accessible
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO ffmpeg_readonly;

