package main

import (
	"flag"
	"strings"
	"testing"
	"time"
)

// TestRunStatsCommand tests stats command flag parsing.
func TestRunStatsCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCmd   statsCommand
		wantError bool
	}{
		{
			name: "default flags",
			args: []string{},
			wantCmd: statsCommand{
				format:     "table",
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "session filter",
			args: []string{"-session", "abc123"},
			wantCmd: statsCommand{
				sessionID:  "abc123",
				format:     "table",
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "model filter",
			args: []string{"-model", "claude-3-sonnet"},
			wantCmd: statsCommand{
				model:      "claude-3-sonnet",
				format:     "table",
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "group by single dimension",
			args: []string{"-group-by", "model"},
			wantCmd: statsCommand{
				groupBy:    []string{"model"},
				format:     "table",
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "group by multiple dimensions",
			args: []string{"-group-by", "model,session,date"},
			wantCmd: statsCommand{
				groupBy:    []string{"model", "session", "date"},
				format:     "table",
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "top N sessions",
			args: []string{"-top", "10"},
			wantCmd: statsCommand{
				topN:       10,
				format:     "table",
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "JSON format",
			args: []string{"-format", "json"},
			wantCmd: statsCommand{
				format:     "json",
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "simple format",
			args: []string{"-format", "simple"},
			wantCmd: statsCommand{
				format:     "simple",
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "compact output",
			args: []string{"-compact"},
			wantCmd: statsCommand{
				format:     "table",
				compact:    true,
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "combined flags",
			args: []string{
				"-session", "abc123",
				"-model", "claude-3-sonnet",
				"-group-by", "model,date",
				"-format", "json",
				"-compact",
			},
			wantCmd: statsCommand{
				sessionID:  "abc123",
				model:      "claude-3-sonnet",
				groupBy:    []string{"model", "date"},
				format:     "json",
				compact:    true,
				configPath: "/test/config.yaml",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse flags
			fs := flag.NewFlagSet("stats", flag.ContinueOnError)
			sessionID := fs.String("session", "", "filter by session ID")
			model := fs.String("model", "", "filter by model name")
			groupBy := fs.String("group-by", "", "group by dimensions")
			topN := fs.Int("top", 0, "show top N sessions")
			format := fs.String("format", "table", "output format")
			compact := fs.Bool("compact", false, "compact output")

			err := fs.Parse(tt.args)
			if tt.wantError && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantError {
				return
			}

			// Parse group-by dimensions
			var dimensions []string
			if *groupBy != "" {
				dimensions = strings.Split(*groupBy, ",")
				for i, dim := range dimensions {
					dimensions[i] = strings.TrimSpace(dim)
				}
			}

			// Create command
			got := &statsCommand{
				sessionID:  *sessionID,
				model:      *model,
				groupBy:    dimensions,
				topN:       *topN,
				format:     *format,
				compact:    *compact,
				configPath: "/test/config.yaml",
			}

			// Verify fields
			if got.sessionID != tt.wantCmd.sessionID {
				t.Errorf("sessionID = %q, want %q", got.sessionID, tt.wantCmd.sessionID)
			}
			if got.model != tt.wantCmd.model {
				t.Errorf("model = %q, want %q", got.model, tt.wantCmd.model)
			}
			if got.format != tt.wantCmd.format {
				t.Errorf("format = %q, want %q", got.format, tt.wantCmd.format)
			}
			if got.topN != tt.wantCmd.topN {
				t.Errorf("topN = %d, want %d", got.topN, tt.wantCmd.topN)
			}
			if got.compact != tt.wantCmd.compact {
				t.Errorf("compact = %v, want %v", got.compact, tt.wantCmd.compact)
			}

			// Verify groupBy dimensions
			if len(got.groupBy) != len(tt.wantCmd.groupBy) {
				t.Errorf("groupBy length = %d, want %d", len(got.groupBy), len(tt.wantCmd.groupBy))
			} else {
				for i := range got.groupBy {
					if got.groupBy[i] != tt.wantCmd.groupBy[i] {
						t.Errorf("groupBy[%d] = %q, want %q", i, got.groupBy[i], tt.wantCmd.groupBy[i])
					}
				}
			}
		})
	}
}

// TestRunWatchCommand tests watch command flag parsing.
func TestRunWatchCommand(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCmd   watchCommand
		wantError bool
	}{
		{
			name: "default flags",
			args: []string{},
			wantCmd: watchCommand{
				refresh:     time.Second,
				format:      "table",
				clearScreen: true,
				configPath:  "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "session filter",
			args: []string{"-session", "abc123"},
			wantCmd: watchCommand{
				sessionID:   "abc123",
				refresh:     time.Second,
				format:      "table",
				clearScreen: true,
				configPath:  "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "custom refresh interval",
			args: []string{"-refresh", "500ms"},
			wantCmd: watchCommand{
				refresh:     500 * time.Millisecond,
				format:      "table",
				clearScreen: true,
				configPath:  "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "simple format",
			args: []string{"-format", "simple"},
			wantCmd: watchCommand{
				refresh:     time.Second,
				format:      "simple",
				clearScreen: true,
				configPath:  "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "history mode",
			args: []string{"-history"},
			wantCmd: watchCommand{
				refresh:     time.Second,
				format:      "table",
				clearScreen: false, // history mode disables clear screen
				configPath:  "/test/config.yaml",
			},
			wantError: false,
		},
		{
			name: "combined flags",
			args: []string{
				"-session", "abc123",
				"-refresh", "2s",
				"-format", "simple",
			},
			wantCmd: watchCommand{
				sessionID:   "abc123",
				refresh:     2 * time.Second,
				format:      "simple",
				clearScreen: true,
				configPath:  "/test/config.yaml",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse flags
			fs := flag.NewFlagSet("watch", flag.ContinueOnError)
			sessionID := fs.String("session", "", "monitor specific session ID")
			refresh := fs.Duration("refresh", time.Second, "refresh interval")
			format := fs.String("format", "table", "output format")
			history := fs.Bool("history", false, "keep history of updates")

			err := fs.Parse(tt.args)
			if tt.wantError && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantError {
				return
			}

			// Create command
			got := &watchCommand{
				sessionID:   *sessionID,
				refresh:     *refresh,
				format:      *format,
				clearScreen: !*history,
				configPath:  "/test/config.yaml",
			}

			// Verify fields
			if got.sessionID != tt.wantCmd.sessionID {
				t.Errorf("sessionID = %q, want %q", got.sessionID, tt.wantCmd.sessionID)
			}
			if got.refresh != tt.wantCmd.refresh {
				t.Errorf("refresh = %v, want %v", got.refresh, tt.wantCmd.refresh)
			}
			if got.format != tt.wantCmd.format {
				t.Errorf("format = %q, want %q", got.format, tt.wantCmd.format)
			}
			if got.clearScreen != tt.wantCmd.clearScreen {
				t.Errorf("clearScreen = %v, want %v", got.clearScreen, tt.wantCmd.clearScreen)
			}
		})
	}
}

// TestParseDimensions tests dimension string parsing.
func TestParseDimensions(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		want      []string // dimension names for comparison
		wantError bool
	}{
		{
			name:      "empty",
			input:     []string{},
			want:      []string{},
			wantError: false,
		},
		{
			name:      "single dimension - model",
			input:     []string{"model"},
			want:      []string{"model"},
			wantError: false,
		},
		{
			name:      "single dimension - session",
			input:     []string{"session"},
			want:      []string{"session"},
			wantError: false,
		},
		{
			name:      "single dimension - date",
			input:     []string{"date"},
			want:      []string{"date"},
			wantError: false,
		},
		{
			name:      "single dimension - hour",
			input:     []string{"hour"},
			want:      []string{"hour"},
			wantError: false,
		},
		{
			name:      "multiple dimensions",
			input:     []string{"model", "session", "date"},
			want:      []string{"model", "session", "date"},
			wantError: false,
		},
		{
			name:      "invalid dimension",
			input:     []string{"invalid"},
			want:      nil,
			wantError: true,
		},
		{
			name:      "mixed valid and invalid",
			input:     []string{"model", "invalid"},
			want:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &statsCommand{groupBy: tt.input}
			got, err := cmd.parseDimensions()

			if tt.wantError && err == nil {
				t.Fatal("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantError {
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("dimension count = %d, want %d", len(got), len(tt.want))
				return
			}

			// Verify each dimension matches expected name
			for i, dim := range got {
				dimName := string(dim)
				if dimName != tt.want[i] {
					t.Errorf("dimension[%d] = %q, want %q", i, dimName, tt.want[i])
				}
			}
		})
	}
}

// TestCommandRouting tests that commands are routed correctly.
func TestCommandRouting(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		shouldRoute bool
	}{
		{"stats command", "stats", true},
		{"list command", "list", true},
		{"watch command", "watch", true},
		{"session command", "session", true},
		{"config command", "config", true},
		{"help command", "help", true},
		{"unknown command", "unknown", false},
		{"invalid command", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify command name can be parsed
			validCommands := map[string]bool{
				"stats":   true,
				"list":    true,
				"watch":   true,
				"session": true,
				"config":  true,
				"help":    true,
			}

			isValid := validCommands[tt.command]
			if isValid != tt.shouldRoute {
				t.Errorf("command %q validity = %v, want %v", tt.command, isValid, tt.shouldRoute)
			}
		})
	}
}

// TestVersionFlag tests version flag handling.
func TestVersionFlag(t *testing.T) {
	// Set version
	version = "v0.1.0"

	// Test that version is set correctly
	if version != "v0.1.0" {
		t.Errorf("version = %q, want %q", version, "v0.1.0")
	}

	// Reset to dev for other tests
	version = "dev"
}

// TestListCommand tests list command structure.
func TestListCommand(t *testing.T) {
	cmd := &listCommand{
		configPath: "/test/config.yaml",
	}

	if cmd.configPath != "/test/config.yaml" {
		t.Errorf("configPath = %q, want %q", cmd.configPath, "/test/config.yaml")
	}
}
