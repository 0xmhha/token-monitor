package discovery

import "errors"

// Common errors returned by the discovery package.
var (
	// ErrProjectNotFound is returned when a project directory does not exist.
	ErrProjectNotFound = errors.New("project directory not found")

	// ErrNoSessionsFound is returned when no session files are discovered.
	ErrNoSessionsFound = errors.New("no session files found")

	// ErrInvalidPath is returned when a path is invalid or inaccessible.
	ErrInvalidPath = errors.New("invalid or inaccessible path")
)
