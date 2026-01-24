# AgentManager Architecture

This document describes the architecture of AgentManager, a cross-platform tool for managing AI development CLI agents.

## High-Level Overview

AgentManager is designed as a modular, cross-platform application that:
1. **Detects** installed AI CLI agents across multiple package managers
2. **Manages** agent lifecycle (install, update, remove)
3. **Provides** multiple interfaces: CLI, TUI, Systray, REST API, and gRPC

The system follows a layered architecture with clear separation between:
- **User Interfaces** (CLI, TUI, Systray) → consume library packages
- **Library Packages** (pkg/*) → reusable, testable core logic
- **Platform Abstraction** → cross-platform support for macOS, Linux, Windows

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           User Interfaces                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │
│  │   CLI        │  │     TUI      │  │   Systray    │  │ REST/gRPC   │  │
│  │ (internal/   │  │ (internal/   │  │ (internal/   │  │  (pkg/api)  │  │
│  │    cli)      │  │    tui)      │  │   systray)   │  │             │  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬──────┘  │
└─────────┼─────────────────┼─────────────────┼─────────────────┼─────────┘
          │                 │                 │                 │
          ▼                 ▼                 ▼                 ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Core Library (pkg/*)                            │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────┐             │
│  │  catalog  │  │ detector  │  │ installer │  │  updater  │             │
│  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘  └─────┬─────┘             │
│        │              │              │              │                    │
│  ┌─────┴─────┐  ┌─────┴─────┐  ┌─────┴─────┐                            │
│  │   agent   │  │  config   │  │  storage  │                            │
│  └───────────┘  └───────────┘  └───────────┘                            │
│                                                                          │
│  ┌───────────┐  ┌───────────┐                                           │
│  │    ipc    │  │  logging  │                                           │
│  └───────────┘  └───────────┘                                           │
└────────────────────────────────────────┬────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                       Platform Abstraction (pkg/platform)                │
│           ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│           │   darwin    │  │    linux    │  │   windows   │             │
│           │   (macOS)   │  │             │  │             │             │
│           └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────────────┘
```

## Binaries

AgentManager builds two executables:

| Binary | Description | Entry Point |
|--------|-------------|-------------|
| `agentmgr` | Main CLI/TUI application for interactive use | `cmd/agentmgr/main.go` |
| `agentmgr-helper` | Background systray helper with notifications | `cmd/agentmgr-helper/main.go` |

## Package Descriptions

### cmd/agentmgr - Main CLI/TUI Binary

The primary user-facing application that provides:
- Command-line interface for agent management
- Interactive terminal UI (TUI) built with Bubble Tea
- Shell completion support (bash, zsh, fish, powershell)

Initializes the application by:
1. Loading configuration
2. Setting up platform abstraction
3. Initializing storage
4. Dispatching to CLI commands or TUI

### cmd/agentmgr-helper - Systray Helper Binary

A background application that:
- Displays a system tray icon with agent status
- Provides quick access to common operations
- Shows notifications for available updates
- Runs IPC and optional REST API servers for CLI communication
- Performs periodic background update checks

### pkg/agent - Agent Types and Version Handling

Core domain types:

```go
type Agent struct {
    ID, Name, Description, Homepage, Repository string
}

type Installation struct {
    AgentID, AgentName string
    Method             InstallMethod  // npm, brew, pip, native, etc.
    InstalledVersion   Version
    LatestVersion      *Version
    ExecutablePath     string
    DetectedAt         time.Time
}

type Version struct {
    Major, Minor, Patch int
    Prerelease, Build   string
}
```

Key responsibilities:
- Define `InstallMethod` enum (npm, brew, pip, pipx, uv, native, etc.)
- Semver-compliant version parsing and comparison
- Installation status tracking (current, outdated, unknown)

### pkg/catalog - Catalog Management

Manages the agent catalog (list of known agents and their installation methods):

```go
type Manager struct {
    config     *config.Config
    store      storage.Store
    catalog    *Catalog
    httpClient *http.Client
}
```

Responsibilities:
- Load catalog from cache, embedded file, or remote URL
- Refresh catalog from remote source with version comparison
- Search and filter agents by platform, name, or ID
- Fetch latest versions and changelogs from GitHub releases

The catalog schema (`Catalog`, `AgentDef`, `InstallMethodDef`) defines the JSON structure used in `catalog.json`.

### pkg/detector - Agent Detection with Strategies

Uses the Strategy pattern to detect installed agents across different package managers:

```go
type Strategy interface {
    Name() string
    Method() agent.InstallMethod
    IsApplicable(p platform.Platform) bool
    Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error)
}

type Detector struct {
    strategies []Strategy
    platform   platform.Platform
}
```

Built-in strategies in `pkg/detector/strategies/`:
| Strategy | File | Description |
|----------|------|-------------|
| Binary | `binary.go` | Searches PATH for known executables |
| NPM | `npm.go` | Checks npm global packages (`npm list -g`) |
| Pip | `pip.go` | Checks pip/pipx/uv installed packages |
| Brew | `brew.go` | Checks Homebrew installed formulae |

Detection runs strategies in parallel for performance, then deduplicates results.

### pkg/installer - Installation with Providers

Uses the Provider pattern for installation/update/uninstall operations:

```go
type Manager struct {
    npm    *providers.NPMProvider
    pip    *providers.PipProvider
    brew   *providers.BrewProvider
    native *providers.NativeProvider
    plat   platform.Platform
}
```

Built-in providers in `pkg/installer/providers/`:
| Provider | File | Supported Methods |
|----------|------|-------------------|
| NPM | `npm.go` | npm |
| Pip | `pip.go` | pip, pipx, uv |
| Brew | `brew.go` | brew |
| Native | `native.go` | native, curl, binary |

Each provider:
- Checks availability of the package manager
- Executes install/update/uninstall commands
- Queries registry for latest versions
- Returns structured results with version info

### pkg/storage - SQLite Persistence

Provides persistent storage using SQLite:

```go
type Store interface {
    Initialize(ctx context.Context) error
    Close() error
    
    // Installations
    SaveInstallation(ctx context.Context, inst *agent.Installation) error
    GetInstallation(ctx context.Context, key string) (*agent.Installation, error)
    ListInstallations(ctx context.Context, filter *agent.Filter) ([]*agent.Installation, error)
    
    // Detection cache
    SaveDetectionCache(ctx context.Context, installations []*agent.Installation) error
    GetDetectionCache(ctx context.Context) ([]*agent.Installation, time.Time, error)
    
    // Catalog cache
    SaveCatalogCache(ctx context.Context, data []byte, etag string) error
    GetCatalogCache(ctx context.Context) ([]byte, string, time.Time, error)
    
    // Update history
    SaveUpdateEvent(ctx context.Context, event *UpdateEvent) error
    GetUpdateHistory(ctx context.Context, agentID string, limit int) ([]*UpdateEvent, error)
}
```

Storage locations (platform-specific):
- **macOS**: `~/Library/Application Support/AgentManager/agentmgr.db`
- **Linux**: `~/.local/share/agentmgr/agentmgr.db`
- **Windows**: `%LOCALAPPDATA%\AgentManager\agentmgr.db`

### pkg/config - Configuration Management

Handles YAML configuration with these sections:

```go
type Config struct {
    Catalog   CatalogConfig    // Remote catalog URL, refresh interval
    Detection DetectionConfig  // Cache duration, enable/disable
    Updates   UpdateConfig     // Auto-check, notifications, auto-update
    UI        UIConfig         // Theme, colors, page size
    API       APIConfig        // REST/gRPC ports, authentication
    Helper    HelperConfig     // Systray settings
    Logging   LoggingConfig    // Level, format, file
    Agents    map[string]AgentConfig  // Per-agent overrides
}
```

Configuration file locations:
- **macOS**: `~/Library/Preferences/AgentManager/config.yaml`
- **Linux**: `~/.config/agentmgr/config.yaml`
- **Windows**: `%APPDATA%\AgentManager\config.yaml`

### pkg/platform - Platform Abstraction

Abstracts OS-specific operations:

```go
type Platform interface {
    ID() ID                    // darwin, linux, windows
    Architecture() string      // amd64, arm64
    
    // Paths
    GetDataDir() string
    GetConfigDir() string
    GetCacheDir() string
    GetIPCSocketPath() string
    
    // Executables
    FindExecutable(name string) (string, error)
    IsExecutableInPath(name string) bool
    
    // Auto-start
    EnableAutoStart(ctx context.Context) error
    DisableAutoStart(ctx context.Context) error
    
    // Notifications
    ShowNotification(title, message string) error
}
```

Platform implementations:
- `darwin.go` - macOS (Launch Agents, osascript notifications)
- `linux.go` - Linux (systemd user services, notify-send)
- `windows.go` - Windows (Startup folder, toast notifications)

### pkg/ipc - Inter-Process Communication

Enables communication between CLI and systray helper:

```go
type Server interface {
    Start(ctx context.Context) error
    Stop() error
    SetHandler(handler Handler)
}

type Client interface {
    Connect() error
    Close() error
    Send(msg *Message) (*Message, error)
}
```

Message types:
- **Requests**: `list_agents`, `get_agent`, `install_agent`, `update_agent`, `check_updates`, `shutdown`
- **Responses**: `success`, `error`, `progress`
- **Notifications**: `update_available`, `agent_installed`, `agent_updated`

Uses Unix domain sockets on macOS/Linux and named pipes on Windows.

### pkg/api - REST and gRPC APIs

#### REST API (`pkg/api/rest/`)

HTTP API for external integrations:

```
GET  /api/v1/agents          - List all detected agents
GET  /api/v1/agents/:key     - Get specific agent
POST /api/v1/agents/install  - Install an agent
POST /api/v1/agents/:key/update  - Update an agent
DELETE /api/v1/agents/:key   - Uninstall an agent
GET  /api/v1/catalog         - Get agent catalog
POST /api/v1/catalog/refresh - Refresh catalog
GET  /api/v1/status          - Get helper status
```

#### gRPC API (`pkg/api/grpc/`)

Protocol buffer-based API for typed integrations. Proto definitions in `pkg/api/proto/`.

### internal/cli - CLI Commands

Implements CLI using [Cobra](https://github.com/spf13/cobra):

```
agentmgr
├── agent           # Agent management
│   ├── list        # List detected agents
│   ├── refresh     # Force re-detection
│   ├── install     # Install an agent
│   ├── update      # Update agent(s)
│   ├── remove      # Remove an agent
│   └── info        # Show agent details
├── catalog         # Catalog management
│   ├── list        # List available agents
│   ├── refresh     # Refresh from remote
│   ├── search      # Search catalog
│   └── show        # Show agent details
├── config          # Configuration
│   ├── show        # Show current config
│   ├── set         # Set config value
│   └── path        # Show config file path
├── helper          # Systray helper control
│   ├── start       # Start helper
│   ├── stop        # Stop helper
│   └── status      # Check status
├── tui             # Launch TUI
├── completion      # Shell completion
└── version         # Show version
```

Output formatting handled by `internal/cli/output/` (tables, JSON, YAML).

### internal/tui - Terminal UI

Interactive TUI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea):

```go
type Model struct {
    config      *config.Config
    platform    platform.Platform
    agents      []*agent.Installation
    catalog     *catalog.Catalog
    currentView View  // Dashboard, AgentList, AgentDetail, Catalog, Settings
    list        list.Model
    spinner     spinner.Model
}
```

Views:
- **Dashboard** - Statistics overview
- **Agent List** - Installed agents with status
- **Agent Detail** - Full installation details
- **Catalog** - Browse available agents
- **Settings** - Configuration viewer

### internal/systray - System Tray App

Background helper using [systray](https://github.com/getlantern/systray):

```go
type App struct {
    config    *config.Config
    platform  platform.Platform
    store     storage.Store
    detector  *detector.Detector
    catalog   *catalog.Manager
    installer *installer.Manager
    ipcServer ipc.Server
    restServer *rest.Server
}
```

Features:
- Menu showing installed agents with update status
- Quick actions: refresh, update all, open TUI
- Periodic background update checks
- Desktop notifications for updates
- IPC server for CLI communication

## Data Flow

### Detection Flow

```
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│   CLI/TUI     │────▶│   Detector    │────▶│  Strategies   │
└───────────────┘     └───────────────┘     └───────┬───────┘
                                                    │
                      ┌───────────────┐             │ parallel
                      │    Storage    │◀────────────┤
                      │  (cache)      │             │
                      └───────────────┘     ┌───────▼───────┐
                                            │ Binary, NPM,  │
                                            │  Pip, Brew    │
                                            └───────────────┘
```

1. User runs `agentmgr agent list`
2. CLI checks detection cache validity (configurable TTL)
3. If cache valid → return cached installations
4. If cache expired → run all applicable strategies in parallel
5. Each strategy checks for agents using its package manager
6. Results are deduplicated and merged
7. Cache is updated with new detections
8. Installations returned to CLI for display

### Installation Flow

```
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│   CLI/TUI     │────▶│   Installer   │────▶│   Provider    │
└───────────────┘     │    Manager    │     │ (npm/pip/etc) │
                      └───────────────┘     └───────┬───────┘
                                                    │
                      ┌───────────────┐             │
                      │   Catalog     │◀────────────┘
                      │   Manager     │
                      └───────────────┘
```

1. User runs `agentmgr agent install claude-code`
2. CLI looks up agent in catalog to get installation methods
3. Selects preferred method (or prompts user)
4. Installer Manager routes to appropriate provider
5. Provider executes package manager commands
6. Result returned with installed version
7. New installation saved to storage

### Update Flow

```
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│   CLI/TUI     │────▶│   Detector    │────▶│ Check latest  │
└───────────────┘     └───────┬───────┘     │  versions     │
                              │             └───────────────┘
                              ▼
                      ┌───────────────┐
                      │   Installer   │────▶ Execute update
                      │    Manager    │
                      └───────────────┘
```

1. User runs `agentmgr agent update --all`
2. Detector fetches current installations (from cache or fresh)
3. For each installation, query registry for latest version
4. Compare versions, identify updates available
5. For each outdated agent, route to appropriate provider
6. Provider executes update command
7. Update event logged to storage

## Extension Points

### Adding a New Detection Strategy

1. Create `pkg/detector/strategies/newmethod.go`:

```go
type NewMethodStrategy struct {
    platform platform.Platform
}

func (s *NewMethodStrategy) Name() string { return "newmethod" }
func (s *NewMethodStrategy) Method() agent.InstallMethod { return agent.InstallMethodNewMethod }
func (s *NewMethodStrategy) IsApplicable(p platform.Platform) bool { /* check if available */ }
func (s *NewMethodStrategy) Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
    // Implementation
}
```

2. Register in `pkg/detector/detector.go`:

```go
func New(p platform.Platform) *Detector {
    d := &Detector{...}
    d.RegisterStrategy(NewNewMethodStrategy(p))
    return d
}
```

### Adding a New Installation Provider

1. Create `pkg/installer/providers/newmethod.go`:

```go
type NewMethodProvider struct {
    platform platform.Platform
}

func (p *NewMethodProvider) IsAvailable() bool { /* check if tool exists */ }
func (p *NewMethodProvider) Install(ctx context.Context, agentDef catalog.AgentDef, method catalog.InstallMethodDef, force bool) (*Result, error) { }
func (p *NewMethodProvider) Update(ctx context.Context, inst *agent.Installation, agentDef catalog.AgentDef, method catalog.InstallMethodDef) (*Result, error) { }
func (p *NewMethodProvider) Uninstall(ctx context.Context, inst *agent.Installation, method catalog.InstallMethodDef) error { }
func (p *NewMethodProvider) GetLatestVersion(ctx context.Context, method catalog.InstallMethodDef) (agent.Version, error) { }
```

2. Add to `pkg/installer/installer.go` Manager and route in switch statements.

### Adding a New Agent to Catalog

Edit `catalog.json`:

```json
{
  "id": "new-agent",
  "name": "New Agent",
  "description": "Description of the new agent",
  "homepage": "https://example.com",
  "repository": "https://github.com/org/repo",
  "install_methods": [
    {
      "method": "npm",
      "package": "@org/new-agent",
      "bin": "new-agent",
      "platforms": ["darwin", "linux", "windows"]
    }
  ],
  "changelog": {
    "type": "github_releases",
    "url": "https://api.github.com/repos/org/repo/releases"
  }
}
```

### Adding Platform Support

1. Create `pkg/platform/newos.go`:

```go
//go:build newos

type newOSPlatform struct{}

func newPlatform() Platform { return &newOSPlatform{} }

func (p *newOSPlatform) ID() ID { return "newos" }
func (p *newOSPlatform) GetDataDir() string { /* OS-specific path */ }
// ... implement all Platform interface methods
```

2. Add build tags and constants to `pkg/platform/platform.go`.

## Caching Strategy

AgentManager uses multi-level caching:

| Cache | Default TTL | Purpose |
|-------|-------------|---------|
| Detection Cache | 1 hour | Avoid re-scanning for installed agents |
| Update Check Cache | 15 minutes | Avoid repeated registry queries |
| Catalog Cache | 1 hour | Cache remote catalog locally |

Caches are stored in SQLite and can be invalidated with `--refresh` flags or by exceeding TTL.

## Thread Safety

- `Detector` uses `sync.RWMutex` for strategy registration
- `catalog.Manager` uses mutex for catalog access
- `systray.App` uses mutexes for agent list and menu items
- Storage operations are serialized by SQLite

## Error Handling

- Detection errors from individual strategies don't fail the entire operation
- Installation/update errors are returned to the caller
- IPC errors return structured error responses
- All errors include context via `fmt.Errorf("context: %w", err)`

## Testing

Test files are co-located with source (`*_test.go`). Run with:

```bash
make test           # Run all tests
make test-coverage  # Run with coverage
```
