package display

import (
	"testing"
	"time"
)

func TestParseWindow_TodayReturnsMidnightInLocation(t *testing.T) {
	t.Parallel()

	loc := time.FixedZone("test", 9*3600) // UTC+9, mimics Asia/Seoul
	now := time.Date(2026, 5, 6, 14, 37, 12, 0, loc)

	got, err := ParseWindow("today", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 6, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got.Location() != loc {
		t.Errorf("location = %v, want %v", got.Location(), loc)
	}
}

func TestParseWindow_EmptyStringTreatedAsToday(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 14, 37, 12, 0, time.UTC)
	got, err := ParseWindow("", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseWindow_AllReturnsZeroTime(t *testing.T) {
	t.Parallel()

	now := time.Now()
	got, err := ParseWindow("all", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("got %v, want zero time.Time{}", got)
	}
}

func TestParseWindow_DaysSubtracts24Hours(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	got, err := ParseWindow("7d", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(-7 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseWindow_HoursSubtractsHours(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	got, err := ParseWindow("24h", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(-24 * time.Hour)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseWindow_CaseInsensitive(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		input string
	}{
		{"upper-today", "TODAY"},
		{"mixed-today", "Today"},
		{"upper-all", "ALL"},
		{"mixed-all", "All"},
		{"with-spaces", "  today  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseWindow(tc.input, now); err != nil {
				t.Errorf("ParseWindow(%q) returned error: %v", tc.input, err)
			}
		})
	}
}

func TestParseWindow_MalformedReturnsError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	cases := []string{
		"abcd", // no recognized format
		"7",    // missing unit
		"7x",   // unknown unit
		"-1d",  // negative
		"1.5h", // non-integer
		"d",    // empty number
		"h",    // empty number
		"0d",   // zero is not a meaningful window
		"week", // unsupported unit
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseWindow(s, now); err == nil {
				t.Errorf("ParseWindow(%q) returned nil error, want error", s)
			}
		})
	}
}

func TestParseWindow_LargeNumberOfDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	got, err := ParseWindow("365d", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := now.Add(-365 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
