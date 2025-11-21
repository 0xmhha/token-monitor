// Package main provides the token-monitor CLI application.
//
// Token Monitor is a real-time monitoring tool for Claude Code CLI token usage.
// It tracks input/output tokens per session with live updates and persistent storage.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

// version is set during build time.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run executes the main application logic.
func run() error {
	// Define global flags.
	configPath := flag.String("config", "", "path to configuration file")
	showVersion := flag.Bool("version", false, "show version information")

	// Parse command.
	flag.Parse()

	// Handle version flag.
	if *showVersion {
		fmt.Printf("token-monitor %s\n", version)
		return nil
	}

	// Get command.
	args := flag.Args()
	if len(args) == 0 {
		return showUsage()
	}

	command := args[0]

	switch command {
	case "stats":
		return runStatsCommand(*configPath, args[1:])
	case "list":
		return runListCommand(*configPath)
	case "watch":
		return runWatchCommand(*configPath, args[1:])
	case "session":
		return runSessionCommand(*configPath, args[1:])
	case "config":
		return runConfigCommand(*configPath, args[1:])
	case "help":
		return showUsage()
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// runStatsCommand runs the stats command.
func runStatsCommand(configPath string, args []string) error {
	// Define stats-specific flags.
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	sessionID := fs.String("session", "", "filter by session ID")
	model := fs.String("model", "", "filter by model name")
	groupBy := fs.String("group-by", "", "group by dimensions (comma-separated: model,session,date,hour)")
	topN := fs.Int("top", 0, "show top N sessions by token usage")
	format := fs.String("format", "table", "output format (table, json, simple)")
	compact := fs.Bool("compact", false, "compact output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Parse group-by dimensions.
	var dimensions []string
	if *groupBy != "" {
		dimensions = strings.Split(*groupBy, ",")
		for i, dim := range dimensions {
			dimensions[i] = strings.TrimSpace(dim)
		}
	}

	cmd := &statsCommand{
		sessionID:  *sessionID,
		model:      *model,
		groupBy:    dimensions,
		topN:       *topN,
		format:     *format,
		compact:    *compact,
		configPath: configPath,
	}

	return cmd.Execute()
}

// runListCommand runs the list command.
func runListCommand(configPath string) error {
	cmd := &listCommand{
		configPath: configPath,
	}
	return cmd.Execute()
}

// runSessionCommand runs the session command.
func runSessionCommand(configPath string, args []string) error {
	cmd := &sessionCommand{
		configPath: configPath,
	}
	return cmd.Execute(args)
}

// runConfigCommand runs the config command.
func runConfigCommand(configPath string, args []string) error {
	cmd := &configCommand{
		configPath: configPath,
	}
	return cmd.Execute(args)
}

// runWatchCommand runs the watch command.
func runWatchCommand(configPath string, args []string) error {
	// Define watch-specific flags.
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	sessionID := fs.String("session", "", "monitor specific session ID")
	refresh := fs.Duration("refresh", time.Second, "refresh interval (e.g., 1s, 500ms)")
	format := fs.String("format", "table", "output format (table, simple)")
	history := fs.Bool("history", false, "keep history of updates (append mode)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cmd := &watchCommand{
		sessionID:   *sessionID,
		refresh:     *refresh,
		format:      *format,
		clearScreen: !*history, // clear screen unless history mode
		configPath:  configPath,
	}

	return cmd.Execute()
}

// showUsage displays usage information.
func showUsage() error {
	usage := `Token Monitor - Claude Code CLI token usage monitoring tool

Usage:
  token-monitor [flags] <command> [command flags]

Commands:
  stats       Display token usage statistics
  list        List all discovered sessions
  watch       Live monitoring of token usage
  session     Session management (name, list, show, delete)
  config      Configuration management (show, path, reset)
  help        Show this help message

Global Flags:
  -config     Path to configuration file
  -version    Show version information

Stats Command Flags:
  -session    Filter by session ID
  -model      Filter by model name
  -group-by   Group by dimensions (comma-separated: model,session,date,hour)
  -top        Show top N sessions by token usage
  -format     Output format (table, json, simple)
  -compact    Compact output

Watch Command Flags:
  -session    Monitor specific session ID
  -refresh    Refresh interval (default: 1s, e.g., 500ms, 2s)
  -format     Output format (table, simple)
  -history    Keep history of updates (append mode, default: false)

Examples:
  # Show overall statistics
  token-monitor stats

  # Show statistics grouped by model
  token-monitor stats -group-by model

  # Show top 10 sessions
  token-monitor stats -top 10

  # Show statistics in JSON format
  token-monitor stats -format json

  # Filter by session ID
  token-monitor stats -session abc123...

  # List all sessions
  token-monitor list

  # Live monitoring of all sessions
  token-monitor watch

  # Live monitoring of specific session
  token-monitor watch -session abc123...

  # Live monitoring with custom refresh
  token-monitor watch -refresh 500ms

  # Live monitoring in simple format
  token-monitor watch -format simple

  # Live monitoring with history (append mode)
  token-monitor watch -history

  # Session management
  token-monitor session name <uuid> <name>
  token-monitor session list
  token-monitor session show <name>
  token-monitor session delete <name>

Version: %s
`

	fmt.Printf(usage, version)
	return nil
}
