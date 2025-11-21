package display

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config Config
		want   string // Type name
	}{
		{
			name:   "default format (table)",
			config: Config{},
			want:   "*display.tableFormatter",
		},
		{
			name:   "table format",
			config: Config{Format: FormatTable},
			want:   "*display.tableFormatter",
		},
		{
			name:   "json format",
			config: Config{Format: FormatJSON},
			want:   "*display.jsonFormatter",
		},
		{
			name:   "simple format",
			config: Config{Format: FormatSimple},
			want:   "*display.simpleFormatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			formatter := New(tt.config)
			if formatter == nil {
				t.Fatal("New() returned nil")
			}

			got := fmt.Sprintf("%T", formatter)
			if got != tt.want {
				t.Errorf("New() type = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTableFormatter_FormatStats(t *testing.T) {
	t.Parallel()

	formatter := New(Config{
		Format:          FormatTable,
		ShowPercentiles: true,
		ShowTimestamps:  true,
	})

	stats := aggregator.Statistics{
		Count:        100,
		SessionCount: 5,
		TotalTokens:  15000,
		InputTokens:  10000,
		OutputTokens: 5000,
		AvgTokens:    150.0,
		MinTokens:    50,
		MaxTokens:    500,
		P50Tokens:    140,
		P95Tokens:    450,
		P99Tokens:    490,
		FirstSeen:    time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		LastSeen:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	var buf bytes.Buffer
	if err := formatter.FormatStats(&buf, stats); err != nil {
		t.Fatalf("FormatStats() error = %v", err)
	}

	output := buf.String()

	// Check for key values.
	if !strings.Contains(output, "100") {
		t.Error("Output missing entry count")
	}
	if !strings.Contains(output, "15,000") {
		t.Error("Output missing total tokens")
	}
	if !strings.Contains(output, "150.00") {
		t.Error("Output missing average")
	}
	if !strings.Contains(output, "P50") {
		t.Error("Output missing percentiles")
	}
	if !strings.Contains(output, "2024-01-01") {
		t.Error("Output missing timestamps")
	}
}

func TestTableFormatter_FormatGroupedStats(t *testing.T) {
	t.Parallel()

	formatter := New(Config{Format: FormatTable})

	grouped := map[string]aggregator.Statistics{
		"model-1": {
			Count:        50,
			TotalTokens:  7500,
			InputTokens:  5000,
			OutputTokens: 2500,
			AvgTokens:    150.0,
			MinTokens:    50,
			MaxTokens:    300,
		},
		"model-2": {
			Count:        30,
			TotalTokens:  4500,
			InputTokens:  3000,
			OutputTokens: 1500,
			AvgTokens:    150.0,
			MinTokens:    50,
			MaxTokens:    250,
		},
	}

	var buf bytes.Buffer
	if err := formatter.FormatGroupedStats(&buf, grouped, []string{"Model"}); err != nil {
		t.Fatalf("FormatGroupedStats() error = %v", err)
	}

	output := buf.String()

	// Check for model names.
	if !strings.Contains(output, "model-1") {
		t.Error("Output missing model-1")
	}
	if !strings.Contains(output, "model-2") {
		t.Error("Output missing model-2")
	}

	// Check for statistics.
	if !strings.Contains(output, "7,500") {
		t.Error("Output missing model-1 total tokens")
	}
}

func TestTableFormatter_FormatTopSessions(t *testing.T) {
	t.Parallel()

	formatter := New(Config{Format: FormatTable})

	sessions := []aggregator.SessionStats{
		{
			SessionID: "session-1",
			Model:     "claude-3-5-sonnet-20241022",
			Statistics: aggregator.Statistics{
				Count:        100,
				TotalTokens:  15000,
				InputTokens:  10000,
				OutputTokens: 5000,
				AvgTokens:    150.0,
			},
		},
		{
			SessionID: "session-2",
			Model:     "claude-3-opus-20240229",
			Statistics: aggregator.Statistics{
				Count:        50,
				TotalTokens:  7500,
				InputTokens:  5000,
				OutputTokens: 2500,
				AvgTokens:    150.0,
			},
		},
	}

	var buf bytes.Buffer
	if err := formatter.FormatTopSessions(&buf, sessions); err != nil {
		t.Fatalf("FormatTopSessions() error = %v", err)
	}

	output := buf.String()

	// Check for session IDs and rankings.
	if !strings.Contains(output, "#1") {
		t.Error("Output missing rank #1")
	}
	if !strings.Contains(output, "#2") {
		t.Error("Output missing rank #2")
	}
	if !strings.Contains(output, "session-1") {
		t.Error("Output missing session-1")
	}
	if !strings.Contains(output, "15,000") {
		t.Error("Output missing session-1 total tokens")
	}
}

func TestJSONFormatter_FormatStats(t *testing.T) {
	t.Parallel()

	formatter := New(Config{Format: FormatJSON})

	stats := aggregator.Statistics{
		Count:        100,
		TotalTokens:  15000,
		InputTokens:  10000,
		OutputTokens: 5000,
		AvgTokens:    150.0,
		MinTokens:    50,
		MaxTokens:    500,
	}

	var buf bytes.Buffer
	if err := formatter.FormatStats(&buf, stats); err != nil {
		t.Fatalf("FormatStats() error = %v", err)
	}

	output := buf.String()

	// Check for JSON structure.
	if !strings.Contains(output, "\"Count\"") {
		t.Error("JSON output missing Count field")
	}
	if !strings.Contains(output, "15000") {
		t.Error("JSON output missing TotalTokens value")
	}
}

func TestSimpleFormatter_FormatStats(t *testing.T) {
	t.Parallel()

	formatter := New(Config{Format: FormatSimple})

	stats := aggregator.Statistics{
		Count:        100,
		SessionCount: 5,
		TotalTokens:  15000,
		AvgTokens:    150.0,
		MinTokens:    50,
		MaxTokens:    500,
	}

	var buf bytes.Buffer
	if err := formatter.FormatStats(&buf, stats); err != nil {
		t.Fatalf("FormatStats() error = %v", err)
	}

	output := buf.String()

	// Check for compact format.
	if !strings.Contains(output, "Entries: 100") {
		t.Error("Simple output missing entry count")
	}
	if !strings.Contains(output, "Total: 15,000") {
		t.Error("Simple output missing total tokens")
	}
}

func TestFormatNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    int
		want string
	}{
		{"zero", 0, "0"},
		{"small", 123, "123"},
		{"thousand", 1000, "1,000"},
		{"ten thousand", 12345, "12,345"},
		{"million", 1234567, "1,234,567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatNumber(tt.n)
			if got != tt.want {
				t.Errorf("formatNumber(%d) = %v, want %v", tt.n, got, tt.want)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		f         float64
		precision int
		want      string
	}{
		{"zero", 0.0, 2, "0.00"},
		{"integer", 123.0, 2, "123.00"},
		{"decimal", 123.456, 2, "123.46"},
		{"one digit", 123.456, 1, "123.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatFloat(tt.f, tt.precision)
			if got != tt.want {
				t.Errorf("formatFloat(%f, %d) = %v, want %v", tt.f, tt.precision, got, tt.want)
			}
		})
	}
}

func TestCompactMode(t *testing.T) {
	t.Parallel()

	stats := aggregator.Statistics{
		Count:        10,
		SessionCount: 1,
		TotalTokens:  1500,
		InputTokens:  1000,
		OutputTokens: 500,
		AvgTokens:    150.0,
		MinTokens:    100,
		MaxTokens:    200,
	}

	// Non-compact.
	formatter1 := New(Config{Format: FormatTable, Compact: false})
	var buf1 bytes.Buffer
	if err := formatter1.FormatStats(&buf1, stats); err != nil {
		t.Fatalf("FormatStats() error = %v", err)
	}

	// Compact.
	formatter2 := New(Config{Format: FormatTable, Compact: true})
	var buf2 bytes.Buffer
	if err := formatter2.FormatStats(&buf2, stats); err != nil {
		t.Fatalf("FormatStats() error = %v", err)
	}

	// Compact output should be shorter.
	if len(buf2.String()) >= len(buf1.String()) {
		t.Error("Compact mode did not reduce output length")
	}
}

func TestEmptyData(t *testing.T) {
	t.Parallel()

	formatter := New(Config{Format: FormatTable})

	// Empty grouped stats.
	var buf bytes.Buffer
	if err := formatter.FormatGroupedStats(&buf, map[string]aggregator.Statistics{}, []string{"Model"}); err != nil {
		t.Fatalf("FormatGroupedStats() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No data") {
		t.Error("Empty grouped stats should show 'No data'")
	}

	// Empty top sessions.
	buf.Reset()
	if err := formatter.FormatTopSessions(&buf, []aggregator.SessionStats{}); err != nil {
		t.Fatalf("FormatTopSessions() error = %v", err)
	}

	output = buf.String()
	if !strings.Contains(output, "No data") {
		t.Error("Empty top sessions should show 'No data'")
	}
}
