package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/0xmhha/token-monitor/pkg/installer"
)

// parseFlagsOrHelp wraps fs.Parse so flag.ErrHelp (raised when the user passes
// --help to a flag.ContinueOnError flagset) does not surface as a non-zero
// exit. The fs.Usage callback already printed the help text before returning.
func parseFlagsOrHelp(fs *flag.FlagSet, args []string) (helpRequested bool, err error) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// runInstallCommand dispatches the `token-monitor install <subcommand>`
// subcommands. It also handles the top-level `--uninstall-all` shortcut.
func runInstallCommand(globalOpts globalOptions, args []string) error {
	_ = globalOpts // reserved for future logging hooks; install is intentionally
	// independent of config files (it has to work on a fresh machine).

	// Pre-scan for top-level uninstall-all flag. We accept it as either
	// `--uninstall-all` or `-uninstall-all` (Go's flag package convention).
	for _, a := range args {
		switch a {
		case "--uninstall-all", "-uninstall-all":
			return installUninstallAll()
		}
	}

	if len(args) == 0 {
		return showInstallUsage()
	}

	// Top-level --help only when no subcommand is given.
	switch args[0] {
	case "--help", "-help", "-h":
		return showInstallUsage()
	}

	sub := args[0]
	rest := args[1:]
	switch sub {
	case "statusline":
		return runInstallStatusline(rest)
	case "mcp":
		return runInstallMCP(rest)
	case "hook":
		return runInstallHook(rest)
	case "all":
		return runInstallAll(rest)
	case "help":
		return showInstallUsage()
	default:
		return fmt.Errorf("unknown install subcommand: %s (run `token-monitor install --help`)", sub)
	}
}

func runInstallStatusline(args []string) error {
	fs := flag.NewFlagSet("install statusline", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show full before/after diff without writing")
	printFlag := fs.Bool("print", false, "print just the managed snippet for manual integration (no diff, no read of existing file, no write)")
	uninstall := fs.Bool("uninstall", false, "remove the managed block")
	target := fs.String("target", "", "override path (default: ~/.claude/statusline-command.sh)")
	fs.Usage = func() { printInstallStatuslineUsage(fs) }
	help, err := parseFlagsOrHelp(fs, args)
	if err != nil {
		return err
	}
	if help {
		return nil
	}

	// --print is snippet-only: emit StatuslineSnippet verbatim, do not touch
	// existing files. Useful for users who want to integrate the block into
	// their own scripts manually. --dry-run remains the full diff preview.
	if *printFlag {
		// Write verbatim — the snippet embeds %s inside a shell printf, which
		// would trip fmt.Print's format-string vet check.
		_, _ = os.Stdout.WriteString(installer.StatuslineSnippet + "\n")
		return nil
	}

	summary, err := installer.InstallStatusline(*target, *dryRun, *uninstall)
	if err != nil {
		return err
	}
	fmt.Println(summary)
	return nil
}

func runInstallMCP(args []string) error {
	fs := flag.NewFlagSet("install mcp", flag.ContinueOnError)
	global := fs.Bool("global", false, "write to ~/.claude.json (default if neither flag set)")
	project := fs.Bool("project", false, "write to ./.mcp.json")
	uninstall := fs.Bool("uninstall", false, "remove the entry")
	absolute := fs.Bool("absolute", false, "write the absolute path of the running binary")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	fs.Usage = func() { printInstallMCPUsage(fs) }
	help, err := parseFlagsOrHelp(fs, args)
	if err != nil {
		return err
	}
	if help {
		return nil
	}

	if *global && *project {
		return fmt.Errorf("install mcp: --global and --project are mutually exclusive")
	}

	scope := installer.MCPScopeGlobal
	if *project {
		scope = installer.MCPScopeProject
	}

	summary, err := installer.InstallMCP(scope, *dryRun, *uninstall, *absolute)
	if err != nil {
		return err
	}
	fmt.Println(summary)
	return nil
}

func runInstallHook(args []string) error {
	fs := flag.NewFlagSet("install hook", flag.ContinueOnError)
	uninstall := fs.Bool("uninstall", false, "remove the managed PostToolUse hook")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	fs.Usage = func() { printInstallHookUsage(fs) }
	help, err := parseFlagsOrHelp(fs, args)
	if err != nil {
		return err
	}
	if help {
		return nil
	}

	summary, err := installer.InstallHook(*dryRun, *uninstall)
	if err != nil {
		return err
	}
	fmt.Println(summary)
	return nil
}

// runInstallAll runs statusline + mcp (global) + hook in sequence.
// The first failing step short-circuits, but already-succeeded steps remain
// applied — we surface partial state rather than rolling back, since rollback
// would itself need backups for files we just touched.
func runInstallAll(args []string) error {
	fs := flag.NewFlagSet("install all", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	fs.Usage = func() { printInstallAllUsage(fs) }
	help, err := parseFlagsOrHelp(fs, args)
	if err != nil {
		return err
	}
	if help {
		return nil
	}

	steps := []struct {
		name string
		fn   func() (string, error)
	}{
		{"statusline", func() (string, error) { return installer.InstallStatusline("", *dryRun, false) }},
		{"mcp", func() (string, error) { return installer.InstallMCP(installer.MCPScopeGlobal, *dryRun, false, false) }},
		{"hook", func() (string, error) { return installer.InstallHook(*dryRun, false) }},
	}

	for i, s := range steps {
		summary, err := s.fn()
		if err != nil {
			if i > 0 && !*dryRun {
				// Earlier steps may have written to disk; surface an actionable
				// recovery path so the user isn't left with a partial install.
				fmt.Println("hint: previous steps may have succeeded; run 'token-monitor install --uninstall-all' to revert")
			}
			return fmt.Errorf("install all: %s step failed: %w", s.name, err)
		}
		fmt.Println(summary)
	}
	return nil
}

// installUninstallAll is the dual of runInstallAll. We continue past per-step
// errors and report them at the end so a partial install still gets cleaned
// up to the maximum extent possible.
func installUninstallAll() error {
	type result struct {
		name    string
		summary string
		err     error
	}

	steps := []struct {
		name string
		fn   func() (string, error)
	}{
		{"statusline", func() (string, error) { return installer.InstallStatusline("", false, true) }},
		{"mcp", func() (string, error) { return installer.InstallMCP(installer.MCPScopeGlobal, false, true, false) }},
		{"hook", func() (string, error) { return installer.InstallHook(false, true) }},
	}

	var results []result
	for _, s := range steps {
		summary, err := s.fn()
		results = append(results, result{name: s.name, summary: summary, err: err})
	}

	var errs []string
	for _, r := range results {
		if r.err != nil {
			fmt.Printf("%s: ERROR: %v\n", r.name, r.err)
			errs = append(errs, fmt.Sprintf("%s: %v", r.name, r.err))
			continue
		}
		fmt.Println(r.summary)
	}
	if len(errs) > 0 {
		return fmt.Errorf("uninstall-all: %d step(s) failed: %s", len(errs), strings.Join(errs, "; "))
	}
	return nil
}

// --- usage strings ---

func showInstallUsage() error {
	fmt.Print(`Token Monitor - install integration into Claude Code

Usage:
  token-monitor install <subcommand> [flags]
  token-monitor install --uninstall-all

Subcommands:
  statusline   Patch ~/.claude/statusline-command.sh with the managed block
  mcp          Register token-monitor in mcpServers (~/.claude.json or ./.mcp.json)
  hook         Register a PostToolUse hook in ~/.claude/settings.json
  all          Run statusline + mcp (global) + hook

Top-level flags:
  --uninstall-all   Remove all three integrations (statusline, mcp, hook)
  --help            Show this help

Run any subcommand with --help for its flags. All subcommands support
--dry-run to preview changes without writing, and back up existing files
to *.bak.YYYYMMDD-HHMMSS before modification.
`)
	return nil
}

func printInstallStatuslineUsage(fs *flag.FlagSet) {
	fmt.Println("Usage: token-monitor install statusline [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fs.PrintDefaults()
}

func printInstallMCPUsage(fs *flag.FlagSet) {
	fmt.Println("Usage: token-monitor install mcp [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fs.PrintDefaults()
}

func printInstallHookUsage(fs *flag.FlagSet) {
	fmt.Println("Usage: token-monitor install hook [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fs.PrintDefaults()
}

func printInstallAllUsage(fs *flag.FlagSet) {
	fmt.Println("Usage: token-monitor install all [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fs.PrintDefaults()
}
