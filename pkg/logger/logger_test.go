package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		check  func(t *testing.T, log Logger)
	}{
		{
			name: "default config",
			config: Config{
				Level:  "info",
				Output: "stderr",
				Format: "text",
			},
			check: func(t *testing.T, log Logger) {
				if log == nil {
					t.Error("New() returned nil")
				}
			},
		},
		{
			name: "debug level",
			config: Config{
				Level:  "debug",
				Output: "stderr",
				Format: "text",
			},
			check: func(t *testing.T, log Logger) {
				if log == nil {
					t.Error("New() returned nil")
				}
			},
		},
		{
			name: "json format",
			config: Config{
				Level:  "info",
				Output: "stderr",
				Format: "json",
			},
			check: func(t *testing.T, log Logger) {
				if log == nil {
					t.Error("New() returned nil")
				}
			},
		},
		{
			name: "stdout output",
			config: Config{
				Level:  "info",
				Output: "stdout",
				Format: "text",
			},
			check: func(t *testing.T, log Logger) {
				if log == nil {
					t.Error("New() returned nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := New(tt.config)
			if tt.check != nil {
				tt.check(t, log)
			}
		})
	}
}

func TestLogLevels(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	log := New(Config{
		Level:  "debug",
		Output: logFile,
		Format: "text",
	})

	// Log messages at different levels
	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	// Read log file
	data, err := os.ReadFile(logFile) // nolint:gosec
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	content := string(data)

	// Verify all messages are present
	if !strings.Contains(content, "debug message") {
		t.Error("Debug message not found in log")
	}
	if !strings.Contains(content, "info message") {
		t.Error("Info message not found in log")
	}
	if !strings.Contains(content, "warn message") {
		t.Error("Warn message not found in log")
	}
	if !strings.Contains(content, "error message") {
		t.Error("Error message not found in log")
	}
}

func TestLogWithFields(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	log := New(Config{
		Level:  "info",
		Output: logFile,
		Format: "text",
	})

	// Log with fields
	log.Info("test message", "key1", "value1", "key2", 42)

	// Read log file
	data, err := os.ReadFile(logFile) // nolint:gosec
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	content := string(data)

	// Verify message and fields are present
	if !strings.Contains(content, "test message") {
		t.Error("Message not found in log")
	}
	if !strings.Contains(content, "key1") || !strings.Contains(content, "value1") {
		t.Error("Field key1=value1 not found in log")
	}
	if !strings.Contains(content, "key2") {
		t.Error("Field key2 not found in log")
	}
}

func TestLogWith(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	baseLog := New(Config{
		Level:  "info",
		Output: logFile,
		Format: "text",
	})

	// Create logger with context
	contextLog := baseLog.With("component", "test", "version", "1.0")

	// Log message
	contextLog.Info("message with context")

	// Read log file
	data, err := os.ReadFile(logFile) // nolint:gosec
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	content := string(data)

	// Verify context fields are present
	if !strings.Contains(content, "component") {
		t.Error("Context field 'component' not found")
	}
	if !strings.Contains(content, "test") {
		t.Error("Context value 'test' not found")
	}
	if !strings.Contains(content, "version") {
		t.Error("Context field 'version' not found")
	}
}

func TestJSONFormat(t *testing.T) {
	// Test the format selection logic
	log := New(Config{
		Level:  "info",
		Output: "stderr",
		Format: "json",
	})

	// Just verify logger was created
	if log == nil {
		t.Error("Failed to create JSON logger")
	}
}

func TestLogLevelFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Create logger with warn level
	log := New(Config{
		Level:  "warn",
		Output: logFile,
		Format: "text",
	})

	// Log at different levels
	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	// Read log file
	data, err := os.ReadFile(logFile) // nolint:gosec
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	content := string(data)

	// Debug and Info should be filtered out
	if strings.Contains(content, "debug message") {
		t.Error("Debug message should be filtered out")
	}
	if strings.Contains(content, "info message") {
		t.Error("Info message should be filtered out")
	}

	// Warn and Error should be present
	if !strings.Contains(content, "warn message") {
		t.Error("Warn message not found")
	}
	if !strings.Contains(content, "error message") {
		t.Error("Error message not found")
	}
}

func TestDefault(t *testing.T) {
	log := Default()
	if log == nil {
		t.Error("Default() returned nil")
	}

	// Should be able to log without panic
	log.Info("test message")
}

func TestNoop(t *testing.T) {
	log := Noop()
	if log == nil {
		t.Error("Noop() returned nil")
	}

	// Should discard all messages without error
	log.Debug("debug")
	log.Info("info")
	log.Warn("warn")
	log.Error("error")
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  string // We'll check the string representation
	}{
		{"debug", "debug", "DEBUG"},
		{"info", "info", "INFO"},
		{"warn", "warn", "WARN"},
		{"warning", "warning", "WARN"},
		{"error", "error", "ERROR"},
		{"unknown", "unknown", "INFO"}, // defaults to info
		{"empty", "", "INFO"},          // defaults to info
		{"uppercase", "DEBUG", "DEBUG"},
		{"mixedcase", "WaRn", "WARN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := parseLevel(tt.level)
			// slog.Level.String() returns uppercase level name
			if level.String() != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.level, level, tt.want)
			}
		})
	}
}

func TestGetWriter(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{"stdout", "stdout", false},
		{"stderr", "stderr", false},
		{"empty defaults to stderr", "", false},
		{"STDOUT uppercase", "STDOUT", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := getWriter(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Error("getWriter() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("getWriter() error = %v, wantErr = false", err)
				return
			}

			if writer == nil {
				t.Error("getWriter() returned nil writer")
			}
		})
	}
}

func TestFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	log := New(Config{
		Level:  "info",
		Output: logFile,
		Format: "text",
	})

	// Log some messages
	log.Info("message 1")
	log.Info("message 2")
	log.Error("error message")

	// Verify file was created
	if _, err := os.Stat(logFile); err != nil {
		t.Errorf("Log file not created: %v", err)
	}

	// Read and verify content
	data, err := os.ReadFile(logFile) // nolint:gosec
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "message 1") {
		t.Error("First message not found")
	}
	if !strings.Contains(content, "message 2") {
		t.Error("Second message not found")
	}
	if !strings.Contains(content, "error message") {
		t.Error("Error message not found")
	}
}

func TestJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.json")

	log := New(Config{
		Level:  "info",
		Output: logFile,
		Format: "json",
	})

	// Log a message with fields
	log.Info("test event", "key", "value", "count", 42)

	// Read file
	data, err := os.ReadFile(logFile) // nolint:gosec
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Parse JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal(data, &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON log: %v", err)
	}

	// Verify JSON structure
	if msg, ok := logEntry["msg"].(string); !ok || msg != "test event" {
		t.Error("Message not found in JSON log")
	}

	if key, ok := logEntry["key"].(string); !ok || key != "value" {
		t.Error("Field 'key' not found or incorrect in JSON log")
	}

	if count, ok := logEntry["count"].(float64); !ok || count != 42 {
		t.Error("Field 'count' not found or incorrect in JSON log")
	}
}

// Benchmark logger performance.
func BenchmarkLogInfo(b *testing.B) {
	log := Noop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark message")
	}
}

func BenchmarkLogWithFields(b *testing.B) {
	log := Noop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark message", "key1", "value1", "key2", 42, "key3", true)
	}
}

func BenchmarkLogWith(b *testing.B) {
	log := Noop().With("component", "test", "version", "1.0")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("benchmark message")
	}
}
