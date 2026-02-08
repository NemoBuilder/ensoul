package util

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// Log levels
const (
	LevelDebug = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger is a simple leveled logger built on top of the standard log package.
type Logger struct {
	level  int
	prefix string
	logger *log.Logger
}

// Global logger instance
var Log *Logger

// InitLogger creates the global logger.
// level: "debug", "info", "warn", "error"
func InitLogger(level string) {
	l := &Logger{
		level:  parseLevel(level),
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
	Log = l
}

// WithPrefix returns a child logger with a tag prefix, e.g. "[chain]".
func (l *Logger) WithPrefix(prefix string) *Logger {
	return &Logger{
		level:  l.level,
		prefix: prefix,
		logger: l.logger,
	}
}

// Debug logs at DEBUG level — suppressed in production.
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.output("DEBUG", format, args...)
	}
}

// Info logs at INFO level — important operational messages.
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.output("INFO", format, args...)
	}
}

// Warn logs at WARN level — non-critical issues.
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.output("WARN", format, args...)
	}
}

// Error logs at ERROR level — always shown.
func (l *Logger) Error(format string, args ...interface{}) {
	l.output("ERROR", format, args...)
}

// Fatal logs at ERROR level and exits the process.
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.output("FATAL", format, args...)
	os.Exit(1)
}

func (l *Logger) output(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if l.prefix != "" {
		l.logger.Printf("[%s] %s %s", level, l.prefix, msg)
	} else {
		l.logger.Printf("[%s] %s", level, msg)
	}
}

func parseLevel(s string) int {
	switch strings.ToLower(s) {
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
