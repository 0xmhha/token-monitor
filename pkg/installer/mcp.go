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

	noopMsg, conflictErr := checkExistingMCPEntry(servers, desired, path)
	if conflictErr != nil {
		return "", conflictErr
	}
	if noopMsg != "" {
		return noopMsg, nil
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

	backupPath, err := commitMCPWrite(path, []byte(updated), fileExisted)
	if err != nil {
		return "", err
	}

	summary := fmt.Sprintf("mcp: registered token-monitor in %s", path)
	if backupPath != "" {
		summary += fmt.Sprintf(" (backup: %s)", backupPath)
	}
	return summary, nil
}

// checkExistingMCPEntry inspects the existing entry for this server, if any.
// It returns:
//   - ("already registered ...", nil) when the existing entry is byte-equivalent
//     to the desired one (caller should return this string and no error).
//   - ("", err) when the existing entry is user-authored and would be overwritten
//     (caller should return the error).
//   - ("", nil) when there is no existing entry, or when the existing entry is
//     ours and stale (caller should proceed to overwrite).
func checkExistingMCPEntry(servers, desired map[string]any, path string) (string, error) {
	cur, ok := servers[mcpServerName]
	if !ok {
		return "", nil
	}
	curMap, ok := cur.(map[string]any)
	if !ok {
		return "", nil
	}
	if mcpEntriesEquivalent(curMap, desired) {
		return fmt.Sprintf("mcp: already registered in %s (no changes)", path), nil
	}
	if !mcpEntryIsOurs(curMap) {
		return "", fmt.Errorf(
			"mcp: refusing to overwrite existing %q entry in %s — "+
				"entry lacks the '_managed_by: \"token-monitor\"' sentinel "+
				"and is treated as user-authored; "+
				"remove the entry manually if you want this installer to manage it",
			mcpServerName, path)
	}
	return "", nil
}

// commitMCPWrite ensures the parent dir exists, takes a backup if the file
// already existed, and atomically writes the updated content. It returns the
// backup path (empty string if no backup was taken).
func commitMCPWrite(path string, updated []byte, fileExisted bool) (string, error) {
	if err := ensureParentDir(path); err != nil {
		return "", err
	}
	backupPath := ""
	if fileExisted {
		bp, err := BackupFile(path)
		if err != nil {
			return "", err
		}
		backupPath = bp
	}
	if err := atomicWriteFile(path, updated, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return backupPath, nil
}

func uninstallMCP(path string, existing, servers map[string]any, fileExisted, dryRun bool) (string, error) {
	if !fileExisted {
		return fmt.Sprintf("mcp: nothing to uninstall (no file at %s)", path), nil
	}
	cur, ok := servers[mcpServerName]
	if !ok {
		return fmt.Sprintf("mcp: nothing to uninstall (no %q entry in %s)", mcpServerName, path), nil
	}

	if curMap, isMap := cur.(map[string]any); isMap && !mcpEntryIsOurs(curMap) {
		// User-authored entry (no '_managed_by' sentinel). Uninstall is a
		// no-op so we never silently delete something the user wrote — even
		// if the command/args happen to match ours.
		return fmt.Sprintf(
			"mcp: skipping %q entry in %s — lacks the '_managed_by: \"token-monitor\"' sentinel; "+
				"treating as user-authored, remove manually if intended",
			mcpServerName, path), nil
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
	if writeErr := atomicWriteFile(path, []byte(updated), 0o644); writeErr != nil {
		return "", fmt.Errorf("write %s: %w", path, writeErr)
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
		if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
			exe = resolved
		}
		cmd = exe
	}
	args := make([]any, len(mcpServerArgs))
	for i, a := range mcpServerArgs {
		args[i] = a
	}
	return map[string]any{
		"command":     cmd,
		"args":        args,
		"_managed_by": "token-monitor",
	}, nil
}

// mcpEntryIsOurs reports whether an entry was created by this installer.
// Strict policy: an entry is ours only if it carries the
// "_managed_by": "token-monitor" sentinel. Entries lacking the sentinel are
// treated as user-authored — we never silently overwrite or remove them, even
// if their command/args happen to match. v0.2 has not shipped, so there are
// no legacy installs to support with looser matching.
func mcpEntryIsOurs(entry map[string]any) bool {
	v, ok := entry["_managed_by"]
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && s == "token-monitor"
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
	return a["_managed_by"] == b["_managed_by"]
}

// readJSONObject reads a JSON object from path. Missing file -> empty map.
// A non-object root (array, scalar) is rejected.
//
// Numbers are decoded as json.Number (via UseNumber) instead of float64. This
// preserves int64 precision above 2^53; we rewrite every key on every install,
// so a float64 round-trip would silently corrupt unrelated user data
// (~/.claude.json holds opaque numeric IDs from Claude Code).
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
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var obj map[string]any
	if decodeErr := dec.Decode(&obj); decodeErr != nil {
		return nil, true, fmt.Errorf("parse %s as JSON object: %w", path, decodeErr)
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
