package config

import (
	"os"
	"path/filepath"
)

// defaultClaudeDirs returns the default Claude Code configuration directories.
//
// Searches in order:
// 1. ~/.config/claude/projects/ (new default)
// 2. ~/.claude/projects/ (legacy)
//
// Returns all directories that exist on the filesystem.
func defaultClaudeDirs() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir not available
		return []string{"."}
	}

	candidates := []string{
		filepath.Join(homeDir, ".config", "claude", "projects"),
		filepath.Join(homeDir, ".claude", "projects"),
	}

	var dirs []string
	for _, dir := range candidates {
		if _, err := os.Stat(dir); err == nil {
			dirs = append(dirs, dir)
		}
	}

	// If no directories found, return the new default path
	// (will be created by the application if needed)
	if len(dirs) == 0 {
		return []string{filepath.Join(homeDir, ".config", "claude", "projects")}
	}

	return dirs
}

// defaultDBPath returns the default database file path.
//
// Returns: ~/.config/token-monitor/sessions.db.
func defaultDBPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./sessions.db"
	}

	return filepath.Join(homeDir, ".config", "token-monitor", "sessions.db")
}

// defaultCacheDir returns the default cache directory.
//
// Returns: ~/.config/token-monitor/cache/.
func defaultCacheDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./cache"
	}

	return filepath.Join(homeDir, ".config", "token-monitor", "cache")
}

// defaultConfigPath returns the default configuration file path.
//
// Returns: ~/.config/token-monitor/config.yaml.
func defaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./config.yaml"
	}

	return filepath.Join(homeDir, ".config", "token-monitor", "config.yaml")
}
