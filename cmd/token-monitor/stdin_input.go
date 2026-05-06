package main

import (
	"encoding/json"
	"io"
	"os"
)

// StatuslineInput captures the subset of Claude Code's statusline JSON that
// token-monitor uses. Claude Code passes this object to the statusline
// command on stdin once per refresh.
//
// The JSON envelope contains many more fields (cost, output_style,
// context tokens, etc.); we deliberately only deserialize what we need so
// schema additions on Claude Code's side don't break us.
type StatuslineInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Model          struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
}

// readStatuslineInput reads stdin if it has piped data and parses it as
// StatuslineInput. Returns nil when:
//
//   - stdin is a TTY (interactive run, not invoked by Claude Code)
//   - stdin has no data
//   - the read fails
//   - the JSON is malformed (intentionally silent — statusline must not error)
//
// Statusline commands run on every prompt refresh; an error here would
// flood the user's terminal. Silent fallback to nil lets the rest of the
// status command proceed with normal session discovery.
func readStatuslineInput() *StatuslineInput {
	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) != 0 {
		return nil
	}
	return readStatuslineInputFrom(os.Stdin)
}

// readStatuslineInputFrom is the testable core of readStatuslineInput.
// It does not perform the TTY check — callers that already know they have
// piped data (the public function and tests) call this directly.
func readStatuslineInputFrom(r io.Reader) *StatuslineInput {
	data, err := io.ReadAll(r)
	if err != nil || len(data) == 0 {
		return nil
	}
	return parseStatuslineInput(data)
}

// parseStatuslineInput unmarshals raw JSON bytes into a StatuslineInput.
// Returns nil on any parse error (see readStatuslineInput for rationale).
func parseStatuslineInput(data []byte) *StatuslineInput {
	if len(data) == 0 {
		return nil
	}
	var in StatuslineInput
	if err := json.Unmarshal(data, &in); err != nil {
		return nil
	}
	return &in
}
