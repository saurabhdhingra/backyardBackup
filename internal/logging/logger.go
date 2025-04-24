package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/yourusername/backyardBackup/config"
)

// Logger handles application logging
type Logger struct {
	level    config.LogLevel
	output   io.Writer
	file     *os.File
	mu       sync.Mutex
	filePath string
}

// NewLogger creates a new logger with the specified level
func NewLogger(level config.LogLevel, filePath string) (*Logger, error) {
	logger := &Logger{
		level:    level,
		output:   os.Stdout,
		filePath: filePath,
	}

	if filePath != "" {
		if err := logger.setupLogFile(); err != nil {
			return nil, err
		}
		logger.output = io.MultiWriter(os.Stdout, logger.file)
	}

	return logger, nil
}

// setupLogFile opens or creates the log file
func (l *Logger) setupLogFile() error {
	dir := filepath.Dir(l.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(l.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.file = file
	return nil
}

// Close closes the logger's file if it exists
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// SetLevel changes the log level
func (l *Logger) SetLevel(level config.LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// formatMessage formats a log message with timestamp and caller info
func (l *Logger) formatMessage(level string, message string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	
	_, file, line, ok := runtime.Caller(3)
	caller := "unknown"
	if ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	
	return fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, level, caller, message)
}

// log writes a message to the log
func (l *Logger) log(level string, levelEnum config.LogLevel, format string, args ...interface{}) {
	if shouldLog(l.level, levelEnum) {
		l.mu.Lock()
		defer l.mu.Unlock()
		
		message := fmt.Sprintf(format, args...)
		formattedMessage := l.formatMessage(level, message)
		fmt.Fprint(l.output, formattedMessage)
	}
}

// shouldLog determines if a message should be logged based on levels
func shouldLog(loggerLevel, messageLevel config.LogLevel) bool {
	levels := map[config.LogLevel]int{
		config.Debug:   0,
		config.Info:    1,
		config.Warning: 2,
		config.Error:   3,
	}
	
	return levels[messageLevel] >= levels[loggerLevel]
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log("DEBUG", config.Debug, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log("INFO", config.Info, format, args...)
}

// Warning logs a warning message
func (l *Logger) Warning(format string, args ...interface{}) {
	l.log("WARNING", config.Warning, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log("ERROR", config.Error, format, args...)
} 