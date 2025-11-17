package parser

import "errors"

// Common errors returned by the parser package.
var (
	// ErrInvalidTimestamp is returned when a usage entry has a zero timestamp.
	ErrInvalidTimestamp = errors.New("invalid timestamp: must not be zero")

	// ErrInvalidSessionID is returned when a usage entry has an empty session ID.
	ErrInvalidSessionID = errors.New("invalid session ID: must not be empty")

	// ErrInvalidModel is returned when a usage entry has an empty model name.
	ErrInvalidModel = errors.New("invalid model: must not be empty")

	// ErrNegativeTokenCount is returned when any token count is negative.
	ErrNegativeTokenCount = errors.New("invalid token count: must be non-negative")

	// ErrMalformedJSON is returned when a JSONL line cannot be parsed.
	ErrMalformedJSON = errors.New("malformed JSON line")

	// ErrFileTooLarge is returned when a file exceeds the maximum size limit.
	ErrFileTooLarge = errors.New("file size exceeds maximum limit")
)

// ParseError provides context about a parsing failure.
type ParseError struct {
	Line int    // Line number where error occurred (1-indexed)
	Data string // The malformed line (truncated if too long)
	Err  error  // Underlying error
}

func (e *ParseError) Error() string {
	maxLen := 100
	data := e.Data
	if len(data) > maxLen {
		data = data[:maxLen] + "..."
	}
	return formatError("parse error", e.Line, data, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// ValidationError provides context about a validation failure.
type ValidationError struct {
	Line      int    // Line number where error occurred (1-indexed)
	SessionID string // Session ID being validated
	Err       error  // Underlying error
}

func (e *ValidationError) Error() string {
	return formatError("validation error", e.Line, e.SessionID, e.Err)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// formatError creates a consistent error message format.
func formatError(prefix string, line int, context string, err error) string {
	if line > 0 {
		return prefix + " at line " + itoa(line) + ": " + context + ": " + err.Error()
	}
	return prefix + ": " + context + ": " + err.Error()
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	negative := i < 0
	if negative {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	if negative {
		pos--
		buf[pos] = '-'
	}

	return string(buf[pos:])
}
