	package logger

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

// InitLogger initializes the global logger
func InitLogger() {
	Log = logrus.New()
	Log.SetOutput(os.Stdout)
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Set log level based on environment
	env := os.Getenv("ENV")
	if env == "production" {
		Log.SetLevel(logrus.InfoLevel)
	} else {
		Log.SetLevel(logrus.DebugLevel)
	}
}

// SetOutput allows setting a custom output writer (useful for testing)
func SetOutput(w io.Writer) {
	if Log != nil {
		Log.SetOutput(w)
	}
}

// Info logs an info message
func Info(args ...interface{}) {
	if Log != nil {
		Log.Info(args...)
	}
}

// Error logs an error message
func Error(args ...interface{}) {
	if Log != nil {
		Log.Error(args...)
	}
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	if Log != nil {
		Log.Debug(args...)
	}
}

// WithFields returns a logger with fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	if Log != nil {
		return Log.WithFields(fields)
	}
	return nil
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	if Log != nil {
		Log.Warn(args...)
	}
}

// SecurityEvent logs security-related events
func SecurityEvent(eventType, email, ipAddress, details string) {
	if Log != nil {
		Log.WithFields(logrus.Fields{
			"event_type": eventType,
			"email":      email,
			"ip_address": ipAddress,
			"details":    details,
			"category":   "security",
		}).Warn("Security event detected")
	}
}
