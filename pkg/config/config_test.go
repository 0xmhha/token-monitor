package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Verify defaults are set
	if len(cfg.ClaudeConfigDirs) == 0 {
		t.Error("ClaudeConfigDirs is empty")
	}

	if cfg.Monitoring.WatchInterval <= 0 {
		t.Error("WatchInterval not set")
	}

	if cfg.Performance.WorkerPoolSize <= 0 {
		t.Error("WorkerPoolSize not set")
	}

	if cfg.Display.DefaultMode == "" {
		t.Error("DefaultMode not set")
	}

	if cfg.Logging.Level == "" {
		t.Error("Log level not set")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			config:  Default(),
			wantErr: false,
		},
		{
			name: "no claude directories",
			config: &Config{
				ClaudeConfigDirs: []string{},
				Monitoring: MonitoringConfig{
					WatchInterval:    1 * time.Second,
					UpdateFrequency:  1 * time.Second,
					SessionRetention: 1 * time.Hour,
				},
				Performance: PerformanceConfig{
					WorkerPoolSize: 5,
					CacheSize:      100,
					BatchWindow:    100 * time.Millisecond,
				},
				Display: DisplayConfig{
					DefaultMode: "live",
					RefreshRate: 1 * time.Second,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "text",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid watch interval",
			config: &Config{
				ClaudeConfigDirs: []string{"/path"},
				Monitoring: MonitoringConfig{
					WatchInterval:    0,
					UpdateFrequency:  1 * time.Second,
					SessionRetention: 1 * time.Hour,
				},
				Performance: PerformanceConfig{
					WorkerPoolSize: 5,
					CacheSize:      100,
					BatchWindow:    100 * time.Millisecond,
				},
				Display: DisplayConfig{
					DefaultMode: "live",
					RefreshRate: 1 * time.Second,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "text",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid worker pool size",
			config: &Config{
				ClaudeConfigDirs: []string{"/path"},
				Monitoring: MonitoringConfig{
					WatchInterval:    1 * time.Second,
					UpdateFrequency:  1 * time.Second,
					SessionRetention: 1 * time.Hour,
				},
				Performance: PerformanceConfig{
					WorkerPoolSize: 0,
					CacheSize:      100,
					BatchWindow:    100 * time.Millisecond,
				},
				Display: DisplayConfig{
					DefaultMode: "live",
					RefreshRate: 1 * time.Second,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "text",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid display mode",
			config: &Config{
				ClaudeConfigDirs: []string{"/path"},
				Monitoring: MonitoringConfig{
					WatchInterval:    1 * time.Second,
					UpdateFrequency:  1 * time.Second,
					SessionRetention: 1 * time.Hour,
				},
				Performance: PerformanceConfig{
					WorkerPoolSize: 5,
					CacheSize:      100,
					BatchWindow:    100 * time.Millisecond,
				},
				Display: DisplayConfig{
					DefaultMode: "invalid",
					RefreshRate: 1 * time.Second,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "text",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid log level",
			config: &Config{
				ClaudeConfigDirs: []string{"/path"},
				Monitoring: MonitoringConfig{
					WatchInterval:    1 * time.Second,
					UpdateFrequency:  1 * time.Second,
					SessionRetention: 1 * time.Hour,
				},
				Performance: PerformanceConfig{
					WorkerPoolSize: 5,
					CacheSize:      100,
					BatchWindow:    100 * time.Millisecond,
				},
				Display: DisplayConfig{
					DefaultMode: "live",
					RefreshRate: 1 * time.Second,
				},
				Logging: LoggingConfig{
					Level:  "invalid",
					Format: "text",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		wantErr bool
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid config file",
			content: `
claude_config_dirs:
  - /path/to/claude1
  - /path/to/claude2
monitoring:
  watch_interval: 2s
  update_frequency: 500ms
  session_retention: 48h
performance:
  worker_pool_size: 10
  cache_size: 200
  batch_window: 200ms
display:
  default_mode: compact
  color_enabled: false
  refresh_rate: 2s
storage:
  db_path: /tmp/test.db
  cache_dir: /tmp/cache
logging:
  level: debug
  output: stdout
  format: json
`,
			wantErr: false,
			check: func(t *testing.T, cfg *Config) {
				if len(cfg.ClaudeConfigDirs) != 2 {
					t.Errorf("got %d claude dirs, want 2", len(cfg.ClaudeConfigDirs))
				}
				if cfg.Monitoring.WatchInterval != 2*time.Second {
					t.Errorf("WatchInterval = %v, want 2s", cfg.Monitoring.WatchInterval)
				}
				if cfg.Performance.WorkerPoolSize != 10 {
					t.Errorf("WorkerPoolSize = %d, want 10", cfg.Performance.WorkerPoolSize)
				}
				if cfg.Display.DefaultMode != "compact" {
					t.Errorf("DefaultMode = %s, want compact", cfg.Display.DefaultMode)
				}
				if cfg.Display.ColorEnabled {
					t.Error("ColorEnabled = true, want false")
				}
				if cfg.Logging.Level != "debug" {
					t.Errorf("LogLevel = %s, want debug", cfg.Logging.Level)
				}
			},
		},
		{
			name:    "invalid yaml",
			content: `invalid: yaml: content: [`,
			wantErr: true,
		},
		{
			name:    "non-existent file",
			content: "", // Will not create file
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string

			if tt.name != "non-existent file" {
				filePath = filepath.Join(tmpDir, tt.name+".yaml")
				if err := os.WriteFile(filePath, []byte(tt.content), 0600); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			} else {
				filePath = filepath.Join(tmpDir, "nonexistent.yaml")
			}

			loader := NewLoader(filePath)
			cfg, err := loader.Load()

			if tt.wantErr {
				if err == nil {
					t.Error("Load() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("Load() error = %v, wantErr = false", err)
				return
			}

			if cfg == nil {
				t.Error("Load() returned nil config")
				return
			}

			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Test default loading (no config file)
	cfg, err := Load()
	if err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil")
	}

	// Should have default values
	if len(cfg.ClaudeConfigDirs) == 0 {
		t.Error("Load() returned config with no claude dirs")
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := Default()
	cfg.Logging.Level = "debug"

	// Save config
	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("Config file not created: %v", err)
	}

	// Load it back and verify
	loadedCfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if loadedCfg.Logging.Level != "debug" {
		t.Errorf("Loaded config LogLevel = %s, want debug", loadedCfg.Logging.Level)
	}
}

func TestEnvVarOverrides(t *testing.T) {
	// Save original env vars
	originalClaudeDir := os.Getenv("CLAUDE_CONFIG_DIR")
	originalDB := os.Getenv("TOKEN_MONITOR_DB")
	originalLogLevel := os.Getenv("TOKEN_MONITOR_LOG_LEVEL")

	// Restore env vars after test
	defer func() {
		if originalClaudeDir != "" {
			_ = os.Setenv("CLAUDE_CONFIG_DIR", originalClaudeDir) // nolint:errcheck
		} else {
			_ = os.Unsetenv("CLAUDE_CONFIG_DIR") // nolint:errcheck
		}
		if originalDB != "" {
			_ = os.Setenv("TOKEN_MONITOR_DB", originalDB) // nolint:errcheck
		} else {
			_ = os.Unsetenv("TOKEN_MONITOR_DB") // nolint:errcheck
		}
		if originalLogLevel != "" {
			_ = os.Setenv("TOKEN_MONITOR_LOG_LEVEL", originalLogLevel) // nolint:errcheck
		} else {
			_ = os.Unsetenv("TOKEN_MONITOR_LOG_LEVEL") // nolint:errcheck
		}
	}()

	// Set test env vars
	if err := os.Setenv("CLAUDE_CONFIG_DIR", "/env/dir1,/env/dir2"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TOKEN_MONITOR_DB", "/env/db.db"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("TOKEN_MONITOR_LOG_LEVEL", "DEBUG"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify env var overrides
	if len(cfg.ClaudeConfigDirs) != 2 {
		t.Errorf("got %d claude dirs, want 2", len(cfg.ClaudeConfigDirs))
	}
	if cfg.ClaudeConfigDirs[0] != "/env/dir1" {
		t.Errorf("ClaudeConfigDirs[0] = %s, want /env/dir1", cfg.ClaudeConfigDirs[0])
	}

	if cfg.Storage.DBPath != "/env/db.db" {
		t.Errorf("DBPath = %s, want /env/db.db", cfg.Storage.DBPath)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("LogLevel = %s, want debug", cfg.Logging.Level)
	}
}

// Benchmark config loading.
func BenchmarkLoad(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidate(b *testing.B) {
	cfg := Default()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cfg.Validate(); err != nil {
			b.Fatal(err)
		}
	}
}
