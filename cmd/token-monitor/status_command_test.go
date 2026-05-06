package main

import (
	"strings"
	"testing"

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
