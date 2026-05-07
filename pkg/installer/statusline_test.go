package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallStatusline_CreatesStubWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")

	summary, err := InstallStatusline(path, false, false)
	if err != nil {
		t.Fatalf("install error: %v", err)
	}
	if !strings.Contains(summary, "stub") {
		t.Errorf("expected summary to mention stub creation, got %q", summary)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "#!/bin/bash") {
		t.Errorf("stub shebang missing, got %q", got)
	}
	if !strings.Contains(got, "input=$(cat)") {
		t.Errorf("stub stdin reader missing, got %q", got)
	}
	if !HasMarkerBlock(got) {
		t.Errorf("managed block missing after install, got %q", got)
	}
}

func TestInstallStatusline_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")

	summary, err := InstallStatusline(path, true, false)
	if err != nil {
		t.Fatalf("dry-run install error: %v", err)
	}
	if !strings.Contains(summary, "dry-run") {
		t.Errorf("expected summary to mention dry-run, got %q", summary)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("dry-run created file at %s; should not exist", path)
	}
}

func TestInstallStatusline_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")

	if _, err := InstallStatusline(path, false, false); err != nil {
		t.Fatalf("first install error: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := InstallStatusline(path, false, false)
	if err != nil {
		t.Fatalf("second install error: %v", err)
	}
	if !strings.Contains(summary, "already installed") {
		t.Errorf("expected idempotent summary, got %q", summary)
	}

	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotent install produced different content:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestInstallStatusline_PreservesUserContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")

	userContent := `#!/bin/bash
# user's custom statusline
input=$(cat)
echo "user prefix: $(date)"
`
	if err := os.WriteFile(path, []byte(userContent), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallStatusline(path, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	if !strings.Contains(got, "user's custom statusline") {
		t.Errorf("user comment lost, got %q", got)
	}
	if !strings.Contains(got, `echo "user prefix: $(date)"`) {
		t.Errorf("user echo lost, got %q", got)
	}
	if !HasMarkerBlock(got) {
		t.Errorf("managed block not added, got %q", got)
	}
}

func TestInstallStatusline_UninstallPreservesUserContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")

	userContent := "#!/bin/bash\ninput=$(cat)\necho user-line\n"
	if err := os.WriteFile(path, []byte(userContent), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallStatusline(path, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	summary, err := InstallStatusline(path, false, true)
	if err != nil {
		t.Fatalf("uninstall error: %v", err)
	}
	if !strings.Contains(summary, "removed managed block") {
		t.Errorf("expected uninstall summary, got %q", summary)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if HasMarkerBlock(got) {
		t.Errorf("managed block should be removed, got %q", got)
	}
	if !strings.Contains(got, "echo user-line") {
		t.Errorf("user content lost on uninstall, got %q", got)
	}
}

func TestInstallStatusline_UninstallDeletesStubOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")

	// Fresh install (creates stub + block).
	if _, err := InstallStatusline(path, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	if _, err := InstallStatusline(path, false, true); err != nil {
		t.Fatalf("uninstall error: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("stub-only file should be deleted on uninstall, but it still exists")
	}
}

func TestInstallStatusline_UninstallDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")

	if _, err := InstallStatusline(path, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}
	beforeStat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := InstallStatusline(path, true, true)
	if err != nil {
		t.Fatalf("uninstall dry-run error: %v", err)
	}
	if !strings.Contains(summary, "dry-run") {
		t.Errorf("expected dry-run summary, got %q", summary)
	}

	afterStat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	afterData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if afterStat.ModTime() != beforeStat.ModTime() {
		t.Errorf("dry-run mutated file mtime")
	}
	if string(beforeData) != string(afterData) {
		t.Errorf("dry-run mutated file contents")
	}
}

func TestInstallStatusline_UninstallNoFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")

	summary, err := InstallStatusline(path, false, true)
	if err != nil {
		t.Fatalf("uninstall on missing file errored: %v", err)
	}
	if !strings.Contains(summary, "nothing to uninstall") {
		t.Errorf("expected noop summary, got %q", summary)
	}
}

func TestInstallStatusline_BackupCreatedOnRealInstall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")
	if err := os.WriteFile(path, []byte("#!/bin/bash\nuser=true\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallStatusline(path, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	foundBackup := false
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak.") {
			foundBackup = true
			break
		}
	}
	if !foundBackup {
		t.Errorf("expected backup file in %s, found none", dir)
	}
}
