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

	"github.com/0xmhha/token-monitor/pkg/tui"
)

// version is set during build time.
var version = "dev"

// globalOptions holds global flags that apply to all commands.
type globalOptions struct {
	configPath string
	logLevel   string
	jsonOutput bool
	noColor    bool
}

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
	logLevel := flag.String("log-level", "", "log level (debug, info, warn, error)")
	jsonOutput := flag.Bool("json", false, "output in JSON format (applies to all commands)")
	noColor := flag.Bool("no-color", false, "disable colored output")

	// Parse command.
	flag.Parse()

	// Handle version flag.
	if *showVersion {
		fmt.Printf("token-monitor %s\n", version)
		return nil
	}

	// Create global options.
	globalOpts := globalOptions{
		configPath: *configPath,
		logLevel:   *logLevel,
		jsonOutput: *jsonOutput,
		noColor:    *noColor,
	}

	// Get command.
	args := flag.Args()
	if len(args) == 0 {
		// Default: launch TUI
		return runTUICommand(globalOpts, nil)
	}

	command := args[0]

	switch command {
	case "tui":
		return runTUICommand(globalOpts, args[1:])
	case "stats":
		return runStatsCommand(globalOpts, args[1:])
	case "list":
		return runListCommand(globalOpts)
	case "watch":
		return runWatchCommand(globalOpts, args[1:])
	case "session":
		return runSessionCommand(globalOpts, args[1:])
	case "config":
		return runConfigCommand(globalOpts, args[1:])
	case "query":
		return runQueryCommand(globalOpts, args[1:])
	case "status":
		return runStatusCommand(globalOpts, args[1:])
	case "serve":
		return runServeCommand(globalOpts, args[1:])
	case "help":
		return showUsage()
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// runTUICommand launches the interactive TUI dashboard.
func runTUICommand(globalOpts globalOptions, args []string) error {
	fs := flag.NewFlagSet("tui", flag.ExitOnError)
	sessionID := fs.String("session", "", "monitor specific session ID")
	refresh := fs.Duration("refresh", time.Second, "refresh interval (e.g., 1s, 500ms)")

	if args != nil {
		if err := fs.Parse(args); err != nil {
			return err
		}
	}

	return tui.New(tui.Options{
		SessionID: *sessionID,
		Refresh:   *refresh,
		LogLevel:  globalOpts.logLevel,
	})
}

// runStatsCommand runs the stats command.
func runStatsCommand(globalOpts globalOptions, args []string) error {
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

	// Override format if global --json flag is set.
	outputFormat := *format
	if globalOpts.jsonOutput {
		outputFormat = "json"
	}

	cmd := &statsCommand{
		sessionID:  *sessionID,
		model:      *model,
		groupBy:    dimensions,
		topN:       *topN,
		format:     outputFormat,
		compact:    *compact,
		configPath: globalOpts.configPath,
		globalOpts: globalOpts,
	}

	return cmd.Execute()
}

// runListCommand runs the list command.
func runListCommand(globalOpts globalOptions) error {
	cmd := &listCommand{
		configPath: globalOpts.configPath,
		globalOpts: globalOpts,
	}
	return cmd.Execute()
}

// runSessionCommand runs the session command.
func runSessionCommand(globalOpts globalOptions, args []string) error {
	cmd := &sessionCommand{
		configPath: globalOpts.configPath,
		globalOpts: globalOpts,
	}
	return cmd.Execute(args)
}

// runConfigCommand runs the config command.
func runConfigCommand(globalOpts globalOptions, args []string) error {
	cmd := &configCommand{
		configPath: globalOpts.configPath,
		globalOpts: globalOpts,
	}
	return cmd.Execute(args)
}

// runWatchCommand runs the watch command.
func runWatchCommand(globalOpts globalOptions, args []string) error {
	// Define watch-specific flags.
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	sessionID := fs.String("session", "", "monitor specific session ID")
	refresh := fs.Duration("refresh", time.Second, "refresh interval (e.g., 1s, 500ms)")
	format := fs.String("format", "table", "output format (table, simple)")
	history := fs.Bool("history", false, "keep history of updates (append mode)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Override format if global --json flag is set.
	outputFormat := *format
	if globalOpts.jsonOutput {
		outputFormat = "json"
	}

	cmd := &watchCommand{
		sessionID:   *sessionID,
		refresh:     *refresh,
		format:      outputFormat,
		clearScreen: !*history, // clear screen unless history mode
		configPath:  globalOpts.configPath,
		globalOpts:  globalOpts,
	}

	return cmd.Execute()
}

// runQueryCommand runs the fast query command (no BoltDB).
func runQueryCommand(globalOpts globalOptions, args []string) error {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	current := fs.Bool("current", false, "auto-detect current session")
	sessionID := fs.String("session", "", "specify session ID directly")
	metric := fs.String("metric", "", "metric to output (total, input, output, count, burn-rate, burn-rate-hour, block-remaining, block-tokens)")
	jsonOut := fs.Bool("json", false, "output all metrics as JSON")
	format := fs.String("format", "", "output format (hook)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cmd := &queryCommand{
		current:    *current,
		sessionID:  *sessionID,
		metric:     *metric,
		jsonOutput: *jsonOut,
		format:     *format,
		configPath: globalOpts.configPath,
		globalOpts: globalOpts,
	}

	return cmd.Execute()
}

// showUsage displays usage information.
func showUsage() error {
	usage := `Token Monitor - Claude Code CLI token usage monitoring tool

Usage:
  token-monitor [flags] <command> [command flags]

Commands:
  tui         Interactive TUI dashboard (default when no command given)
  stats       Display token usage statistics
  list        List all discovered sessions
  watch       Live monitoring of token usage
  session     Session management (name, list, show, delete)
  config      Configuration management (show, path, set, validate, reset)
  query       Fast single-metric token lookup (for hooks)
  status      Compact status line output (for Claude Code status)
  serve       MCP server mode (for Claude Code MCP integration)
  help        Show this help message

Global Flags:
  -config       Path to configuration file
  -version      Show version information
  -log-level    Set log level (debug, info, warn, error)
  -json         Output in JSON format (overrides command-specific format flags)
  -no-color     Disable colored output

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

Query Command Flags:
  -current    Auto-detect current session
  -session    Specify session ID directly
  -metric     Metric to output (total, input, output, count, burn-rate, burn-rate-hour, block-remaining, block-tokens)
  -json       Output all metrics as JSON
  -format     Output format (hook)

Status Command Flags:
  -current    Auto-detect current session
  -session    Specify session ID directly
  -compact    Minimal output (~13 chars)
  -full       Verbose output (~75 chars)
  -no-emoji   Omit emoji prefix
  -watch      Continuous output mode
  -interval   Watch refresh interval (default: 5s)

Serve Command Flags:
  -stdio      Use stdio for MCP communication (default: true)

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

  # Configuration management
  token-monitor config show
  token-monitor config set logging.level debug
  token-monitor config validate
  token-monitor config reset

Integration Examples:
  # Fast token query (for PostToolUse hooks)
  token-monitor query --current --metric total

  # All metrics as JSON
  token-monitor query --current --json

  # Compact status for Claude Code status line
  token-monitor status --current --compact

  # Start MCP server
  token-monitor serve --stdio

Version: %s
`

	fmt.Printf(usage, version)
	return nil
}
