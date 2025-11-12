package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// LogLevel represents the minimum log level to output
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger provides structured logging with file rotation support
// Supports both JSON and plain text formats
type Logger struct {
	level      LogLevel
	jsonFormat bool
	writer     io.Writer
	info       *log.Logger
	error      *log.Logger
	warn       *log.Logger
	debug      *log.Logger
}

// LogEntry represents a structured log entry for JSON output
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// New creates a new logger instance
// Supports file rotation, JSON format, and configurable log levels
func New(level string, logToFile bool, logFilePath string, logFormat string) *Logger {
	// Parse log level
	logLevel := parseLogLevel(level)
	jsonFormat := logFormat == "json"

	// Determine output writer
	var writer io.Writer = os.Stdout
	if logToFile && logFilePath != "" {
		// Ensure directory exists
		dir := filepath.Dir(logFilePath)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}

		// Use lumberjack for file rotation
		fileWriter := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    100, // megabytes
			MaxBackups: 7,
			MaxAge:     30, // days
			Compress:   true,
		}

		// Write to both file and stdout
		writer = io.MultiWriter(os.Stdout, fileWriter)
	}

	flags := log.LstdFlags
	if !jsonFormat {
		flags |= log.Lshortfile
	}

	logger := &Logger{
		level:      logLevel,
		jsonFormat: jsonFormat,
		writer:     writer,
		info:       log.New(writer, "", flags),
		error:      log.New(writer, "", flags),
		warn:       log.New(writer, "", flags),
		debug:      log.New(writer, "", flags),
	}

	return logger
}

// parseLogLevel converts string level to LogLevel enum
func parseLogLevel(level string) LogLevel {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// log writes a log entry with appropriate formatting
func (l *Logger) log(level string, levelEnum LogLevel, format string, v ...interface{}) {
	if levelEnum < l.level {
		return // Skip if below minimum log level
	}

	message := format
	if len(v) > 0 {
		message = fmt.Sprintf(format, v...)
	}

	if l.jsonFormat {
		l.logJSON(level, message, nil)
	} else {
		prefix := fmt.Sprintf("[%s] ", level)
		switch levelEnum {
		case LevelError:
			l.error.SetPrefix(prefix)
			l.error.Println(message)
		case LevelWarn:
			l.warn.SetPrefix(prefix)
			l.warn.Println(message)
		case LevelInfo:
			l.info.SetPrefix(prefix)
			l.info.Println(message)
		case LevelDebug:
			l.debug.SetPrefix(prefix)
			l.debug.Println(message)
		}
	}
}

// logJSON writes a structured JSON log entry
func (l *Logger) logJSON(level, message string, fields map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		// Fallback to plain text if JSON marshaling fails
		l.info.Printf("[%s] %s", level, message)
		return
	}

	fmt.Fprintln(l.writer, string(data))
}

// Info logs an info-level message
func (l *Logger) Info(format string, v ...interface{}) {
	l.log("INFO", LevelInfo, format, v...)
}

// Error logs an error-level message
func (l *Logger) Error(format string, v ...interface{}) {
	l.log("ERROR", LevelError, format, v...)
}

// Warn logs a warning-level message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log("WARN", LevelWarn, format, v...)
}

// Debug logs a debug-level message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.log("DEBUG", LevelDebug, format, v...)
}

// WithFields logs a message with additional structured fields (JSON only)
func (l *Logger) WithFields(level string, message string, fields map[string]interface{}) {
	if !l.jsonFormat {
		// Fallback to plain format
		l.Info("%s: %v", message, fields)
		return
	}

	levelEnum := parseLogLevel(level)
	if levelEnum < l.level {
		return
	}

	l.logJSON(level, message, fields)
}
