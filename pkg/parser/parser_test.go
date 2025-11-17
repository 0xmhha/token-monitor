package parser

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		wantErr bool
		check   func(t *testing.T, entry *UsageEntry)
	}{
		{
			name:    "valid entry with all fields",
			line:    `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","version":"1.0.0","cwd":"/path/to/project","message":{"id":"msg_123","model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":10},"content":[{"type":"text","text":"response"}]},"costUSD":0.05,"requestId":"req_123"}`,
			wantErr: false,
			check: func(t *testing.T, entry *UsageEntry) {
				if entry.SessionID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
					t.Errorf("SessionID = %s, want a1b2c3d4-e5f6-7890-abcd-ef1234567890", entry.SessionID)
				}
				if entry.Message.Usage.InputTokens != 100 {
					t.Errorf("InputTokens = %d, want 100", entry.Message.Usage.InputTokens)
				}
				if entry.Message.Usage.OutputTokens != 50 {
					t.Errorf("OutputTokens = %d, want 50", entry.Message.Usage.OutputTokens)
				}
				if entry.Message.Usage.TotalTokens() != 180 {
					t.Errorf("TotalTokens = %d, want 180", entry.Message.Usage.TotalTokens())
				}
			},
		},
		{
			name:    "valid entry minimal fields",
			line:    `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"test-session","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":10,"output_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"content":[]}}`,
			wantErr: false,
			check: func(t *testing.T, entry *UsageEntry) {
				if entry.Message.Usage.TotalTokens() != 15 {
					t.Errorf("TotalTokens = %d, want 15", entry.Message.Usage.TotalTokens())
				}
			},
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "invalid json",
			line:    `{"invalid json`,
			wantErr: true,
		},
		{
			name:    "missing timestamp",
			line:    `{"sessionId":"test","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":10,"output_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"content":[]}}`,
			wantErr: true,
		},
		{
			name:    "missing session id",
			line:    `{"timestamp":"2024-01-15T10:30:00Z","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":10,"output_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"content":[]}}`,
			wantErr: true,
		},
		{
			name:    "missing model",
			line:    `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"test","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","usage":{"input_tokens":10,"output_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"content":[]}}`,
			wantErr: true,
		},
		{
			name:    "negative input tokens",
			line:    `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"test","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":-10,"output_tokens":5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"content":[]}}`,
			wantErr: true,
		},
		{
			name:    "negative output tokens",
			line:    `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"test","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":10,"output_tokens":-5,"cache_creation_input_tokens":0,"cache_read_input_tokens":0},"content":[]}}`,
			wantErr: true,
		},
	}

	p := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := p.ParseLine(tt.line)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseLine() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseLine() error = %v, wantErr = false", err)
				return
			}

			if entry == nil {
				t.Error("ParseLine() returned nil entry")
				return
			}

			if tt.check != nil {
				tt.check(t, entry)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		content    string
		offset     int64
		wantCount  int
		wantErr    bool
		checkCount bool
	}{
		{
			name: "valid file with multiple entries",
			content: `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"test1","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":10},"content":[]}}
{"timestamp":"2024-01-15T10:31:00Z","sessionId":"test1","version":"1.0.0","cwd":"/path","message":{"id":"msg_2","model":"claude-sonnet-4","usage":{"input_tokens":200,"output_tokens":100,"cache_creation_input_tokens":30,"cache_read_input_tokens":15},"content":[]}}
{"timestamp":"2024-01-15T10:32:00Z","sessionId":"test1","version":"1.0.0","cwd":"/path","message":{"id":"msg_3","model":"claude-sonnet-4","usage":{"input_tokens":150,"output_tokens":75,"cache_creation_input_tokens":25,"cache_read_input_tokens":12},"content":[]}}`,
			offset:     0,
			wantCount:  3,
			checkCount: true,
		},
		{
			name: "file with malformed line (should skip)",
			content: `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"test1","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":10},"content":[]}}
{"invalid json line
{"timestamp":"2024-01-15T10:32:00Z","sessionId":"test1","version":"1.0.0","cwd":"/path","message":{"id":"msg_3","model":"claude-sonnet-4","usage":{"input_tokens":150,"output_tokens":75,"cache_creation_input_tokens":25,"cache_read_input_tokens":12},"content":[]}}`,
			offset:     0,
			wantCount:  2,
			checkCount: true,
		},
		{
			name:       "empty file",
			content:    "",
			offset:     0,
			wantCount:  0,
			checkCount: true,
		},
		{
			name:    "non-existent file",
			content: "", // Will not create file
			offset:  0,
			wantErr: true,
		},
	}

	p := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string

			if tt.name != "non-existent file" {
				filePath = filepath.Join(tmpDir, tt.name+".jsonl")
				if err := os.WriteFile(filePath, []byte(tt.content), 0600); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			} else {
				filePath = filepath.Join(tmpDir, "nonexistent.jsonl")
			}

			entries, newOffset, err := p.ParseFile(filePath, tt.offset)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseFile() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseFile() error = %v, wantErr = false", err)
				return
			}

			if tt.checkCount && len(entries) != tt.wantCount {
				t.Errorf("ParseFile() got %d entries, want %d", len(entries), tt.wantCount)
			}

			if newOffset <= tt.offset && len(tt.content) > 0 {
				t.Errorf("ParseFile() newOffset = %d, should be > %d", newOffset, tt.offset)
			}
		})
	}
}

func TestUsageValidate(t *testing.T) {
	tests := []struct {
		name    string
		usage   Usage
		wantErr bool
	}{
		{
			name: "valid usage",
			usage: Usage{
				InputTokens:              100,
				OutputTokens:             50,
				CacheCreationInputTokens: 20,
				CacheReadInputTokens:     10,
			},
			wantErr: false,
		},
		{
			name: "zero values",
			usage: Usage{
				InputTokens:              0,
				OutputTokens:             0,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     0,
			},
			wantErr: false,
		},
		{
			name: "negative input tokens",
			usage: Usage{
				InputTokens:              -1,
				OutputTokens:             50,
				CacheCreationInputTokens: 20,
				CacheReadInputTokens:     10,
			},
			wantErr: true,
		},
		{
			name: "negative output tokens",
			usage: Usage{
				InputTokens:              100,
				OutputTokens:             -1,
				CacheCreationInputTokens: 20,
				CacheReadInputTokens:     10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.usage.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Usage.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUsageTotalTokens(t *testing.T) {
	tests := []struct {
		name  string
		usage Usage
		want  int
	}{
		{
			name: "all token types",
			usage: Usage{
				InputTokens:              100,
				OutputTokens:             50,
				CacheCreationInputTokens: 20,
				CacheReadInputTokens:     10,
			},
			want: 180,
		},
		{
			name: "only input tokens",
			usage: Usage{
				InputTokens: 100,
			},
			want: 100,
		},
		{
			name:  "zero tokens",
			usage: Usage{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.usage.TotalTokens()
			if got != tt.want {
				t.Errorf("Usage.TotalTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUsageEntryValidate(t *testing.T) {
	validTime := time.Now()
	zeroTime := time.Time{}

	tests := []struct {
		name    string
		entry   UsageEntry
		wantErr bool
	}{
		{
			name: "valid entry",
			entry: UsageEntry{
				Timestamp: validTime,
				SessionID: "test-session",
				Message: Message{
					Model: "claude-sonnet-4",
					Usage: Usage{
						InputTokens:  100,
						OutputTokens: 50,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "zero timestamp",
			entry: UsageEntry{
				Timestamp: zeroTime,
				SessionID: "test-session",
				Message: Message{
					Model: "claude-sonnet-4",
					Usage: Usage{
						InputTokens:  100,
						OutputTokens: 50,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty session id",
			entry: UsageEntry{
				Timestamp: validTime,
				SessionID: "",
				Message: Message{
					Model: "claude-sonnet-4",
					Usage: Usage{
						InputTokens:  100,
						OutputTokens: 50,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty model",
			entry: UsageEntry{
				Timestamp: validTime,
				SessionID: "test-session",
				Message: Message{
					Model: "",
					Usage: Usage{
						InputTokens:  100,
						OutputTokens: 50,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid usage",
			entry: UsageEntry{
				Timestamp: validTime,
				SessionID: "test-session",
				Message: Message{
					Model: "claude-sonnet-4",
					Usage: Usage{
						InputTokens:  -1,
						OutputTokens: 50,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("UsageEntry.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Benchmark tests for performance validation.

func BenchmarkParseLine(b *testing.B) {
	line := `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567890","version":"1.0.0","cwd":"/path/to/project","message":{"id":"msg_123","model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":10},"content":[{"type":"text","text":"response"}]},"costUSD":0.05,"requestId":"req_123"}`
	p := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.ParseLine(line)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFile(b *testing.B) {
	// Create test file with 1000 entries
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "benchmark.jsonl")

	content := ""
	for i := 0; i < 1000; i++ {
		content += `{"timestamp":"2024-01-15T10:30:00Z","sessionId":"test","version":"1.0.0","cwd":"/path","message":{"id":"msg_1","model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":10},"content":[]}}` + "\n"
	}

	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		b.Fatal(err)
	}

	p := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := p.ParseFile(filePath, 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}
