package installer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPatchMarkerBlock_AppendToEmpty(t *testing.T) {
	body := MarkerStart + "\necho hello\n" + MarkerEnd

	got := PatchMarkerBlock("", body)

	if !strings.Contains(got, MarkerStart) {
		t.Fatalf("expected start marker, got %q", got)
	}
	if !strings.Contains(got, MarkerEnd) {
		t.Fatalf("expected end marker, got %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline, got %q", got)
	}
}

func TestPatchMarkerBlock_AppendPreservesExistingContent(t *testing.T) {
	existing := "#!/bin/bash\nexisting=true\n"
	body := MarkerStart + "\nnew=true\n" + MarkerEnd

	got := PatchMarkerBlock(existing, body)

	if !strings.HasPrefix(got, existing) {
		t.Fatalf("existing content not preserved at start, got %q", got)
	}
	if !strings.Contains(got, "new=true") {
		t.Fatalf("new body not appended, got %q", got)
	}
}

func TestPatchMarkerBlock_ReplaceExistingBlock(t *testing.T) {
	original := "before\n" + MarkerStart + "\nold body\n" + MarkerEnd + "\nafter\n"
	newBody := MarkerStart + "\nnew body line 1\nnew body line 2\n" + MarkerEnd

	got := PatchMarkerBlock(original, newBody)

	if strings.Contains(got, "old body") {
		t.Fatalf("old body should have been replaced, got %q", got)
	}
	if !strings.Contains(got, "new body line 1") {
		t.Fatalf("new body missing, got %q", got)
	}
	if !strings.Contains(got, "before") {
		t.Fatalf("content before block lost, got %q", got)
	}
	if !strings.Contains(got, "after") {
		t.Fatalf("content after block lost, got %q", got)
	}
}

func TestPatchMarkerBlock_Idempotent(t *testing.T) {
	body := MarkerStart + "\nstable body\n" + MarkerEnd

	once := PatchMarkerBlock("user content\n", body)
	twice := PatchMarkerBlock(once, body)

	if once != twice {
		t.Fatalf("not idempotent:\nonce:  %q\ntwice: %q", once, twice)
	}
}

func TestPatchMarkerBlock_RemoveBlock(t *testing.T) {
	original := "before line\n" + MarkerStart + "\nmanaged stuff\n" + MarkerEnd + "\nafter line\n"

	got := PatchMarkerBlock(original, "")

	if strings.Contains(got, MarkerStart) {
		t.Fatalf("start marker should have been removed, got %q", got)
	}
	if strings.Contains(got, MarkerEnd) {
		t.Fatalf("end marker should have been removed, got %q", got)
	}
	if !strings.Contains(got, "before line") {
		t.Fatalf("content before block lost, got %q", got)
	}
	if !strings.Contains(got, "after line") {
		t.Fatalf("content after block lost, got %q", got)
	}
}

func TestPatchMarkerBlock_RemoveOnAbsentIsNoop(t *testing.T) {
	original := "user content with no markers\n"

	got := PatchMarkerBlock(original, "")

	if got != original {
		t.Fatalf("uninstall on missing block changed content: %q -> %q", original, got)
	}
}

func TestHasMarkerBlock(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", false},
		{"no markers", "just user content", false},
		{"complete block", MarkerStart + "\nbody\n" + MarkerEnd + "\n", true},
		{"start only", MarkerStart + "\nbody only", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HasMarkerBlock(tc.content); got != tc.want {
				t.Errorf("HasMarkerBlock(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestBackupFile_CreatesTimestampedCopy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "config.txt")
	contents := "original contents\n"
	if err := os.WriteFile(src, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := BackupFile(src)
	if err != nil {
		t.Fatalf("BackupFile error: %v", err)
	}
	if got == "" {
		t.Fatalf("expected non-empty backup path")
	}

	// Verify naming convention: <path>.bak.YYYYMMDD-HHMMSS
	pattern := regexp.MustCompile(regexp.QuoteMeta(src) + `\.bak\.\d{8}-\d{6}$`)
	if !pattern.MatchString(got) {
		t.Errorf("backup path %q does not match expected pattern *.bak.YYYYMMDD-HHMMSS", got)
	}

	// Verify contents copied.
	data, err := os.ReadFile(got) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != contents {
		t.Errorf("backup contents mismatch: want %q, got %q", contents, string(data))
	}
}

func TestBackupFile_MissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")

	got, err := BackupFile(missing)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty backup path for missing source, got %q", got)
	}
}
