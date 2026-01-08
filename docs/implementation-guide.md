# AgentManager Implementation Guide

## Project Structure

```
agent-manager/
├── cmd/
│   └── agentmgr/
│       ├── main.go                 # Entry point
│       └── root.go                 # Root command setup
├── internal/
│   ├── cli/                        # CLI commands (Cobra)
│   │   ├── agent/
│   │   │   ├── list.go
│   │   │   ├── install.go
│   │   │   ├── update.go
│   │   │   ├── info.go
│   │   │   └── remove.go
│   │   ├── catalog/
│   │   │   ├── list.go
│   │   │   ├── refresh.go
│   │   │   └── search.go
│   │   ├── config/
│   │   │   ├── show.go
│   │   │   ├── set.go
│   │   │   └── path.go
│   │   ├── helper/
│   │   │   ├── start.go
│   │   │   ├── stop.go
│   │   │   └── status.go
│   │   └── version/
│   │       └── version.go
│   ├── tui/                        # TUI components (Bubble Tea)
│   │   ├── app.go                  # Main TUI app
│   │   ├── components/             # Reusable components
│   │   │   ├── header.go
│   │   │   ├── table.go
│   │   │   ├── list.go
│   │   │   ├── panel.go
│   │   │   ├── statusbar.go
│   │   │   ├── progress.go
│   │   │   ├── spinner.go
│   │   │   └── form.go
│   │   ├── styles/                 # Lipgloss styles
│   │   │   └── styles.go
│   │   ├── views/                  # View models
│   │   │   ├── dashboard.go
│   │   │   ├── agentlist.go
│   │   │   ├── agentdetail.go
│   │   │   ├── catalog.go
│   │   │   ├── install.go
│   │   │   ├── update.go
│   │   │   ├── settings.go
│   │   │   └── help.go
│   │   └── messages/               # Bubble Tea messages
│   │       └── messages.go
│   ├── ui/                         # Shared UI utilities
│   │   ├── output.go               # Output formatters (table, JSON, etc.)
│   │   ├── colors.go               # Color detection and NO_COLOR
│   │   └── progress.go             # Progress indicators for CLI
│   ├── manager/                    # Business logic
│   │   ├── agent.go                # Agent operations
│   │   ├── catalog.go              # Catalog operations
│   │   ├── installer.go            # Installation logic
│   │   └── updater.go              # Update logic
│   └── config/                     # Configuration
│       ├── config.go               # Config struct and loading
│       └── paths.go                # Path resolution
├── pkg/
│   └── types/                      # Shared types
│       ├── agent.go
│       ├── catalog.go
│       └── config.go
├── go.mod
└── go.sum
```

---

## Dependencies

### Required Packages

```go
// go.mod
module github.com/kevinelliott/agent-manager

go 1.22

require (
    // CLI framework
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.2

    // TUI framework
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/charmbracelet/bubbles v0.18.0

    // UI components
    github.com/olekukonko/tablewriter v0.0.5
    github.com/briandowns/spinner v1.23.0
    github.com/schollz/progressbar/v3 v3.14.1

    // Utilities
    github.com/go-resty/resty/v2 v2.11.0
    gopkg.in/yaml.v3 v3.0.1
    github.com/Masterminds/semver/v3 v3.2.1
)
```

### Installation

```bash
go get github.com/spf13/cobra
go get github.com/spf13/viper
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/bubbles
go get github.com/olekukonko/tablewriter
go get github.com/briandowns/spinner
go get github.com/schollz/progressbar/v3
go get github.com/go-resty/resty/v2
go get gopkg.in/yaml.v3
go get github.com/Masterminds/semver/v3
```

---

## Core Implementation Examples

### 1. Main Entry Point

```go
// cmd/agentmgr/main.go
package main

import (
    "fmt"
    "os"

    "github.com/kevinelliott/agent-manager/internal/cli/agent"
    "github.com/kevinelliott/agent-manager/internal/cli/catalog"
    "github.com/kevinelliott/agent-manager/internal/cli/config"
    "github.com/kevinelliott/agent-manager/internal/cli/helper"
    "github.com/kevinelliott/agent-manager/internal/cli/version"
    "github.com/kevinelliott/agent-manager/internal/tui"
    "github.com/spf13/cobra"
)

var (
    Version   = "dev"
    GitCommit = "unknown"
    BuildDate = "unknown"
)

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

var rootCmd = &cobra.Command{
    Use:   "agentmgr",
    Short: "AgentManager - Manage AI agents across your system",
    Long: `AgentManager helps you install, update, and manage AI agents
like Claude Desktop, Cursor, Zed, and more from a single tool.`,
    Version: Version,
}

func init() {
    // Global flags
    rootCmd.PersistentFlags().Bool("json", false, "Output as JSON")
    rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Minimal output")
    rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose output")
    rootCmd.PersistentFlags().Bool("no-color", false, "Disable colors")

    // Add command groups
    rootCmd.AddCommand(agent.NewAgentCmd())
    rootCmd.AddCommand(catalog.NewCatalogCmd())
    rootCmd.AddCommand(config.NewConfigCmd())
    rootCmd.AddCommand(helper.NewHelperCmd())
    rootCmd.AddCommand(version.NewVersionCmd(Version, GitCommit, BuildDate))
    rootCmd.AddCommand(tui.NewTUICmd())
}
```

### 2. Agent List Command (CLI)

```go
// internal/cli/agent/list.go
package agent

import (
    "fmt"
    "os"

    "github.com/kevinelliott/agent-manager/internal/manager"
    "github.com/kevinelliott/agent-manager/internal/ui"
    "github.com/spf13/cobra"
)

func NewListCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "list",
        Short: "List all detected agents",
        Long:  "List all AI agents currently installed on your system",
        RunE:  runList,
    }

    return cmd
}

func runList(cmd *cobra.Command, args []string) error {
    // Get flags from root command
    jsonOutput, _ := cmd.Flags().GetBool("json")
    quiet, _ := cmd.Flags().GetBool("quiet")
    verbose, _ := cmd.Flags().GetBool("verbose")

    // Load agents
    mgr := manager.NewAgentManager()
    agents, err := mgr.ListAgents()
    if err != nil {
        return fmt.Errorf("failed to list agents: %w", err)
    }

    // Format output based on flags
    out := ui.NewOutput(os.Stdout, ui.OutputOptions{
        JSON:    jsonOutput,
        Quiet:   quiet,
        Verbose: verbose,
    })

    return out.RenderAgentList(agents)
}
```

### 3. Output Formatter

```go
// internal/ui/output.go
package ui

import (
    "encoding/json"
    "fmt"
    "io"
    "os"

    "github.com/kevinelliott/agent-manager/pkg/types"
    "github.com/olekukonko/tablewriter"
)

type OutputOptions struct {
    JSON    bool
    Quiet   bool
    Verbose bool
}

type Output struct {
    writer  io.Writer
    options OutputOptions
    colors  *ColorScheme
}

func NewOutput(w io.Writer, opts OutputOptions) *Output {
    return &Output{
        writer:  w,
        options: opts,
        colors:  DetectColorScheme(),
    }
}

func (o *Output) RenderAgentList(agents []types.Agent) error {
    if o.options.JSON {
        return o.renderJSON(map[string]interface{}{
            "agents": agents,
            "summary": map[string]int{
                "total":            len(agents),
                "updates_available": countUpdatesAvailable(agents),
            },
        })
    }

    if o.options.Quiet {
        for _, agent := range agents {
            fmt.Fprintln(o.writer, agent.Name)
        }
        return nil
    }

    if o.options.Verbose {
        return o.renderVerboseAgentList(agents)
    }

    return o.renderTableAgentList(agents)
}

func (o *Output) renderTableAgentList(agents []types.Agent) error {
    // Print header
    o.printBox("Installed Agents")

    // Create table
    table := tablewriter.NewWriter(o.writer)
    table.SetHeader([]string{"NAME", "VERSION", "LATEST", "METHOD", "STATUS"})
    table.SetBorder(true)
    table.SetAutoWrapText(false)
    table.SetColumnAlignment([]int{
        tablewriter.ALIGN_LEFT,
        tablewriter.ALIGN_LEFT,
        tablewriter.ALIGN_LEFT,
        tablewriter.ALIGN_LEFT,
        tablewriter.ALIGN_LEFT,
    })

    // Add rows
    for _, agent := range agents {
        status := "Up to date"
        if agent.UpdateAvailable {
            status = o.colors.Warning("Update Available")
        }

        table.Append([]string{
            agent.Name,
            agent.Version,
            agent.LatestVersion,
            agent.InstallMethod,
            status,
        })
    }

    table.Render()

    // Print summary
    updatesAvailable := countUpdatesAvailable(agents)
    fmt.Fprintf(o.writer, "\n %s\n\n",
        o.colors.Muted(fmt.Sprintf("%d agents installed • %d update available",
            len(agents), updatesAvailable)))

    if updatesAvailable > 0 {
        fmt.Fprintf(o.writer, "%s\n",
            o.colors.Info("💡 Run 'agentmgr agent update --all' to update all agents"))
    }

    return nil
}

func (o *Output) renderJSON(data interface{}) error {
    enc := json.NewEncoder(o.writer)
    enc.SetIndent("", "  ")
    return enc.Encode(data)
}

func (o *Output) printBox(title string) {
    border := "─"
    corner := "─"
    width := 76

    fmt.Fprintln(o.writer, "┌"+border+corner+border+"┐")
    fmt.Fprintf(o.writer, "│ %s%s │\n", title,
        fmt.Sprintf("%*s", width-len(title)-2, ""))
    fmt.Fprintln(o.writer, "└"+border+corner+border+"┘")
}

func countUpdatesAvailable(agents []types.Agent) int {
    count := 0
    for _, agent := range agents {
        if agent.UpdateAvailable {
            count++
        }
    }
    return count
}
```

### 4. Color Detection

```go
// internal/ui/colors.go
package ui

import (
    "fmt"
    "os"
)

type ColorScheme struct {
    enabled bool
}

func DetectColorScheme() *ColorScheme {
    // Check NO_COLOR environment variable
    _, noColor := os.LookupEnv("NO_COLOR")

    // Check if output is a terminal
    fileInfo, _ := os.Stdout.Stat()
    isTTY := (fileInfo.Mode() & os.ModeCharDevice) != 0

    return &ColorScheme{
        enabled: !noColor && isTTY,
    }
}

func (c *ColorScheme) Success(s string) string {
    if !c.enabled {
        return s
    }
    return fmt.Sprintf("\033[32m%s\033[0m", s)
}

func (c *ColorScheme) Warning(s string) string {
    if !c.enabled {
        return s
    }
    return fmt.Sprintf("\033[33m%s\033[0m", s)
}

func (c *ColorScheme) Error(s string) string {
    if !c.enabled {
        return s
    }
    return fmt.Sprintf("\033[31m%s\033[0m", s)
}

func (c *ColorScheme) Info(s string) string {
    if !c.enabled {
        return s
    }
    return fmt.Sprintf("\033[36m%s\033[0m", s)
}

func (c *ColorScheme) Muted(s string) string {
    if !c.enabled {
        return s
    }
    return fmt.Sprintf("\033[2m%s\033[0m", s)
}

func (c *ColorScheme) Primary(s string) string {
    if !c.enabled {
        return s
    }
    return fmt.Sprintf("\033[35m%s\033[0m", s)
}
```

### 5. TUI Main App

```go
// internal/tui/app.go
package tui

import (
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/kevinelliott/agent-manager/internal/tui/styles"
    "github.com/kevinelliott/agent-manager/internal/tui/views"
    "github.com/kevinelliott/agent-manager/pkg/types"
    "github.com/spf13/cobra"
)

func NewTUICmd() *cobra.Command {
    return &cobra.Command{
        Use:   "tui",
        Short: "Launch TUI interface",
        Long:  "Launch the interactive terminal user interface",
        RunE:  runTUI,
    }
}

func runTUI(cmd *cobra.Command, args []string) error {
    p := tea.NewProgram(
        NewModel(),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )

    _, err := p.Run()
    return err
}

type ViewType int

const (
    DashboardView ViewType = iota
    AgentListView
    AgentDetailView
    CatalogView
    InstallView
    UpdateView
    SettingsView
    HelpView
)

type Model struct {
    currentView ViewType
    width       int
    height      int

    // Sub-models
    dashboard   *views.Dashboard
    agentList   *views.AgentList
    agentDetail *views.AgentDetail
    catalog     *views.Catalog
    install     *views.Install
    update      *views.Update
    settings    *views.Settings
    help        *views.Help

    // Shared state
    agents      []types.Agent
    catalogData []types.CatalogEntry

    // UI state
    showHelp    bool
    err         error
    loading     bool
}

func NewModel() Model {
    return Model{
        currentView: DashboardView,
        dashboard:   views.NewDashboard(),
        agentList:   views.NewAgentList(),
        agentDetail: views.NewAgentDetail(),
        catalog:     views.NewCatalog(),
        install:     views.NewInstall(),
        update:      views.NewUpdate(),
        settings:    views.NewSettings(),
        help:        views.NewHelp(),
    }
}

func (m Model) Init() tea.Cmd {
    return tea.Batch(
        loadAgentsCmd,
        loadCatalogCmd,
    )
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        return m, nil

    case tea.KeyMsg:
        switch msg.String() {
        case "ctrl+c", "q":
            if m.currentView == DashboardView {
                return m, tea.Quit
            }
            // Go back to dashboard
            m.currentView = DashboardView
            return m, nil

        case "?":
            m.showHelp = !m.showHelp
            return m, nil

        case "i":
            m.currentView = InstallView
            return m, nil

        case "c":
            m.currentView = CatalogView
            return m, nil

        case "s":
            m.currentView = SettingsView
            return m, nil
        }

    case agentsLoadedMsg:
        m.agents = msg.agents
        m.loading = false
        return m, nil

    case catalogLoadedMsg:
        m.catalogData = msg.entries
        return m, nil

    case errorMsg:
        m.err = msg.err
        return m, nil
    }

    // Route to appropriate view
    return m.updateCurrentView(msg)
}

func (m Model) updateCurrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd

    switch m.currentView {
    case DashboardView:
        *m.dashboard, cmd = m.dashboard.Update(msg)
    case AgentListView:
        *m.agentList, cmd = m.agentList.Update(msg)
    case AgentDetailView:
        *m.agentDetail, cmd = m.agentDetail.Update(msg)
    case CatalogView:
        *m.catalog, cmd = m.catalog.Update(msg)
    case InstallView:
        *m.install, cmd = m.install.Update(msg)
    case UpdateView:
        *m.update, cmd = m.update.Update(msg)
    case SettingsView:
        *m.settings, cmd = m.settings.Update(msg)
    }

    return m, cmd
}

func (m Model) View() string {
    if m.showHelp {
        return m.help.View()
    }

    var content string
    switch m.currentView {
    case DashboardView:
        content = m.dashboard.View()
    case AgentListView:
        content = m.agentList.View()
    case AgentDetailView:
        content = m.agentDetail.View()
    case CatalogView:
        content = m.catalog.View()
    case InstallView:
        content = m.install.View()
    case UpdateView:
        content = m.update.View()
    case SettingsView:
        content = m.settings.View()
    }

    return styles.AppStyle.Render(content)
}

// Commands

type agentsLoadedMsg struct {
    agents []types.Agent
}

type catalogLoadedMsg struct {
    entries []types.CatalogEntry
}

type errorMsg struct {
    err error
}

func loadAgentsCmd() tea.Msg {
    // TODO: Actual implementation
    return agentsLoadedMsg{agents: []types.Agent{}}
}

func loadCatalogCmd() tea.Msg {
    // TODO: Actual implementation
    return catalogLoadedMsg{entries: []types.CatalogEntry{}}
}
```

### 6. Dashboard View

```go
// internal/tui/views/dashboard.go
package views

import (
    "fmt"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/kevinelliott/agent-manager/internal/tui/components"
    "github.com/kevinelliott/agent-manager/internal/tui/styles"
    "github.com/kevinelliott/agent-manager/pkg/types"
)

type Dashboard struct {
    agents      []types.Agent
    cursor      int
    width       int
    height      int
}

func NewDashboard() *Dashboard {
    return &Dashboard{}
}

func (d Dashboard) Update(msg tea.Msg) (Dashboard, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "j", "down":
            if d.cursor < len(d.agents)-1 {
                d.cursor++
            }
        case "k", "up":
            if d.cursor > 0 {
                d.cursor--
            }
        case "enter":
            // View agent details
            return d, func() tea.Msg {
                return agentSelectedMsg{agent: d.agents[d.cursor]}
            }
        }
    }

    return d, nil
}

func (d Dashboard) View() string {
    header := components.Header{
        Title:   "AgentManager v1.0.0",
        Width:   d.width,
    }

    status := d.renderStatus()
    quickActions := d.renderQuickActions()
    agentList := d.renderAgentList()
    activity := d.renderActivity()

    // Layout
    topRow := lipgloss.JoinHorizontal(
        lipgloss.Top,
        status,
        quickActions,
    )

    content := lipgloss.JoinVertical(
        lipgloss.Left,
        header.View(),
        topRow,
        agentList,
        activity,
        d.renderFooter(),
    )

    return content
}

func (d Dashboard) renderStatus() string {
    updatesAvailable := 0
    for _, agent := range d.agents {
        if agent.UpdateAvailable {
            updatesAvailable++
        }
    }

    content := fmt.Sprintf(`
  Total Agents:        %d
  Up to Date:          %d
  Updates Available:   %s
  Systray Helper:      %s
  Last Checked:        2 minutes ago
`,
        len(d.agents),
        len(d.agents)-updatesAvailable,
        styles.Warning(fmt.Sprint(updatesAvailable)),
        styles.Success("● Running"),
    )

    return styles.PanelStyle.
        Width(45).
        Render(content)
}

func (d Dashboard) renderQuickActions() string {
    actions := []string{
        "[i] Install Agent",
        "[u] Update Selected",
        "[U] Update All",
        "[c] Browse Catalog",
        "[s] Settings",
        "[r] Refresh",
    }

    content := ""
    for _, action := range actions {
        content += "  " + action + "\n"
    }

    return styles.PanelStyle.
        Width(25).
        Render(content)
}

func (d Dashboard) renderAgentList() string {
    content := "\n"
    for i, agent := range d.agents {
        cursor := "  "
        if i == d.cursor {
            cursor = "▸ "
        }

        status := styles.Success("Up to date")
        version := fmt.Sprintf("v%s", agent.Version)

        if agent.UpdateAvailable {
            status = styles.Warning("Update Available")
            version = fmt.Sprintf("v%s → v%s", agent.Version, agent.LatestVersion)
        }

        line := fmt.Sprintf("%s%-20s %-25s %s\n",
            cursor,
            agent.Name,
            version,
            status,
        )

        if i == d.cursor {
            content += styles.SelectedItemStyle.Render(line)
        } else {
            content += line
        }
    }

    content += "\n  [↑↓] Navigate  [Enter] Details  [u] Update  [d] Delete  [/] Filter\n"

    return styles.PanelStyle.
        BorderTop(true).
        Width(74).
        Render(content)
}

func (d Dashboard) renderActivity() string {
    activities := []string{
        "2 minutes ago    Checked for updates (1 found)",
        "1 day ago        Updated claude-desktop: 1.2.2 → 1.2.3",
        "3 days ago       Installed windsurf v1.0.1",
    }

    content := "\n"
    for _, activity := range activities {
        content += "  " + styles.Muted(activity) + "\n"
    }

    return styles.PanelStyle.
        BorderTop(true).
        Width(74).
        Render(content)
}

func (d Dashboard) renderFooter() string {
    help := " Press [?] for keyboard shortcuts • [i] to install agents • [u] to update"
    return styles.Muted(help)
}

type agentSelectedMsg struct {
    agent types.Agent
}
```

### 7. Styles Definition

```go
// internal/tui/styles/styles.go
package styles

import (
    "os"

    "github.com/charmbracelet/lipgloss"
)

var (
    // Detect NO_COLOR
    noColor = os.Getenv("NO_COLOR") != ""

    // Colors
    ColorSuccess  = lipgloss.Color("#00D787")
    ColorWarning  = lipgloss.Color("#FFB86C")
    ColorError    = lipgloss.Color("#FF5555")
    ColorInfo     = lipgloss.Color("#8BE9FD")
    ColorMuted    = lipgloss.Color("#6272A4")
    ColorPrimary  = lipgloss.Color("#BD93F9")
    ColorBorder   = lipgloss.Color("#44475A")

    // Base style
    BaseStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("#F8F8F2"))

    // App container
    AppStyle = BaseStyle.Copy().
        Padding(1, 2)

    // Header
    HeaderStyle = BaseStyle.Copy().
        Bold(true).
        Foreground(ColorPrimary).
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(ColorBorder).
        Padding(0, 1)

    // Panel
    PanelStyle = BaseStyle.Copy().
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(ColorBorder).
        Padding(1, 2)

    // List items
    SelectedItemStyle = BaseStyle.Copy().
        Foreground(ColorPrimary).
        Bold(true)

    ItemStyle = BaseStyle.Copy()

    // Status styles
    SuccessStyle = BaseStyle.Copy().
        Foreground(ColorSuccess).
        Bold(true)

    ErrorStyle = BaseStyle.Copy().
        Foreground(ColorError).
        Bold(true)

    WarningStyle = BaseStyle.Copy().
        Foreground(ColorWarning).
        Bold(true)

    MutedStyle = BaseStyle.Copy().
        Foreground(ColorMuted)
)

// Helper functions
func Success(s string) string {
    if noColor {
        return s
    }
    return SuccessStyle.Render(s)
}

func Error(s string) string {
    if noColor {
        return s
    }
    return ErrorStyle.Render(s)
}

func Warning(s string) string {
    if noColor {
        return s
    }
    return WarningStyle.Render(s)
}

func Muted(s string) string {
    if noColor {
        return s
    }
    return MutedStyle.Render(s)
}
```

---

## Build and Run

### Build Script

```bash
#!/bin/bash
# scripts/build.sh

VERSION=$(git describe --tags --always --dirty)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

go build \
    -ldflags="-X main.Version=${VERSION} -X main.GitCommit=${COMMIT} -X main.BuildDate=${DATE}" \
    -o bin/agentmgr \
    ./cmd/agentmgr
```

### Run

```bash
# Build
./scripts/build.sh

# Run CLI
./bin/agentmgr agent list

# Run TUI
./bin/agentmgr tui

# Install globally
go install ./cmd/agentmgr
```

---

## Testing

### Unit Tests

```go
// internal/cli/agent/list_test.go
package agent_test

import (
    "bytes"
    "testing"

    "github.com/kevinelliott/agent-manager/internal/cli/agent"
    "github.com/stretchr/testify/assert"
)

func TestListCommand(t *testing.T) {
    cmd := agent.NewListCmd()
    b := bytes.NewBufferString("")
    cmd.SetOut(b)
    cmd.SetErr(b)

    err := cmd.Execute()
    assert.NoError(t, err)
}
```

### TUI Tests

```go
// internal/tui/app_test.go
package tui_test

import (
    "testing"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/kevinelliott/agent-manager/internal/tui"
    "github.com/stretchr/testify/assert"
)

func TestTUIInit(t *testing.T) {
    m := tui.NewModel()
    cmd := m.Init()
    assert.NotNil(t, cmd)
}

func TestKeyboardNavigation(t *testing.T) {
    m := tui.NewModel()

    // Test down key
    newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
    assert.NotNil(t, newModel)
}
```

---

## Next Steps

1. Implement core CLI commands
2. Create TUI views one by one
3. Add agent manager business logic
4. Implement installation/update flows
5. Add configuration management
6. Create systray helper integration
7. Write comprehensive tests
8. Build platform-specific binaries
9. Create release automation
10. Write user documentation
