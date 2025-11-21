package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yourusername/token-monitor/pkg/config"
	"gopkg.in/yaml.v3"
)

// configCommand handles configuration management subcommands.
type configCommand struct {
	configPath string
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
  show      Display current configuration
  path      Show configuration file paths
  reset     Reset configuration to defaults

Show Flags:
  -format   Output format (yaml, json) (default: yaml)

Reset Flags:
  -force    Skip confirmation prompt
  -output   Output path for config file

Examples:
  # Show current configuration
  token-monitor config show

  # Show configuration in JSON format
  token-monitor config show -format json

  # Show configuration file paths
  token-monitor config path

  # Reset configuration to defaults
  token-monitor config reset

  # Reset without confirmation
  token-monitor config reset -force
`
	fmt.Print(help)
	return nil
}
