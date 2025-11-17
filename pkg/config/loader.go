package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader provides methods for loading configuration from various sources.
type Loader interface {
	// Load loads configuration with the following precedence:
	// 1. Environment variables
	// 2. Configuration file
	// 3. Default values
	//
	// Returns the merged configuration or an error if validation fails.
	Load() (*Config, error)

	// LoadFromFile loads configuration from a specific file.
	LoadFromFile(path string) (*Config, error)
}

// loader implements the Loader interface.
type loader struct {
	configPath string
}

// NewLoader creates a new configuration loader.
//
// If configPath is empty, searches for config file in:
// 1. ./config.yaml (current directory)
// 2. ~/.config/token-monitor/config.yaml.
func NewLoader(configPath string) Loader {
	return &loader{
		configPath: configPath,
	}
}

// Load implements Loader.Load.
func (l *loader) Load() (*Config, error) {
	// Start with default configuration
	cfg := Default()

	// Find config file path
	configPath := l.configPath
	if configPath == "" {
		configPath = l.findConfigFile()
	}

	// Load from file if it exists
	if configPath != "" {
		fileCfg, err := l.LoadFromFile(configPath)
		if err != nil {
			// If file is specified but can't be loaded, return error
			if l.configPath != "" {
				return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
			}
			// Otherwise, just use defaults
		} else {
			cfg = l.mergeConfigs(cfg, fileCfg)
		}
	}

	// Apply environment variable overrides
	cfg = l.applyEnvVars(cfg)

	// Validate final configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// LoadFromFile implements Loader.LoadFromFile.
func (l *loader) LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path) // nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidYAML, err)
	}

	return &cfg, nil
}

// findConfigFile searches for a config file in standard locations.
//
// Searches in order:
// 1. ./config.yaml
// 2. ~/.config/token-monitor/config.yaml
//
// Returns empty string if no config file is found.
func (l *loader) findConfigFile() string {
	candidates := []string{
		"./config.yaml",
		defaultConfigPath(),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// mergeConfigs merges file configuration into default configuration.
//
// File values override defaults, but only if they are non-zero.
func (l *loader) mergeConfigs(base, override *Config) *Config {
	result := *base

	// Merge Claude directories
	if len(override.ClaudeConfigDirs) > 0 {
		result.ClaudeConfigDirs = override.ClaudeConfigDirs
	}

	// Merge monitoring config
	if override.Monitoring.WatchInterval > 0 {
		result.Monitoring.WatchInterval = override.Monitoring.WatchInterval
	}
	if override.Monitoring.UpdateFrequency > 0 {
		result.Monitoring.UpdateFrequency = override.Monitoring.UpdateFrequency
	}
	if override.Monitoring.SessionRetention > 0 {
		result.Monitoring.SessionRetention = override.Monitoring.SessionRetention
	}

	// Merge performance config
	if override.Performance.WorkerPoolSize > 0 {
		result.Performance.WorkerPoolSize = override.Performance.WorkerPoolSize
	}
	if override.Performance.CacheSize > 0 {
		result.Performance.CacheSize = override.Performance.CacheSize
	}
	if override.Performance.BatchWindow > 0 {
		result.Performance.BatchWindow = override.Performance.BatchWindow
	}

	// Merge display config
	if override.Display.DefaultMode != "" {
		result.Display.DefaultMode = override.Display.DefaultMode
	}
	// ColorEnabled is a bool, so we always take the override value
	result.Display.ColorEnabled = override.Display.ColorEnabled
	if override.Display.RefreshRate > 0 {
		result.Display.RefreshRate = override.Display.RefreshRate
	}

	// Merge storage config
	if override.Storage.DBPath != "" {
		result.Storage.DBPath = override.Storage.DBPath
	}
	if override.Storage.CacheDir != "" {
		result.Storage.CacheDir = override.Storage.CacheDir
	}

	// Merge logging config
	if override.Logging.Level != "" {
		result.Logging.Level = override.Logging.Level
	}
	if override.Logging.Output != "" {
		result.Logging.Output = override.Logging.Output
	}
	if override.Logging.Format != "" {
		result.Logging.Format = override.Logging.Format
	}

	return &result
}

// applyEnvVars applies environment variable overrides to the configuration.
//
// Supported environment variables:
//   - CLAUDE_CONFIG_DIR: Comma-separated list of Claude directories
//   - TOKEN_MONITOR_CONFIG: Path to config file
//   - TOKEN_MONITOR_DB: Path to database file
//   - TOKEN_MONITOR_LOG_LEVEL: Log level
func (l *loader) applyEnvVars(cfg *Config) *Config {
	result := *cfg

	// CLAUDE_CONFIG_DIR: comma-separated paths
	if envDirs := os.Getenv("CLAUDE_CONFIG_DIR"); envDirs != "" {
		dirs := strings.Split(envDirs, ",")
		for i := range dirs {
			dirs[i] = strings.TrimSpace(dirs[i])
		}
		result.ClaudeConfigDirs = dirs
	}

	// TOKEN_MONITOR_DB: database path
	if dbPath := os.Getenv("TOKEN_MONITOR_DB"); dbPath != "" {
		result.Storage.DBPath = dbPath
	}

	// TOKEN_MONITOR_LOG_LEVEL: log level
	if logLevel := os.Getenv("TOKEN_MONITOR_LOG_LEVEL"); logLevel != "" {
		result.Logging.Level = strings.ToLower(logLevel)
	}

	return &result
}

// Load is a convenience function that creates a loader and loads configuration.
//
// Equivalent to:
//
//	loader := NewLoader("")
//	return loader.Load()
func Load() (*Config, error) {
	return NewLoader("").Load()
}

// LoadFromFile is a convenience function that loads configuration from a file.
//
// Equivalent to:
//
//	loader := NewLoader(path)
//	return loader.Load()
func LoadFromFile(path string) (*Config, error) {
	return NewLoader(path).Load()
}

// Save writes the configuration to a YAML file.
//
// Creates parent directories if they don't exist.
// File is created with 0600 permissions (read/write for owner only).
func Save(cfg *Config, path string) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
