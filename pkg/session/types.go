// Package session provides session metadata management with persistent storage.
//
// It maps session UUIDs to user-friendly names and metadata, providing
// CRUD operations with indexing for fast lookups.
//
// Example usage:
//
//	mgr, err := session.New(session.Config{
//	    DBPath: "~/.config/token-monitor/sessions.db",
//	}, logger.Default())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer mgr.Close()
//
//	// Create session metadata
//	metadata := &session.Metadata{
//	    UUID:        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
//	    Name:        "my-session",
//	    ProjectPath: "/path/to/project",
//	    Tags:        []string{"dev", "backend"},
//	    Description: "Development session for API work",
//	}
//
//	if err := mgr.Create(metadata); err != nil {
//	    log.Fatal(err)
//	}
package session

import "time"

// Metadata represents session metadata stored in the database.
type Metadata struct {
	// UUID is the session identifier (36 chars, 8-4-4-4-12 format).
	UUID string `json:"uuid"`

	// Name is the user-friendly session name (must be unique).
	Name string `json:"name"`

	// ProjectPath is the project directory containing the session.
	ProjectPath string `json:"project_path"`

	// CreatedAt is the session creation timestamp.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the last update timestamp.
	UpdatedAt time.Time `json:"updated_at"`

	// Tags are user-defined labels for categorization.
	Tags []string `json:"tags,omitempty"`

	// Description is an optional session description.
	Description string `json:"description,omitempty"`
}

// Manager provides session metadata CRUD operations.
type Manager interface {
	// Create creates a new session metadata entry.
	//
	// Returns error if:
	//   - UUID is invalid
	//   - Name is already taken
	//   - Database operation fails
	Create(metadata *Metadata) error

	// GetByUUID retrieves session metadata by UUID.
	//
	// Returns:
	//   - Metadata if found
	//   - ErrSessionNotFound if not found
	//   - Error for database failures
	GetByUUID(uuid string) (*Metadata, error)

	// GetByName retrieves session metadata by name.
	//
	// Returns:
	//   - Metadata if found
	//   - ErrSessionNotFound if not found
	//   - Error for database failures
	GetByName(name string) (*Metadata, error)

	// Update updates existing session metadata.
	//
	// Parameters:
	//   - uuid: Session UUID to update
	//   - metadata: New metadata values
	//
	// Returns error if:
	//   - Session not found
	//   - Name conflicts with another session
	//   - Database operation fails
	Update(uuid string, metadata *Metadata) error

	// Delete removes a session metadata entry.
	//
	// Parameters:
	//   - uuid: Session UUID to delete
	//
	// Returns error if database operation fails.
	// Does not error if session doesn't exist.
	Delete(uuid string) error

	// List returns all session metadata entries.
	//
	// Returns:
	//   - Slice of all sessions (empty if none exist)
	//   - Error for database failures
	List() ([]*Metadata, error)

	// SetName assigns or updates a session's friendly name.
	//
	// Parameters:
	//   - uuid: Session UUID
	//   - name: New friendly name
	//
	// Returns error if:
	//   - UUID is invalid
	//   - Name is already taken by another session
	//   - Database operation fails
	SetName(uuid, name string) error

	// Close closes the database connection and releases resources.
	//
	// Returns error if database cannot be closed cleanly.
	Close() error
}

// Config contains session manager configuration.
type Config struct {
	// DBPath is the BoltDB file path.
	DBPath string

	// Timeout is the database operation timeout (default: 1 second).
	Timeout time.Duration
}
