package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/token-monitor/pkg/aggregator"
)

func TestAbbreviateModel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		model string
		want  string
	}{
		{"claude-sonnet-4-6", "son"},
		{"claude-opus-4-7", "opus"},
		{"claude-haiku-3-5", "hai"},
		{"CLAUDE-SONNET-4-6", "son"}, // case-insensitive substring match
		{"gpt-4-turbo", "gpt-"},      // unknown -> first 4 chars (lowercased)
		{"abc", "abc"},               // shorter than 4 chars, returned as-is
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.model, func(t *testing.T) {
			t.Parallel()
			if got := abbreviateModel(tc.model); got != tc.want {
				t.Errorf("abbreviateModel(%q) = %q, want %q", tc.model, got, tc.want)
			}
		})
	}
}

func TestWindowLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		window string
		want   string
	}{
		{"today", "day"},
		{"TODAY", "day"},
		{"", "day"},
		{"  today  ", "day"},
		{"7d", "7d"},
		{"24h", "24h"},
		{"all", "all"},
	}
	for _, tc := range cases {
		t.Run(tc.window, func(t *testing.T) {
			t.Parallel()
			if got := windowLabel(tc.window); got != tc.want {
				t.Errorf("windowLabel(%q) = %q, want %q", tc.window, got, tc.want)
			}
		})
	}
}

func TestFormatBreakdown_TotalAndModelsSorted(t *testing.T) {
	t.Parallel()

	// Use a map with deliberate non-alphabetical insertion order to
	// confirm the sort stabilizes output.
	breakdown := map[string]aggregator.ModelBreakdown{
		"claude-opus-4-7":   {Model: "claude-opus-4-7", TotalTokens: 212_000},
		"claude-sonnet-4-6": {Model: "claude-sonnet-4-6", TotalTokens: 128_000},
	}
	got := formatBreakdown("today", breakdown)

	// Total = 340K, sorted abbreviations: opus < son, so opus comes first.
	want := "day:340.0K | opus:212.0K | son:128.0K"
	if got != want {
		t.Errorf("formatBreakdown =\n  %q\nwant\n  %q", got, want)
	}
}

func TestFormatBreakdown_EmptyMap(t *testing.T) {
	t.Parallel()

	got := formatBreakdown("today", map[string]aggregator.ModelBreakdown{})
	want := "day:0"
	if got != want {
		t.Errorf("formatBreakdown(empty) = %q, want %q", got, want)
	}
}

func TestFormatBreakdown_NilMap(t *testing.T) {
	t.Parallel()

	got := formatBreakdown("7d", nil)
	want := "7d:0"
	if got != want {
		t.Errorf("formatBreakdown(nil) = %q, want %q", got, want)
	}
}

func TestFormatBreakdown_SingleModel(t *testing.T) {
	t.Parallel()

	breakdown := map[string]aggregator.ModelBreakdown{
		"claude-sonnet-4-6": {Model: "claude-sonnet-4-6", TotalTokens: 128_000},
	}
	got := formatBreakdown("24h", breakdown)
	want := "24h:128.0K | son:128.0K"
	if got != want {
		t.Errorf("formatBreakdown =\n  %q\nwant\n  %q", got, want)
	}
}

func TestFormatBreakdown_MergesAbbreviations(t *testing.T) {
	t.Parallel()

	// Two different exact opus identifiers must collapse into one
	// "opus:" row in the compact output.
	breakdown := map[string]aggregator.ModelBreakdown{
		"claude-opus-4-7": {Model: "claude-opus-4-7", TotalTokens: 100_000},
		"claude-opus-4-1": {Model: "claude-opus-4-1", TotalTokens: 50_000},
	}
	got := formatBreakdown("today", breakdown)
	want := "day:150.0K | opus:150.0K"
	if got != want {
		t.Errorf("formatBreakdown =\n  %q\nwant\n  %q", got, want)
	}
}

func TestFormatBreakdown_TotalSumsAllModels(t *testing.T) {
	t.Parallel()

	breakdown := map[string]aggregator.ModelBreakdown{
		"claude-sonnet-4-6": {Model: "claude-sonnet-4-6", TotalTokens: 100_000},
		"claude-opus-4-7":   {Model: "claude-opus-4-7", TotalTokens: 50_000},
		"claude-haiku-3-5":  {Model: "claude-haiku-3-5", TotalTokens: 10_000},
	}
	got := formatBreakdown("today", breakdown)

	// Verify total = 160K appears at the start.
	if !strings.HasPrefix(got, "day:160.0K |") {
		t.Errorf("expected total prefix 'day:160.0K |', got %q", got)
	}
	// Verify all three model abbreviations are present.
	for _, abbr := range []string{"hai:", "opus:", "son:"} {
		if !strings.Contains(got, abbr) {
			t.Errorf("expected %q in output, got %q", abbr, got)
		}
	}
}

// TestExecute_BreakdownWithWatchRejected verifies that combining --breakdown
// with --watch returns an explicit error rather than silently dropping one
// of the flags. Previously printBreakdown returned before the watch check,
// so --watch was a no-op alongside --breakdown.
func TestExecute_BreakdownWithWatchRejected(t *testing.T) {
	t.Parallel()

	cmd := &statusCommand{breakdown: true, watch: true}
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when combining --breakdown with --watch, got nil")
	}
	if !strings.Contains(err.Error(), "--watch") || !strings.Contains(err.Error(), "--breakdown") {
		t.Errorf("expected error to name both flags, got %q", err.Error())
	}
}

// TestExecute_BreakdownWithCurrentRejected verifies that --breakdown with a
// single-session selector (--current or --session) is rejected. Breakdown is
// inherently cross-session so the combination is incoherent.
func TestExecute_BreakdownWithCurrentRejected(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cmd  *statusCommand
	}{
		{name: "current", cmd: &statusCommand{breakdown: true, current: true}},
		{name: "session", cmd: &statusCommand{breakdown: true, sessionID: "abc"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cmd.Execute()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "--breakdown") {
				t.Errorf("expected error to mention --breakdown, got %q", err.Error())
			}
		})
	}
}

// TestPrintBreakdown_AcrossSessions exercises the breakdown path end-to-end:
// it constructs two synthetic sessions in a temp dir (one sonnet-only, one
// opus-only), points config at it via CLAUDE_CONFIG_DIR, runs Execute(), and
// asserts the printed line contains both model abbreviations and a total
// equal to the sum across both sessions.
//
// This test is NOT t.Parallel(): it mutates CLAUDE_CONFIG_DIR and replaces
// os.Stdout for the duration of the call, both of which are process-global.
func TestPrintBreakdown_AcrossSessions(t *testing.T) {
	tmpDir := t.TempDir()

	// Two project subdirs, each with one session JSONL. Discovery treats
	// the configured CLAUDE_CONFIG_DIR as the *base* dir and scans its
	// immediate subdirectories for project-hashed folders containing
	// session UUIDs — see pkg/discovery/discovery.go scanBaseDirectory.
	projectA := filepath.Join(tmpDir, "project-a")
	projectB := filepath.Join(tmpDir, "project-b")
	if err := os.MkdirAll(projectA, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectB, 0700); err != nil {
		t.Fatal(err)
	}

	// Use timestamps a few minutes ago so they fall inside the "today"
	// window regardless of when the test runs (as long as it doesn't run
	// across midnight, which would also trip the production code).
	now := time.Now().UTC()
	tsA1 := now.Add(-10 * time.Minute).Format(time.RFC3339)
	tsA2 := now.Add(-5 * time.Minute).Format(time.RFC3339)
	tsB1 := now.Add(-7 * time.Minute).Format(time.RFC3339)

	sessionA := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	sessionB := "b2c3d4e5-f6a7-8901-bcde-f12345678901"

	// Session A: 100 + 50 = 150 sonnet tokens.
	contentA := `{"timestamp":"` + tsA1 + `","sessionId":"` + sessionA + `","message":{"id":"m1","model":"claude-sonnet-4-6","usage":{"input_tokens":80,"output_tokens":20}}}
{"timestamp":"` + tsA2 + `","sessionId":"` + sessionA + `","message":{"id":"m2","model":"claude-sonnet-4-6","usage":{"input_tokens":40,"output_tokens":10}}}
`
	// Session B: 200 + 100 = 300 opus tokens.
	contentB := `{"timestamp":"` + tsB1 + `","sessionId":"` + sessionB + `","message":{"id":"m3","model":"claude-opus-4-7","usage":{"input_tokens":200,"output_tokens":100}}}
`

	if err := os.WriteFile(filepath.Join(projectA, sessionA+".jsonl"), []byte(contentA), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectB, sessionB+".jsonl"), []byte(contentB), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)
	t.Setenv("TOKEN_MONITOR_LOG_LEVEL", "error")

	out := captureStdout(t, func() {
		cmd := &statusCommand{breakdown: true, window: "today"}
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	got := strings.TrimSpace(out)

	// Both abbreviations must appear; the cross-session total is 450.
	for _, want := range []string{"day:", "son:", "opus:", "450"} {
		if !strings.Contains(got, want) {
			t.Errorf("breakdown output missing %q\n  got: %q", want, got)
		}
	}
}

// TestPrintBreakdown_EmptyResult covers the "everything filtered out" case:
// a session exists but its only entry sits outside the requested window, so
// the line collapses to "<label>:0".
func TestPrintBreakdown_EmptyResult(t *testing.T) {
	tmpDir := t.TempDir()

	project := filepath.Join(tmpDir, "project-old")
	if err := os.MkdirAll(project, 0700); err != nil {
		t.Fatal(err)
	}

	// Timestamp is 10 days ago — outside the "today" window.
	old := time.Now().Add(-10 * 24 * time.Hour).UTC().Format(time.RFC3339)
	sess := "c3d4e5f6-a7b8-9012-cdef-123456789012"
	content := `{"timestamp":"` + old + `","sessionId":"` + sess + `","message":{"id":"m1","model":"claude-sonnet-4-6","usage":{"input_tokens":50,"output_tokens":10}}}
`
	if err := os.WriteFile(filepath.Join(project, sess+".jsonl"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)
	t.Setenv("TOKEN_MONITOR_LOG_LEVEL", "error")

	out := captureStdout(t, func() {
		cmd := &statusCommand{breakdown: true, window: "today"}
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	got := strings.TrimSpace(out)
	if got != "day:0" {
		t.Errorf("expected empty-window output 'day:0', got %q", got)
	}
}

// captureStdout replaces os.Stdout with a pipe for the duration of fn, then
// restores it and returns whatever fn wrote. If anything in the plumbing
// fails the test is aborted via t.Fatal — this helper is only used in tests
// that genuinely need to read what Execute() printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		buf, _ := io.ReadAll(r)
		done <- string(buf)
	}()

	defer func() {
		os.Stdout = orig
	}()

	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("pipe writer close: %v", err)
	}
	return <-done
}
