package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const (
	// MaxFileSize is the maximum allowed JSONL file size (100MB).
	// Files larger than this will be rejected to prevent memory exhaustion.
	MaxFileSize = 100 * 1024 * 1024

	// MaxLineLength is the maximum allowed line length (1MB).
	// Lines longer than this will be truncated in error messages.
	MaxLineLength = 1024 * 1024
)

// Parser provides methods for parsing Claude Code JSONL files.
type Parser interface {
	// ParseFile reads a JSONL file from the given offset and returns
	// the parsed entries along with the new file offset.
	//
	// Parameters:
	//   - path: Path to the JSONL file
	//   - offset: Byte offset to start reading from (0 for beginning)
	//
	// Returns:
	//   - Slice of successfully parsed entries
	//   - New offset position after reading
	//   - Error if file cannot be read or is too large
	//
	// Malformed lines are logged and skipped rather than causing failure.
	// The returned offset can be used for incremental reading.
	//
	// Thread-safety: This method is safe to call concurrently with different files.
	ParseFile(path string, offset int64) ([]UsageEntry, int64, error)

	// ParseLine parses a single JSONL line into a UsageEntry.
	//
	// Parameters:
	//   - line: A single line of JSONL (without newline character)
	//
	// Returns:
	//   - Parsed UsageEntry
	//   - Error if line is not valid JSON or fails validation
	//
	// Thread-safety: This method is thread-safe.
	ParseLine(line string) (*UsageEntry, error)
}

// jsonlParser implements the Parser interface.
type jsonlParser struct {
	// TODO: Add logger for reporting skipped lines
}

// New creates a new Parser instance.
func New() Parser {
	return &jsonlParser{}
}

// ParseFile implements Parser.ParseFile.
func (p *jsonlParser) ParseFile(path string, offset int64) ([]UsageEntry, int64, error) {
	// Check file size
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() > MaxFileSize {
		return nil, 0, fmt.Errorf("%w: size=%d, max=%d",
			ErrFileTooLarge, info.Size(), MaxFileSize)
	}

	// Open file - #nosec G304: path is validated by caller
	f, err := os.Open(path) // nolint:gosec
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			// TODO: Log close error when logger is implemented
			_ = closeErr // Explicitly ignore for now
		}
	}()

	// Seek to offset for incremental reading
	if offset > 0 {
		if _, seekErr := f.Seek(offset, io.SeekStart); seekErr != nil {
			return nil, 0, fmt.Errorf("failed to seek to offset %d: %w", offset, seekErr)
		}
	}

	// Pre-allocate slice with reasonable capacity
	entries := make([]UsageEntry, 0, 100)
	scanner := bufio.NewScanner(f)

	// Set maximum line size
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, MaxLineLength)

	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		entry, parseErr := p.ParseLine(line)
		if parseErr != nil {
			// TODO: Log warning about skipped line
			// For now, skip malformed lines silently
			continue
		}

		entries = append(entries, *entry)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return entries, 0, fmt.Errorf("scanner error at line %d: %w", lineNum, scanErr)
	}

	// Get current position as new offset
	newOffset, seekErr := f.Seek(0, io.SeekCurrent)
	if seekErr != nil {
		// If we can't get offset, return file size
		newOffset = info.Size()
	}

	return entries, newOffset, nil
}

// ParseLine implements Parser.ParseLine.
func (p *jsonlParser) ParseLine(line string) (*UsageEntry, error) {
	if line == "" {
		return nil, fmt.Errorf("%w: empty line", ErrMalformedJSON)
	}

	var entry UsageEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}

	// Validate the parsed entry
	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &entry, nil
}
