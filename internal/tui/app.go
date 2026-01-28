// Package tui provides the terminal user interface.
package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevinelliott/agentmanager/internal/tui/styles"
	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/detector"
	"github.com/kevinelliott/agentmanager/pkg/installer"
	"github.com/kevinelliott/agentmanager/pkg/platform"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

// View represents the current view in the TUI.
type View int

const (
	ViewDashboard View = iota
	ViewAgentList
	ViewAgentDetail
	ViewCatalog
	ViewSettings
)

// Model is the main TUI model.
type Model struct {
	// Configuration
	config   *config.Config
	platform platform.Platform

	// Data
	agents      []*agent.Installation
	catalog     *catalog.Catalog
	selectedIdx int

	// UI state
	currentView View
	width       int
	height      int
	loading     bool
	err         error

	// Components
	list    list.Model
	spinner spinner.Model

	// Key bindings
	keys keyMap
}

// keyMap defines key bindings.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Back    key.Binding
	Quit    key.Binding
	Refresh key.Binding
	Install key.Binding
	Update  key.Binding
	Remove  key.Binding
	Help    key.Binding
	Tab     key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Install: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "install"),
		),
		Update: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "update"),
		),
		Remove: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "remove"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch view"),
		),
	}
}

// agentItem implements list.Item for the agent list.
type agentItem struct {
	installation *agent.Installation
}

func (i agentItem) Title() string       { return i.installation.AgentName }
func (i agentItem) Description() string { return i.installation.InstalledVersion.String() }
func (i agentItem) FilterValue() string { return i.installation.AgentName }

// New creates a new TUI model.
func New(cfg *config.Config, plat platform.Platform) Model {
	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.Spinner

	// Create list
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = styles.SelectedItem
	delegate.Styles.SelectedDesc = styles.SelectedItem

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Installed Agents"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = styles.Title

	return Model{
		config:      cfg,
		platform:    plat,
		currentView: ViewDashboard,
		keys:        DefaultKeyMap(),
		spinner:     s,
		list:        l,
		loading:     true,
	}
}

// Init initializes the TUI.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadData,
	)
}

// loadData loads initial data from storage and detector (uses cache if available).
func (m Model) loadData() tea.Msg {
	return m.loadDataWithRefresh(false)
}

// loadDataWithRefresh loads data, optionally bypassing the cache.
func (m Model) loadDataWithRefresh(forceRefresh bool) tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Initialize storage
	store, err := storage.NewSQLiteStore(m.platform.GetDataDir())
	if err != nil {
		return dataLoadedMsg{err: fmt.Errorf("failed to create storage: %w", err)}
	}
	defer store.Close()

	if err := store.Initialize(ctx); err != nil {
		return dataLoadedMsg{err: fmt.Errorf("failed to initialize storage: %w", err)}
	}

	// Load catalog
	catMgr := catalog.NewManager(m.config, store)
	cat, err := catMgr.Get(ctx)
	if err != nil {
		return dataLoadedMsg{err: fmt.Errorf("failed to load catalog: %w", err)}
	}

	// Get agents for current platform
	agentDefs, err := catMgr.GetAgentsForPlatform(ctx, string(m.platform.ID()))
	if err != nil {
		return dataLoadedMsg{err: fmt.Errorf("failed to get agents for platform: %w", err)}
	}

	// Build a map of agent ID -> AgentDef for quick lookup
	agentDefMap := make(map[string]catalog.AgentDef)
	for _, def := range agentDefs {
		agentDefMap[def.ID] = def
	}

	var installations []*agent.Installation
	var usedDetectionCache bool
	var needUpdateCheck bool

	// Check if we can use cached detection results
	if m.config.Detection.CacheEnabled && !forceRefresh {
		cachedInstallations, cachedAt, err := store.GetDetectionCache(ctx)
		if err == nil && cachedInstallations != nil {
			// Check if detection cache is still valid
			if time.Since(cachedAt) < m.config.Detection.CacheDuration {
				installations = cachedInstallations
				usedDetectionCache = true

				// Check if we need to refresh update check
				lastUpdateCheck, err := store.GetLastUpdateCheckTime(ctx)
				if err != nil || lastUpdateCheck.IsZero() || time.Since(lastUpdateCheck) >= m.config.Detection.UpdateCheckCacheDuration {
					needUpdateCheck = true
				}
			}
		}
	}

	// If no cache or cache expired, detect agents
	if !usedDetectionCache {
		// Clear cache if forcing refresh
		if forceRefresh {
			//nolint:errcheck // best-effort cache clear; will be refreshed on next detection
			_ = store.ClearDetectionCache(ctx)
		}

		// Detect installed agents
		det := detector.New(m.platform)
		installations, err = det.DetectAll(ctx, agentDefs)
		if err != nil {
			return dataLoadedMsg{err: fmt.Errorf("detection failed: %w", err)}
		}

		// Always check for updates on fresh detection
		needUpdateCheck = true
	}

	// Check for updates if needed
	if needUpdateCheck {
		instMgr := installer.NewManager(m.platform)

		// Check for latest versions
		for _, inst := range installations {
			if agentDef, ok := agentDefMap[inst.AgentID]; ok {
				methodStr := string(inst.Method)
				if method, ok := agentDef.InstallMethods[methodStr]; ok {
					latestVer, err := instMgr.GetLatestVersion(ctx, method)
					if err == nil {
						inst.LatestVersion = &latestVer
					}
				}
			}
		}

		// Save last update check time
		_ = store.SetLastUpdateCheckTime(ctx, time.Now()) //nolint:errcheck // best-effort timestamp; non-critical if this fails

		// Save to cache if enabled (with updated version info)
		if m.config.Detection.CacheEnabled {
			_ = store.SaveDetectionCache(ctx, installations) //nolint:errcheck // best-effort cache; agents will be redetected on next run
		}
	}

	return dataLoadedMsg{
		agents:  installations,
		catalog: cat,
	}
}

// dataLoadedMsg is sent when data has been loaded.
type dataLoadedMsg struct {
	agents  []*agent.Installation
	catalog *catalog.Catalog
	err     error
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Tab):
			m.currentView = (m.currentView + 1) % 5

		case key.Matches(msg, m.keys.Back):
			if m.currentView != ViewDashboard {
				m.currentView = ViewDashboard
			}

		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			return m, func() tea.Msg {
				return m.loadDataWithRefresh(true) // Force refresh, bypass cache
			}

		case key.Matches(msg, m.keys.Enter):
			if m.currentView == ViewAgentList && len(m.agents) > 0 {
				m.currentView = ViewAgentDetail
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-8)

	case dataLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.agents = msg.agents
			m.catalog = msg.catalog
			m.updateList()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update list
	if m.currentView == ViewAgentList {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// updateList updates the list items from agents.
func (m *Model) updateList() {
	// Sort agents alphabetically by name (case-insensitive)
	sort.Slice(m.agents, func(i, j int) bool {
		return strings.ToLower(m.agents[i].AgentName) < strings.ToLower(m.agents[j].AgentName)
	})

	items := make([]list.Item, len(m.agents))
	for i, a := range m.agents {
		items[i] = agentItem{installation: a}
	}
	m.list.SetItems(items)
}

// View renders the TUI.
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var content string

	switch m.currentView {
	case ViewDashboard:
		content = m.dashboardView()
	case ViewAgentList:
		content = m.agentListView()
	case ViewAgentDetail:
		content = m.agentDetailView()
	case ViewCatalog:
		content = m.catalogView()
	case ViewSettings:
		content = m.settingsView()
	}

	// Add header
	header := m.headerView()

	// Add footer
	footer := m.footerView()

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// headerView renders the header.
func (m Model) headerView() string {
	title := styles.TitleBar.Render(" AgentManager ")

	tabs := []string{"Dashboard", "Agents", "Detail", "Catalog", "Settings"}
	var tabViews []string
	for i, tab := range tabs {
		if View(i) == m.currentView {
			tabViews = append(tabViews, styles.TabActive.Render(tab))
		} else {
			tabViews = append(tabViews, styles.Tab.Render(tab))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		tabBar,
		"",
	)
}

// footerView renders the footer.
func (m Model) footerView() string {
	helpKeys := []string{
		styles.HelpKey.Render("tab") + styles.Help.Render(" switch view"),
		styles.HelpKey.Render("r") + styles.Help.Render(" refresh"),
		styles.HelpKey.Render("q") + styles.Help.Render(" quit"),
	}

	if m.currentView == ViewAgentList {
		helpKeys = append(helpKeys,
			styles.HelpKey.Render("i")+styles.Help.Render(" install"),
			styles.HelpKey.Render("u")+styles.Help.Render(" update"),
		)
	}

	help := strings.Join(helpKeys, "  ")
	return styles.StatusBar.Width(m.width).Render(help)
}

// dashboardView renders the dashboard.
func (m Model) dashboardView() string {
	if m.loading {
		return fmt.Sprintf("\n  %s Loading...\n", m.spinner.View())
	}

	if m.err != nil {
		return styles.ErrorMessage.Render(fmt.Sprintf("\n  Error: %v\n", m.err))
	}

	// Count statistics
	installed := len(m.agents)
	updatesAvailable := 0
	for _, a := range m.agents {
		if a.HasUpdate() {
			updatesAvailable++
		}
	}

	// Build dashboard
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styles.Title.Render("  Dashboard"))
	b.WriteString("\n\n")

	// Stats boxes
	statsBox := styles.Box.Render(fmt.Sprintf(
		"%s\n\n  Installed: %s\n  Updates:   %s",
		styles.Subtitle.Render("Agent Statistics"),
		styles.Version.Render(fmt.Sprintf("%d", installed)),
		func() string {
			if updatesAvailable > 0 {
				return styles.StatusUpdateAvailable.Render(fmt.Sprintf("%d available", updatesAvailable))
			}
			return styles.StatusInstalled.Render("All up to date")
		}(),
	))

	// Platform info
	platformBox := styles.Box.Render(fmt.Sprintf(
		"%s\n\n  Platform: %s\n  Arch:     %s",
		styles.Subtitle.Render("System Info"),
		styles.Version.Render(m.platform.Name()),
		styles.Version.Render(m.platform.Architecture()),
	))

	// Recent activity placeholder
	activityBox := styles.Box.Render(fmt.Sprintf(
		"%s\n\n  No recent activity",
		styles.Subtitle.Render("Recent Activity"),
	))

	// Layout boxes horizontally
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, "  ", statsBox, "  ", platformBox)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, "  ", activityBox)

	b.WriteString(row1)
	b.WriteString("\n\n")
	b.WriteString(row2)

	return b.String()
}

// agentListView renders the agent list.
func (m Model) agentListView() string {
	if m.loading {
		return fmt.Sprintf("\n  %s Loading agents...\n", m.spinner.View())
	}

	if len(m.agents) == 0 {
		return styles.InfoMessage.Render("\n  No agents installed. Press 'i' to install an agent.\n")
	}

	return "\n" + m.list.View()
}

// agentDetailView renders agent details.
func (m Model) agentDetailView() string {
	if len(m.agents) == 0 || m.selectedIdx >= len(m.agents) {
		return styles.InfoMessage.Render("\n  Select an agent to view details.\n")
	}

	inst := m.agents[m.selectedIdx]

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styles.Title.Render("  " + inst.AgentName))
	b.WriteString("\n\n")

	details := styles.Box.Render(fmt.Sprintf(
		"%s\n\n"+
			"  ID:       %s\n"+
			"  Method:   %s\n"+
			"  Version:  %s\n"+
			"  Path:     %s\n"+
			"  Status:   %s",
		styles.Subtitle.Render("Installation Details"),
		styles.Version.Render(inst.AgentID),
		styles.Badge.Render(string(inst.Method)),
		func() string {
			if inst.HasUpdate() {
				return styles.VersionOld.Render(inst.InstalledVersion.String()) +
					" → " + styles.VersionNew.Render(inst.LatestVersion.String())
			}
			return styles.Version.Render(inst.InstalledVersion.String())
		}(),
		styles.InfoMessage.Render(inst.ExecutablePath),
		styles.FormatStatus(string(inst.GetStatus())),
	))

	b.WriteString("  ")
	b.WriteString(details)
	b.WriteString("\n\n")

	// Action buttons
	if inst.HasUpdate() {
		b.WriteString("  ")
		b.WriteString(styles.ButtonActive.Render("Update"))
		b.WriteString(styles.Button.Render("Remove"))
	} else {
		b.WriteString("  ")
		b.WriteString(styles.Button.Render("Remove"))
	}

	return b.String()
}

// catalogView renders the catalog browser.
func (m Model) catalogView() string {
	if m.loading {
		return fmt.Sprintf("\n  %s Loading catalog...\n", m.spinner.View())
	}

	if m.catalog == nil {
		return styles.InfoMessage.Render("\n  Catalog not loaded. Press 'r' to refresh.\n")
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styles.Title.Render("  Available Agents"))
	b.WriteString("\n\n")

	// Get agents for current platform
	agents := m.catalog.GetAgentsByPlatform(string(m.platform.ID()))

	if len(agents) == 0 {
		b.WriteString(styles.InfoMessage.Render("  No agents available for this platform.\n"))
		return b.String()
	}

	// Sort agents alphabetically by name (case-insensitive)
	sort.Slice(agents, func(i, j int) bool {
		return strings.ToLower(agents[i].Name) < strings.ToLower(agents[j].Name)
	})

	// Build a simple table of agents
	for _, agentDef := range agents {
		// Check if installed
		installed := false
		for _, inst := range m.agents {
			if inst.AgentID == agentDef.ID {
				installed = true
				break
			}
		}

		status := styles.StatusNotInstalled.Render("Not installed")
		if installed {
			status = styles.StatusInstalled.Render("Installed")
		}

		methods := agentDef.GetSupportedMethods(string(m.platform.ID()))
		methodStr := ""
		if len(methods) > 0 {
			methodStr = methods[0].Method
			if len(methods) > 1 {
				methodStr += fmt.Sprintf(" +%d", len(methods)-1)
			}
		}

		line := fmt.Sprintf("  %s  %s  %s  %s\n",
			styles.Badge.Render(fmt.Sprintf("%-12s", agentDef.ID)),
			styles.Version.Render(fmt.Sprintf("%-20s", agentDef.Name)),
			styles.Help.Render(fmt.Sprintf("%-10s", methodStr)),
			status,
		)
		b.WriteString(line)
	}

	b.WriteString(fmt.Sprintf("\n  %d agents available\n", len(agents)))

	return b.String()
}

// settingsView renders settings.
func (m Model) settingsView() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styles.Title.Render("  Settings"))
	b.WriteString("\n\n")

	settings := styles.Box.Render(fmt.Sprintf(
		"%s\n\n"+
			"  Auto-check updates:    %s\n"+
			"  Catalog auto-refresh:  %s\n"+
			"  Notifications:         %s\n"+
			"  Auto-update:           %s",
		styles.Subtitle.Render("Update Settings"),
		func() string {
			if m.config.Updates.AutoCheck {
				return styles.StatusInstalled.Render("Enabled")
			}
			return styles.StatusNotInstalled.Render("Disabled")
		}(),
		func() string {
			if m.config.Catalog.RefreshOnStart {
				return styles.StatusInstalled.Render("Enabled")
			}
			return styles.StatusNotInstalled.Render("Disabled")
		}(),
		func() string {
			if m.config.Updates.Notify {
				return styles.StatusInstalled.Render("Enabled")
			}
			return styles.StatusNotInstalled.Render("Disabled")
		}(),
		func() string {
			if m.config.Updates.AutoUpdate {
				return styles.StatusInstalled.Render("Enabled")
			}
			return styles.StatusNotInstalled.Render("Disabled")
		}(),
	))

	b.WriteString("  ")
	b.WriteString(settings)

	return b.String()
}

// Run starts the TUI.
func Run(cfg *config.Config, plat platform.Platform) error {
	p := tea.NewProgram(
		New(cfg, plat),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}
