# Token Monitor — Testing Guide

Testing reference for **this** repo. Generic Go testing fundamentals (table-driven
patterns, mocking, `t.TempDir`, etc.) are intentionally NOT covered here — see
[Resources](#resources) for canonical sources. This doc covers what is unique
to token-monitor: fixtures, mocks, gotchas, and per-package coverage.

## Contents

- [Strategy & Targets](#strategy--targets)
- [Running Tests](#running-tests)
- [Coverage Snapshot](#coverage-snapshot)
- [Project Test Fixtures](#project-test-fixtures)
- [Project-Specific Patterns](#project-specific-patterns)
- [Gotchas](#gotchas)
- [CI/CD](#cicd)
- [Resources](#resources)

---

## Strategy & Targets

Standard pyramid: ~80% unit / ~15% integration / ~5% end-to-end.

| Tier | What | Where |
|------|------|-------|
| **Unit** | Pure functions, type behavior, single-package logic | `*_test.go` next to implementation |
| **Integration** | Multiple packages — e.g. install command writing real files, MCP tool dispatching through registry | `pkg/installer/*_test.go`, `pkg/mcp/*_test.go`, `cmd/token-monitor/status_command_test.go` |
| **End-to-end** | CLI invocation, JSON-RPC over stdio, statusline JSON envelope | Smoke-tested via README examples and `gh pr checks` matrix |

**Coverage targets** (per package):
- Critical (parser, aggregator, sessionloader): ≥90%
- Standard (config, discovery, display, installer, logger, mcp): ≥80%
- I/O-heavy (monitor, reader, session, watcher): ≥70%
- TUI (`pkg/tui`): no automated tests yet — manual smoke only
- `cmd/token-monitor`: low coverage acceptable (thin glue around tested packages)

---

## Running Tests

```bash
# Default — all packages, race detector, no caching
go test -race -count=1 ./...

# Coverage profile
go test -race -coverprofile=coverage.out -covermode=atomic ./...
go tool cover -func=coverage.out          # text summary
go tool cover -html=coverage.out          # HTML browser

# Single package, verbose
go test -v ./pkg/aggregator

# Single test
go test -v ./pkg/mcp -run TestGetTodayUsage

# Benchmarks
go test -bench=. -benchmem ./pkg/parser
```

The CI runs `go test -race -coverprofile=coverage.out -covermode=atomic ./...`
on every push/PR (see [`.github/workflows/ci.yml`](../.github/workflows/ci.yml)).
Match this locally before pushing.

### Lint locally with the exact CI version

CI pins `golangci-lint v1.64.8`. Local v2 will not work against this repo's
v1-format `.golangci.yml`. Use the official prebuilt binary, not `go install`:

```bash
curl -sL https://github.com/golangci/golangci-lint/releases/download/v1.64.8/golangci-lint-1.64.8-darwin-arm64.tar.gz \
  | tar xz -C /tmp
/tmp/golangci-lint-1.64.8-darwin-arm64/golangci-lint run --timeout=5m
```

(`go install ...@v1.64.8` rebuilds with whatever local Go is on `PATH`, which
can be older than 1.24 and refuse to lint the repo's go1.24.2 source. The
official binary is built with go1.24.)

---

## Coverage Snapshot

Current per-package coverage (`go test -cover ./...`, 2026-05-07):

| Package | Coverage | Notes |
|---------|----------|-------|
| `pkg/sessionloader` | 100.0% | Fully covered — small surface |
| `pkg/logger` | 93.3% | |
| `pkg/parser` | 90.7% | Critical path — must stay ≥90% |
| `pkg/analysis` | 89.9% | |
| `pkg/aggregator` | 88.7% | Critical — includes breakdown helpers |
| `pkg/config` | 86.6% | |
| `pkg/mcp` | 84.5% | 9 tools incl. cross-session breakdown |
| `pkg/discovery` | 84.3% | |
| `pkg/display` | 82.9% | Includes `ParseWindow`, format helpers |
| `pkg/installer` | 80.4% | Atomic write, marker block, sentinel |
| `pkg/session` | 75.6% | BoltDB interaction |
| `pkg/monitor` | 74.5% | |
| `pkg/reader` | 71.1% | I/O-heavy |
| `pkg/watcher` | 70.1% | fsnotify integration |
| `pkg/tui` | 0.0% | No tests; manual smoke only |
| `cmd/token-monitor` | 4.4% | Thin CLI glue |

Generate fresh numbers any time with `go test -cover ./... 2>&1 | grep coverage:`.

---

## Project Test Fixtures

These are the reusable test helpers actually present in the repo. New tests
should reuse them rather than re-inventing.

| Helper | Location | Purpose |
|--------|----------|---------|
| `makeEntry(model, ts, in, out, cc, cr)` | `pkg/aggregator/breakdown_test.go` | Builds a `parser.UsageEntry` with the supplied tokens for breakdown/window tests |
| `makeProjectDir(t, baseDir, name)` | `pkg/discovery/current_test.go` | Creates a project subdirectory under a temp Claude config dir |
| `makeMultiModelSession(t, sessionID, []modelEntry)` | `pkg/mcp/breakdown_tools_test.go` | Writes a JSONL fixture with one entry per `modelEntry` (model, input, output, cache, timestampOffset) |
| `makeSessionFile(t, sessionID, in, out)` | `pkg/mcp/tools_test.go` | Single-model JSONL fixture for single-session MCP tests |
| `mockDiscoverer{sessions []discovery.SessionFile}` | `pkg/mcp/tools_test.go` | Discovery without filesystem scan |
| `newTestReaderFactory()` | `pkg/mcp/tools_test.go` | Returns a reader factory wired to `MemoryPositionStore` |
| `testLogger{}` | `pkg/mcp/server_test.go` | No-op logger satisfying `mcp.Logger` |
| `mockWatcher`, `mockReader`, `mockDiscovery` | `pkg/monitor/live_test.go` | Live-monitor pipeline doubles |
| `fakeReader{byPath, errs}`, `fakeLogger{warnings []string}` | `pkg/sessionloader/sessionloader_test.go` | Drives `LoadEntries` without real I/O; captures Warn calls |
| `captureStdout(t, fn)` | `cmd/token-monitor/status_command_test.go` | Replaces `os.Stdout` with a pipe, returns whatever `fn` printed |

When adding a new MCP tool test, prefer composing
`mockDiscoverer` + `makeMultiModelSession` (or `makeSessionFile`) +
`newTestReaderFactory` — that triple covers the full discovery → reader →
aggregator pipeline in-memory.

---

## Project-Specific Patterns

### Pattern 1 — `t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)` for integration tests

Discovery and config-loading paths look at `CLAUDE_CONFIG_DIR`. Integration
tests for the install command and breakdown path use this to redirect
to a temp dir:

```go
tmpDir := t.TempDir()
t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)
t.Setenv("TOKEN_MONITOR_LOG_LEVEL", "error")  // suppress warning spam
```

`t.Setenv` automatically restores at test end. **It also marks the test as
non-parallel** — Go's `testing` package will fatal-error if you call
`t.Parallel()` after `t.Setenv()` in the same test.

Live examples: `cmd/token-monitor/status_command_test.go:TestPrintBreakdown_AcrossSessions`,
`pkg/installer/mcp_test.go`, `pkg/installer/hook_test.go`.

### Pattern 2 — Capturing stdout for command tests

Status and install command tests need the printed output. Use the existing
`captureStdout` helper rather than rolling your own `os.Pipe`:

```go
out := captureStdout(t, func() {
    cmd := &statusCommand{breakdown: true, window: "today"}
    if err := cmd.Execute(); err != nil {
        t.Fatalf("Execute() error = %v", err)
    }
})
```

The helper is currently in `cmd/token-monitor/status_command_test.go`. If a
second package needs it, lift to a shared `internal/testutil` package.

### Pattern 3 — `MemoryPositionStore` to avoid BoltDB lock conflicts

`pkg/reader.New` accepts a `PositionStore`. The default in production is BoltDB.
Tests **must** use `reader.NewMemoryPositionStore()` instead — running tests
concurrently with a real `token-monitor watch`/`stats` process otherwise
deadlocks on the BoltDB file lock. Same applies if test runs `token-monitor
serve --stdio` in a goroutine.

```go
factory := func() (reader.Reader, error) {
    return reader.New(reader.Config{
        PositionStore: reader.NewMemoryPositionStore(),
        Parser:        parser.New(),
    }, log)
}
```

### Pattern 4 — Time-zone-safe timestamps in window-filtered tests

Tests that exercise `--window today` or `get_today_usage` filter entries
against **midnight in the test machine's local TZ**. CI runs in UTC; local
dev usually doesn't. Hour-scale offsets (`-2 * time.Hour`) cross midnight
and silently drop entries.

❌ Fragile:
```go
{model: "claude-opus-4-7", timestampOffset: -2 * time.Hour}
```

✅ Robust:
```go
{model: "claude-opus-4-7", timestampOffset: -5 * time.Second}
```

Second-scale offsets only fail in the first ~5 seconds of UTC midnight, which
is essentially never. Confirmed by simulating CI:

```bash
TZ=UTC go test ./pkg/mcp/... -run TestGetTodayUsage -v
```

If a test genuinely needs a long-ago entry (e.g. testing the `all` window includes
distant past), use 90+ days to put it unambiguously outside any "today" window.

### Pattern 5 — `os.Chdir` in tests breaks parallelism

`pkg/installer/mcp_test.go::TestInstallMCP_ProjectScope` chdirs into a temp dir.
This is process-global — it cannot run with `t.Parallel()`, and any future
parallel test in the same package would get random cwd flips. The current test
documents this explicitly. If you add another `os.Chdir` test, restore on
cleanup:

```go
orig, _ := os.Getwd()
t.Cleanup(func() { _ = os.Chdir(orig) })
require.NoError(t, os.Chdir(tmpDir))
```

---

## Gotchas

- **Race detector with cgo**: `-race` is mandatory in CI. Run locally too;
  failures here block PR merges.
- **`<synthetic>` model entries**: `BreakdownByModel` and the v0.2 MCP
  breakdown tools intentionally skip these. `aggregator.Add` (used by the
  legacy single-session path) does NOT skip them. Tests asserting "totals
  match across paths" must account for this discrepancy or use real model
  IDs only.
- **`time.Now()` in fixtures**: any test using real `time.Now()` is
  deterministic only within the single test run. For window-edge cases use
  fixed `time.Date(...)` anchors.
- **`flag.ExitOnError` in CLI tests**: subcommands use `flag.ExitOnError`,
  which calls `os.Exit(2)` on a parse failure. Tests can't catch this with
  `recover` — feed only well-formed args to `runXxxCommand`.
- **`atomicWriteFile` rejects symlinks**: tests that pre-create a symlink at
  the target path and then call install will get the safety error, not a
  silent rewrite. This is intentional; the corresponding test in
  `pkg/installer/atomic_test.go` asserts it.

---

## CI/CD

Authoritative source: [`.github/workflows/ci.yml`](../.github/workflows/ci.yml).
Three jobs:

| Job | What | Failure mode |
|-----|------|--------------|
| **Test** | `go test -race -coverprofile=... ./...` | Any package failing test |
| **Lint** | `golangci-lint run --timeout=5m` (v1.64.8 pinned by `actions/setup-go@v5` + `golangci/golangci-lint-action@v4`) | Any lint warning above the configured set |
| **Build** | `go build` matrix across 5 OS×arch combos (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64) | Any platform fails to compile |

Codecov upload is best-effort (`fail_ci_if_error: false`) — coverage drops
don't block merge but are visible on the PR.

To match CI before pushing:

```bash
go test -race -coverprofile=coverage.out -covermode=atomic ./...
/tmp/golangci-lint-1.64.8-darwin-arm64/golangci-lint run --timeout=5m
GOOS=linux  GOARCH=amd64 go build ./...
GOOS=linux  GOARCH=arm64 go build ./...
GOOS=darwin GOARCH=amd64 go build ./...
GOOS=darwin GOARCH=arm64 go build ./...
GOOS=windows GOARCH=amd64 go build ./...
```

---

## Resources

Standard Go testing material — refer here, don't duplicate in this repo:

- [Testing package docs](https://pkg.go.dev/testing) — `t.Helper`, `t.TempDir`, `t.Setenv`, `t.Cleanup`
- [Dave Cheney — Prefer table-driven tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [golang/go wiki — TestComments](https://github.com/golang/go/wiki/TestComments)
- [go-cmp](https://pkg.go.dev/github.com/google/go-cmp/cmp) and [testify](https://pkg.go.dev/github.com/stretchr/testify) for richer assertions

Project navigation:

- [CONTRIBUTING.md](../CONTRIBUTING.md) — Setup and workflow
- [architecture.md](architecture.md) — System design (test fixtures align with package boundaries described there)
