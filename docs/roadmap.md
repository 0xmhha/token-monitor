# Token Monitor - Roadmap

> **Purpose**: Development roadmap and planned features for token-monitor project.

**Current Version**: v0.1.0 (Released 2025-11-21)

**Last Updated**: 2025-11-27

---

## v0.2.0 - CLI Enhancements

**Status**: In Progress

**Target**: Interactive session management and improved installation experience

### Features

| Feature | Priority | Status | Description |
|---------|----------|--------|-------------|
| Interactive session selection | High | Planned | Show numbered list, allow user selection |
| Session navigation shortcuts | Medium | Planned | `↑/↓` keys to navigate between sessions |
| Session tagging | Medium | Planned | Add custom tags to sessions |
| Session descriptions | Medium | Planned | Add custom descriptions to sessions |
| Database migrations | Medium | Planned | Version schema, auto-migration on startup |
| Homebrew tap | High | Planned | Easy installation for macOS/Linux |
| Scoop bucket | Low | Planned | Easy installation for Windows |

### Technical Tasks

- [ ] Implement interactive session picker with numbered list
- [ ] Add `↑/↓` keyboard navigation in watch mode
- [ ] Create session tagging system (`session tag <name> <tags>`)
- [ ] Add session description field (`session describe <name> <desc>`)
- [ ] Implement database schema versioning
- [ ] Create homebrew-token-monitor tap repository
- [ ] Create scoop-token-monitor bucket repository

---

## v0.3.0 - Data Management

**Status**: Planned

**Target**: Robust data handling and backup capabilities

### Features

| Feature | Priority | Status | Description |
|---------|----------|--------|-------------|
| Data backup | High | Planned | Export all session metadata |
| Data restore | High | Planned | Import from backup file |
| Scheduled backups | Medium | Planned | Automatic periodic backups |
| Project path indexing | Medium | Planned | Faster session lookups by project |

### Technical Tasks

- [ ] Implement `backup create` command
- [ ] Implement `backup restore` command
- [ ] Add backup scheduling configuration
- [ ] Create project path index in BoltDB
- [ ] Add backup retention policy

---

## v0.4.0 - TUI Dashboard

**Status**: Planned

**Target**: Full-featured terminal UI with Bubbletea

### Features

| Feature | Priority | Status | Description |
|---------|----------|--------|-------------|
| Full-screen dashboard | High | Planned | Real-time token usage display |
| Progress bars | Medium | Planned | Visual token consumption indicators |
| Multi-pane layout | Medium | Planned | Split views for multiple sessions |
| Color schemes | Low | Planned | Customizable color themes |
| Responsive layout | Medium | Planned | Adapt to terminal size |

### Technical Tasks

- [ ] Integrate Bubbletea framework
- [ ] Design dashboard layout components
- [ ] Implement real-time data binding
- [ ] Add burn rate visualization
- [ ] Create billing block timeline view
- [ ] Add terminal size detection and adaptation

---

## v0.5.0 - Performance & Testing

**Status**: Planned

**Target**: Optimized performance and comprehensive testing

### Performance Optimizations

| Feature | Priority | Status | Description |
|---------|----------|--------|-------------|
| LRU cache | Medium | Planned | Cache session stats (100 sessions) |
| Worker pool | Medium | Planned | Concurrent file processing |
| Batched DB writes | Low | Planned | 100ms write window |
| Memory pooling | Low | Planned | Reduce GC pressure |

### Testing Improvements

| Feature | Priority | Status | Description |
|---------|----------|--------|-------------|
| E2E tests | High | Planned | Full workflow integration tests |
| Performance benchmarks | Medium | Planned | Target: >10K lines/sec |
| Memory leak detection | Medium | Planned | 24-hour stress test |
| Concurrent access tests | Low | Planned | Multi-session benchmarks |

---

## Future Releases

### Cost Analysis (v0.6.0+)

- [ ] Integrate LiteLLM pricing API
- [ ] Calculate actual costs per session
- [ ] Budget tracking and alerts
- [ ] Cost projections and forecasting
- [ ] Generate cost reports

### Web Dashboard (v0.7.0+)

- [ ] HTTP REST API
- [ ] WebSocket for live updates
- [ ] React/Vue frontend
- [ ] Authentication & multi-user support
- [ ] Real-time charts and analytics

### Alerting System (v0.8.0+)

- [ ] Token limit warnings
- [ ] Cost threshold notifications
- [ ] Slack/Discord integration
- [ ] Email notifications
- [ ] Webhook support

### Advanced Analytics (v0.9.0+)

- [ ] Historical trend analysis
- [ ] Usage pattern detection
- [ ] Anomaly detection
- [ ] Prometheus metrics export
- [ ] Grafana dashboard templates

---

## Completed Releases

### v0.1.0 (2025-11-21)

**Core Features**
- JSONL parser with validation
- Claude data directory discovery
- Session metadata storage (BoltDB)
- File watcher with debouncing
- Token aggregator with burn rate
- Billing block detection

**CLI Commands**
- `stats`, `list`, `watch`
- `session name/list/show/delete/export`
- `config show/path/set/validate/reset`

**Post-Release Additions (2025-11-27)**
- Session list filters (`--project`, `--from`, `--to`, `--min-tokens`)
- Enhanced session show output (token breakdown, timeline, statistics)
- Keyboard shortcuts (`q`, `r`, `?`)
- Session export to CSV/JSON

---

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines.

To propose new features or changes to the roadmap, please open an issue on GitHub.
