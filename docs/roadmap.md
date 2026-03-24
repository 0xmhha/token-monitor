# Token Monitor - Roadmap

> Planned features and upcoming milestones.

**Last Updated**: 2026-03-24

---

## v0.2.0 - CLI Enhancements

| Feature | Priority | Description |
|---------|----------|-------------|
| Interactive session selection | High | Show numbered list, allow user selection |
| Session navigation shortcuts | Medium | Arrow key navigation in watch mode |
| Session tagging | Medium | Add custom tags to sessions |
| Database migrations | Medium | Version schema, auto-migration on startup |
| Homebrew tap | High | Easy installation for macOS/Linux |

---

## v0.3.0 - Daemon Mode & Performance

| Feature | Priority | Description |
|---------|----------|-------------|
| Daemon mode | High | Unix socket server for <10ms queries |
| Auto-start daemon | Medium | Start on first `query --current` call |
| PID file management | Medium | Prevent duplicate daemon processes |
| LRU cache | Medium | Cache session stats (100 sessions) |
| Batched DB writes | Low | 100ms write window for performance |

---

## Future

| Version | Theme | Key Features |
|---------|-------|-------------|
| v0.4.0 | Data Management | Backup/restore, scheduled backups, project indexing |
| v0.5.0 | Cost Analysis | LiteLLM pricing, budget tracking, cost projections |
| v0.6.0 | Alerting | Token limit warnings, Slack/Discord notifications |
| v0.7.0 | Web Dashboard | HTTP API, WebSocket live updates, React frontend |
| v0.8.0 | Analytics | Trend analysis, anomaly detection, Prometheus export |

---

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines.

To propose features or roadmap changes, please open an issue on GitHub.
