package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPScope identifies where the mcpServers entry is written.
type MCPScope int

const (
	// MCPScopeGlobal targets ~/.claude.json (user-wide config).
	MCPScopeGlobal MCPScope = iota
	// MCPScopeProject targets ./.mcp.json (per-project config in CWD).
	MCPScopeProject
)

// mcpServerName is the canonical key under mcpServers. Hardcoded for
// idempotent identification across install / uninstall.
const mcpServerName = "token-monitor"

// mcpServerCommand is the value written when --absolute is *not* used. The
// install assumes the user has token-monitor on PATH at runtime.
const mcpServerCommand = "token-monitor"

// mcpServerArgs is the args list written into the entry. `serve --stdio` is
// the canonical MCP transport for token-monitor.
var mcpServerArgs = []string{"serve", "--stdio"}

// InstallMCP registers, updates, or removes the token-monitor entry in
// mcpServers of the chosen config file.
//
// useAbsolutePath: when true, write the absolute path of the running binary
// (via os.Executable). Otherwise write bare "token-monitor" (must be on PATH).
//
// Conflict policy: if an existing entry uses a *different* command (i.e.
// installed by some other tool) and is not equivalent to ours, the function
// refuses with a clear error. The caller can future-proof a --force flag here
// without changing this signature.
//
// Other unrelated keys in the JSON file are preserved byte-for-byte (modulo
// re-formatting from `encoding/json`).
func InstallMCP(scope MCPScope, dryRun, uninstall, useAbsolutePath bool) (string, error) {
	path, err := mcpConfigPath(scope)
	if err != nil {
		return "", err
	}

	desired, err := buildMCPEntry(useAbsolutePath)
	if err != nil {
		return "", err
	}

	existing, fileExisted, err := readJSONObject(path)
	if err != nil {
		return "", err
	}

	servers, _ := existing["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}

	if uninstall {
		return uninstallMCP(path, existing, servers, fileExisted, dryRun)
	}

	if cur, ok := servers[mcpServerName]; ok {
		if curMap, ok := cur.(map[string]any); ok {
			if mcpEntriesEquivalent(curMap, desired) {
				return fmt.Sprintf("mcp: already registered in %s (no changes)", path), nil
			}
			if !mcpEntryIsOurs(curMap) {
				return "", fmt.Errorf(
					"mcp: refusing to overwrite existing %q entry in %s — "+
						"the existing command differs from ours; "+
						"remove the entry manually or run with --uninstall first",
					mcpServerName, path)
			}
		}
	}

	servers[mcpServerName] = desired
	existing["mcpServers"] = servers

	updated, err := marshalJSON(existing)
	if err != nil {
		return "", err
	}

	if dryRun {
		return fmt.Sprintf("mcp (dry-run): would write %s\n--- after ---\n%s", path, updated), nil
	}

	if err := ensureParentDir(path); err != nil {
		return "", err
	}
	backupPath := ""
	if fileExisted {
		backupPath, err = BackupFile(path)
		if err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil { //nolint:gosec // user config file
		return "", fmt.Errorf("write %s: %w", path, err)
	}

	summary := fmt.Sprintf("mcp: registered token-monitor in %s", path)
	if backupPath != "" {
		summary += fmt.Sprintf(" (backup: %s)", backupPath)
	}
	return summary, nil
}

func uninstallMCP(path string, existing, servers map[string]any, fileExisted, dryRun bool) (string, error) {
	if !fileExisted {
		return fmt.Sprintf("mcp: nothing to uninstall (no file at %s)", path), nil
	}
	cur, ok := servers[mcpServerName]
	if !ok {
		return fmt.Sprintf("mcp: nothing to uninstall (no %q entry in %s)", mcpServerName, path), nil
	}

	if curMap, ok := cur.(map[string]any); ok && !mcpEntryIsOurs(curMap) {
		return "", fmt.Errorf(
			"mcp: refusing to remove %q entry in %s — "+
				"command does not look like ours; remove manually if intended",
			mcpServerName, path)
	}

	delete(servers, mcpServerName)
	if len(servers) == 0 {
		delete(existing, "mcpServers")
	} else {
		existing["mcpServers"] = servers
	}

	updated, err := marshalJSON(existing)
	if err != nil {
		return "", err
	}

	if dryRun {
		return fmt.Sprintf("mcp (dry-run): would update %s\n--- after ---\n%s", path, updated), nil
	}

	backupPath, err := BackupFile(path)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil { //nolint:gosec // user config file
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return fmt.Sprintf("mcp: removed token-monitor entry from %s (backup: %s)", path, backupPath), nil
}

func mcpConfigPath(scope MCPScope) (string, error) {
	switch scope {
	case MCPScopeGlobal:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate home directory: %w", err)
		}
		return filepath.Join(home, ".claude.json"), nil
	case MCPScopeProject:
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("locate working directory: %w", err)
		}
		return filepath.Join(cwd, ".mcp.json"), nil
	default:
		return "", fmt.Errorf("unknown mcp scope: %d", scope)
	}
}

func buildMCPEntry(useAbsolutePath bool) (map[string]any, error) {
	cmd := mcpServerCommand
	if useAbsolutePath {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve executable path: %w", err)
		}
		// Resolve symlinks for stability — goreleaser/homebrew may symlink the
		// installed binary, but Claude Code accepts either; we prefer the
		// concrete path.
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		cmd = exe
	}
	args := make([]any, len(mcpServerArgs))
	for i, a := range mcpServerArgs {
		args[i] = a
	}
	return map[string]any{
		"command":      cmd,
		"args":         args,
		"_managed_by":  "token-monitor",
	}, nil
}

// mcpEntryIsOurs reports whether an entry was created by this installer.
// A value-only "_managed_by": "token-monitor" sentinel is the source of
// truth; we never silently delete entries placed there by users.
func mcpEntryIsOurs(entry map[string]any) bool {
	v, ok := entry["_managed_by"]
	if !ok {
		// Backwards compat: an entry whose command equals the bare binary name
		// or whose command path's base equals the binary name AND whose args
		// match ours is also considered ours. This handles cases where the
		// user (or an older version of this tool) registered without the
		// sentinel.
		return looksLikeOurMCPEntry(entry)
	}
	s, ok := v.(string)
	return ok && s == "token-monitor"
}

func looksLikeOurMCPEntry(entry map[string]any) bool {
	cmd, _ := entry["command"].(string)
	if cmd == "" {
		return false
	}
	if cmd != mcpServerCommand && filepath.Base(cmd) != mcpServerCommand {
		return false
	}
	rawArgs, _ := entry["args"].([]any)
	if len(rawArgs) != len(mcpServerArgs) {
		return false
	}
	for i, want := range mcpServerArgs {
		got, _ := rawArgs[i].(string)
		if got != want {
			return false
		}
	}
	return true
}

func mcpEntriesEquivalent(a, b map[string]any) bool {
	if a["command"] != b["command"] {
		return false
	}
	aArgs, _ := a["args"].([]any)
	bArgs, _ := b["args"].([]any)
	if len(aArgs) != len(bArgs) {
		return false
	}
	for i := range aArgs {
		if aArgs[i] != bArgs[i] {
			return false
		}
	}
	if a["_managed_by"] != b["_managed_by"] {
		return false
	}
	return true
}

// readJSONObject reads a JSON object from path. Missing file -> empty map.
// A non-object root (array, scalar) is rejected.
func readJSONObject(path string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path supplied by caller
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, true, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, true, fmt.Errorf("parse %s as JSON object: %w", path, err)
	}
	if obj == nil {
		obj = map[string]any{}
	}
	return obj, true, nil
}

// marshalJSON renders the object as 2-space-indented JSON with a trailing
// newline (matching Claude Code's own conventions and most editor formatters).
func marshalJSON(obj map[string]any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(obj); err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}
	return buf.String(), nil
}
