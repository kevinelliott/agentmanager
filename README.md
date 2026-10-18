# AgentManager (agentmgr)

[![CI](https://github.com/kevinelliott/agentmanager/actions/workflows/ci.yml/badge.svg)](https://github.com/kevinelliott/agentmanager/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/kevinelliott/agentmanager)](https://github.com/kevinelliott/agentmanager/releases)
[![Go Version](https://img.shields.io/github/go-mod-go-version/kevinelliott/agentmanager)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/kevinelliott/agentmanager)](https://goreportcard.com/report/github.com/kevinelliott/agentmanager)
[![Go Reference](https://pkg.go.dev/badge/github.com/kevinelliott/agentmanager.svg)](https://pkg.go.dev/github.com/kevinelliott/agentmanager)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#installation)
[![Agents in Catalog](https://img.shields.io/badge/agents%20in%20catalog-269-blueviolet)](#supported-agents)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

A comprehensive CLI/TUI/Library application for detecting, managing, installing, and updating AI development CLI agents across macOS, Linux, and Windows.

> **Catalog:** 269 agents and counting. See the full list in [Supported Agents](#supported-agents).

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
agentmgr catalog list    # Confirm the embedded catalog loads
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

Catalog listings show tools supported on the selected platform; this example uses Linux.

```console
$ agentmgr catalog list --platform linux

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

145 agents available
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
agentmgr agent list --local      # List installations without remote version checks
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
agentmgr catalog refresh         # Refresh from remote if cache is stale
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

The catalog ships with **269 agents**, embedded in the binary and refreshable from the
remote catalog with `agentmgr catalog refresh`. Use the `ID` with the agent commands,
e.g. `agentmgr agent install claude-code` or `agentmgr catalog show gptme`.

See [catalog research and update records](docs/catalog/README.md) for upstream
verification sources and installation/setup limits.

| Agent | ID | Installation Methods |
|-------|----|----------------------|
| abtop | `abtop` | cargo, native, powershell |
| Agent CLI | `agent-cli` | pip, pipx, uv |
| Agent Deck | `agent-deck` | brew, go, native |
| Agent Eval | `agent-eval` | npm |
| Agent Manager | `agent-manager` | brew |
| Agent of Empires | `agent-of-empires` | brew, native |
| agent-browser | `agent-browser` | brew, npm |
| agent-memory | `agent-memory` | native |
| Agents CLI | `agents-cli` | pip, pipx, uv |
| AIChat | `aichat` | binary, brew, cargo, scoop |
| Aider | `aider` | pip, pipx, uv |
| Amazon Q Developer CLI | `amazon-q` | brew, dmg, native |
| Amp | `amp` | brew, chocolatey, native, npm |
| Amplifier | `amplifier` | native |
| amux | `amux` | brew |
| Antigravity CLI | `antigravity-cli` | brew, native, powershell |
| apfel | `apfel` | brew |
| Apify CLI | `apify-cli` | npm |
| Arize Phoenix CLI | `phoenix` | uv |
| Atomic Agent | `atomic-agent` | native, powershell |
| Auggie | `auggie` | npm |
| Autohand | `autohand` | npm |
| AWS Documentation MCP | `aws-documentation-mcp` | uv |
| Azure MCP Server | `azure-mcp` | npm |
| BBARIT Agent | `bbarit-agent` | native, powershell |
| Beads | `beads` | brew, native, npm |
| Beads Rust | `beads-rust` | brew |
| BentoML CLI | `bentoml` | uv |
| Bernstein | `bernstein` | native, pip, pipx, uv |
| Blackbox CLI | `blackbox-cli` | native, npm, powershell |
| Brave Search MCP | `brave-search-mcp` | npm |
| Browser Use | `browser-use` | uv |
| Browserbase Browse | `browserbase-browse` | npm |
| ByteRover CLI | `byterover-cli` | native, npm |
| CASS | `cass` | brew |
| Caveman Code | `caveman-code` | npm |
| CC Switch CLI | `cc-switch-cli` | brew |
| CCManager | `ccmanager` | npm |
| ccstatusline | `ccstatusline` | npm |
| ccusage | `ccusage` | brew, npm, npx |
| Chroma CLI | `chroma` | uv |
| Chrome DevTools MCP | `chrome-devtools-mcp` | npm |
| Cisco MCP Scanner | `mcp-scanner` | uv |
| Cisco Skill Scanner | `skill-scanner` | uv |
| Claude Code | `claude-code` | native, npm |
| Claude Code Log | `claude-code-log` | uv |
| Claude Code Router | `claude-code-router` | brew, npm, npx |
| Claude Code Templates | `claude-code-templates` | npm |
| Claude Replay | `claude-replay` | npm |
| Claude Squad | `claude-squad` | brew, native |
| Claude-Codex Bridge | `claude-codex-bridge` | npm, npx |
| Claw Orchestrator | `claw-orchestrator` | native, npm |
| ClawCodex | `clawcodex` | native |
| ClawHub | `clawhub` | npm |
| Cline CLI | `cline-cli` | npm |
| Cloudflare cf | `cloudflare-cf` | npm |
| cmux | `cmux` | binary, brew |
| CocoIndex Code | `cocoindex-code` | pip, pipx, uv |
| Code2Prompt | `code2prompt` | brew |
| Codebuff | `codebuff` | binary, npm |
| CodeWhale | `codewhale` | cargo, npm |
| Codex | `codex` | binary, brew, npm |
| CodexBar CLI | `codexbar` | brew |
| Command Code | `command-code` | npm |
| Context Mode | `context-mode` | npm |
| Context7 CLI | `context7` | npm |
| Continue CLI | `continue-cli` | npm |
| CoreCoder | `corecoder` | pip, pipx, uv |
| Cortex Code | `cortex-code` | native |
| Crawl4AI CLI | `crawl4ai` | uv |
| CrewAI CLI | `crewai` | uv |
| Crush | `crush` | brew, go, npm, scoop, winget |
| Cursor CLI | `cursor-cli` | native |
| dao-code | `dao-code` | native, npm, npx |
| DBHub | `dbhub` | npm |
| Deep Agents CLI | `deepagents-cli` | native, pip, uv |
| DeepEval | `deepeval` | uv |
| DeepSeek CLI | `deepseek-cli` | npm |
| DeepSeek Harness | `deepseek-harness` | npm |
| DeepSeek-Reasonix | `reasonix` | homebrew, npm |
| Destructive Command Guard | `dcg` | brew |
| Dexter | `dexter` | git |
| Dirac | `dirac` | npm |
| Docker Agent | `docker-agent` | brew |
| Droid | `droid` | brew, native, powershell |
| DSCODE | `dscode` | native |
| E2B CLI | `e2b-cli` | npm |
| ElevenLabs CLI | `elevenlabs-cli` | npm |
| Engram | `engram` | brew |
| Exa MCP Server | `exa-mcp` | npm |
| Fabric AI | `fabric` | brew |
| fast-agent | `fast-agent` | pip, pipx, uv |
| FastMCP CLI | `fastmcp` | uv |
| fauxnix | `fauxnix` | npm |
| fence | `fence` | brew, go, native |
| FetchCoder | `fetchcoder` | npm |
| Firecrawl CLI | `firecrawl-cli` | npm |
| Firecrawl MCP Server | `firecrawl-mcp` | npm |
| Foremerge | `foremerge` | npm |
| Forge CLI | `forge-cli` | npm |
| ForgeCode | `forgecode` | native, npm |
| FrontierAgent | `frontier-agent` | native |
| Gajae Code | `gajae-code` | native |
| garak | `garak` | uv |
| Gas Town | `gastown` | brew |
| Gemini CLI | `gemini-cli` | npm |
| GitHub Copilot CLI | `copilot-cli` | brew, npm, winget |
| GitHub MCP Server | `github-mcp` | brew |
| GitIngest | `gitingest` | uv |
| Google ADK CLI | `google-adk` | uv |
| Google Workspace CLI | `gws` | npm |
| Goose | `goose` | brew, brew-cask, native, powershell |
| gptme | `gptme` | brew, pip, pipx, uv |
| Grafana MCP Server | `grafana-mcp` | brew |
| grepai | `grepai` | brew |
| Grok Build | `grok-build` | native |
| Grok CLI | `grok-cli` | bun, npm |
| Happy | `happy` | npm |
| Hayhooks | `hayhooks` | uv |
| Herdr | `herdr` | binary, brew, native, nix |
| Hermes Agent | `hermes-agent` | native, source |
| Hugging Face CLI | `huggingface-cli` | uv |
| Hunk | `hunk` | brew, native, npm |
| iCode | `icode` | uv |
| Inspect AI | `inspect-ai` | uv |
| IronClaw | `ironclaw` | native |
| jcode | `jcode` | binary, brew, native |
| jevgrep | `jevgrep` | npm |
| Junie CLI | `junie-cli` | brew, native, npm |
| Juno Code | `juno-code` | npm |
| Kilocode CLI | `kilocode-cli` | npm |
| Kimchi | `kimchi` | brew, native |
| Kimi Code | `kimi-code` | binary, native, pip, pipx, uv |
| Kiro CLI | `kiro-cli` | brew, native |
| KlaatCode | `klaatcode` | brew, native, npm, npx |
| Kode CLI | `kode-cli` | npm |
| kubectl-ai | `kubectl-ai` | krew, native, nix |
| Kubernetes MCP Server | `kubernetes-mcp` | npm |
| Langflow CLI | `langflow` | uv |
| LangGraph CLI | `langgraph-cli` | uv |
| late | `late-cli` | binary, brew, native |
| Letta Code | `letta-code` | npm |
| Leviathan | `leviathan` | cargo |
| little-coder | `little-coder` | native, npm |
| llama-swap | `llama-swap` | brew |
| llama.cpp CLI | `llama-cpp` | brew |
| LLM | `llm` | uv |
| llmfit | `llmfit` | brew |
| macOS Harness | `macos-harness` | uv |
| Magentic UI CLI | `magentic-ui` | uv |
| MarkItDown | `markitdown` | uv |
| Marvin CLI | `marvin` | uv |
| Mastra Code | `mastracode` | npm |
| MCO | `mco` | npm |
| MCP Filesystem Server | `mcp-filesystem` | npm |
| MCP Inspector | `mcp-inspector` | npm |
| MCP Proxy | `mcp-proxy` | uv |
| MCP Publisher | `mcp-publisher` | brew |
| MCP Remote | `mcp-remote` | npm |
| mcp-use CLI | `mcp-use` | npm |
| mcp2cli | `mcp2cli` | pip, uv |
| MCPJam Inspector | `mcpjam-inspector` | npm |
| MCPM | `mcpm` | uv |
| MCPorter | `mcporter` | npm |
| Meta Muse Code | `muse-code` | native |
| Mini SWE Agent | `mini-swe-agent` | uv |
| MiniMax Code | `minimax-code` | native, powershell |
| Mistral Vibe | `mistral-vibe` | brew, native, pip, uv |
| MLX LM | `mlx-lm` | uv |
| Moltis | `moltis` | brew |
| MongoDB MCP Server | `mongodb-mcp` | npm |
| n8n CLI | `n8n` | npm |
| Nanobot | `nanobot` | uv |
| Nanocoder | `nanocoder` | brew, npm |
| Neon MCP Server | `neon-mcp` | npm |
| nono | `nono` | brew, cargo |
| NTM | `ntm` | brew |
| NullClaw | `nullclaw` | brew |
| Oh My ClaudeCode | `oh-my-claudecode` | npm |
| Oh My Codex | `oh-my-codex` | npm |
| oh-my-pi | `oh-my-pi` | bun, native, npm, powershell |
| Ollama CLI | `ollama` | brew |
| Omnigent | `omnigent` | brew, native, pipx, uv |
| OmO | `omo` | npm |
| Ona | `ona` | binary, brew |
| Only CLI / oc | `only-cli` | npm |
| Open Interpreter | `open-interpreter` | pip, pipx, uv |
| open_scion | `open-scion` | native |
| OpenAgents Control | `openagents-control` | npm |
| OpenAgents Launcher | `openagents-launcher` | npm |
| openclaude | `openclaude` | npm, npx |
| OpenClaw | `openclaw` | brew, native, npm |
| OpenCode | `opencode` | brew, chocolatey, curl, npm, scoop |
| OpenHands CLI | `openhands` | native, pip, pipx, uv |
| OpenQodex | `openqodex` | npm |
| OpenSpec | `openspec` | brew, npm |
| OpenViking | `openviking` | uv |
| OptiLLM | `optillm` | uv |
| Oracle | `oracle` | npm |
| Ouroboros | `ouroboros` | native, pip, pipx, uv |
| Paperclip | `paperclip` | npm, npx |
| Peekaboo | `peekaboo` | brew |
| Penguin Harness | `penguin-harness` | native, npm, npx |
| Perplexity MCP | `perplexity-mcp` | npm |
| Pi Agent Rust | `pi-agent-rust` | binary, native |
| Pi Coding Agent | `pi-coding-agent` | binary, npm |
| PicoClaw | `picoclaw` | brew |
| Plandex | `plandex` | native |
| Plannotator TUI | `plannotator-tui` | brew |
| Playwright CLI | `playwright-cli` | npm |
| Playwright MCP | `playwright-mcp` | npm |
| Playwright Test CLI | `playwright-test` | npm |
| Pochi | `pochi` | npm |
| Promptfoo | `promptfoo` | npm |
| Pydantic AI CLI | `pydantic-clai` | uv |
| Qdrant MCP Server | `qdrant-mcp` | uv |
| QMD | `qmd` | npm |
| Qoder CLI | `qoder-cli` | binary |
| Qwen Code | `qwen-code` | brew, npm |
| RA.Aid | `ra-aid` | brew, pip, pipx, uv |
| Rallies CLI | `rallies-cli` | pip, pipx |
| Ralph TUI | `ralph-tui` | bun, bunx |
| Ralphex | `ralphex` | brew |
| Redis MCP Server | `redis-mcp` | uv |
| Repomix | `repomix` | npm |
| rmux | `rmux` | brew, cargo |
| Roo Code CLI | `roo-code-cli` | native |
| rtk | `rtk` | binary, brew, cargo, native |
| Ruflo | `ruflo` | native, npm, npx |
| RuVector CLI | `ruvector` | npm |
| Sentry CLI | `sentry-cli` | npm |
| Serena | `serena` | uv |
| skills | `skills` | npm, npx |
| Skillshare CLI | `skillshare` | brew |
| SmallCode | `smallcode` | native, npm, npx, powershell |
| Sno Station | `sno-station` | native |
| Snyk Agent Scan | `snyk-agent-scan` | uv |
| Spec Kit | `spec-kit` | uv |
| Summarize | `summarize` | npm |
| Supabase MCP Server | `supabase-mcp` | npm |
| Supergateway | `supergateway` | npm |
| Symphony | `symphony` | native |
| Tabnine CLI | `tabnine-cli` | native |
| Tavily MCP | `tavily-mcp` | npm |
| Tenere | `tenere` | brew, cargo, nix |
| Terraform MCP Server | `terraform-mcp` | brew |
| TokenTracker | `tokentracker` | npm, npx |
| Tokscale | `tokscale` | bun, npm |
| TOON CLI | `toon-cli` | npm |
| Trae Agent | `trae-agent` | source |
| tsk | `tsk` | brew, native |
| tuicr | `tuicr` | brew |
| TUIOS | `tuios` | brew, native |
| TunaCode CLI | `tunacode-cli` | pip, pipx, uv |
| Unreal Agent | `unreal-agent` | brew, native |
| Valyu CLI | `valyu-cli` | npm |
| Vibe Kanban | `vibe-kanban` | npm |
| VibeMux | `vibemux` | binary, native |
| VT Code | `vtcode` | brew, native |
| wcgw | `wcgw` | uv |
| WhipCode | `whipcode` | native |
| Workmux | `workmux` | brew |
| Worktrunk | `worktrunk` | brew |
| YYLO | `yylo` | npm |
| ZCode | `zcode` | native |
| zero | `zero` | native, npm |
| ZeroClaw | `zeroclaw` | native |
| Zeroshot | `zeroshot` | npm |
| zerostack | `zerostack` | cargo, native |

YYLO currently supports macOS and Linux and requires Bash.

For agent-browser, npm and Homebrew installs also download Chrome. On Linux, if
Chrome reports missing system libraries, run `agent-browser install --with-deps`
in an interactive terminal with the required privileges.

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
  refresh_interval: 24h
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

### Performance checks

```bash
make benchmark-performance
agentmgr -v agent list --refresh   # Log time spent in each detection and version-check phase
```

The benchmarks cover local inventory subprocesses, cold and warm SQLite caches,
expired version metadata, and catalog rendering. CI saves benchmark results as
an artifact. Compare results on the same machine and Go version.

The TUI displays installations before remote version checks finish. Its catalog
shows a page of agents; use the up and down arrows to scroll.

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
