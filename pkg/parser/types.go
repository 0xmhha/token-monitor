// Package parser provides JSONL parsing functionality for Claude Code
// usage logs. It extracts token usage metrics from JSONL files and
// validates entries for correctness.
//
// The parser is designed to handle malformed lines gracefully by
// logging warnings and skipping invalid entries rather than failing.
//
// Example usage:
//
//	p := parser.New()
//	entries, offset, err := p.ParseFile("/path/to/session.jsonl", 0)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, entry := range entries {
//	    fmt.Printf("Tokens: %d\n", entry.Message.Usage.InputTokens)
//	}
package parser

import (
	"time"
)

// UsageEntry represents a single entry from Claude Code's JSONL log file.
// Each entry corresponds to one API call made by Claude Code.
//
// Invariant: Timestamp must not be zero value.
// Invariant: SessionID must be a valid UUID.
// Invariant: Message.Usage token counts must be non-negative.
type UsageEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	SessionID  string    `json:"sessionId"`
	Version    string    `json:"version"`
	CurrentDir string    `json:"cwd"`
	Message    Message   `json:"message"`
	CostUSD    *float64  `json:"costUSD,omitempty"`
	RequestID  *string   `json:"requestId,omitempty"`
}

// Message contains the API response details including token usage.
type Message struct {
	ID      string    `json:"id"`
	Model   string    `json:"model"`
	Usage   Usage     `json:"usage"`
	Content []Content `json:"content"`
}

// Usage contains token consumption metrics for a single API call.
//
// Token types:
// - InputTokens: Regular input tokens (charged at standard rate)
// - OutputTokens: Generated output tokens (charged at higher rate)
// - CacheCreationInputTokens: Tokens written to cache (1/2 price of input)
// - CacheReadInputTokens: Tokens read from cache (1/10 price of input)
//
// Invariant: All token counts must be >= 0.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// Content represents a content block in the message.
type Content struct {
	Type string  `json:"type"`
	Text *string `json:"text,omitempty"`
}

// TotalTokens returns the sum of all token types.
func (u Usage) TotalTokens() int {
	return u.InputTokens + u.OutputTokens +
		u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// Validate checks if the usage entry satisfies all invariants.
//
// Returns an error if:
//   - Timestamp is zero value
//   - SessionID is empty
//   - Any token count is negative
//   - Model is empty
//
// Thread-safety: This method is read-only and thread-safe.
func (e *UsageEntry) Validate() error {
	if e.Timestamp.IsZero() {
		return ErrInvalidTimestamp
	}

	if e.SessionID == "" {
		return ErrInvalidSessionID
	}

	if e.Message.Model == "" {
		return ErrInvalidModel
	}

	if err := e.Message.Usage.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate checks if all token counts are non-negative.
func (u Usage) Validate() error {
	if u.InputTokens < 0 {
		return ErrNegativeTokenCount
	}
	if u.OutputTokens < 0 {
		return ErrNegativeTokenCount
	}
	if u.CacheCreationInputTokens < 0 {
		return ErrNegativeTokenCount
	}
	if u.CacheReadInputTokens < 0 {
		return ErrNegativeTokenCount
	}
	return nil
}
