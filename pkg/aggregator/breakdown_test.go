package aggregator

import (
	"testing"
	"time"

	"github.com/0xmhha/token-monitor/pkg/parser"
)

// makeEntry is a small helper to build a UsageEntry for tests.
func makeEntry(model string, ts time.Time, in, out, cc, cr int) parser.UsageEntry {
	return parser.UsageEntry{
		SessionID: "session-test",
		Timestamp: ts,
		Message: parser.Message{
			Model: model,
			Usage: parser.Usage{
				InputTokens:              in,
				OutputTokens:             out,
				CacheCreationInputTokens: cc,
				CacheReadInputTokens:     cr,
			},
		},
	}
}

func TestBreakdownByModel_GroupsByModel(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []parser.UsageEntry{
		makeEntry("claude-sonnet-4-6", now, 100, 50, 10, 5),
		makeEntry("claude-sonnet-4-6", now, 200, 100, 20, 10),
		makeEntry("claude-opus-4-7", now, 300, 150, 30, 15),
		makeEntry("claude-haiku-3-5", now, 50, 25, 5, 2),
	}

	got := BreakdownByModel(entries)

	if len(got) != 3 {
		t.Fatalf("want 3 model groups, got %d (%v)", len(got), got)
	}

	sonnet, ok := got["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("missing sonnet group")
	}
	if sonnet.EntryCount != 2 {
		t.Errorf("sonnet EntryCount = %d, want 2", sonnet.EntryCount)
	}
	if sonnet.InputTokens != 300 {
		t.Errorf("sonnet InputTokens = %d, want 300", sonnet.InputTokens)
	}
	if sonnet.OutputTokens != 150 {
		t.Errorf("sonnet OutputTokens = %d, want 150", sonnet.OutputTokens)
	}
	if sonnet.CacheCreate != 30 {
		t.Errorf("sonnet CacheCreate = %d, want 30", sonnet.CacheCreate)
	}
	if sonnet.CacheRead != 15 {
		t.Errorf("sonnet CacheRead = %d, want 15", sonnet.CacheRead)
	}
	// Total = 300 + 150 + 30 + 15 = 495
	if sonnet.TotalTokens != 495 {
		t.Errorf("sonnet TotalTokens = %d, want 495", sonnet.TotalTokens)
	}
	if sonnet.Model != "claude-sonnet-4-6" {
		t.Errorf("sonnet Model = %q, want claude-sonnet-4-6", sonnet.Model)
	}

	opus := got["claude-opus-4-7"]
	if opus.EntryCount != 1 || opus.TotalTokens != 495 {
		t.Errorf("opus = %+v, want EntryCount=1 TotalTokens=495", opus)
	}

	haiku := got["claude-haiku-3-5"]
	if haiku.EntryCount != 1 || haiku.TotalTokens != 82 {
		t.Errorf("haiku = %+v, want EntryCount=1 TotalTokens=82", haiku)
	}
}

func TestBreakdownByModel_SkipsSyntheticAndEmpty(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []parser.UsageEntry{
		makeEntry("<synthetic>", now, 100, 50, 0, 0),
		makeEntry("", now, 200, 100, 0, 0),
		makeEntry("claude-sonnet-4-6", now, 10, 5, 0, 0),
	}

	got := BreakdownByModel(entries)

	if len(got) != 1 {
		t.Fatalf("want 1 model group (synthetic + empty skipped), got %d (%v)", len(got), got)
	}
	if _, ok := got["<synthetic>"]; ok {
		t.Error("<synthetic> should be skipped but was included")
	}
	if _, ok := got[""]; ok {
		t.Error("empty model should be skipped but was included")
	}
	if got["claude-sonnet-4-6"].EntryCount != 1 {
		t.Errorf("expected only the sonnet entry to remain, got %+v", got["claude-sonnet-4-6"])
	}
}

func TestBreakdownByModel_EmptyInput(t *testing.T) {
	t.Parallel()

	got := BreakdownByModel(nil)
	if got == nil {
		t.Fatal("BreakdownByModel(nil) returned nil map, want empty map")
	}
	if len(got) != 0 {
		t.Errorf("want empty map, got %d entries", len(got))
	}

	got = BreakdownByModel([]parser.UsageEntry{})
	if len(got) != 0 {
		t.Errorf("want empty map for empty slice, got %d entries", len(got))
	}
}

func TestMatchModel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		model string
		glob  string
		want  bool
	}{
		{"sonnet-glob-matches-sonnet", "claude-sonnet-4-6", "*sonnet*", true},
		{"sonnet-glob-no-match-opus", "claude-opus-4-7", "*sonnet*", false},
		{"case-insensitive-uppercase-glob", "claude-sonnet-4-6", "*SONNET*", true},
		{"case-insensitive-uppercase-model", "CLAUDE-SONNET-4-6", "*sonnet*", true},
		{"empty-glob-matches-anything", "claude-opus-4-7", "", true},
		{"empty-glob-matches-empty-model", "", "", true},
		{"malformed-glob-returns-false", "claude-sonnet-4-6", "[abc", false},
		{"exact-match", "claude-haiku-3-5", "claude-haiku-3-5", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MatchModel(tc.model, tc.glob); got != tc.want {
				t.Errorf("MatchModel(%q, %q) = %v, want %v", tc.model, tc.glob, got, tc.want)
			}
		})
	}
}

func TestFilterByModelGlob_EmptyGlobPassThrough(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []parser.UsageEntry{
		makeEntry("claude-sonnet-4-6", now, 1, 1, 0, 0),
		makeEntry("claude-opus-4-7", now, 1, 1, 0, 0),
	}

	got := FilterByModelGlob(entries, "")
	if len(got) != len(entries) {
		t.Fatalf("empty glob: want %d entries (pass-through), got %d", len(entries), len(got))
	}
}

func TestFilterByModelGlob_FiltersByPattern(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []parser.UsageEntry{
		makeEntry("claude-sonnet-4-6", now, 1, 1, 0, 0),
		makeEntry("claude-opus-4-7", now, 1, 1, 0, 0),
		makeEntry("claude-sonnet-4-5", now, 1, 1, 0, 0),
	}

	got := FilterByModelGlob(entries, "*sonnet*")
	if len(got) != 2 {
		t.Fatalf("want 2 sonnet entries, got %d", len(got))
	}
	for _, e := range got {
		if e.Message.Model != "claude-sonnet-4-6" && e.Message.Model != "claude-sonnet-4-5" {
			t.Errorf("unexpected model in filtered output: %q", e.Message.Model)
		}
	}
}

func TestFilterSince_CutoffExact(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	entries := []parser.UsageEntry{
		makeEntry("m", base.Add(-1*time.Second), 1, 0, 0, 0), // before cutoff: excluded
		makeEntry("m", base, 1, 0, 0, 0),                     // exactly at cutoff: included
		makeEntry("m", base.Add(1*time.Second), 1, 0, 0, 0),  // after cutoff: included
	}

	got := FilterSince(entries, base)
	if len(got) != 2 {
		t.Fatalf("want 2 entries (>= cutoff), got %d", len(got))
	}
	for _, e := range got {
		if e.Timestamp.Before(base) {
			t.Errorf("entry with timestamp %v should not be included (before cutoff %v)", e.Timestamp, base)
		}
	}
}

func TestFilterSince_ZeroCutoffIncludesAll(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []parser.UsageEntry{
		makeEntry("m", now.Add(-24*time.Hour), 1, 0, 0, 0),
		makeEntry("m", now, 1, 0, 0, 0),
	}

	got := FilterSince(entries, time.Time{})
	if len(got) != len(entries) {
		t.Errorf("zero cutoff: want all %d entries, got %d", len(entries), len(got))
	}
}
