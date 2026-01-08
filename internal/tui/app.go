// Package tui provides the terminal user interface.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kevinelliott/agentmgr/internal/tui/styles"
	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/platform"
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

// loadData loads initial data.
func (m Model) loadData() tea.Msg {
	// This would normally load from storage/detector
	// For now, return empty data
	return dataLoadedMsg{
		agents:  []*agent.Installation{},
		catalog: nil,
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
			return m, m.loadData

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
	return styles.InfoMessage.Render("\n  Catalog browser coming soon...\n")
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
