package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xmhha/token-monitor/pkg/config"
	"gopkg.in/yaml.v3"
)

// configCommand handles configuration management subcommands.
type configCommand struct {
	configPath string
	globalOpts globalOptions
}

// Execute runs the config command with given arguments.
func (c *configCommand) Execute(args []string) error {
	if len(args) == 0 {
		return c.showHelp()
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "show":
		return c.runShow(subargs)
	case "path":
		return c.runPath()
	case "reset":
		return c.runReset(subargs)
	case "set":
		return c.runSet(subargs)
	case "validate":
		return c.runValidate()
	case "help":
		return c.showHelp()
	default:
		return fmt.Errorf("unknown config subcommand: %s", subcommand)
	}
}

// runShow displays the current configuration.
func (c *configCommand) runShow(args []string) error {
	fs := flag.NewFlagSet("config show", flag.ExitOnError)
	format := fs.String("format", "yaml", "output format (yaml, json)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch *format {
	case "json":
		return c.showJSON(cfg)
	default:
		return c.showYAML(cfg)
	}
}

// showYAML displays configuration in YAML format.
func (c *configCommand) showYAML(cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println("# Current Configuration")
	fmt.Println("# Source: ", c.getConfigSource())
	fmt.Println()
	fmt.Print(string(data))
	return nil
}

// showJSON displays configuration in JSON format.
func (c *configCommand) showJSON(cfg *config.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// runPath shows the configuration file path.
func (c *configCommand) runPath() error {
	paths := []string{
		"./token-monitor.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "token-monitor", "config.yaml"),
		"/etc/token-monitor/config.yaml",
	}

	fmt.Println("Configuration file search paths (in order of precedence):")
	fmt.Println()

	for i, p := range paths {
		exists := "not found"
		if _, err := os.Stat(p); err == nil {
			exists = "found"
		}
		fmt.Printf("  %d. %s [%s]\n", i+1, p, exists)
	}

	fmt.Println()
	fmt.Println("Active configuration:", c.getConfigSource())
	return nil
}

// runReset resets configuration to defaults.
func (c *configCommand) runReset(args []string) error {
	fs := flag.NewFlagSet("config reset", flag.ExitOnError)
	force := fs.Bool("force", false, "skip confirmation prompt")
	output := fs.String("output", "", "output path for config file (default: ~/.config/token-monitor/config.yaml)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Determine output path
	outputPath := *output
	if outputPath == "" {
		outputPath = filepath.Join(os.Getenv("HOME"), ".config", "token-monitor", "config.yaml")
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil && !*force {
		fmt.Printf("Configuration file already exists at: %s\n", outputPath)
		fmt.Print("Overwrite? [y/N]: ")

		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			// If Scanln fails, treat as "no"
			fmt.Println("\nReset cancelled.")
			return nil
		}
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "y" && response != "yes" {
			fmt.Println("Reset cancelled.")
			return nil
		}
	}

	// Create default configuration
	cfg := config.Default()

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Configuration reset to defaults at: %s\n", outputPath)
	return nil
}

// runSet updates a configuration value.
func (c *configCommand) runSet(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: token-monitor config set <key> <value>")
	}

	key := args[0]
	value := args[1]

	// Determine config file path
	configPath := c.getConfigSource()
	if configPath == "defaults (no config file found)" {
		configPath = filepath.Join(os.Getenv("HOME"), ".config", "token-monitor", "config.yaml")
	}

	// Load current configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Update the configuration value
	if err := c.setConfigValue(cfg, key, value); err != nil {
		return err
	}

	// Validate the updated configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration after update: %w", err)
	}

	// Save to file
	if err := config.Save(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Set %s = %s\n", key, value)
	fmt.Printf("✓ Configuration saved to: %s\n", configPath)
	return nil
}

// runValidate validates the current configuration.
func (c *configCommand) runValidate() error {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("✗ Configuration validation failed:")
		fmt.Printf("  Error: %v\n", err)
		return err
	}

	if err := cfg.Validate(); err != nil {
		fmt.Println("✗ Configuration validation failed:")
		fmt.Printf("  Error: %v\n", err)
		fmt.Println()
		fmt.Println("Suggestions:")
		c.printValidationSuggestions(err)
		return err
	}

	fmt.Println("✓ Configuration is valid")
	fmt.Println()
	fmt.Println("Configuration summary:")
	fmt.Printf("  Claude directories: %d configured\n", len(cfg.ClaudeConfigDirs))
	fmt.Printf("  Watch interval: %v\n", cfg.Monitoring.WatchInterval)
	fmt.Printf("  Log level: %s\n", cfg.Logging.Level)
	fmt.Printf("  Database path: %s\n", cfg.Storage.DBPath)
	return nil
}

// setConfigValue updates a configuration value by key.
func (c *configCommand) setConfigValue(cfg *config.Config, key, value string) error {
	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid key format: %s (expected format: section.field)", key)
	}

	section := parts[0]
	field := parts[1]

	switch section {
	case "logging":
		return c.setLoggingValue(cfg, field, value)
	case "monitoring":
		return c.setMonitoringValue(cfg, field, value)
	case "performance":
		return c.setPerformanceValue(cfg, field, value)
	case "display":
		return c.setDisplayValue(cfg, field, value)
	case "storage":
		return c.setStorageValue(cfg, field, value)
	default:
		return fmt.Errorf("unknown config section: %s", section)
	}
}

// setLoggingValue updates a logging configuration value.
func (c *configCommand) setLoggingValue(cfg *config.Config, field, value string) error {
	switch field {
	case "level":
		validLevels := []string{"debug", "info", "warn", "error"}
		if !contains(validLevels, value) {
			return fmt.Errorf("invalid log level: %s (must be one of: %s)", value, strings.Join(validLevels, ", "))
		}
		cfg.Logging.Level = value
	case "format":
		validFormats := []string{"text", "json"}
		if !contains(validFormats, value) {
			return fmt.Errorf("invalid log format: %s (must be one of: %s)", value, strings.Join(validFormats, ", "))
		}
		cfg.Logging.Format = value
	case "output":
		cfg.Logging.Output = value
	default:
		return fmt.Errorf("unknown logging field: %s", field)
	}
	return nil
}

// setMonitoringValue updates a monitoring configuration value.
func (c *configCommand) setMonitoringValue(cfg *config.Config, field, value string) error {
	switch field {
	case "watch_interval":
		duration, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid watch_interval: %w", err)
		}
		cfg.Monitoring.WatchInterval = duration
	case "update_frequency":
		duration, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid update_frequency: %w", err)
		}
		cfg.Monitoring.UpdateFrequency = duration
	case "session_retention":
		duration, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid session_retention: %w", err)
		}
		cfg.Monitoring.SessionRetention = duration
	default:
		return fmt.Errorf("unknown monitoring field: %s", field)
	}
	return nil
}

// setPerformanceValue updates a performance configuration value.
func (c *configCommand) setPerformanceValue(cfg *config.Config, field, value string) error {
	switch field {
	case "worker_pool_size":
		size, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("invalid worker_pool_size: %w", err)
		}
		cfg.Performance.WorkerPoolSize = size
	case "cache_size":
		size, err := parseInt(value)
		if err != nil {
			return fmt.Errorf("invalid cache_size: %w", err)
		}
		cfg.Performance.CacheSize = size
	case "batch_window":
		duration, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid batch_window: %w", err)
		}
		cfg.Performance.BatchWindow = duration
	default:
		return fmt.Errorf("unknown performance field: %s", field)
	}
	return nil
}

// setDisplayValue updates a display configuration value.
func (c *configCommand) setDisplayValue(cfg *config.Config, field, value string) error {
	switch field {
	case "default_mode":
		validModes := []string{"live", "compact", "table", "json"}
		if !contains(validModes, value) {
			return fmt.Errorf("invalid default_mode: %s (must be one of: %s)", value, strings.Join(validModes, ", "))
		}
		cfg.Display.DefaultMode = value
	case "color_enabled":
		enabled, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("invalid color_enabled: %w", err)
		}
		cfg.Display.ColorEnabled = enabled
	case "refresh_rate":
		duration, err := parseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid refresh_rate: %w", err)
		}
		cfg.Display.RefreshRate = duration
	default:
		return fmt.Errorf("unknown display field: %s", field)
	}
	return nil
}

// setStorageValue updates a storage configuration value.
func (c *configCommand) setStorageValue(cfg *config.Config, field, value string) error {
	switch field {
	case "db_path":
		cfg.Storage.DBPath = value
	case "cache_dir":
		cfg.Storage.CacheDir = value
	default:
		return fmt.Errorf("unknown storage field: %s", field)
	}
	return nil
}

// printValidationSuggestions prints helpful suggestions based on validation errors.
func (c *configCommand) printValidationSuggestions(err error) {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "log level"):
		fmt.Println("  - Valid log levels: debug, info, warn, error")
		fmt.Println("  - Example: token-monitor config set logging.level info")
	case strings.Contains(errStr, "log format"):
		fmt.Println("  - Valid log formats: text, json")
		fmt.Println("  - Example: token-monitor config set logging.format text")
	case strings.Contains(errStr, "display mode"):
		fmt.Println("  - Valid display modes: live, compact, table, json")
		fmt.Println("  - Example: token-monitor config set display.default_mode live")
	case strings.Contains(errStr, "watch interval"):
		fmt.Println("  - Watch interval must be greater than 0")
		fmt.Println("  - Example: token-monitor config set monitoring.watch_interval 1s")
	case strings.Contains(errStr, "worker pool"):
		fmt.Println("  - Worker pool size must be greater than 0")
		fmt.Println("  - Example: token-monitor config set performance.worker_pool_size 5")
	default:
		fmt.Println("  - Run 'token-monitor config show' to see current configuration")
		fmt.Println("  - Run 'token-monitor config reset' to restore defaults")
	}
}

// getConfigSource returns the path of the active configuration file.
func (c *configCommand) getConfigSource() string {
	paths := []string{
		"./token-monitor.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "token-monitor", "config.yaml"),
		"/etc/token-monitor/config.yaml",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return "defaults (no config file found)"
}

// showHelp displays help for config command.
func (c *configCommand) showHelp() error {
	help := `Config - Configuration management

Usage:
  token-monitor config <subcommand> [flags]

Subcommands:
  show          Display current configuration
  path          Show configuration file paths
  reset         Reset configuration to defaults
  set           Set a configuration value
  validate      Validate current configuration

Show Flags:
  -format       Output format (yaml, json) (default: yaml)

Reset Flags:
  -force        Skip confirmation prompt
  -output       Output path for config file

Set Usage:
  token-monitor config set <key> <value>

  Supported keys:
    logging.level           Log level (debug, info, warn, error)
    logging.format          Log format (text, json)
    logging.output          Log output (stdout, stderr, or file path)
    monitoring.watch_interval        Watch interval (e.g., 1s, 500ms)
    monitoring.update_frequency      Update frequency (e.g., 1s)
    monitoring.session_retention     Session retention (e.g., 720h)
    performance.worker_pool_size     Worker pool size (integer > 0)
    performance.cache_size           Cache size (integer > 0)
    performance.batch_window         Batch window (e.g., 100ms)
    display.default_mode             Display mode (live, compact, table, json)
    display.color_enabled            Color enabled (true, false)
    display.refresh_rate             Refresh rate (e.g., 1s)
    storage.db_path                  Database file path
    storage.cache_dir                Cache directory path

Examples:
  # Show current configuration
  token-monitor config show

  # Show configuration in JSON format
  token-monitor config show -format json

  # Show configuration file paths
  token-monitor config path

  # Set log level to debug
  token-monitor config set logging.level debug

  # Set watch interval to 2 seconds
  token-monitor config set monitoring.watch_interval 2s

  # Set worker pool size to 10
  token-monitor config set performance.worker_pool_size 10

  # Validate current configuration
  token-monitor config validate

  # Reset configuration to defaults
  token-monitor config reset

  # Reset without confirmation
  token-monitor config reset -force
`
	fmt.Print(help)
	return nil
}

// Helper functions

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// parseDuration parses a duration string.
func parseDuration(s string) (time.Duration, error) {
	duration, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration format: %s (examples: 1s, 500ms, 1h)", s)
	}
	return duration, nil
}

// parseInt parses an integer string.
func parseInt(s string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("invalid integer: %s", s)
	}
	return n, nil
}

// parseBool parses a boolean string.
func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "yes", "1", "on":
		return true, nil
	case "false", "no", "0", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s (use true/false, yes/no, 1/0, or on/off)", s)
	}
}
