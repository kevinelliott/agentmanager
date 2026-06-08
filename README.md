# AgentManager (agentmgr)

[![CI](https://github.com/kevinelliott/agentmanager/actions/workflows/ci.yml/badge.svg)](https://github.com/kevinelliott/agentmanager/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/kevinelliott/agentmanager)](https://github.com/kevinelliott/agentmanager/releases)
[![Go Version](https://img.shields.io/github/go-mod-go-version/kevinelliott/agentmanager)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/kevinelliott/agentmanager)](https://goreportcard.com/report/github.com/kevinelliott/agentmanager)
[![Go Reference](https://pkg.go.dev/badge/github.com/kevinelliott/agentmanager.svg)](https://pkg.go.dev/github.com/kevinelliott/agentmanager)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#installation)
[![Agents in Catalog](https://img.shields.io/badge/agents%20in%20catalog-93-blueviolet)](#supported-agents)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

A comprehensive CLI/TUI/Library application for detecting, managing, installing, and updating AI development CLI agents across macOS, Linux, and Windows.

> **Catalog:** 93 agents and counting. See the full list in [Supported Agents](#supported-agents).

## Features

- **Agent Detection**: Automatically detect installed AI CLI agents (Claude Code, Amp, Aider, GitHub Copilot CLI, Gemini CLI, and more)
- **Version Management**: Check for updates from package registries (npm, PyPI, Homebrew) and manage agent versions
- **Multiple Installation Methods**: Support for npm, pip, pipx, uv, Homebrew, native installers, and more
- **Fast Parallel Checks**: Version checks for dozens of installed agents run concurrently (10–20× faster than sequential)
- **Offline-First Catalog**: A baseline catalog is embedded into the binary, so `agentmgr agent list` works on a fresh `go install` before the first remote refresh
- **Live Install Output**: Pass `-v` to `agent install` / `agent update` to stream live subprocess output (brew / npm / pip / native) instead of a silent spinner
- **Beautiful CLI Output**: Colored output with animated spinners and properly aligned tables (honors `NO_COLOR`, `TERM=dumb`, and non-TTY pipes)
- **Beautiful TUI**: Interactive terminal interface built with Bubble Tea
- **Background Helper**: System tray application with notifications for available updates
- **REST & gRPC APIs**: Expose agent management via HTTP and gRPC for integration
- **Structured Logs**: `cfg.Logging` drives `log/slog` level / format / file destination across both binaries
- **Cross-Platform**: Works on macOS, Linux, and Windows

## Installation

Pick whichever method fits your setup. After installing, verify with `agentmgr doctor`.

### Homebrew (macOS / Linux)

```bash
brew install kevinelliott/tap/agentmanager
```

### Go Install

Requires Go 1.25+. Installs the latest tagged release into `$(go env GOPATH)/bin`:

```bash
go install github.com/kevinelliott/agentmanager/cmd/agentmgr@latest
```

### From Source

Requires Go 1.25+ and `make`:

```bash
# Clone the repository (creates ./agentmanager)
git clone https://github.com/kevinelliott/agentmanager.git
cd agentmanager

# Build both binaries into ./bin
make build

# Install to PATH
make install
```

### Verify the install

```bash
agentmgr doctor          # Check system health and configuration
agentmgr catalog list    # Confirm the embedded catalog loads (93 agents)
```

> **Offline-first:** the catalog is embedded into the binary, so `agentmgr catalog list`
> and `agentmgr agent list` work immediately on a fresh install — no network required
> until you run `agentmgr catalog refresh`.

## Quick Start

```bash
# List all detected agents (shows installed version and latest available)
agentmgr agent list

# Install a new agent
agentmgr agent install claude-code

# Update all agents
agentmgr agent update --all

# Stream subprocess output during install/update (brew, npm, pip, native)
agentmgr agent update -v aider

# Launch the interactive TUI
agentmgr tui

# Disable colored output
agentmgr --no-color agent list
```

### Example Output

```console
$ agentmgr catalog list

ID                NAME                METHODS     DESCRIPTION
----------------- ------------------- ----------- ----------------------------------
abtop             abtop               cargo +2    htop-style TUI monitoring AI agent se…
aichat            AIChat              binary +3   All-in-one LLM CLI with shell assista…
aider             Aider               pip +2      AI pair programming in your terminal
claude-code       Claude Code         native +1   Anthropic's official CLI for Claude A…
codex             Codex               binary +2   Lightweight coding agent from OpenAI …
gptme             gptme               brew +3     Terminal AI agent with local tools: w…
jcode             jcode               binary +2   Terminal AI coding agent with semanti…
oh-my-pi          oh-my-pi            bun +3      Terminal AI coding agent with LSP, su…
open-interpreter  Open Interpreter    pip +2      Natural-language interface that write…
opencode          OpenCode            brew +4     The open source AI coding agent for y…
reasonix          DeepSeek-Reasonix   homebrew +1 DeepSeek-native AI coding agent for t…
smallcode         SmallCode           native +3   Terminal-native coding agent built to…
…

93 agents available
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

### System Health

```bash
agentmgr doctor                  # Check system health and configuration
agentmgr doctor --verbose        # Show detailed output
```

### Self-Update

```bash
agentmgr upgrade                 # Check for and install updates
agentmgr upgrade --check         # Check for updates only
agentmgr upgrade --force         # Force reinstall
```

### Detection Plugins

```bash
agentmgr plugin list             # List installed plugins
agentmgr plugin create <name>    # Create a new plugin
agentmgr plugin validate <file>  # Validate plugin config
agentmgr plugin enable <name>    # Enable a plugin
agentmgr plugin disable <name>   # Disable a plugin
```

See [docs/plugins.md](docs/plugins.md) for detailed plugin documentation.

### API Documentation

```bash
agentmgr api docs                # Show REST API documentation
agentmgr api endpoints           # List all API endpoints
agentmgr api spec                # Output OpenAPI specification
```

### Global Options

```bash
--no-color        # Disable colored output (also respects NO_COLOR env var)
--config, -c      # Specify custom config file path
--verbose, -v     # Enable verbose output
--format, -f      # Output format (table, json, yaml)
```

## Supported Agents

The catalog ships with **93 agents**, embedded in the binary and refreshable from the
remote catalog with `agentmgr catalog refresh`. Use the `ID` with the agent commands,
e.g. `agentmgr agent install claude-code` or `agentmgr catalog show gptme`.

| Agent | ID | Installation Methods |
|-------|----|----------------------|
| abtop | `abtop` | cargo, native, powershell |
| Agent CLI | `agent-cli` | pip, pipx, uv |
| Agent Deck | `agent-deck` | brew, go, native |
| Agents CLI | `agents-cli` | pip, pipx, uv |
| AIChat | `aichat` | binary, brew, cargo, scoop |
| Aider | `aider` | pip, pipx, uv |
| Amazon Q Developer CLI | `amazon-q` | brew, dmg, native |
| Amp | `amp` | brew, chocolatey, native, npm |
| Antigravity CLI | `antigravity-cli` | brew, native, powershell |
| apfel | `apfel` | brew |
| Auggie | `auggie` | npm |
| Bernstein | `bernstein` | native, pip, pipx, uv |
| Blackbox CLI | `blackbox-cli` | native, npm, powershell |
| ByteRover CLI | `byterover-cli` | native, npm |
| Caveman Code | `caveman-code` | npm |
| Claude Code | `claude-code` | native, npm |
| Claude Squad | `claude-squad` | brew, native |
| Claw Orchestrator | `claw-orchestrator` | native, npm |
| Cline CLI | `cline-cli` | npm |
| cmux | `cmux` | binary, brew |
| cocoindex-code | `cocoindex-code` | pip, pipx, uv |
| Codebuff | `codebuff` | binary, npm |
| CodeWhale | `codewhale` | cargo, npm |
| Codex | `codex` | binary, brew, npm |
| Continue CLI | `continue-cli` | npm |
| CoreCoder | `corecoder` | pip, pipx, uv |
| Cortex Code | `cortex-code` | native |
| Crush | `crush` | brew, go, npm, scoop, winget |
| Cursor CLI | `cursor-cli` | native |
| Deep Agents CLI | `deepagents-cli` | native, pip, uv |
| DeepSeek CLI | `deepseek-cli` | npm |
| DeepSeek-Reasonix | `reasonix` | homebrew, npm |
| Dexter | `dexter` | git |
| Droid | `droid` | brew, native, powershell |
| ElevenLabs CLI | `elevenlabs-cli` | npm |
| fast-agent | `fast-agent` | pip, pipx, uv |
| fence | `fence` | brew, go, native |
| FetchCoder | `fetchcoder` | npm |
| Forge CLI | `forge-cli` | npm |
| ForgeCode | `forgecode` | native, npm |
| Gemini CLI | `gemini-cli` | npm |
| GitHub Copilot CLI | `copilot-cli` | brew, npm, winget |
| Goose | `goose` | brew, brew-cask, native, powershell |
| gptme | `gptme` | brew, pip, pipx, uv |
| Grok Build | `grok-build` | native |
| Grok CLI | `grok-cli` | bun, npm |
| Herdr | `herdr` | binary, brew, native, nix |
| Hermes Agent | `hermes-agent` | native, source |
| jcode | `jcode` | binary, brew, native |
| Junie CLI | `junie-cli` | brew, native, npm |
| Juno Code | `juno-code` | npm |
| Kilocode CLI | `kilocode-cli` | npm |
| Kimi Code | `kimi-code` | binary, native, pip, pipx, uv |
| Kiro CLI | `kiro-cli` | brew, native |
| Kode CLI | `kode-cli` | npm |
| kubectl-ai | `kubectl-ai` | krew, native, nix |
| late | `late-cli` | binary, brew, native |
| Letta Code | `letta-code` | npm |
| little-coder | `little-coder` | native, npm |
| Mastra Code | `mastracode` | npm |
| mcp2cli | `mcp2cli` | pip, uv |
| Mistral Vibe | `mistral-vibe` | brew, native, pip, uv |
| Nanocoder | `nanocoder` | brew, npm |
| nono | `nono` | brew, cargo |
| oh-my-pi | `oh-my-pi` | bun, native, npm, powershell |
| Ona | `ona` | binary, brew |
| Open Interpreter | `open-interpreter` | pip, pipx, uv |
| OpenClaw | `openclaw` | brew, native, npm |
| OpenCode | `opencode` | brew, chocolatey, curl, npm, scoop |
| OpenHands CLI | `openhands` | native, pip, pipx, uv |
| Paperclip | `paperclip` | npm, npx |
| Pi Agent Rust | `pi-agent-rust` | binary, native |
| Pi Coding Agent | `pi-coding-agent` | binary, npm |
| Plandex | `plandex` | native |
| Qoder CLI | `qoder-cli` | binary |
| Qwen Code | `qwen-code` | brew, npm |
| RA.Aid | `ra-aid` | brew, pip, pipx, uv |
| Rallies CLI | `rallies-cli` | pip, pipx |
| Ralph TUI | `ralph-tui` | bun, bunx |
| Roo Code CLI | `roo-code-cli` | native |
| rtk | `rtk` | binary, brew, cargo, native |
| Ruflo | `ruflo` | native, npm, npx |
| SmallCode | `smallcode` | native, npm, npx, powershell |
| Tabnine CLI | `tabnine-cli` | native |
| Tenere | `tenere` | brew, cargo, nix |
| TokenTracker | `tokentracker` | npm, npx |
| Tokscale | `tokscale` | bun, npm |
| Trae Agent | `trae-agent` | source |
| TunaCode CLI | `tunacode-cli` | pip, pipx, uv |
| Valyu CLI | `valyu-cli` | npm |
| VibeMux | `vibemux` | binary, native |
| Zeroshot | `zeroshot` | npm |
| zerostack | `zerostack` | cargo, native |

> Don't see your agent? [Contributions are welcome](CONTRIBUTING.md#adding-new-agents-to-the-catalog) — adding one is a single entry in `catalog.json`.

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

- Go 1.25+
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
