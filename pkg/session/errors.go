package session

import "errors"

// Common errors returned by the session manager.
var (
	// ErrSessionNotFound is returned when a session is not found.
	ErrSessionNotFound = errors.New("session not found")

	// ErrInvalidUUID is returned when a UUID format is invalid.
	ErrInvalidUUID = errors.New("invalid UUID format")

	// ErrNameConflict is returned when a session name is already taken.
	ErrNameConflict = errors.New("session name already exists")

	// ErrEmptyName is returned when a session name is empty.
	ErrEmptyName = errors.New("session name cannot be empty")

	// ErrInvalidMetadata is returned when metadata is invalid.
	ErrInvalidMetadata = errors.New("invalid metadata")
)
