package display

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatCompact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
		want  string
	}{
		{"zero", 0, "0"},
		{"one", 1, "1"},
		{"just below threshold", 999, "999"},
		{"exactly one thousand", 1000, "1.0K"},
		{"rounds down to 1.0K", 1049, "1.0K"},
		{"rounds up to 1.1K", 1050, "1.1K"},
		{"mid-range K", 12534, "12.5K"},
		{"near top of K range", 99999, "100.0K"},
		{"just below one million", 999999, "1000.0K"},
		{"exactly one million", 1000000, "1.0M"},
		{"1.2M", 1200000, "1.2M"},
		{"1.5M", 1500000, "1.5M"},
		{"10.0M", 10000000, "10.0M"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatCompact(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{"zero", 0, "0s"},
		{"seconds only", 12 * time.Second, "12s"},
		{"minutes only", 45 * time.Minute, "45m"},
		{"hours and minutes", 3*time.Hour + 42*time.Minute, "3h42m"},
		{"exactly one hour", 1 * time.Hour, "1h0m"},
		{"ninety minutes", 90 * time.Minute, "1h30m"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatDuration(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatRate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{"zero", 0.0, "0.0"},
		{"large rate", 2145.3, "2145.3"},
		{"small rate", 0.1, "0.1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatRate(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatTokenCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
		want  string
	}{
		{"zero", 0, "0"},
		{"below comma threshold", 999, "999"},
		{"exactly one thousand", 1000, "1,000"},
		{"mid-range", 12534, "12,534"},
		{"millions", 1234567, "1,234,567"},
		{"negative thousand", -1000, "-1,000"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := FormatTokenCount(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
