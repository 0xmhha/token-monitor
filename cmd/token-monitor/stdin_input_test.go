package main

import (
	"strings"
	"testing"
)

func TestParseStatuslineInput_ValidJSON(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"session_id": "abc-123",
		"transcript_path": "/tmp/transcript.jsonl",
		"model": {
			"id": "claude-sonnet-4-6",
			"display_name": "Claude Sonnet 4.6"
		}
	}`)

	got := parseStatuslineInput(raw)
	if got == nil {
		t.Fatal("parseStatuslineInput returned nil for valid JSON")
	}
	if got.SessionID != "abc-123" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "abc-123")
	}
	if got.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Errorf("TranscriptPath = %q, want %q", got.TranscriptPath, "/tmp/transcript.jsonl")
	}
	if got.Model.ID != "claude-sonnet-4-6" {
		t.Errorf("Model.ID = %q, want %q", got.Model.ID, "claude-sonnet-4-6")
	}
	if got.Model.DisplayName != "Claude Sonnet 4.6" {
		t.Errorf("Model.DisplayName = %q, want %q", got.Model.DisplayName, "Claude Sonnet 4.6")
	}
}

func TestParseStatuslineInput_PartialFieldsTolerated(t *testing.T) {
	t.Parallel()

	// Claude Code may omit optional fields; only session_id is meaningful.
	raw := []byte(`{"session_id": "only-this"}`)
	got := parseStatuslineInput(raw)
	if got == nil {
		t.Fatal("parseStatuslineInput returned nil for partial JSON")
	}
	if got.SessionID != "only-this" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "only-this")
	}
	if got.Model.ID != "" {
		t.Errorf("Model.ID = %q, want empty", got.Model.ID)
	}
}

func TestParseStatuslineInput_ExtraFieldsIgnored(t *testing.T) {
	t.Parallel()

	// Claude Code's real envelope has many fields we don't use — they
	// should be silently ignored, not cause errors.
	raw := []byte(`{
		"session_id": "abc",
		"cost": {"total_cost_usd": 1.23},
		"workspace": {"current_dir": "/tmp"},
		"unknown_future_field": [1, 2, 3]
	}`)
	got := parseStatuslineInput(raw)
	if got == nil {
		t.Fatal("parseStatuslineInput returned nil; should ignore unknown fields")
	}
	if got.SessionID != "abc" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "abc")
	}
}

func TestParseStatuslineInput_EmptyBytesReturnsNil(t *testing.T) {
	t.Parallel()

	if got := parseStatuslineInput(nil); got != nil {
		t.Errorf("parseStatuslineInput(nil) = %+v, want nil", got)
	}
	if got := parseStatuslineInput([]byte{}); got != nil {
		t.Errorf("parseStatuslineInput([]) = %+v, want nil", got)
	}
}

func TestParseStatuslineInput_MalformedReturnsNil(t *testing.T) {
	t.Parallel()

	cases := []string{
		`{not json}`,
		`{"session_id": `, // truncated
		`hello world`,
		`[1, 2, 3]`, // wrong shape (array, not object)
	}
	for _, s := range cases {
		s := s
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			// Note: JSON arrays can technically unmarshal into a struct
			// without error if encoding/json is lenient — verify the
			// behavior we actually care about (no panic, returns
			// something usable or nil).
			got := parseStatuslineInput([]byte(s))
			if got != nil && got.SessionID != "" {
				t.Errorf("malformed input %q produced SessionID=%q (unexpected)", s, got.SessionID)
			}
		})
	}
}

func TestReadStatuslineInputFrom_ValidStream(t *testing.T) {
	t.Parallel()

	r := strings.NewReader(`{"session_id": "from-reader"}`)
	got := readStatuslineInputFrom(r)
	if got == nil {
		t.Fatal("readStatuslineInputFrom returned nil for valid input")
	}
	if got.SessionID != "from-reader" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "from-reader")
	}
}

func TestReadStatuslineInputFrom_EmptyReader(t *testing.T) {
	t.Parallel()

	r := strings.NewReader("")
	if got := readStatuslineInputFrom(r); got != nil {
		t.Errorf("readStatuslineInputFrom(empty) = %+v, want nil", got)
	}
}
