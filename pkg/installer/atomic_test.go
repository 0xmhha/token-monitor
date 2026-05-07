package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAtomicWriteFile_WritesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	want := []byte("hello world\n")
	if err := atomicWriteFile(path, want, 0o644); err != nil {
		t.Fatalf("atomicWriteFile error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content mismatch: got %q, want %q", got, want)
	}
}

func TestAtomicWriteFile_PreservesExistingMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode semantics differ on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.json")

	// Pre-create a file with restrictive 0o600 mode (user secret).
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	// Caller passes 0o644, but we should preserve the existing 0o600.
	if err := atomicWriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("atomicWriteFile error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("expected mode 0600 (preserved), got %o", got)
	}
}

func TestAtomicWriteFile_NewFileUsesProvidedMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file mode semantics differ on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "new.sh")

	if err := atomicWriteFile(path, []byte("#!/bin/bash\n"), 0o755); err != nil {
		t.Fatalf("atomicWriteFile error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Errorf("expected mode 0755 (new file uses provided), got %o", got)
	}
}

func TestAtomicWriteFile_RefusesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires admin on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	link := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(target, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	err := atomicWriteFile(link, []byte("rewritten"), 0o644)
	if err == nil {
		t.Fatalf("expected refusal error, got nil")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink, got: %v", err)
	}

	// Target file must remain untouched.
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Errorf("symlink target was rewritten: got %q, want %q", got, "original")
	}
}

func TestAtomicWriteFile_CleansUpTempOnRenameFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only-directory semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permission checks")
	}
	dir := t.TempDir()
	// Make the target directory read-only so we can't create a temp file.
	// This simulates a write/rename failure path.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	path := filepath.Join(dir, "file.txt")
	err := atomicWriteFile(path, []byte("data"), 0o644)
	if err == nil {
		t.Fatalf("expected error from write to read-only dir, got nil")
	}

	// Verify no stray temp files were left behind. The dir is read-only so
	// we restore perms to inspect contents.
	if chmodErr := os.Chmod(dir, 0o755); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "token-monitor-tmp-") {
			t.Errorf("temp file leaked after failure: %s", e.Name())
		}
	}
}
