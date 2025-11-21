package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/yourusername/token-monitor/pkg/config"
	"github.com/yourusername/token-monitor/pkg/discovery"
	"github.com/yourusername/token-monitor/pkg/logger"
	"github.com/yourusername/token-monitor/pkg/session"
)

// sessionCommand handles session management subcommands.
type sessionCommand struct {
	configPath string
}

// Execute runs the session command.
func (c *sessionCommand) Execute(args []string) error {
	if len(args) == 0 {
		return c.showHelp()
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "name":
		return c.runName(subargs)
	case "list":
		return c.runList(subargs)
	case "show":
		return c.runShow(subargs)
	case "delete":
		return c.runDelete(subargs)
	case "help":
		return c.showHelp()
	default:
		return fmt.Errorf("unknown session subcommand: %s", subcommand)
	}
}

// runName assigns a name to a session.
func (c *sessionCommand) runName(args []string) error {
	fs := flag.NewFlagSet("session name", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		return fmt.Errorf("usage: token-monitor session name <uuid> <name>")
	}

	uuid := fs.Arg(0)
	name := fs.Arg(1)

	// Validate name.
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name cannot be empty")
	}

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger.
	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Initialize session manager.
	mgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	defer func() {
		if closeErr := mgr.Close(); closeErr != nil {
			log.Error("failed to close session manager", "error", closeErr)
		}
	}()

	// Try to get existing session.
	existing, err := mgr.GetByUUID(uuid)
	if err != nil {
		if err == session.ErrSessionNotFound {
			// Session doesn't exist, create it with the name.
			// First, find the session file to get project path.
			disc := discovery.New(cfg.ClaudeConfigDirs, log)
			sessions, discErr := disc.Discover()
			if discErr != nil {
				return fmt.Errorf("failed to discover sessions: %w", discErr)
			}

			var projectPath string
			for _, s := range sessions {
				if s.SessionID == uuid {
					projectPath = s.ProjectPath
					break
				}
			}

			if projectPath == "" {
				return fmt.Errorf("session %s not found in discovered sessions", uuid)
			}

			// Create new session metadata.
			metadata := &session.Metadata{
				UUID:        uuid,
				Name:        name,
				ProjectPath: projectPath,
			}

			if createErr := mgr.Create(metadata); createErr != nil {
				return fmt.Errorf("failed to create session: %w", createErr)
			}

			fmt.Printf("Created session '%s' with name '%s'\n", uuid[:8], name)
			return nil
		}
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Session exists, update name.
	oldName := existing.Name
	if err := mgr.SetName(uuid, name); err != nil {
		if err == session.ErrNameConflict {
			return fmt.Errorf("name '%s' is already used by another session", name)
		}
		return fmt.Errorf("failed to set name: %w", err)
	}

	switch {
	case oldName == name:
		fmt.Printf("Session '%s' already has name '%s'\n", uuid[:8], name)
	case oldName == "":
		fmt.Printf("Set name '%s' for session '%s'\n", name, uuid[:8])
	default:
		fmt.Printf("Renamed session '%s' from '%s' to '%s'\n", uuid[:8], oldName, name)
	}

	return nil
}

// runList lists all sessions with metadata.
func (c *sessionCommand) runList(args []string) error {
	fs := flag.NewFlagSet("session list", flag.ExitOnError)
	sortBy := fs.String("sort", "name", "sort by: name, date, uuid")
	showAll := fs.Bool("all", false, "show all sessions including unnamed")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger.
	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Initialize session manager.
	mgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	defer func() {
		if closeErr := mgr.Close(); closeErr != nil {
			log.Error("failed to close session manager", "error", closeErr)
		}
	}()

	// Discover sessions.
	disc := discovery.New(cfg.ClaudeConfigDirs, log)
	discoveredSessions, err := disc.Discover()
	if err != nil {
		return fmt.Errorf("failed to discover sessions: %w", err)
	}

	// Get named sessions from database.
	namedSessions, err := mgr.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// Build a map of named sessions.
	namedMap := make(map[string]*session.Metadata)
	for _, s := range namedSessions {
		namedMap[s.UUID] = s
	}

	// Combine discovered and named sessions.
	type displaySession struct {
		UUID        string
		Name        string
		ProjectPath string
		UpdatedAt   time.Time
	}

	var sessions []displaySession

	for _, ds := range discoveredSessions {
		if named, ok := namedMap[ds.SessionID]; ok {
			sessions = append(sessions, displaySession{
				UUID:        ds.SessionID,
				Name:        named.Name,
				ProjectPath: ds.ProjectPath,
				UpdatedAt:   named.UpdatedAt,
			})
		} else if *showAll {
			sessions = append(sessions, displaySession{
				UUID:        ds.SessionID,
				Name:        "(unnamed)",
				ProjectPath: ds.ProjectPath,
				UpdatedAt:   time.Time{},
			})
		}
	}

	if len(sessions) == 0 {
		if *showAll {
			fmt.Println("No sessions found")
		} else {
			fmt.Println("No named sessions found. Use -all to show all sessions.")
		}
		return nil
	}

	// Sort sessions.
	switch *sortBy {
	case "name":
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].Name < sessions[j].Name
		})
	case "date":
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
		})
	case "uuid":
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].UUID < sessions[j].UUID
		})
	}

	// Display sessions.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "NAME\tUUID\tPROJECT\tLAST UPDATED"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintln(w, "----\t----\t-------\t------------"); err != nil {
		return fmt.Errorf("failed to write header separator: %w", err)
	}

	for _, s := range sessions {
		shortUUID := s.UUID[:8] + "..."
		projectName := s.ProjectPath
		if len(projectName) > 30 {
			projectName = "..." + projectName[len(projectName)-27:]
		}

		updated := "-"
		if !s.UpdatedAt.IsZero() {
			updated = s.UpdatedAt.Format("2006-01-02 15:04")
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, shortUUID, projectName, updated); err != nil {
			return fmt.Errorf("failed to write session: %w", err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}

// runShow displays detailed session information.
func (c *sessionCommand) runShow(args []string) error {
	fs := flag.NewFlagSet("session show", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: token-monitor session show <name|uuid>")
	}

	identifier := fs.Arg(0)

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger.
	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Initialize session manager.
	mgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	defer func() {
		if closeErr := mgr.Close(); closeErr != nil {
			log.Error("failed to close session manager", "error", closeErr)
		}
	}()

	// Try to find session by name first, then by UUID.
	var metadata *session.Metadata

	metadata, err = mgr.GetByName(identifier)
	if err != nil {
		if err == session.ErrSessionNotFound {
			// Try by UUID.
			metadata, err = mgr.GetByUUID(identifier)
			if err != nil {
				return fmt.Errorf("session not found: %s", identifier)
			}
		} else {
			return fmt.Errorf("failed to get session: %w", err)
		}
	}

	// Display session details.
	fmt.Println("Session Details")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("UUID:        %s\n", metadata.UUID)
	fmt.Printf("Name:        %s\n", metadata.Name)
	fmt.Printf("Project:     %s\n", metadata.ProjectPath)
	fmt.Printf("Created:     %s\n", metadata.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:     %s\n", metadata.UpdatedAt.Format("2006-01-02 15:04:05"))

	if len(metadata.Tags) > 0 {
		fmt.Printf("Tags:        %s\n", strings.Join(metadata.Tags, ", "))
	}

	if metadata.Description != "" {
		fmt.Printf("Description: %s\n", metadata.Description)
	}

	return nil
}

// runDelete removes session metadata.
func (c *sessionCommand) runDelete(args []string) error {
	fs := flag.NewFlagSet("session delete", flag.ExitOnError)
	force := fs.Bool("force", false, "skip confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("usage: token-monitor session delete <name|uuid>")
	}

	identifier := fs.Arg(0)

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger.
	log := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Initialize session manager.
	mgr, err := session.New(session.Config{
		DBPath: cfg.Storage.DBPath,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	defer func() {
		if closeErr := mgr.Close(); closeErr != nil {
			log.Error("failed to close session manager", "error", closeErr)
		}
	}()

	// Try to find session by name first, then by UUID.
	var metadata *session.Metadata

	metadata, err = mgr.GetByName(identifier)
	if err != nil {
		if err == session.ErrSessionNotFound {
			// Try by UUID.
			metadata, err = mgr.GetByUUID(identifier)
			if err != nil {
				return fmt.Errorf("session not found: %s", identifier)
			}
		} else {
			return fmt.Errorf("failed to get session: %w", err)
		}
	}

	// Confirm deletion.
	if !*force {
		fmt.Printf("Delete session '%s' (%s)? [y/N]: ", metadata.Name, metadata.UUID[:8])
		var response string
		if _, scanErr := fmt.Scanln(&response); scanErr != nil {
			return fmt.Errorf("cancelled")
		}

		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Delete session.
	if err := mgr.Delete(metadata.UUID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Printf("Deleted session '%s' (%s)\n", metadata.Name, metadata.UUID[:8])
	fmt.Println("Note: JSONL data files are preserved.")

	return nil
}

// showHelp displays session command help.
func (c *sessionCommand) showHelp() error {
	help := `Session Management Commands

Usage:
  token-monitor session <subcommand> [flags]

Subcommands:
  name <uuid> <name>    Assign a friendly name to a session
  list [flags]          List all sessions with metadata
  show <name|uuid>      Display detailed session information
  delete <name|uuid>    Remove session metadata (preserves data files)
  help                  Show this help message

List Flags:
  -sort    Sort by: name, date, uuid (default: name)
  -all     Show all sessions including unnamed

Delete Flags:
  -force   Skip confirmation prompt

Examples:
  # Name a session
  token-monitor session name a1b2c3d4-e5f6-7890-abcd-ef1234567890 my-project

  # List named sessions
  token-monitor session list

  # List all sessions
  token-monitor session list -all

  # Show session details
  token-monitor session show my-project

  # Delete session metadata
  token-monitor session delete my-project
`
	fmt.Print(help)
	return nil
}
