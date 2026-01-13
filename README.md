# AgentManager (agentmgr)

[![CI](https://github.com/kevinelliott/agentmgr/actions/workflows/ci.yml/badge.svg)](https://github.com/kevinelliott/agentmgr/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/kevinelliott/agentmgr)](https://github.com/kevinelliott/agentmgr/releases)
[![Go Version](https://img.shields.io/github/go-mod-go-version/kevinelliott/agentmgr)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/kevinelliott/agentmgr)](https://goreportcard.com/report/github.com/kevinelliott/agentmgr)
[![Go Reference](https://pkg.go.dev/badge/github.com/kevinelliott/agentmgr.svg)](https://pkg.go.dev/github.com/kevinelliott/agentmgr)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

A comprehensive CLI/TUI/Library application for detecting, managing, installing, and updating AI development CLI agents across macOS, Linux, and Windows.

## Features

- **Agent Detection**: Automatically detect installed AI CLI agents (Claude Code, Amp, Aider, GitHub Copilot CLI, Gemini CLI, and more)
- **Version Management**: Check for updates from package registries (npm, PyPI, Homebrew) and manage agent versions
- **Multiple Installation Methods**: Support for npm, pip, pipx, uv, Homebrew, native installers, and more
- **Beautiful CLI Output**: Colored output with animated spinners and properly aligned tables
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
# List all detected agents (shows installed version and latest available)
agentmgr agent list

# Install a new agent
agentmgr agent install claude-code

# Update all agents
agentmgr agent update --all

# Launch the interactive TUI
agentmgr tui

# Disable colored output
agentmgr --no-color agent list
```

### Example Output

```console
$ agentmgr catalog list

ID            NAME                    METHODS    DESCRIPTION
------------  ----------------------  ---------  ----------------------------------------
agent-cli     Agent CLI               npm        Unified CLI for interacting with multi...
agent-deck    Agent Deck              go +2      Terminal session manager for AI codin...
aider         Aider                   pip +2     AI pair programming in your terminal
amazon-q      Amazon Q Developer CLI  brew +1    Agentic chat experience in your termi...
amp           Amp                     native +3  Frontier coding agent by Sourcegraph ...
blackbox-cli  Blackbox CLI            native +1  AI-powered CLI for natural language c...
claude-code   Claude Code             npm +1     Anthropic's official CLI for Claude A...
claude-squad  Claude Squad            brew +1    Terminal application for managing mul...
codex         Codex                   npm +2     Lightweight coding agent from OpenAI ...
continue-cli  Continue CLI            npm        Open-source AI code assistant CLI
crush         Crush                   go +2      Terminal-based AI coding agent from C...
cursor-cli    Cursor CLI              native     Cursor AI editor CLI agent
deepseek-cli  DeepSeek CLI            npm        Command-line AI coding assistant leve...
dexter        Dexter                  npm +2     AI coding agent built on Claude Code
droid         Droid                   brew +1    AI-powered software engineering agent...
gemini-cli    Gemini CLI              npm        Google's Gemini AI in your terminal
goose         Goose                   native +3  Open-source AI agent from Block that ...
copilot-cli   GitHub Copilot CLI      npm +1     GitHub Copilot in the command line
juno-code     Juno Code               npm        AI coding agent with web interface
kilocode-cli  Kilocode CLI            npm        Terminal UI for Kilo Code
kubectl-ai    kubectl-ai              native +2  AI-powered Kubernetes Assistant
nanocoder     Nanocoder               npm +1     Community-driven local-first CLI codi...
opencode      OpenCode                npm +2     The open source AI coding agent
openhands     OpenHands CLI           uv +3      AI-driven development CLI with termin...
pi-coding-agent  Pi Coding Agent      npm        AI coding assistant from Pi
plandex       Plandex                 native     Open source AI coding agent for large...
qoder-cli     Qoder CLI               binary     Qoder AI coding assistant CLI
qwen-code     Qwen Code               npm +1     Open-source AI coding agent
rallies-cli   Rallies CLI             pip +1     AI-powered investment research CLI
ralph-tui     Ralph TUI               native     Autonomous AI agent with TUI
tokscale      Tokscale                bun +1     High-performance CLI for monitoring A...
tunacode-cli  Tunacode CLI            npm        AI coding assistant CLI

32 agents available
```

```console
$ agentmgr agent list

ID            AGENT               METHOD  VERSION              LATEST   STATUS
------------  ------------------  ------  -------------------  -------  ------
aider         Aider               pip     0.86.1               0.86.1   ●
amp           Amp                 npm     1.0.25               1.0.25   ●
blackbox-cli  Blackbox CLI        npm     0.0.9                0.8.1    ⬆
claude-code   Claude Code         npm     2.1.3                2.1.3    ●
claude-squad  Claude Squad        native  1.0.13               -        ●
continue-cli  Continue CLI        npm     1.5.29               1.5.29   ●
crush         Crush               native  0.24.0               -        ●
cursor-cli    Cursor CLI          native  2025.11.25           -        ●
gemini-cli    Gemini CLI          native  0.15.1               -        ●
copilot-cli   GitHub Copilot CLI  npm     0.0.340              0.0.377  ⬆
opencode      OpenCode            npm     1.0.119              1.1.10   ⬆
qoder-cli     Qoder CLI           native  0.1.15               -        ●
qwen-code     Qwen Code           npm     0.2.3                0.6.1    ⬆
tokscale      Tokscale            npm     1.0.22               1.0.22   ●
```

> **Legend:** ● = up to date, ⬆ = update available

## Commands

### Agent Management

```bash
agentmgr agent list              # List all detected agents (uses cache)
agentmgr agent list --refresh    # Force re-detection, ignore cache
agentmgr agent refresh           # Force re-detection and update cache
agentmgr agent install <name>    # Install an agent
agentmgr agent update <name>     # Update specific agent
agentmgr agent update --all      # Update all agents
agentmgr agent info <name>       # Show agent details
agentmgr agent remove <name>     # Remove an agent
```

> **Note:** Agent detection results are cached for 1 hour by default. Use `agent refresh` or `agent list --refresh` to force re-detection.

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

### Global Options

```bash
--no-color        # Disable colored output (also respects NO_COLOR env var)
--config, -c      # Specify custom config file path
--verbose, -v     # Enable verbose output
--format, -f      # Output format (table, json, yaml)
```

## Supported Agents

| Agent | Installation Methods |
|-------|---------------------|
| Agent CLI | npm |
| Agent Deck | brew, go, native |
| Aider | pip, pipx, uv |
| Amazon Q Developer CLI | brew, native, dmg |
| Amp | native, npm, brew, chocolatey |
| Blackbox CLI | npm, native, powershell |
| Claude Code | npm, native |
| Claude Squad | brew, native |
| Codex | npm, brew, binary |
| Continue CLI | npm |
| Crush | brew, npm, go, winget, scoop |
| Cursor CLI | native |
| DeepSeek CLI | npm |
| Dexter | npm, brew, native |
| Droid | brew, native, powershell |
| Gemini CLI | npm |
| GitHub Copilot CLI | npm, brew, winget |
| Goose | native, brew, pip, cargo |
| Juno Code | npm |
| Kilocode CLI | npm |
| kubectl-ai | native, krew, nix |
| Nanocoder | npm, brew |
| OpenCode | npm, brew, scoop, chocolatey, curl |
| OpenHands CLI | uv, pip, pipx, native |
| Pi Coding Agent | npm |
| Plandex | native |
| Qoder CLI | binary |
| Qwen Code | npm, brew |
| Rallies CLI | pip, pipx |
| Ralph TUI | native |
| Tokscale | bun, npm |
| Tunacode CLI | npm |

## Architecture

AgentManager consists of two binaries:

1. **`agentmgr`** - Main CLI/TUI application for interactive use
2. **`agentmgr-helper`** - Background systray helper with notifications

### Library Usage

AgentManager can be used as a Go library:

```go
import (
    "github.com/kevinelliott/agentmgr/pkg/detector"
    "github.com/kevinelliott/agentmgr/pkg/catalog"
    "github.com/kevinelliott/agentmgr/pkg/installer"
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

detection:
  cache_duration: 1h              # How long to cache detected agents
  update_check_cache_duration: 15m # How long to cache update check results
  cache_enabled: true             # Set to false to always detect fresh

updates:
  check_interval: 6h
  auto_check: true
  notify: true

ui:
  theme: auto
  compact: false
  use_colors: true  # Set to false to disable colored output

logging:
  level: info
  file: ""
```

## Development

### Prerequisites

- Go 1.24+
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

# Run all checks (fmt, vet, lint, test)
make check

# Run tests with coverage
make test-coverage
```

### Project Structure

```
agentmgr/
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
