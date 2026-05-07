# Token Monitor - Roadmap

> Planned features and upcoming milestones. Shipped releases are tracked in
> [CHANGELOG.md](../CHANGELOG.md); this file lists *future* work only.

**Last Updated**: 2026-05-07

---

## v0.3.0 - Distribution & DX Polish

Items deferred from earlier planning, now sequenced post-v0.2.

| Feature | Priority | Description |
|---------|----------|-------------|
| Homebrew tap | High | Easy installation for macOS/Linux without manual binary download |
| Interactive session selection | High | Numbered-list picker for `session show`/`watch` commands |
| Session navigation shortcuts | Medium | Arrow-key navigation in TUI dashboard (already partly via `pkg/tui`) |
| Session tagging | Medium | Custom tags persisted alongside session metadata in BoltDB |
| Database migrations | Medium | Versioned schema with auto-migration on startup |
| Configurable model abbreviation | Low | Override the built-in `son`/`opus`/`hai` abbreviations via config |

---

## v0.4.0 - Daemon Mode & Performance

The `INTEGRATION.md` design document (removed in 2026-05-07 cleanup) outlined
this work; specs preserved here.

| Feature | Priority | Description |
|---------|----------|-------------|
| Daemon mode | High | Long-running Unix-socket server for sub-10ms `query` and `status` |
| Auto-start daemon | Medium | First `query --current` call spawns the daemon if not running |
| PID file management | Medium | Prevent duplicate daemon processes; clean shutdown |
| LRU cache for session stats | Medium | Cache up to 100 sessions in-memory |
| Batched DB writes | Low | 100ms write window to reduce BoltDB churn under load |

---

## Future

| Version | Theme | Key Features |
|---------|-------|-------------|
| v0.5.0 | Data Management | Backup/restore, scheduled backups, project indexing |
| v0.6.0 | Cost Analysis | LiteLLM pricing integration, budget tracking, cost projections |
| v0.7.0 | Alerting | Token-limit warnings, Slack/Discord/webhook notifications |
| v0.8.0 | Web Dashboard | HTTP API, WebSocket live updates, React frontend |
| v0.9.0 | Analytics | Trend analysis, anomaly detection, Prometheus export |

---

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines.

To propose features or roadmap changes, open an issue on GitHub.
