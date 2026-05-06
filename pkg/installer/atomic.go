package installer

import (
	"fmt"
	"os"
	"path/filepath"
)

// atomicWriteFile writes content to path using a write-then-rename pattern so
// the destination is never observed in a half-written state. Power loss, OOM,
// or signal-induced exits between O_TRUNC and the final byte cannot truncate
// the user's existing file (~/.claude.json can be hundreds of KB).
//
// Behaviour:
//   - Refuses to write through a symlink. If path is a symlink, returns an
//     error that resolves the target so the user knows what would have been
//     rewritten (a dotfiles symlink to a managed repo, for example).
//   - Preserves the existing file's permission bits when present; falls back
//     to the provided mode for new files.
//   - Creates the temp file in the SAME directory as path so the final
//     os.Rename is a same-filesystem rename (atomic on POSIX).
//   - Best-effort cleanup of the temp file on any failure.
func atomicWriteFile(path string, content []byte, mode os.FileMode) error {
	// Refuse symlinks before any write — silently following the link would
	// rewrite a target outside the expected location (e.g., dotfiles repo).
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		real, _ := filepath.EvalSymlinks(path)
		return fmt.Errorf(
			"refusing to write through symlink %s -> %s; "+
				"resolve manually or remove the symlink",
			path, real)
	}

	// Capture existing mode so we don't widen a 0600 user secret to 0644.
	finalMode := mode
	if info, err := os.Stat(path); err == nil {
		finalMode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".token-monitor-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	// Track success so we can clean up on any failure path.
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, finalMode); err != nil {
		return fmt.Errorf("chmod temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpPath, path, err)
	}
	committed = true
	return nil
}
