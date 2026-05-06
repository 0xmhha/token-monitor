package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHook_RegistersIntoEmptyFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	summary, err := InstallHook(false, false)
	if err != nil {
		t.Fatalf("install error: %v", err)
	}
	if !strings.Contains(summary, "registered") {
		t.Errorf("expected registered summary, got %q", summary)
	}

	path := filepath.Join(home, ".claude", "settings.json")
	obj := readJSON(t, path)
	hooks, ok := obj["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks missing: %v", obj)
	}
	groups, ok := hooks["PostToolUse"].([]any)
	if !ok || len(groups) == 0 {
		t.Fatalf("PostToolUse missing or empty: %v", hooks["PostToolUse"])
	}
	group := groups[0].(map[string]any)
	inner := group["hooks"].([]any)
	first := inner[0].(map[string]any)
	if first["_managed_by"] != "token-monitor" {
		t.Errorf("expected _managed_by sentinel, got %v", first["_managed_by"])
	}
	if cmd, _ := first["command"].(string); !strings.Contains(cmd, "token-monitor status") {
		t.Errorf("expected token-monitor status command, got %v", first["command"])
	}
}

func TestInstallHook_PreservesUnmanagedSiblings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "echo user-hook",
						},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallHook(false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	obj := readJSON(t, settingsPath)
	hooks := obj["hooks"].(map[string]any)
	groups := hooks["PostToolUse"].([]any)
	group := groups[0].(map[string]any)
	inner := group["hooks"].([]any)
	if len(inner) != 2 {
		t.Fatalf("expected 2 inner hooks (user + managed), got %d: %v", len(inner), inner)
	}
	foundUser, foundManaged := false, false
	for _, h := range inner {
		hm := h.(map[string]any)
		if hm["command"] == "echo user-hook" {
			foundUser = true
		}
		if isManagedEntry(hm) {
			foundManaged = true
		}
	}
	if !foundUser {
		t.Errorf("user hook lost")
	}
	if !foundManaged {
		t.Errorf("managed hook missing")
	}
}

func TestInstallHook_ConflictRefusal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing UNMANAGED hook with same command (no _managed_by sentinel).
	initial := map[string]any{
		"hooks": map[string]any{
			"PostToolUse": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": hookCommand,
						},
					},
				},
			},
		},
	}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := InstallHook(false, false)
	if err == nil {
		t.Fatalf("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to install") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallHook_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := InstallHook(false, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}

	summary, err := InstallHook(false, false)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !strings.Contains(summary, "already installed") {
		t.Errorf("expected idempotent summary, got %q", summary)
	}
	second, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotent install produced different bytes")
	}
}

func TestInstallHook_UninstallPreservesOtherHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Install our hook.
	if _, err := InstallHook(false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	// Add an unmanaged sibling in the same group.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	obj := readJSON(t, settingsPath)
	hooks := obj["hooks"].(map[string]any)
	groups := hooks["PostToolUse"].([]any)
	group := groups[0].(map[string]any)
	inner := group["hooks"].([]any)
	inner = append(inner, map[string]any{
		"type":    "command",
		"command": "echo sibling",
	})
	group["hooks"] = inner
	groups[0] = group
	hooks["PostToolUse"] = groups
	obj["hooks"] = hooks
	data, _ := json.MarshalIndent(obj, "", "  ")
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallHook(false, true); err != nil {
		t.Fatalf("uninstall error: %v", err)
	}

	got := readJSON(t, settingsPath)
	gotHooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks key removed but sibling exists: %v", got)
	}
	gotGroups, ok := gotHooks["PostToolUse"].([]any)
	if !ok || len(gotGroups) == 0 {
		t.Fatalf("PostToolUse should still exist with sibling: %v", gotHooks["PostToolUse"])
	}
	gotGroup := gotGroups[0].(map[string]any)
	gotInner := gotGroup["hooks"].([]any)
	if len(gotInner) != 1 {
		t.Errorf("expected 1 sibling remaining, got %d: %v", len(gotInner), gotInner)
	}
	got0 := gotInner[0].(map[string]any)
	if got0["command"] != "echo sibling" {
		t.Errorf("sibling lost: %v", got0)
	}
}

func TestInstallHook_UninstallRemovesEmptyHooksKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := InstallHook(false, false); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := InstallHook(false, true); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	obj := readJSON(t, filepath.Join(home, ".claude", "settings.json"))
	if _, present := obj["hooks"]; present {
		t.Errorf("hooks key should be removed when empty, got %v", obj["hooks"])
	}
}

func TestInstallHook_DryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	summary, err := InstallHook(true, false)
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	if !strings.Contains(summary, "dry-run") {
		t.Errorf("expected dry-run summary, got %q", summary)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Errorf("dry-run created file")
	}
}

func TestInstallHook_UninstallNoFileIsNoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	summary, err := InstallHook(false, true)
	if err != nil {
		t.Fatalf("uninstall error: %v", err)
	}
	if !strings.Contains(summary, "nothing to uninstall") {
		t.Errorf("expected noop summary, got %q", summary)
	}
}

// TestInstallHook_CorruptedJSONLeavesFileUntouched: malformed settings.json
// must surface as an error without rewriting the file.
func TestInstallHook_CorruptedJSONLeavesFileUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := []byte("{not valid json")
	if err := os.WriteFile(settingsPath, bad, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := InstallHook(false, false)
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}

	got, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if string(got) != string(bad) {
		t.Errorf("corrupted file was modified: got %q, want %q", got, bad)
	}
}
