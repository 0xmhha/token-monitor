package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xmhha/token-monitor/pkg/config"
	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/logger"
	"github.com/0xmhha/token-monitor/pkg/mcp"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
)

// serveCommand runs an MCP server that exposes token monitoring data as tools.
type serveCommand struct {
	stdio      bool
	configPath string
	globalOpts globalOptions
}

// Execute starts the MCP server loop.
func (c *serveCommand) Execute() error {
	cfg, err := config.NewLoader(c.configPath).Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	log := c.buildLogger(cfg)
	log.Info("starting MCP server", "version", version)

	disc := discovery.New(cfg.ClaudeConfigDirs, log)

	readerFactory := func() (reader.Reader, error) {
		return reader.New(reader.Config{
			PositionStore: reader.NewMemoryPositionStore(),
			Parser:        parser.New(),
		}, log)
	}

	registry := mcp.NewToolRegistry()
	mcp.RegisterTokenTools(registry, disc, readerFactory, log)

	srv := mcp.NewServer(os.Stdin, os.Stdout, registry, version, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Run()
	}()

	select {
	case <-sigChan:
		log.Info("received shutdown signal")
		cancel()
		return nil
	case err := <-errChan:
		_ = ctx // suppress unused warning
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}

// buildLogger creates a logger for the serve command.
func (c *serveCommand) buildLogger(cfg *config.Config) logger.Logger {
	level := cfg.Logging.Level
	if c.globalOpts.logLevel != "" {
		level = c.globalOpts.logLevel
	}
	// MCP protocol uses stdout; log to stderr to avoid interference.
	return logger.New(logger.Config{
		Level:  level,
		Format: cfg.Logging.Format,
		Output: "stderr",
	})
}

// runServeCommand parses flags and runs the serve command.
func runServeCommand(globalOpts globalOptions, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	stdio := fs.Bool("stdio", true, "use stdin/stdout transport (required for MCP)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cmd := &serveCommand{
		stdio:      *stdio,
		configPath: globalOpts.configPath,
		globalOpts: globalOpts,
	}

	return cmd.Execute()
}
