# AgentManager (agentmgr)

A comprehensive CLI/TUI/Library application for detecting, managing, installing, and updating AI development CLI agents across macOS, Linux, and Windows.

## Features

- **Agent Detection**: Automatically detect installed AI CLI agents (Claude Code, Aider, GitHub Copilot CLI, Gemini CLI, and more)
- **Version Management**: Check for updates and manage agent versions
- **Multiple Installation Methods**: Support for npm, pip, pipx, uv, Homebrew, native installers, and more
- **Beautiful TUI**: Interactive terminal interface built with Bubble Tea
- **Background Helper**: System tray application with notifications for available updates
- **REST & gRPC APIs**: Expose agent management via HTTP and gRPC for integration
- **Cross-Platform**: Works on macOS, Linux, and Windows

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/kevinelliott/agentmgr.git
cd agentmgr

# Build
make build

# Install to PATH
make install
```

### Homebrew (macOS)

```bash
brew install kevinelliott/tap/agentmanager
```

### Go Install

```bash
go install github.com/kevinelliott/agentmanager/cmd/agentmgr@latest
```

## Quick Start

```bash
# List all detected agents
agentmgr agent list

# Check for updates
agentmgr agent list --check-updates

# Install a new agent
agentmgr agent install claude-code

# Update all agents
agentmgr agent update --all

# Launch the interactive TUI
agentmgr tui
```

## Commands

### Agent Management

```bash
agentmgr agent list              # List all detected agents
agentmgr agent install <name>    # Install an agent
agentmgr agent update <name>     # Update specific agent
agentmgr agent update --all      # Update all agents
agentmgr agent info <name>       # Show agent details
agentmgr agent remove <name>     # Remove an agent
```

### Catalog Management

```bash
agentmgr catalog list            # List available agents
agentmgr catalog refresh         # Refresh from remote
agentmgr catalog search <query>  # Search catalog
agentmgr catalog show <name>     # Show agent details
```

### Configuration

```bash
agentmgr config show             # Show current config
agentmgr config set <key> <val>  # Set config value
agentmgr config path             # Show config file path
```

### Background Helper

```bash
agentmgr helper start            # Start systray helper
agentmgr helper stop             # Stop systray helper
agentmgr helper status           # Check helper status
```

## Supported Agents

| Agent | Installation Methods |
|-------|---------------------|
| Claude Code | npm, native installer |
| Aider | pip, pipx, uv |
| GitHub Copilot CLI | npm, brew, winget |
| Gemini CLI | npm |
| Continue CLI | npm |
| OpenCode | npm, brew, scoop, chocolatey, curl |
| Cursor CLI | native installer |
| Qoder CLI | native binary |
| Amazon Q Developer | brew, native |

## Architecture

AgentManager consists of two binaries:

1. **`agentmgr`** - Main CLI/TUI application for interactive use
2. **`agentmgr-helper`** - Background systray helper with notifications

### Library Usage

AgentManager can be used as a Go library:

```go
import (
    "github.com/kevinelliott/agentmanager/pkg/detector"
    "github.com/kevinelliott/agentmanager/pkg/catalog"
    "github.com/kevinelliott/agentmanager/pkg/installer"
)

// Create a detector
d := detector.New(platform.Current())

// Detect all installed agents
installations, err := d.DetectAll(ctx, agentDefs)

// Install an agent
mgr := installer.NewManager(platform.Current())
result, err := mgr.Install(ctx, agentDef, method, false)
```

## Configuration

Configuration is stored in:
- macOS: `~/Library/Preferences/AgentManager/config.yaml`
- Linux: `~/.config/agentmgr/config.yaml`
- Windows: `%APPDATA%\AgentManager\config.yaml`

Example configuration:

```yaml
catalog:
  refresh_interval: 1h
  github_token: ""  # Optional: for higher rate limits

updates:
  check_interval: 6h
  auto_check: true
  notify: true

ui:
  theme: auto
  compact: false

logging:
  level: info
  file: ""
```

## Development

### Prerequisites

- Go 1.22+
- Make
- golangci-lint (for linting)

### Building

```bash
# Build all binaries
make build

# Run tests
make test

# Run linter
make lint

# Run tests with coverage
make test-coverage
```

### Project Structure

```
agentmanager/
├── cmd/
│   ├── agentmgr/           # CLI/TUI binary
│   └── agentmgr-helper/    # Systray binary
├── pkg/                    # Public library packages
│   ├── agent/              # Agent types, versions
│   ├── catalog/            # Catalog management
│   ├── detector/           # Agent detection
│   ├── installer/          # Installation management
│   ├── storage/            # SQLite storage
│   ├── config/             # Configuration
│   ├── ipc/                # IPC communication
│   ├── api/                # gRPC & REST APIs
│   └── platform/           # Platform abstraction
├── internal/
│   ├── cli/                # CLI commands
│   ├── tui/                # TUI interface
│   └── systray/            # Systray helper
└── catalog.json            # Default agent catalog
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
