// Package discovery provides functionality for discovering Claude Code
// session files and mapping them to projects.
//
// It scans configured directories for JSONL files and extracts project
// and session information.
//
// Example usage:
//
//	d := discovery.New([]string{"~/.config/claude/projects"}, logger.Default())
//	sessions, err := d.Discover()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, session := range sessions {
//	    fmt.Printf("Session: %s, Project: %s\n", session.ID, session.ProjectPath)
//	}
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Logger defines the logging interface used by the discovery package.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// SessionFile represents a discovered session JSONL file.
type SessionFile struct {
	// SessionID is the UUID extracted from the filename.
	SessionID string

	// FilePath is the absolute path to the JSONL file.
	FilePath string

	// ProjectPath is the directory containing the session file.
	// This corresponds to the project directory in Claude Code's structure.
	ProjectPath string

	// Size is the file size in bytes.
	Size int64

	// ModTime is the last modification time.
	ModTime int64 // Unix timestamp
}

// Discoverer provides methods for discovering Claude Code session files.
type Discoverer interface {
	// Discover scans configured directories and returns all session files found.
	//
	// Returns:
	//   - Slice of discovered session files
	//   - Error if directories cannot be accessed
	//
	// Skips files that don't match the expected pattern (UUID.jsonl).
	Discover() ([]SessionFile, error)

	// DiscoverProject returns session files for a specific project directory.
	//
	// Parameters:
	//   - projectPath: Absolute or relative path to project directory
	//
	// Returns:
	//   - Slice of session files in the project
	//   - Error if directory cannot be accessed
	DiscoverProject(projectPath string) ([]SessionFile, error)
}

// discoverer implements the Discoverer interface.
type discoverer struct {
	baseDirs []string // Claude config directories to scan
	logger   Logger
}

// New creates a new Discoverer instance.
//
// Parameters:
//   - baseDirs: List of base directories to scan (e.g., ~/.config/claude/projects)
//   - logger: Logger instance for diagnostic messages
//
// Returns a configured Discoverer.
func New(baseDirs []string, logger Logger) Discoverer {
	return &discoverer{
		baseDirs: baseDirs,
		logger:   logger,
	}
}

// Discover implements Discoverer.Discover.
func (d *discoverer) Discover() ([]SessionFile, error) {
	var allSessions []SessionFile

	for _, baseDir := range d.baseDirs {
		// Expand home directory if present
		expandedDir := expandHome(baseDir)

		// Check if directory exists
		if _, err := os.Stat(expandedDir); err != nil {
			if os.IsNotExist(err) {
				d.logger.Warn("directory not found, skipping", "path", expandedDir)
				continue
			}
			return nil, fmt.Errorf("failed to stat directory %s: %w", expandedDir, err)
		}

		// Scan directory for projects
		sessions, err := d.scanBaseDirectory(expandedDir)
		if err != nil {
			return nil, fmt.Errorf("failed to scan directory %s: %w", expandedDir, err)
		}

		allSessions = append(allSessions, sessions...)
	}

	d.logger.Info("discovery complete", "total_sessions", len(allSessions))
	return allSessions, nil
}

// DiscoverProject implements Discoverer.DiscoverProject.
func (d *discoverer) DiscoverProject(projectPath string) ([]SessionFile, error) {
	expandedPath := expandHome(projectPath)

	// Check if directory exists
	if _, err := os.Stat(expandedPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, expandedPath)
		}
		return nil, fmt.Errorf("failed to stat directory %s: %w", expandedPath, err)
	}

	return d.scanProjectDirectory(expandedPath)
}

// scanBaseDirectory scans a base directory for project subdirectories.
//
// Claude Code structure: basedir/project-hash/session-uuid.jsonl.
func (d *discoverer) scanBaseDirectory(baseDir string) ([]SessionFile, error) {
	var sessions []SessionFile

	// Read all entries in base directory
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := filepath.Join(baseDir, entry.Name())
		projectSessions, err := d.scanProjectDirectory(projectPath)
		if err != nil {
			d.logger.Warn("failed to scan project directory",
				"path", projectPath,
				"error", err)
			continue
		}

		sessions = append(sessions, projectSessions...)
	}

	return sessions, nil
}

// scanProjectDirectory scans a project directory for session JSONL files.
func (d *discoverer) scanProjectDirectory(projectDir string) ([]SessionFile, error) {
	sessions := make([]SessionFile, 0, 10) // Pre-allocate with reasonable capacity

	// Read all files in project directory
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check if file matches session pattern (UUID.jsonl)
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		// Extract session ID from filename (remove .jsonl extension)
		sessionID := strings.TrimSuffix(entry.Name(), ".jsonl")

		// Validate session ID format (basic UUID check)
		if !isValidSessionID(sessionID) {
			d.logger.Debug("skipping non-session file",
				"file", entry.Name(),
				"reason", "invalid session ID format")
			continue
		}

		// Get file info
		filePath := filepath.Join(projectDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			d.logger.Warn("failed to get file info",
				"path", filePath,
				"error", err)
			continue
		}

		sessions = append(sessions, SessionFile{
			SessionID:   sessionID,
			FilePath:    filePath,
			ProjectPath: projectDir,
			Size:        info.Size(),
			ModTime:     info.ModTime().Unix(),
		})
	}

	d.logger.Debug("scanned project directory",
		"path", projectDir,
		"sessions_found", len(sessions))

	return sessions, nil
}

// expandHome expands ~ in file paths to the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return homeDir
	}

	return filepath.Join(homeDir, path[2:])
}

// isValidSessionID performs basic validation on session ID format.
//
// Expected format: UUID v4 (8-4-4-4-12 hex digits with dashes)
// Example: a1b2c3d4-e5f6-7890-abcd-ef1234567890.
func isValidSessionID(id string) bool {
	// Basic length check (UUID v4 is 36 characters)
	if len(id) != 36 {
		return false
	}

	// Check for dashes at correct positions
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return false
	}

	// Check that other characters are hex digits
	for i, c := range id {
		// Skip dash positions
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}

		// Check if character is hex digit
		if !isHexDigit(c) {
			return false
		}
	}

	return true
}

// isHexDigit checks if a rune is a hexadecimal digit.
func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'f') ||
		(r >= 'A' && r <= 'F')
}
