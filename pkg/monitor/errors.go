package monitor

import "errors"

var (
	// ErrMonitorClosed is returned when operations are attempted on a closed monitor.
	ErrMonitorClosed = errors.New("monitor is closed")

	// ErrMonitorRunning is returned when trying to start an already running monitor.
	ErrMonitorRunning = errors.New("monitor is already running")

	// ErrMonitorNotRunning is returned when trying to stop a non-running monitor.
	ErrMonitorNotRunning = errors.New("monitor is not running")

	// ErrNoSessions is returned when no sessions are found to monitor.
	ErrNoSessions = errors.New("no sessions found to monitor")

	// ErrInvalidConfig is returned when the configuration is invalid.
	ErrInvalidConfig = errors.New("invalid monitor configuration")
)
