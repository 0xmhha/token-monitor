// Package logger provides structured logging functionality for token-monitor.
//
// The logger supports multiple output formats (text, JSON), configurable log levels,
// and context-aware logging with fields.
//
// Example usage:
//
//	log := logger.New(logger.Config{
//	    Level:  "info",
//	    Output: "stderr",
//	    Format: "text",
//	})
//	log.Info("starting application", "version", "1.0.0")
//	log.Error("operation failed", "error", err, "path", "/data")
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger provides structured logging with levels and fields.
type Logger interface {
	// Debug logs a debug message with optional key-value pairs.
	Debug(msg string, keysAndValues ...interface{})

	// Info logs an informational message with optional key-value pairs.
	Info(msg string, keysAndValues ...interface{})

	// Warn logs a warning message with optional key-value pairs.
	Warn(msg string, keysAndValues ...interface{})

	// Error logs an error message with optional key-value pairs.
	Error(msg string, keysAndValues ...interface{})

	// With returns a new logger with additional context fields.
	With(keysAndValues ...interface{}) Logger
}

// Config contains logger configuration.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string

	// Output is the destination (stdout, stderr, or file path).
	Output string

	// Format is the output format (text, json).
	Format string
}

// logger implements the Logger interface using slog.
type logger struct {
	slogger *slog.Logger
}

// New creates a new logger with the given configuration.
//
// Parameters:
//   - cfg: Logger configuration
//
// Returns a configured logger instance.
//
// If configuration is invalid, returns a logger with default settings
// (info level, stderr, text format).
func New(cfg Config) Logger {
	// Parse log level
	level := parseLevel(cfg.Level)

	// Get output writer
	writer, err := getWriter(cfg.Output)
	if err != nil {
		// Fallback to stderr
		writer = os.Stderr
	}

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	default: // "text" or anything else
		handler = slog.NewTextHandler(writer, opts)
	}

	return &logger{
		slogger: slog.New(handler),
	}
}

// Debug implements Logger.Debug.
func (l *logger) Debug(msg string, keysAndValues ...interface{}) {
	l.slogger.Debug(msg, keysAndValues...)
}

// Info implements Logger.Info.
func (l *logger) Info(msg string, keysAndValues ...interface{}) {
	l.slogger.Info(msg, keysAndValues...)
}

// Warn implements Logger.Warn.
func (l *logger) Warn(msg string, keysAndValues ...interface{}) {
	l.slogger.Warn(msg, keysAndValues...)
}

// Error implements Logger.Error.
func (l *logger) Error(msg string, keysAndValues ...interface{}) {
	l.slogger.Error(msg, keysAndValues...)
}

// With implements Logger.With.
func (l *logger) With(keysAndValues ...interface{}) Logger {
	return &logger{
		slogger: l.slogger.With(keysAndValues...),
	}
}

// parseLevel converts a string log level to slog.Level.
//
// Supported levels: debug, info, warn, error.
// Defaults to info for unrecognized levels.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// getWriter returns an io.Writer for the given output destination.
//
// Supported destinations:
//   - "stdout": Standard output
//   - "stderr": Standard error (default)
//   - file path: Opens file for appending (creates if not exists)
//
// Returns error if file cannot be opened.
func getWriter(output string) (io.Writer, error) {
	switch strings.ToLower(output) {
	case "stdout":
		return os.Stdout, nil
	case "stderr", "":
		return os.Stderr, nil
	default:
		// Treat as file path
		// #nosec G304: output path comes from trusted config
		f, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // nolint:gosec
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", output, err)
		}
		return f, nil
	}
}

// Default returns a logger with default configuration.
//
// Default settings:
//   - Level: info
//   - Output: stderr
//   - Format: text
func Default() Logger {
	return New(Config{
		Level:  "info",
		Output: "stderr",
		Format: "text",
	})
}

// Noop returns a logger that discards all log messages.
//
// Useful for testing or when logging should be disabled.
func Noop() Logger {
	return &logger{
		slogger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}
