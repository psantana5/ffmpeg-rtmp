package logging

import "fmt"

// GenerateLogrotateConfig creates a logrotate configuration for a component
func GenerateLogrotateConfig(component string) string {
	return fmt.Sprintf(`# Logrotate configuration for FFmpeg-RTMP %s
# Install: sudo cp this file to /etc/logrotate.d/ffrtmp-%s

/var/log/ffrtmp/%s/*.log {
    # Rotate daily
    daily
    
    # Keep 14 days of logs
    rotate 14
    
    # Compress old logs
    compress
    delaycompress
    
    # Don't error if log is missing
    missingok
    
    # Don't rotate empty logs
    notifempty
    
    # Create new log with these permissions
    create 0644 ffrtmp ffrtmp
    
    # Run postrotate script only once for all logs
    sharedscripts
    
    # Reload service after rotation
    postrotate
        systemctl reload ffrtmp-%s 2>/dev/null || true
    endscript
}
`, component, component, component, component)
}

// GenerateMasterLogrotate generates logrotate config for master
func GenerateMasterLogrotate() string {
	return GenerateLogrotateConfig("master")
}

// GenerateWorkerLogrotate generates logrotate config for worker
func GenerateWorkerLogrotate() string {
	return GenerateLogrotateConfig("worker")
}

// GenerateWrapperLogrotate generates logrotate config for wrapper
func GenerateWrapperLogrotate() string {
	return GenerateLogrotateConfig("wrapper")
}
