# Token Monitor v0.1.0

Initial release of Token Monitor - A comprehensive CLI tool for monitoring Claude Code token usage.

## 🎯 Key Features

### Live Token Monitoring
- **Real-time monitoring** with auto-discovery of Claude Code sessions
- **Incremental file reading** with position tracking for efficiency
- **Live terminal updates** without flickering

### Token Analytics
- **Comprehensive statistics** with breakdown by token type (input, output, cache)
- **Burn rate calculation** (tokens/min, tokens/hour) with 5-minute sliding window
- **Billing block tracking** with UTC-based 5-hour billing windows
- **Multi-dimensional grouping** by model, session, date, or hour

### Session Management
- **Persistent metadata storage** using BoltDB
- **Friendly session naming** with UUID mapping
- **Fast lookups** by name or UUID
- **Full CRUD operations** for sessions

### CLI Commands

```bash
# Display aggregated statistics
token-monitor stats [--group-by model,session] [--top 10]

# Live monitoring with real-time updates
token-monitor watch [--session my-session] [--refresh 1s]

# Session management
token-monitor session name <uuid> <name>
token-monitor session list [--sort name]
token-monitor session show <name|uuid>
token-monitor session delete <name|uuid>

# Configuration management
token-monitor config show [--format yaml|json]
token-monitor config path
token-monitor config reset
```

## 📊 Output Formats

- **Table**: Pretty-printed tables with formatting
- **JSON**: Structured JSON for programmatic use
- **Simple**: Plain text for scripting

## 🏗️ Technical Highlights

- **Go 1.22.6+** with modern Go patterns
- **Multi-platform**: Linux, macOS, Windows (amd64 & arm64)
- **High test coverage**: 71-90% across packages
- **Race-detector enabled** tests
- **Comprehensive documentation**: API docs, testing guide, usage examples

## 📦 Installation

### Download Binary
Download pre-built binaries from the [releases page](https://github.com/0xmhha/token-monitor/releases/tag/v0.1.0).

### Using Go
```bash
go install github.com/0xmhha/token-monitor/cmd/token-monitor@v0.1.0
```

### From Source
```bash
git clone https://github.com/0xmhha/token-monitor.git
cd token-monitor
go build -o token-monitor ./cmd/token-monitor
```

## 📚 Documentation

- [README.md](README.md) - Quick start and overview
- [USAGE.md](USAGE.md) - Comprehensive usage guide
- [ARCHITECTURE.md](docs/ARCHITECTURE.md) - Technical architecture
- [API.md](docs/API.md) - Complete API reference
- [TESTING.md](docs/TESTING.md) - Testing guide
- [CONTRIBUTING.md](CONTRIBUTING.md) - Development guide
- [CHANGELOG.md](CHANGELOG.md) - Detailed changelog

## ⚠️ Known Limitations

- Session export limited to JSON output (CSV planned)
- No interactive session selection from list
- No keyboard shortcuts in live monitoring
- Cost calculation not yet implemented

## 🔄 What's Next

See [todolist.md](docs/todolist.md) for planned improvements, including:
- Keyboard shortcuts for live monitoring
- Interactive session selection
- Enhanced export formats (CSV, PDF)
- TUI dashboard with bubbletea
- Cost calculation integration

## 🙏 Acknowledgments

Built for monitoring [Claude Code](https://claude.com/claude-code) token usage.

## 📝 License

See [LICENSE](LICENSE) file for details.

---

**Full Changelog**: https://github.com/0xmhha/token-monitor/blob/main/CHANGELOG.md
