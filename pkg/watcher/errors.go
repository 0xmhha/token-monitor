package watcher

import "errors"

// Common errors returned by the watcher.
var (
	// ErrWatcherClosed is returned when attempting to use a closed watcher.
	ErrWatcherClosed = errors.New("watcher is closed")

	// ErrAlreadyStarted is returned when Start is called on a running watcher.
	ErrAlreadyStarted = errors.New("watcher already started")

	// ErrNotStarted is returned when Stop is called on a non-running watcher.
	ErrNotStarted = errors.New("watcher not started")

	// ErrCircuitBreakerOpen is returned when the circuit breaker is open.
	ErrCircuitBreakerOpen = errors.New("circuit breaker open")

	// ErrInvalidPath is returned when a watch path is invalid.
	ErrInvalidPath = errors.New("invalid watch path")
)
