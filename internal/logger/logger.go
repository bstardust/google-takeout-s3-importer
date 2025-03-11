// internal/logger/logger.go
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// Log levels
const (
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	level    = LevelInfo
	mu       sync.Mutex
	debugLog = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags)
	infoLog  = log.New(os.Stdout, "[INFO] ", log.LstdFlags)
	warnLog  = log.New(os.Stdout, "[WARN] ", log.LstdFlags)
	errorLog = log.New(os.Stderr, "[ERROR] ", log.LstdFlags)
)

// Init initializes the logger
func Init() {
	// Default initialization
}

// SetOutput sets the output for all loggers
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()

	debugLog.SetOutput(w)
	infoLog.SetOutput(w)
	warnLog.SetOutput(w)
	errorLog.SetOutput(w)
}

// SetLevel sets the log level
func SetLevel(levelStr string) {
	mu.Lock()
	defer mu.Unlock()

	switch strings.ToLower(levelStr) {
	case "debug":
		level = LevelDebug
	case "info":
		level = LevelInfo
	case "warn", "warning":
		level = LevelWarn
	case "error":
		level = LevelError
	default:
		level = LevelInfo
	}
}

// Debug logs a debug message
func Debug(format string, v ...interface{}) {
	if level <= LevelDebug {
		debugLog.Output(2, fmt.Sprintf(format, v...))
	}
}

// Info logs an info message
func Info(format string, v ...interface{}) {
	if level <= LevelInfo {
		infoLog.Output(2, fmt.Sprintf(format, v...))
	}
}

// Warn logs a warning message
func Warn(format string, v ...interface{}) {
	if level <= LevelWarn {
		warnLog.Output(2, fmt.Sprintf(format, v...))
	}
}

// Error logs an error message
func Error(format string, v ...interface{}) {
	if level <= LevelError {
		errorLog.Output(2, fmt.Sprintf(format, v...))
	}
}
