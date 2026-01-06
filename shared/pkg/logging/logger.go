package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging with file output support
type Logger struct {
	level      Level
	jsonFormat bool
	output     io.Writer
	fields     map[string]interface{}
	logFile    *os.File
	component  string
}

// NewLogger creates a new logger
func NewLogger(level Level, jsonFormat bool) *Logger {
	return &Logger{
		level:      level,
		jsonFormat: jsonFormat,
		output:     os.Stdout,
		fields:     make(map[string]interface{}),
	}
}

// NewFileLogger creates a logger that writes to /var/log/ffrtmp/<component>/<subcomponent>.log
// Falls back to ./logs/<component>/ if /var/log is not writable
func NewFileLogger(component, subComponent string, level Level, jsonFormat bool) (*Logger, error) {
	// Try /var/log/ffrtmp first
	baseDir := "/var/log/ffrtmp"
	if !isWritable(baseDir) {
		// Fallback to local logs directory
		baseDir = "./logs"
	}
	
	// Create directory structure: /var/log/ffrtmp/<component>/
	logDir := filepath.Join(baseDir, component)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory %s: %w", logDir, err)
	}
	
	// Determine log file name
	logFileName := component + ".log"
	if subComponent != "" {
		logFileName = subComponent + ".log"
	}
	logPath := filepath.Join(logDir, logFileName)
	
	// Open log file (create or append)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}
	
	// Create multi-writer (both file and stdout)
	multiWriter := io.MultiWriter(logFile, os.Stdout)
	
	logger := &Logger{
		level:      level,
		jsonFormat: jsonFormat,
		output:     multiWriter,
		fields:     make(map[string]interface{}),
		logFile:    logFile,
		component:  component + "/" + subComponent,
	}
	
	logger.Info(fmt.Sprintf("Logger initialized: %s -> %s", logger.component, logPath))
	
	return logger, nil
}

// SetOutput sets the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.output = w
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// log writes a log entry
func (l *Logger) log(level Level, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	// Merge logger fields and call fields
	mergedFields := make(map[string]interface{})
	for k, v := range l.fields {
		mergedFields[k] = v
	}
	for k, v := range fields {
		mergedFields[k] = v
	}

	if l.jsonFormat {
		entry := LogEntry{
			Timestamp: time.Now().Format(time.RFC3339),
			Level:     level.String(),
			Message:   message,
			Fields:    mergedFields,
		}
		data, err := json.Marshal(entry)
		if err != nil {
			log.Printf("Failed to marshal log entry: %v", err)
			return
		}
		fmt.Fprintln(l.output, string(data))
	} else {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(l.output, "[%s] %s: %s", timestamp, level.String(), message)
		if len(mergedFields) > 0 {
			fmt.Fprintf(l.output, " %v", mergedFields)
		}
		fmt.Fprintln(l.output)
	}

	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, message, f)
}

// Info logs an info message
func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, message, f)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, message, f)
}

// Error logs an error message
func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, message, f)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, message, f)
}

// WithField adds a field to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	// Copy fields to avoid mutation
	newFields := make(map[string]interface{}, len(l.fields)+1)
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value
	return &Logger{
		level:      l.level,
		jsonFormat: l.jsonFormat,
		output:     l.output,
		fields:     newFields,
	}
}

// ParseLevel parses a log level string
func ParseLevel(level string) Level {
	switch level {
	case "DEBUG", "debug":
		return DEBUG
	case "INFO", "info":
		return INFO
	case "WARN", "warn", "WARNING", "warning":
		return WARN
	case "ERROR", "error":
		return ERROR
	case "FATAL", "fatal":
		return FATAL
	default:
		return INFO
	}
}

// Close closes the log file if opened
func (l *Logger) Close() error {
	if l.logFile != nil {
		l.Info("Logger closing")
		return l.logFile.Close()
	}
	return nil
}

// RotateIfNeeded rotates log file if it exceeds maxSize (in bytes)
func (l *Logger) RotateIfNeeded(maxSize int64) error {
	if l.logFile == nil {
		return nil
	}
	
	info, err := l.logFile.Stat()
	if err != nil {
		return err
	}
	
	if info.Size() > maxSize {
		// Close current file
		l.logFile.Close()
		
		// Rename to timestamped backup
		oldPath := l.logFile.Name()
		timestamp := time.Now().Format("20060102-150405")
		backupPath := oldPath + "." + timestamp
		
		if err := os.Rename(oldPath, backupPath); err != nil {
			return err
		}
		
		// Reopen
		newFile, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		
		l.logFile = newFile
		
		// Update output to multi-writer
		multiWriter := io.MultiWriter(l.logFile, os.Stdout)
		l.output = multiWriter
		
		l.Info(fmt.Sprintf("Log rotated: %s -> %s", oldPath, backupPath))
	}
	
	return nil
}

// isWritable checks if directory is writable
func isWritable(path string) bool {
	// Try to create directory if it doesn't exist
	if err := os.MkdirAll(path, 0755); err != nil {
		return false
	}
	
	// Test write permissions
	testFile := filepath.Join(path, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

// GetLogPath returns the expected log path for a component
func GetLogPath(component, subComponent string) string {
	baseDir := "/var/log/ffrtmp"
	if !isWritable(baseDir) {
		baseDir = "./logs"
	}
	
	logDir := filepath.Join(baseDir, component)
	
	logFileName := component + ".log"
	if subComponent != "" {
		logFileName = subComponent + ".log"
	}
	
	return filepath.Join(logDir, logFileName)
}
