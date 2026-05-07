// Package sessionloader centralizes the "create reader, iterate session
// files, accumulate entries" pattern shared by status output, MCP breakdown
// tools, and any other consumer that needs the raw entry stream from a set
// of sessions.
//
// Per-session read errors are logged at Warn level and skipped — a single
// corrupt JSONL file should not poison cross-session aggregation.
package sessionloader

import (
	"context"
	"fmt"

	"github.com/0xmhha/token-monitor/pkg/discovery"
	"github.com/0xmhha/token-monitor/pkg/parser"
	"github.com/0xmhha/token-monitor/pkg/reader"
)

// Logger is the minimal logging contract this package needs. It is a
// structural subset of pkg/logger.Logger and pkg/mcp.Logger so callers
// in either package can pass their existing logger directly.
type Logger interface {
	Warn(msg string, keysAndValues ...any)
}

// ReaderFactory produces a fresh Reader for one LoadEntries call.
// The factory is used (not a pre-built Reader) so that each call gets
// an isolated position store / scratch state.
type ReaderFactory func() (reader.Reader, error)

// LoadEntries reads usage entries from each of the given session files
// using a fresh reader. Per-session read failures are logged and skipped;
// the caller receives the union of successful reads.
//
// ctx is forwarded to reader.ReadFrom, allowing cancellation to short-
// circuit a long discovery+read cycle.
func LoadEntries(
	ctx context.Context,
	sessions []discovery.SessionFile,
	factory ReaderFactory,
	log Logger,
) ([]parser.UsageEntry, error) {
	r, err := factory()
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	defer r.Close() //nolint:errcheck

	all := make([]parser.UsageEntry, 0, 1024)
	for _, sess := range sessions {
		entries, _, readErr := r.ReadFrom(ctx, sess.FilePath, 0)
		if readErr != nil {
			log.Warn("failed to read session", "session", sess.SessionID, "error", readErr)
			continue
		}
		all = append(all, entries...)
	}
	return all, nil
}
