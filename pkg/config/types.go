// Package config provides configuration management for token-monitor.
//
// Configuration is loaded from multiple sources with the following precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Configuration file
// 4. Default values (lowest priority)
//
// Example usage:
//
//	cfg, err := config.Load()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Claude dirs: %v\n", cfg.ClaudeConfigDirs)
package config

import (
	"time"
)

// Config represents the complete application configuration.
//
// Invariants:
// - ClaudeConfigDirs must have at least one directory
// - WatchInterval must be > 0
// - UpdateFrequency must be > 0
// - SessionRetention must be > 0
// - WorkerPoolSize must be > 0
// - CacheSize must be > 0
// - BatchWindow must be > 0.
type Config struct {
	// Claude data directories to monitor
	ClaudeConfigDirs []string `yaml:"claude_config_dirs"`

	// Monitoring settings
	Monitoring MonitoringConfig `yaml:"monitoring"`

	// Performance settings
	Performance PerformanceConfig `yaml:"performance"`

	// Display settings
	Display DisplayConfig `yaml:"display"`

	// Storage settings
	Storage StorageConfig `yaml:"storage"`

	// Logging settings
	Logging LoggingConfig `yaml:"logging"`
}

// MonitoringConfig contains monitoring-related settings.
type MonitoringConfig struct {
	// How often to check for file changes
	WatchInterval time.Duration `yaml:"watch_interval"`

	// UI refresh rate
	UpdateFrequency time.Duration `yaml:"update_frequency"`

	// How long to keep session data
	SessionRetention time.Duration `yaml:"session_retention"`
}

// PerformanceConfig contains performance tuning settings.
type PerformanceConfig struct {
	// Number of concurrent file processors
	WorkerPoolSize int `yaml:"worker_pool_size"`

	// Maximum sessions to keep in memory cache
	CacheSize int `yaml:"cache_size"`

	// Database write batching window
	BatchWindow time.Duration `yaml:"batch_window"`
}

// DisplayConfig contains display-related settings.
type DisplayConfig struct {
	// Default display mode (live, compact, table, json)
	DefaultMode string `yaml:"default_mode"`

	// Enable colored output
	ColorEnabled bool `yaml:"color_enabled"`

	// Display refresh rate
	RefreshRate time.Duration `yaml:"refresh_rate"`
}

// StorageConfig contains storage-related settings.
type StorageConfig struct {
	// Path to BoltDB database file
	DBPath string `yaml:"db_path"`

	// Directory for cache files
	CacheDir string `yaml:"cache_dir"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `yaml:"level"`

	// Log output destination (stdout, stderr, file path)
	Output string `yaml:"output"`

	// Log format (text, json)
	Format string `yaml:"format"`
}

// Validate checks if the configuration satisfies all invariants.
//
// Returns an error if any invariant is violated:
//   - No Claude config directories specified
//   - Invalid time durations (must be > 0)
//   - Invalid worker pool size (must be > 0)
//   - Invalid cache size (must be > 0)
//   - Invalid display mode
//   - Invalid log level
//
// Thread-safety: This method is read-only and thread-safe.
func (c *Config) Validate() error {
	if len(c.ClaudeConfigDirs) == 0 {
		return ErrNoClaudeDirs
	}

	// Validate monitoring config
	if c.Monitoring.WatchInterval <= 0 {
		return ErrInvalidWatchInterval
	}
	if c.Monitoring.UpdateFrequency <= 0 {
		return ErrInvalidUpdateFrequency
	}
	if c.Monitoring.SessionRetention <= 0 {
		return ErrInvalidSessionRetention
	}

	// Validate performance config
	if c.Performance.WorkerPoolSize <= 0 {
		return ErrInvalidWorkerPoolSize
	}
	if c.Performance.CacheSize <= 0 {
		return ErrInvalidCacheSize
	}
	if c.Performance.BatchWindow <= 0 {
		return ErrInvalidBatchWindow
	}

	// Validate display config
	validModes := map[string]bool{
		"live":    true,
		"compact": true,
		"table":   true,
		"json":    true,
	}
	if !validModes[c.Display.DefaultMode] {
		return ErrInvalidDisplayMode
	}

	if c.Display.RefreshRate <= 0 {
		return ErrInvalidRefreshRate
	}

	// Validate logging config
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return ErrInvalidLogLevel
	}

	validFormats := map[string]bool{
		"text": true,
		"json": true,
	}
	if !validFormats[c.Logging.Format] {
		return ErrInvalidLogFormat
	}

	return nil
}

// Default returns a configuration with sensible default values.
//
// Default values are based on the architecture specifications
// and designed for typical usage patterns.
func Default() *Config {
	return &Config{
		ClaudeConfigDirs: defaultClaudeDirs(),
		Monitoring: MonitoringConfig{
			WatchInterval:    1 * time.Second,
			UpdateFrequency:  1 * time.Second,
			SessionRetention: 720 * time.Hour, // 30 days
		},
		Performance: PerformanceConfig{
			WorkerPoolSize: 5,
			CacheSize:      100,
			BatchWindow:    100 * time.Millisecond,
		},
		Display: DisplayConfig{
			DefaultMode:  "live",
			ColorEnabled: true,
			RefreshRate:  1 * time.Second,
		},
		Storage: StorageConfig{
			DBPath:   defaultDBPath(),
			CacheDir: defaultCacheDir(),
		},
		Logging: LoggingConfig{
			Level:  "info",
			Output: "stderr",
			Format: "text",
		},
	}
}
