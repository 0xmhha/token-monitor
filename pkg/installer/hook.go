package installer

import (
	"fmt"
	"os"
	"path/filepath"
)

// hookCommand is what we run on PostToolUse. status (not query) is used to
// produce the breakdown-aware compact line; --compact keeps it terse so the
// hook does not flood Claude Code's UI.
const hookCommand = "token-monitor status --current --compact 2>/dev/null || true"

// hookManagedKey is the sentinel embedded in the inner hook entry. We never
// remove or replace inner hooks that lack this key.
const hookManagedKey = "_managed_by"

// hookManagedValue is the sentinel value identifying our entry.
const hookManagedValue = "token-monitor"

// hookEvent is the Claude Code hook event we register against.
const hookEvent = "PostToolUse"

// InstallHook registers (or removes) the PostToolUse hook in
// ~/.claude/settings.json.
//
// The settings.json schema is:
//
//	{
//	  "hooks": {
//	    "PostToolUse": [
//	      {
//	        "matcher": "...",
//	        "hooks": [
//	          { "type": "command", "command": "...", "_managed_by": "token-monitor" }
//	        ]
//	      }
//	    ]
//	  }
//	}
//
// We identify our entry by the "_managed_by" sentinel on the *inner* hook
// object. Outer matcher groups that contain a mix of our hook and user hooks
// are handled correctly: only the inner managed entry is replaced / removed.
//
// On conflict (a non-managed inner hook with the same command string), the
// function refuses with a clear error. The caller may add a --force flag in
// the future without changing this signature.
func InstallHook(dryRun, uninstall bool) (string, error) {
	path, err := hookConfigPath()
	if err != nil {
		return "", err
	}

	settings, fileExisted, err := readJSONObject(path)
	if err != nil {
		return "", err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	groups, _ := hooks[hookEvent].([]any)

	if uninstall {
		newGroups, removed := removeManagedHookEntries(groups)
		if !removed {
			return fmt.Sprintf("hook: nothing to uninstall (no managed %s hook in %s)", hookEvent, path), nil
		}
		return writeHooks(path, settings, hooks, newGroups, fileExisted, dryRun, "remove")
	}

	// Install: scan for existing managed entry.
	if hasManagedHook(groups) {
		// Already installed and command unchanged -> noop. If the command
		// changed (e.g. version bump rewrote the snippet), we update in place.
		if managedHookCommandMatches(groups, hookCommand) {
			return fmt.Sprintf("hook: already installed in %s (no changes)", path), nil
		}
	}

	// Conflict check: an unmanaged hook running our exact command would be
	// confused by us claiming ownership.
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := group["hooks"].([]any)
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if isManagedEntry(hm) {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == hookCommand {
				return "", fmt.Errorf(
					"hook: refusing to install — an unmanaged %s hook in %s already runs the same command; "+
						"remove it manually or rename our command before retrying",
					hookEvent, path)
			}
		}
	}

	newGroups := upsertManagedHook(groups)
	return writeHooks(path, settings, hooks, newGroups, fileExisted, dryRun, "register")
}

func hookConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// upsertManagedHook returns a groups slice with our managed hook present
// exactly once. If a group with matcher=="" already exists, append into it;
// otherwise create a new group at the end.
func upsertManagedHook(groups []any) []any {
	managedEntry := map[string]any{
		"type":         "command",
		"command":      hookCommand,
		hookManagedKey: hookManagedValue,
	}

	// First pass: replace existing managed entry in place.
	for gi, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := group["hooks"].([]any)
		replaced := false
		for ii, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if isManagedEntry(hm) {
				inner[ii] = managedEntry
				replaced = true
			}
		}
		if replaced {
			group["hooks"] = inner
			groups[gi] = group
			return groups
		}
	}

	// No managed entry existed. Find an empty-matcher group to append into.
	for gi, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		matcher, _ := group["matcher"].(string)
		if matcher != "" {
			continue
		}
		inner, _ := group["hooks"].([]any)
		inner = append(inner, managedEntry)
		group["hooks"] = inner
		groups[gi] = group
		return groups
	}

	// No empty-matcher group: create a new one.
	newGroup := map[string]any{
		"matcher": "",
		"hooks":   []any{managedEntry},
	}
	return append(groups, newGroup)
}

// removeManagedHookEntries strips inner managed hooks across all groups and
// drops groups that become empty. Returns (newGroups, anythingRemoved).
func removeManagedHookEntries(groups []any) ([]any, bool) {
	removedAny := false
	out := make([]any, 0, len(groups))
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			out = append(out, g)
			continue
		}
		inner, _ := group["hooks"].([]any)
		kept := make([]any, 0, len(inner))
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				kept = append(kept, h)
				continue
			}
			if isManagedEntry(hm) {
				removedAny = true
				continue
			}
			kept = append(kept, h)
		}
		if len(kept) == 0 {
			// Drop group entirely.
			continue
		}
		group["hooks"] = kept
		out = append(out, group)
	}
	return out, removedAny
}

func hasManagedHook(groups []any) bool {
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := group["hooks"].([]any)
		for _, h := range inner {
			if hm, ok := h.(map[string]any); ok && isManagedEntry(hm) {
				return true
			}
		}
	}
	return false
}

func managedHookCommandMatches(groups []any, want string) bool {
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			continue
		}
		inner, _ := group["hooks"].([]any)
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if !isManagedEntry(hm) {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == want {
				return true
			}
		}
	}
	return false
}

func isManagedEntry(entry map[string]any) bool {
	v, ok := entry[hookManagedKey]
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && s == hookManagedValue
}

func writeHooks(path string, settings, hooks map[string]any, newGroups []any, fileExisted, dryRun bool, action string) (string, error) {
	if len(newGroups) == 0 {
		delete(hooks, hookEvent)
	} else {
		hooks[hookEvent] = newGroups
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

	updated, err := marshalJSON(settings)
	if err != nil {
		return "", err
	}

	pastTense := map[string]string{"register": "registered", "remove": "removed"}[action]
	if pastTense == "" {
		pastTense = action + "d"
	}

	if dryRun {
		return fmt.Sprintf("hook (dry-run): would %s %s in %s\n--- after ---\n%s", action, hookEvent, path, updated), nil
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

	summary := fmt.Sprintf("hook: %s %s in %s", pastTense, hookEvent, path)
	if backupPath != "" {
		summary += fmt.Sprintf(" (backup: %s)", backupPath)
	}
	return summary, nil
}
