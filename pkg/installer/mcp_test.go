package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readJSON unmarshals the file at path into a generic map, failing the test on error.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return obj
}

func TestInstallMCP_GlobalCreatesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	summary, err := InstallMCP(MCPScopeGlobal, false, false, false)
	if err != nil {
		t.Fatalf("install error: %v", err)
	}
	if !strings.Contains(summary, "registered") {
		t.Errorf("expected registered summary, got %q", summary)
	}

	path := filepath.Join(home, ".claude.json")
	obj := readJSON(t, path)
	servers, ok := obj["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or wrong type: %v", obj["mcpServers"])
	}
	entry, ok := servers["token-monitor"].(map[string]any)
	if !ok {
		t.Fatalf("token-monitor entry missing")
	}
	if cmd, _ := entry["command"].(string); cmd != "token-monitor" {
		t.Errorf("expected command 'token-monitor', got %v", entry["command"])
	}
	if mb, _ := entry["_managed_by"].(string); mb != "token-monitor" {
		t.Errorf("expected _managed_by sentinel, got %v", entry["_managed_by"])
	}
}

func TestInstallMCP_AbsolutePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := InstallMCP(MCPScopeGlobal, false, false, true); err != nil {
		t.Fatalf("install error: %v", err)
	}
	obj := readJSON(t, filepath.Join(home, ".claude.json"))
	servers := obj["mcpServers"].(map[string]any)
	entry := servers["token-monitor"].(map[string]any)
	cmd, _ := entry["command"].(string)
	if cmd == "" {
		t.Fatalf("command empty")
	}
	if cmd == "token-monitor" {
		t.Errorf("expected absolute path, got bare %q", cmd)
	}
	if !filepath.IsAbs(cmd) {
		t.Errorf("expected absolute path, got %q", cmd)
	}
}

func TestInstallMCP_PreservesUnrelatedKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".claude.json")
	initial := map[string]any{
		"theme":   "dark",
		"telemetry": map[string]any{"enabled": false},
	}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallMCP(MCPScopeGlobal, false, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	obj := readJSON(t, path)
	if obj["theme"] != "dark" {
		t.Errorf("theme key lost: %v", obj["theme"])
	}
	tel, ok := obj["telemetry"].(map[string]any)
	if !ok || tel["enabled"] != false {
		t.Errorf("telemetry.enabled lost: %v", obj["telemetry"])
	}
}

func TestInstallMCP_ConflictRefusal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".claude.json")
	initial := map[string]any{
		"mcpServers": map[string]any{
			"token-monitor": map[string]any{
				"command": "/some/other/path/different-binary",
				"args":    []any{"different", "args"},
			},
		},
	}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := InstallMCP(MCPScopeGlobal, false, false, false)
	if err == nil {
		t.Fatalf("expected conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Verify pre-existing entry untouched.
	obj := readJSON(t, path)
	servers := obj["mcpServers"].(map[string]any)
	entry := servers["token-monitor"].(map[string]any)
	if entry["command"] != "/some/other/path/different-binary" {
		t.Errorf("pre-existing entry mutated: %v", entry)
	}
}

func TestInstallMCP_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := InstallMCP(MCPScopeGlobal, false, false, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}

	summary, err := InstallMCP(MCPScopeGlobal, false, false, false)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if !strings.Contains(summary, "already registered") {
		t.Errorf("expected idempotent summary, got %q", summary)
	}
	second, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotent install produced different bytes")
	}
}

func TestInstallMCP_UninstallPreservesSiblings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := InstallMCP(MCPScopeGlobal, false, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	// Add a sibling MCP server entry.
	path := filepath.Join(home, ".claude.json")
	obj := readJSON(t, path)
	servers := obj["mcpServers"].(map[string]any)
	servers["other-tool"] = map[string]any{
		"command": "other",
		"args":    []any{"--stdio"},
	}
	data, _ := json.MarshalIndent(obj, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallMCP(MCPScopeGlobal, false, true, false); err != nil {
		t.Fatalf("uninstall error: %v", err)
	}

	got := readJSON(t, path)
	gotServers, ok := got["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers should still exist (sibling preserved): %v", got)
	}
	if _, present := gotServers["token-monitor"]; present {
		t.Errorf("token-monitor entry should be removed, still present")
	}
	if _, present := gotServers["other-tool"]; !present {
		t.Errorf("sibling other-tool entry lost")
	}
}

func TestInstallMCP_UninstallRemovesEmptyServersKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := InstallMCP(MCPScopeGlobal, false, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}
	if _, err := InstallMCP(MCPScopeGlobal, false, true, false); err != nil {
		t.Fatalf("uninstall error: %v", err)
	}

	obj := readJSON(t, filepath.Join(home, ".claude.json"))
	if _, present := obj["mcpServers"]; present {
		t.Errorf("mcpServers should be removed when empty, got %v", obj["mcpServers"])
	}
}

func TestInstallMCP_DryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	summary, err := InstallMCP(MCPScopeGlobal, true, false, false)
	if err != nil {
		t.Fatalf("dry-run error: %v", err)
	}
	if !strings.Contains(summary, "dry-run") {
		t.Errorf("expected dry-run summary, got %q", summary)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run created file")
	}
}

func TestInstallMCP_ProjectScope(t *testing.T) {
	cwd := t.TempDir()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallMCP(MCPScopeProject, false, false, false); err != nil {
		t.Fatalf("install error: %v", err)
	}

	path := filepath.Join(cwd, ".mcp.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("project .mcp.json not created: %v", err)
	}
	obj := readJSON(t, path)
	servers, ok := obj["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing in project file")
	}
	if _, ok := servers["token-monitor"]; !ok {
		t.Errorf("token-monitor entry missing in project file")
	}
}

func TestInstallMCP_UninstallNoFileIsNoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	summary, err := InstallMCP(MCPScopeGlobal, false, true, false)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(summary, "nothing to uninstall") {
		t.Errorf("expected noop summary, got %q", summary)
	}
}
