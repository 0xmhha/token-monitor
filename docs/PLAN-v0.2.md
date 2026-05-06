# Token Monitor v0.2 — Cross-Session Breakdown + Install Automation

> Plan file for subagent-driven implementation. Each phase produces one commit.
> Branch: `feat/v0.2-cross-session-breakdown` (create on Phase 0).

## Goal

Claude Code의 statusline JSON이 5h/1w 사용률·reset 시각·세션 비용·context %를 이미 제공하므로, **token-monitor는 Claude Code가 주지 않는 정보**에 집중한다:

1. **세션 횡단 누적** — 오늘/7일/30일 단위 모든 세션 합산
2. **모델별 분리** — `message.model` 글롭 매칭으로 동적 분류 (하드코딩 X)
3. **MCP tools로 sub-agent 노출** — sub-agent가 자기 사용량을 직접 조회
4. **statusline 자동 통합** — `~/.claude/statusline-command.sh`에 marker block 주입

## Non-Goals

- ❌ Rolling 5h/1w 윈도우 자체 구현 (Claude Code statusline JSON이 이미 제공)
- ❌ Plan limit 절대값 하드코딩 (정확하지 않고 변동됨; `used_percentage`로 충분)
- ❌ Sonnet/Opus 분류 enum (글롭 매칭으로 충분)

## Phase 0 — `.mcp.json` 경로 버그 수정

**파일**: `/Users/wm-it-22-00661/Work/github/tools/token-monitor/.mcp.json`

**현재** (잘못됨):
```json
"command": "/Users/wm-it-22-00661/Work/github/token-monitor/bin/token-monitor"
```

**수정** (`tools/` 추가):
```json
"command": "/Users/wm-it-22-00661/Work/github/tools/token-monitor/bin/token-monitor"
```

**커밋 메시지**: `fix(mcp): correct binary path in .mcp.json`

---

## Phase A — Aggregator: 모델 분리 + 시간 윈도우 집계

**목적**: 세션 횡단으로 entries를 집계하되, 모델 글롭과 시간 윈도우로 필터링.

### 신규 파일

#### `pkg/aggregator/breakdown.go`

```go
package aggregator

import (
    "path/filepath"
    "strings"
    "time"

    "github.com/0xmhha/token-monitor/pkg/parser"
)

// ModelBreakdown holds per-model token totals.
type ModelBreakdown struct {
    Model        string  // exact model string from JSONL (e.g., "claude-sonnet-4-6")
    InputTokens  int
    OutputTokens int
    CacheCreate  int
    CacheRead    int
    TotalTokens  int
    EntryCount   int
}

// BreakdownByModel groups entries by exact model name.
// Skips entries with empty model or model == "<synthetic>".
func BreakdownByModel(entries []parser.UsageEntry) map[string]ModelBreakdown {
    out := make(map[string]ModelBreakdown)
    for _, e := range entries {
        m := e.Message.Model
        if m == "" || m == "<synthetic>" {
            continue
        }
        b := out[m]
        b.Model = m
        b.InputTokens += e.Message.Usage.InputTokens
        b.OutputTokens += e.Message.Usage.OutputTokens
        b.CacheCreate += e.Message.Usage.CacheCreationInputTokens
        b.CacheRead += e.Message.Usage.CacheReadInputTokens
        b.TotalTokens = b.InputTokens + b.OutputTokens + b.CacheCreate + b.CacheRead
        b.EntryCount++
        out[m] = b
    }
    return out
}

// MatchModel reports whether model matches glob (case-insensitive).
// Empty glob matches everything. Glob supports `*` wildcard via filepath.Match.
func MatchModel(model, glob string) bool {
    if glob == "" {
        return true
    }
    ok, err := filepath.Match(strings.ToLower(glob), strings.ToLower(model))
    if err != nil {
        return false
    }
    return ok
}

// FilterByModelGlob returns entries whose model matches the glob.
func FilterByModelGlob(entries []parser.UsageEntry, glob string) []parser.UsageEntry {
    if glob == "" {
        return entries
    }
    out := make([]parser.UsageEntry, 0, len(entries))
    for _, e := range entries {
        if MatchModel(e.Message.Model, glob) {
            out = append(out, e)
        }
    }
    return out
}

// FilterSince returns entries with Timestamp >= since.
func FilterSince(entries []parser.UsageEntry, since time.Time) []parser.UsageEntry {
    out := make([]parser.UsageEntry, 0, len(entries))
    for _, e := range entries {
        if !e.Timestamp.Before(since) {
            out = append(out, e)
        }
    }
    return out
}
```

#### `pkg/aggregator/breakdown_test.go`

다음 시나리오를 cover:
- `BreakdownByModel`: 3개 모델 entries → 3개 그룹, `<synthetic>`는 무시
- `MatchModel`: `*sonnet*` → `claude-sonnet-4-6` true, `claude-opus-4-7` false; case-insensitive
- `FilterByModelGlob`: 빈 글롭은 모두 통과
- `FilterSince`: timestamp 기준 정확히 cutoff

**최소 테스트 케이스**: 각 함수당 2~3개 (positive + edge), 총 8~10 케이스.

### 검증

```bash
go test ./pkg/aggregator/... -run Breakdown -v
go test ./pkg/aggregator/...   # 기존 테스트 깨지지 않는지
go vet ./...
```

**커밋**: `feat(aggregator): add model breakdown and window filtering`

---

## Phase B — `status` 명령에 stdin-aware 모드 + `breakdown` 서브출력

**목적**: Claude Code statusline에서 호출 시 stdin JSON을 받아 세션 정확히 식별. 출력은 **Claude Code 미제공 정보만**.

### 변경: `cmd/token-monitor/status_command.go`

기존 `statusCommand` struct에 추가:
```go
fromStdin bool   // --from-stdin
breakdown bool   // --breakdown (오늘 누적 + 모델별 분리)
window    string // --window: today, 7d, 30d, all (default: today)
modelGlob string // --model-glob: e.g., *sonnet* (default: empty = all)
```

### 새 동작 흐름 (`--from-stdin --breakdown` 시)

1. stdin JSON 파싱 (없으면 silent skip)
2. `session_id`로 현재 세션 JSONL 식별 (없으면 discovery fallback)
3. **모든 세션 entries 로드** → `FilterSince(now-window) → FilterByModelGlob → BreakdownByModel`
4. 출력 포맷 (compact 기본):
   ```
   day:340K | son:128K | opus:212K
   ```
   - 모델은 substring으로 abbreviate: sonnet→son, opus→opus, haiku→hai
   - 합계는 `display.FormatCompact()` 재사용

### 새 helper: `cmd/token-monitor/stdin_input.go`

```go
package main

import (
    "encoding/json"
    "io"
    "os"
)

// StatuslineInput captures the subset of Claude Code's stdin JSON we use.
type StatuslineInput struct {
    SessionID      string `json:"session_id"`
    TranscriptPath string `json:"transcript_path"`
    Model          struct {
        ID          string `json:"id"`
        DisplayName string `json:"display_name"`
    } `json:"model"`
}

// readStatuslineInput reads stdin if it has data, else returns nil.
// Non-blocking: returns nil if stdin is a TTY or empty.
func readStatuslineInput() *StatuslineInput {
    fi, err := os.Stdin.Stat()
    if err != nil || (fi.Mode()&os.ModeCharDevice) != 0 {
        return nil
    }
    data, err := io.ReadAll(os.Stdin)
    if err != nil || len(data) == 0 {
        return nil
    }
    var in StatuslineInput
    if err := json.Unmarshal(data, &in); err != nil {
        return nil
    }
    return &in
}
```

### Window 파싱

`pkg/display/window.go` 신규:
```go
package display

import (
    "fmt"
    "strings"
    "time"
)

// ParseWindow converts "today", "7d", "30d", "24h", "all" to a since time.
func ParseWindow(s string, now time.Time) (time.Time, error) {
    s = strings.ToLower(strings.TrimSpace(s))
    switch s {
    case "", "today":
        return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
    case "all":
        return time.Time{}, nil
    }
    if strings.HasSuffix(s, "d") {
        var n int
        if _, err := fmt.Sscanf(s, "%dd", &n); err == nil {
            return now.Add(-time.Duration(n) * 24 * time.Hour), nil
        }
    }
    if strings.HasSuffix(s, "h") {
        var n int
        if _, err := fmt.Sscanf(s, "%dh", &n); err == nil {
            return now.Add(-time.Duration(n) * time.Hour), nil
        }
    }
    return time.Time{}, fmt.Errorf("invalid window: %q", s)
}
```

### 테스트

- `pkg/display/window_test.go`: ParseWindow 5+ 케이스
- `cmd/token-monitor/stdin_input_test.go`: JSON unmarshal smoke test

### 검증

```bash
go build ./...
echo '{"session_id":"test","model":{"id":"sonnet","display_name":"Sonnet"}}' | ./bin/token-monitor status --from-stdin --breakdown --window today
```

**커밋**: `feat(status): add --from-stdin --breakdown --window flags`

---

## Phase C — MCP tools 3개 신규 추가

**목적**: sub-agent가 자기 사용량을 동적으로 조회.

### 변경: `pkg/mcp/tools.go`

기존 `RegisterTokenTools()`에 3개 도구 추가:

#### Tool 1: `get_session_breakdown`

```
{
  "name": "get_session_breakdown",
  "description": "Get token usage broken down by model for the current or specified session.",
  "input_schema": {
    "type": "object",
    "properties": {
      "session_id": {"type": "string", "description": "Session ID. If omitted, uses current session."}
    }
  }
}
```

**Output**: `{"session_id": "...", "breakdown": [{"model": "claude-sonnet-4-6", "total_tokens": 12345, "input_tokens": ..., "output_tokens": ..., "entry_count": 42}, ...]}`

#### Tool 2: `get_today_usage`

```
{
  "name": "get_today_usage",
  "description": "Get cumulative token usage today across all sessions, optionally filtered by model glob.",
  "input_schema": {
    "type": "object",
    "properties": {
      "model_glob": {"type": "string", "description": "Glob pattern like '*sonnet*'. Empty = all models."}
    }
  }
}
```

**Output**: `{"window":"today", "since":"2026-05-06T00:00:00Z", "total_tokens":..., "by_model":[...], "session_count":3}`

#### Tool 3: `get_usage_by_window`

```
{
  "name": "get_usage_by_window",
  "description": "Get token usage for an arbitrary time window with optional model filter.",
  "input_schema": {
    "type": "object",
    "properties": {
      "window": {"type": "string", "description": "today, 7d, 30d, 24h, all"},
      "model_glob": {"type": "string"}
    },
    "required": ["window"]
  }
}
```

**Output**: 위와 동일 shape.

### 구현 위치

`pkg/mcp/tools.go` 끝에 3개 tool handler 추가. 각각 ~30줄. discovery+reader+aggregator 재사용.

### 테스트

`pkg/mcp/tools_test.go`에 신규 테스트 6개 (각 도구당 2개: success + error):
- 빈 세션 → empty breakdown
- glob 매칭 → 정확한 필터링
- invalid window → JSON-RPC error

### 검증

```bash
go test ./pkg/mcp/... -v
go build ./...
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./bin/token-monitor serve --stdio | jq '.result.tools | map(.name)'
# expect: ["get_token_usage", ..., "get_session_breakdown", "get_today_usage", "get_usage_by_window"]
```

**커밋**: `feat(mcp): add session breakdown and window-based usage tools`

---

## Phase D — Install 자동화 (`token-monitor install ...`)

**목적**: 다른 머신에서 새로 설치할 때 statusline + MCP + hook 등록을 한 명령으로.

### 신규 패키지: `pkg/installer/`

#### `pkg/installer/marker.go`

Marker 기반 멱등 패치 + 백업.

```go
package installer

const (
    MarkerStart = "# >>> token-monitor >>> (managed block, do not edit)"
    MarkerEnd   = "# <<< token-monitor <<<"
)

// PatchMarkerBlock returns updated content with the marker block replaced or appended.
// If body is empty, removes the block (uninstall).
func PatchMarkerBlock(content, body string) string { ... }

// BackupFile creates path.bak.YYYYMMDD-HHMMSS and returns the backup path.
func BackupFile(path string) (string, error) { ... }
```

#### `pkg/installer/statusline.go`

```go
const StatuslineSnippet = `# >>> token-monitor >>> (managed block, do not edit)
if command -v token-monitor >/dev/null 2>&1; then
  tm_extra=$(echo "$input" | token-monitor status --from-stdin --breakdown --compact 2>/dev/null)
  [ -n "$tm_extra" ] && printf " \033[2m|\033[0m \033[0;35m%s\033[0m" "$tm_extra"
fi
# <<< token-monitor <<<
`

// InstallStatusline patches the script at path. Creates stub if missing.
// dryRun=true returns diff without writing.
func InstallStatusline(path string, dryRun, uninstall bool) (diff string, err error) { ... }
```

#### `pkg/installer/mcp.go`

`.mcp.json` 또는 `~/.claude.json`의 mcpServers 항목 등록. 기존 항목과 다른 경로면 거부.

#### `pkg/installer/hook.go`

`~/.claude/settings.json`의 hooks.PostToolUse 등록 (선택적).

### 신규 파일: `cmd/token-monitor/install_command.go`

Subcommands:
- `install statusline [--dry-run|--uninstall|--target PATH]`
- `install mcp [--global|--project|--uninstall]`
- `install hook [--uninstall]`
- `install all`
- `install --uninstall-all`

### 테스트

`pkg/installer/marker_test.go`:
- 빈 파일에 patch → marker block만 추가
- 기존 marker block 있는 파일 → body 교체 (idempotent)
- body 빈 문자열 → block 제거 (uninstall)
- backup 함수 → 파일명 형식 검증

`pkg/installer/statusline_test.go`:
- stub 생성 (파일 없음 + create=true)
- dry-run → 파일 변경 없음
- uninstall → marker 블록 제거 후 나머지 보존

### 검증

```bash
go test ./pkg/installer/... -v
./bin/token-monitor install statusline --dry-run --target /tmp/test-statusline.sh
./bin/token-monitor install statusline --target /tmp/test-statusline.sh
diff /tmp/test-statusline.sh /tmp/test-statusline.sh.bak.* | head -20
./bin/token-monitor install statusline --uninstall --target /tmp/test-statusline.sh
```

**커밋**: `feat(installer): add install/uninstall for statusline, mcp, and hook`

---

## Phase E — 문서 갱신 (선택, 시간 남으면)

- `README.md`의 Claude Code Integration 섹션을 `install` 명령 사용 흐름으로 갱신
- `docs/INTEGRATION.md`에 신규 MCP tools 추가
- `CHANGELOG.md`에 v0.2.0 entry

**커밋**: `docs: update integration guide for v0.2`

---

## 진행 원칙

- 각 Phase 끝마다 commit (Co-Authored-By 헤더 사용 안 함, 사용자 user 글로벌 룰)
- 각 Phase 끝마다 code review 서브에이전트 dispatch
- Critical 이슈 발견 시 즉시 fix subagent
- 전체 완료 후 final review + finishing-a-development-branch 진행
