package config

import "errors"

// Common errors returned by the config package.
var (
	// ErrNoClaudeDirs is returned when no Claude config directories are specified.
	ErrNoClaudeDirs = errors.New("no Claude config directories specified")

	// ErrInvalidWatchInterval is returned when watch interval is <= 0.
	ErrInvalidWatchInterval = errors.New("invalid watch interval: must be > 0")

	// ErrInvalidUpdateFrequency is returned when update frequency is <= 0.
	ErrInvalidUpdateFrequency = errors.New("invalid update frequency: must be > 0")

	// ErrInvalidSessionRetention is returned when session retention is <= 0.
	ErrInvalidSessionRetention = errors.New("invalid session retention: must be > 0")

	// ErrInvalidWorkerPoolSize is returned when worker pool size is <= 0.
	ErrInvalidWorkerPoolSize = errors.New("invalid worker pool size: must be > 0")

	// ErrInvalidCacheSize is returned when cache size is <= 0.
	ErrInvalidCacheSize = errors.New("invalid cache size: must be > 0")

	// ErrInvalidBatchWindow is returned when batch window is <= 0.
	ErrInvalidBatchWindow = errors.New("invalid batch window: must be > 0")

	// ErrInvalidDisplayMode is returned when display mode is not recognized.
	ErrInvalidDisplayMode = errors.New("invalid display mode: must be live, compact, table, or json")

	// ErrInvalidRefreshRate is returned when refresh rate is <= 0.
	ErrInvalidRefreshRate = errors.New("invalid refresh rate: must be > 0")

	// ErrInvalidLogLevel is returned when log level is not recognized.
	ErrInvalidLogLevel = errors.New("invalid log level: must be debug, info, warn, or error")

	// ErrInvalidLogFormat is returned when log format is not recognized.
	ErrInvalidLogFormat = errors.New("invalid log format: must be text or json")

	// ErrConfigNotFound is returned when config file is not found.
	ErrConfigNotFound = errors.New("config file not found")

	// ErrInvalidYAML is returned when config file has invalid YAML syntax.
	ErrInvalidYAML = errors.New("invalid YAML syntax in config file")
)
