package sessionloader

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
)

// fakeLogger captures Warn calls so tests can assert per-session error
// reporting without spewing to stderr.
type fakeLogger struct {
	warnings []string
}

func (l *fakeLogger) Warn(msg string, _ ...any) {
	l.warnings = append(l.warnings, msg)
}

// fakeReader implements reader.Reader for tests. ReadFrom returns the
// scripted entries for a file path, or the scripted error if set.
type fakeReader struct {
	byPath map[string][]parser.UsageEntry
	errs   map[string]error
	closed bool
}

func (r *fakeReader) Read(_ context.Context, path string) ([]parser.UsageEntry, error) {
	return r.byPath[path], r.errs[path]
}

func (r *fakeReader) ReadFrom(_ context.Context, path string, _ int64) ([]parser.UsageEntry, int64, error) {
	if err := r.errs[path]; err != nil {
		return nil, 0, err
	}
	return r.byPath[path], 0, nil
}

func (r *fakeReader) Reset(_ string) error { return nil }

func (r *fakeReader) Close() error {
	r.closed = true
	return nil
}

func TestLoadEntries_Empty(t *testing.T) {
	t.Parallel()

	log := &fakeLogger{}
	entries, err := LoadEntries(
		context.Background(),
		nil,
		func() (reader.Reader, error) { return &fakeReader{}, nil },
		log,
	)
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %d, want 0", len(entries))
	}
	if len(log.warnings) != 0 {
		t.Errorf("warnings = %d, want 0", len(log.warnings))
	}
}

func TestLoadEntries_MergesAcrossSessions(t *testing.T) {
	t.Parallel()

	now := time.Now()
	sessions := []discovery.SessionFile{
		{SessionID: "a", FilePath: "/tmp/a.jsonl"},
		{SessionID: "b", FilePath: "/tmp/b.jsonl"},
	}

	r := &fakeReader{
		byPath: map[string][]parser.UsageEntry{
			"/tmp/a.jsonl": {
				{SessionID: "a", Timestamp: now},
				{SessionID: "a", Timestamp: now.Add(time.Minute)},
			},
			"/tmp/b.jsonl": {
				{SessionID: "b", Timestamp: now},
			},
		},
	}

	log := &fakeLogger{}
	entries, err := LoadEntries(
		context.Background(),
		sessions,
		func() (reader.Reader, error) { return r, nil },
		log,
	)
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("entries = %d, want 3", len(entries))
	}
	if !r.closed {
		t.Error("expected reader Close() called via defer")
	}
}

func TestLoadEntries_PerSessionFailureIsSkippedAndLogged(t *testing.T) {
	t.Parallel()

	sessions := []discovery.SessionFile{
		{SessionID: "ok-1", FilePath: "/tmp/ok1.jsonl"},
		{SessionID: "broken", FilePath: "/tmp/broken.jsonl"},
		{SessionID: "ok-2", FilePath: "/tmp/ok2.jsonl"},
	}

	r := &fakeReader{
		byPath: map[string][]parser.UsageEntry{
			"/tmp/ok1.jsonl": {{SessionID: "ok-1"}},
			"/tmp/ok2.jsonl": {{SessionID: "ok-2"}, {SessionID: "ok-2"}},
		},
		errs: map[string]error{
			"/tmp/broken.jsonl": errors.New("disk on fire"),
		},
	}

	log := &fakeLogger{}
	entries, err := LoadEntries(
		context.Background(),
		sessions,
		func() (reader.Reader, error) { return r, nil },
		log,
	)
	if err != nil {
		t.Fatalf("LoadEntries should not propagate per-session error, got: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("entries = %d, want 3 (1 + 0 + 2)", len(entries))
	}
	if len(log.warnings) != 1 {
		t.Errorf("warnings = %d, want 1 (one for the broken session)", len(log.warnings))
	}
}

func TestLoadEntries_FactoryFailurePropagates(t *testing.T) {
	t.Parallel()

	log := &fakeLogger{}
	_, err := LoadEntries(
		context.Background(),
		[]discovery.SessionFile{{SessionID: "a", FilePath: "/tmp/a.jsonl"}},
		func() (reader.Reader, error) { return nil, errors.New("no reader for you") },
		log,
	)
	if err == nil {
		t.Fatal("expected error from factory failure, got nil")
	}
}
