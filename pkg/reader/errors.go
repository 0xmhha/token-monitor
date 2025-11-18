package reader

import "errors"

// Common errors returned by the reader.
var (
	// ErrFileLocked is returned when a file is locked by another process.
	ErrFileLocked = errors.New("file is locked")

	// ErrFileNotFound is returned when a file does not exist.
	ErrFileNotFound = errors.New("file not found")

	// ErrPermissionDenied is returned when file access is denied.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrFileTooLarge is returned when a file exceeds the maximum size.
	ErrFileTooLarge = errors.New("file exceeds maximum size")

	// ErrFileTruncated is returned when a file was truncated (offset > size).
	ErrFileTruncated = errors.New("file was truncated")

	// ErrInvalidOffset is returned when an offset is negative.
	ErrInvalidOffset = errors.New("invalid offset")

	// ErrReaderClosed is returned when using a closed reader.
	ErrReaderClosed = errors.New("reader is closed")
)
