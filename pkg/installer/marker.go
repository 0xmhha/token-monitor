// Package installer provides idempotent installation of token-monitor into
// Claude Code: statusline scripts, MCP server registration, and PostToolUse
// hooks.
//
// All file mutations follow two safety invariants:
//
//  1. Marker-based block identification — managed regions are bracketed by
//     well-known sentinel strings so the user's surrounding content is
//     preserved across install / uninstall cycles.
//  2. Backup-before-write — every mutation produces a timestamped *.bak.* file
//     so a hand-edit accident can be reverted by `mv` of the backup.
package installer

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Marker constants delimit the managed block in shell / text files.
//
// They are hardcoded (not configurable) so multiple invocations of the
// installer — even across versions — recognise each other's blocks.
const (
	// MarkerStart begins the managed block.
	MarkerStart = "# >>> token-monitor >>> (managed block, do not edit)"
	// MarkerEnd ends the managed block.
	MarkerEnd = "# <<< token-monitor <<<"
)

// PatchMarkerBlock returns updated content with the managed block replaced or
// appended.
//
// Behaviour:
//   - If body is empty, the existing block (if any) is removed.
//   - If a block is already present, its inner body is replaced with the new
//     body (idempotent — same body produces identical output).
//   - If no block is present and body is non-empty, the block is appended at
//     the end of content (with a leading newline if needed).
//
// The body argument is the *full* block content including the marker lines.
// This keeps the marker strings in one canonical place — the caller embeds
// MarkerStart / MarkerEnd at the start / end of `body` exactly as they want
// them rendered.
func PatchMarkerBlock(content, body string) string {
	startIdx := strings.Index(content, MarkerStart)

	if startIdx == -1 {
		// No existing block.
		if body == "" {
			return content
		}
		return appendBlock(content, body)
	}

	// Find the end marker after the start.
	endIdx := strings.Index(content[startIdx:], MarkerEnd)
	if endIdx == -1 {
		// Malformed — start present but no end. Treat as if absent and append.
		// We deliberately do NOT mutate the malformed region; the user must
		// inspect manually.
		if body == "" {
			return content
		}
		return appendBlock(content, body)
	}
	endIdx += startIdx + len(MarkerEnd)

	// Extend the removed range to include a single trailing newline so we don't
	// leave a blank line behind on uninstall.
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	if body == "" {
		// Uninstall: remove the block.
		return content[:startIdx] + content[endIdx:]
	}

	// Replace the block. Ensure body terminates with newline.
	replacement := body
	if !strings.HasSuffix(replacement, "\n") {
		replacement += "\n"
	}
	return content[:startIdx] + replacement + content[endIdx:]
}

// appendBlock concatenates body to content with sane separator handling.
// Ensures one blank line between existing content and the block when content
// is non-empty and does not already end with two newlines.
func appendBlock(content, body string) string {
	if content == "" {
		out := body
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		return out
	}

	var sep string
	switch {
	case strings.HasSuffix(content, "\n\n"):
		sep = ""
	case strings.HasSuffix(content, "\n"):
		sep = "\n"
	default:
		sep = "\n\n"
	}

	out := content + sep + body
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}

// HasMarkerBlock reports whether content contains both markers (start before
// end). Used by callers to detect already-installed state.
func HasMarkerBlock(content string) bool {
	startIdx := strings.Index(content, MarkerStart)
	if startIdx == -1 {
		return false
	}
	endIdx := strings.Index(content[startIdx:], MarkerEnd)
	return endIdx != -1
}

// BackupFile copies path to path.bak.YYYYMMDD-HHMMSS using the local clock and
// returns the new backup path.
//
// If the source file does not exist, BackupFile is a no-op and returns ("",
// nil) — install flows that create a brand-new file have nothing to back up.
// Other errors (permission denied, I/O failure) are returned verbatim.
func BackupFile(path string) (string, error) {
	src, err := os.Open(path) //nolint:gosec // path is supplied by caller (CLI flag / well-known location)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("backup: open source: %w", err)
	}
	defer func() { _ = src.Close() }() //nolint:errcheck // best-effort close after read

	info, err := src.Stat()
	if err != nil {
		return "", fmt.Errorf("backup: stat source: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.bak.%s", path, timestamp)

	// Collision avoidance: when install + uninstall fire in the same second
	// the timestamp matches an existing backup. Append a short suffix
	// (.1, .2, ...) until O_EXCL succeeds. Cap at 100 attempts so a stuck
	// filesystem can't loop forever.
	// #nosec G304 -- path is caller-supplied; BackupFile is an intentional file-copy primitive
	dst, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	for attempt := 1; err != nil && os.IsExist(err) && attempt < 100; attempt++ {
		backupPath = fmt.Sprintf("%s.bak.%s.%d", path, timestamp, attempt)
		// #nosec G304 -- path is caller-supplied; BackupFile is an intentional file-copy primitive
		dst, err = os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	}
	if err != nil {
		return "", fmt.Errorf("backup: create %s: %w", backupPath, err)
	}
	defer func() { _ = dst.Close() }() //nolint:errcheck // best-effort close after copy

	if _, copyErr := io.Copy(dst, src); copyErr != nil {
		return "", fmt.Errorf("backup: copy: %w", copyErr)
	}
	return backupPath, nil
}
