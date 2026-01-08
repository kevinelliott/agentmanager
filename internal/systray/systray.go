// Package systray provides the system tray helper application.
package systray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/systray"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/api/rest"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/detector"
	"github.com/kevinelliott/agentmgr/pkg/installer"
	"github.com/kevinelliott/agentmgr/pkg/ipc"
	"github.com/kevinelliott/agentmgr/pkg/platform"
	"github.com/kevinelliott/agentmgr/pkg/storage"
)

// App represents the systray helper application.
type App struct {
	config    *config.Config
	platform  platform.Platform
	store     storage.Store
	detector  *detector.Detector
	catalog   *catalog.Manager
	installer *installer.Manager

	// IPC server
	ipcServer ipc.Server

	// REST API server (optional)
	restServer *rest.Server

	// State
	agents      []agent.Installation
	agentsMu    sync.RWMutex
	startTime   time.Time
	lastRefresh time.Time
	lastCheck   time.Time

	// Menu items
	mStatus    *systray.MenuItem
	mAgents    *systray.MenuItem
	mRefresh   *systray.MenuItem
	mUpdateAll *systray.MenuItem
	mOpenTUI   *systray.MenuItem
	mAutoStart *systray.MenuItem
	mQuit      *systray.MenuItem
	agentItems []*agentMenuItem

	// Channels
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// agentMenuItem represents a menu item for an agent.
type agentMenuItem struct {
	installation agent.Installation
	menuItem     *systray.MenuItem
	updateItem   *systray.MenuItem
	infoItem     *systray.MenuItem
}

// New creates a new systray application.
func New(cfg *config.Config, plat platform.Platform, store storage.Store, det *detector.Detector, cat *catalog.Manager, inst *installer.Manager) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		config:    cfg,
		platform:  plat,
		store:     store,
		detector:  det,
		catalog:   cat,
		installer: inst,
		startTime: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
}

// Run starts the systray application.
func (a *App) Run() error {
	// Start IPC server
	if err := a.startIPCServer(); err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}

	// Start REST API server if enabled
	if a.config.API.EnableREST {
		if err := a.startRESTServer(); err != nil {
			return fmt.Errorf("failed to start REST server: %w", err)
		}
	}

	// Run systray (blocks until quit)
	systray.Run(a.onReady, a.onExit)
	return nil
}

// startRESTServer starts the REST API server.
func (a *App) startRESTServer() error {
	a.restServer = rest.NewServer(a.config, a.platform, a.store, a.detector, a.catalog, a.installer)
	return a.restServer.Start(a.ctx, rest.ServerConfig{
		Address: fmt.Sprintf(":%d", a.config.API.RESTPort),
	})
}

// startIPCServer starts the IPC server for CLI communication.
func (a *App) startIPCServer() error {
	a.ipcServer = ipc.NewServer("")
	a.ipcServer.SetHandler(ipc.HandlerFunc(a.handleIPCMessage))
	return a.ipcServer.Start(a.ctx)
}

// handleIPCMessage handles incoming IPC messages.
func (a *App) handleIPCMessage(ctx context.Context, msg *ipc.Message) (*ipc.Message, error) {
	switch msg.Type {
	case ipc.MessageTypeListAgents:
		return a.handleListAgents(ctx, msg)
	case ipc.MessageTypeGetAgent:
		return a.handleGetAgent(ctx, msg)
	case ipc.MessageTypeRefreshCatalog:
		return a.handleRefreshCatalog(ctx, msg)
	case ipc.MessageTypeCheckUpdates:
		return a.handleCheckUpdates(ctx, msg)
	case ipc.MessageTypeGetStatus:
		return a.handleGetStatus(ctx, msg)
	case ipc.MessageTypeShutdown:
		go func() {
			time.Sleep(100 * time.Millisecond)
			systray.Quit()
		}()
		return ipc.NewMessage(ipc.MessageTypeSuccess, nil)
	default:
		return ipc.NewMessage(ipc.MessageTypeError, ipc.ErrorResponse{
			Code:    "unknown_message",
			Message: fmt.Sprintf("unknown message type: %s", msg.Type),
		})
	}
}

// handleListAgents handles list_agents requests.
func (a *App) handleListAgents(ctx context.Context, msg *ipc.Message) (*ipc.Message, error) {
	a.agentsMu.RLock()
	agents := make([]agent.Installation, len(a.agents))
	copy(agents, a.agents)
	a.agentsMu.RUnlock()

	return ipc.NewMessage(ipc.MessageTypeSuccess, ipc.ListAgentsResponse{
		Agents: agents,
		Total:  len(agents),
	})
}

// handleGetAgent handles get_agent requests.
func (a *App) handleGetAgent(ctx context.Context, msg *ipc.Message) (*ipc.Message, error) {
	var req ipc.GetAgentRequest
	if err := msg.DecodePayload(&req); err != nil {
		return ipc.NewMessage(ipc.MessageTypeError, ipc.ErrorResponse{
			Code:    "invalid_payload",
			Message: err.Error(),
		})
	}

	a.agentsMu.RLock()
	var found *agent.Installation
	for _, ag := range a.agents {
		if ag.Key() == req.Key {
			agCopy := ag
			found = &agCopy
			break
		}
	}
	a.agentsMu.RUnlock()

	return ipc.NewMessage(ipc.MessageTypeSuccess, ipc.GetAgentResponse{
		Agent: found,
	})
}

// handleRefreshCatalog handles refresh_catalog requests.
func (a *App) handleRefreshCatalog(ctx context.Context, msg *ipc.Message) (*ipc.Message, error) {
	if err := a.refreshAgents(ctx); err != nil {
		return ipc.NewMessage(ipc.MessageTypeError, ipc.ErrorResponse{
			Code:    "refresh_failed",
			Message: err.Error(),
		})
	}
	return ipc.NewMessage(ipc.MessageTypeSuccess, nil)
}

// handleCheckUpdates handles check_updates requests.
func (a *App) handleCheckUpdates(ctx context.Context, msg *ipc.Message) (*ipc.Message, error) {
	if err := a.checkUpdates(ctx); err != nil {
		return ipc.NewMessage(ipc.MessageTypeError, ipc.ErrorResponse{
			Code:    "check_failed",
			Message: err.Error(),
		})
	}
	return ipc.NewMessage(ipc.MessageTypeSuccess, nil)
}

// handleGetStatus handles get_status requests.
func (a *App) handleGetStatus(ctx context.Context, msg *ipc.Message) (*ipc.Message, error) {
	a.agentsMu.RLock()
	agentCount := len(a.agents)
	updatesAvailable := 0
	for _, ag := range a.agents {
		if ag.HasUpdate() {
			updatesAvailable++
		}
	}
	a.agentsMu.RUnlock()

	return ipc.NewMessage(ipc.MessageTypeSuccess, ipc.StatusResponse{
		Running:            true,
		Uptime:             int64(time.Since(a.startTime).Seconds()),
		AgentCount:         agentCount,
		UpdatesAvailable:   updatesAvailable,
		LastCatalogRefresh: a.lastRefresh,
		LastUpdateCheck:    a.lastCheck,
	})
}

// onReady is called when systray is ready.
func (a *App) onReady() {
	// Set icon and tooltip
	systray.SetIcon(getIcon())
	systray.SetTitle("AgentManager")
	systray.SetTooltip("AgentManager - AI Agent Manager")

	// Create menu items
	a.mStatus = systray.AddMenuItem("Loading...", "Status")
	a.mStatus.Disable()

	systray.AddSeparator()

	a.mAgents = systray.AddMenuItem("Agents", "Installed Agents")
	a.mRefresh = systray.AddMenuItem("Refresh", "Re-detect agents")
	a.mUpdateAll = systray.AddMenuItem("Update All", "Update all agents with available updates")
	a.mUpdateAll.Disable()

	systray.AddSeparator()

	a.mOpenTUI = systray.AddMenuItem("Open TUI", "Launch terminal interface")
	a.mAutoStart = systray.AddMenuItem("Start at Login", "Toggle auto-start on login")

	systray.AddSeparator()

	a.mQuit = systray.AddMenuItem("Quit", "Quit AgentManager Helper")

	// Check auto-start status
	if enabled, err := a.platform.IsAutoStartEnabled(a.ctx); err == nil && enabled {
		a.mAutoStart.Check()
	}

	// Initial refresh
	go a.refreshAgents(a.ctx)

	// Start background tasks
	go a.backgroundLoop()

	// Handle menu clicks
	go a.handleMenuClicks()
}

// onExit is called when systray is exiting.
func (a *App) onExit() {
	a.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop REST server
	if a.restServer != nil {
		a.restServer.Stop(ctx)
	}

	// Stop IPC server
	if a.ipcServer != nil {
		a.ipcServer.Stop(ctx)
	}

	close(a.done)
}

// handleMenuClicks handles menu item clicks.
func (a *App) handleMenuClicks() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.mRefresh.ClickedCh:
			go a.refreshAgents(a.ctx)
		case <-a.mUpdateAll.ClickedCh:
			go a.updateAllAgents(a.ctx)
		case <-a.mOpenTUI.ClickedCh:
			go a.openTUI()
		case <-a.mAutoStart.ClickedCh:
			go a.toggleAutoStart()
		case <-a.mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

// backgroundLoop runs periodic background tasks.
func (a *App) backgroundLoop() {
	// Catalog refresh ticker
	refreshTicker := time.NewTicker(a.config.Catalog.RefreshInterval)
	defer refreshTicker.Stop()

	// Update check ticker
	checkTicker := time.NewTicker(a.config.Updates.CheckInterval)
	defer checkTicker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-refreshTicker.C:
			a.refreshAgents(a.ctx)
		case <-checkTicker.C:
			if a.config.Updates.AutoCheck {
				a.checkUpdates(a.ctx)
			}
		}
	}
}

// refreshAgents refreshes the list of detected agents.
func (a *App) refreshAgents(ctx context.Context) error {
	a.mStatus.SetTitle("Refreshing...")

	// Get agent definitions from catalog
	agentDefs, err := a.catalog.GetAgentsForPlatform(ctx, string(a.platform.ID()))
	if err != nil {
		// If catalog fails, use empty list for detection
		agentDefs = nil
	}

	// Detect agents
	detected, err := a.detector.DetectAll(ctx, agentDefs)
	if err != nil {
		a.mStatus.SetTitle("Error detecting agents")
		return err
	}

	// Convert []*agent.Installation to []agent.Installation
	agents := make([]agent.Installation, len(detected))
	for i, inst := range detected {
		agents[i] = *inst
	}

	a.agentsMu.Lock()
	a.agents = agents
	a.lastRefresh = time.Now()
	a.agentsMu.Unlock()

	a.updateMenu()
	return nil
}

// checkUpdates checks for available updates.
func (a *App) checkUpdates(ctx context.Context) error {
	a.agentsMu.Lock()
	a.lastCheck = time.Now()
	a.agentsMu.Unlock()

	// In a real implementation, this would check the catalog for newer versions
	// and update the LatestVersion field on each installation

	a.updateMenu()

	// Show notification if updates available
	a.agentsMu.RLock()
	updatesAvailable := 0
	for _, ag := range a.agents {
		if ag.HasUpdate() {
			updatesAvailable++
		}
	}
	a.agentsMu.RUnlock()

	if updatesAvailable > 0 && a.config.Updates.Notify {
		a.platform.ShowNotification(
			"Updates Available",
			fmt.Sprintf("%d agent update(s) available", updatesAvailable),
		)
	}

	return nil
}

// updateMenu updates the systray menu to reflect current state.
func (a *App) updateMenu() {
	a.agentsMu.RLock()
	agentCount := len(a.agents)
	updatesAvailable := 0
	for _, ag := range a.agents {
		if ag.HasUpdate() {
			updatesAvailable++
		}
	}
	a.agentsMu.RUnlock()

	// Update status
	if updatesAvailable > 0 {
		a.mStatus.SetTitle(fmt.Sprintf("%d agents (%d updates)", agentCount, updatesAvailable))
		a.mUpdateAll.Enable()
		systray.SetTooltip(fmt.Sprintf("AgentManager - %d updates available", updatesAvailable))
	} else {
		a.mStatus.SetTitle(fmt.Sprintf("%d agents (up to date)", agentCount))
		a.mUpdateAll.Disable()
		systray.SetTooltip("AgentManager - All agents up to date")
	}

	// Update agent submenu
	a.updateAgentSubmenu()
}

// updateAgentSubmenu updates the agents submenu.
func (a *App) updateAgentSubmenu() {
	// Clear existing items
	for _, item := range a.agentItems {
		item.menuItem.Hide()
		if item.updateItem != nil {
			item.updateItem.Hide()
		}
		if item.infoItem != nil {
			item.infoItem.Hide()
		}
	}
	a.agentItems = nil

	a.agentsMu.RLock()
	defer a.agentsMu.RUnlock()

	// Add agent items
	for _, ag := range a.agents {
		var title string
		if ag.HasUpdate() {
			title = fmt.Sprintf("↑ %s (%s → %s)", ag.AgentName, ag.InstalledVersion.String(), ag.LatestVersion.String())
		} else {
			title = fmt.Sprintf("● %s (%s)", ag.AgentName, ag.InstalledVersion.String())
		}

		item := a.mAgents.AddSubMenuItem(title, ag.Key())

		agentItem := &agentMenuItem{
			installation: ag,
			menuItem:     item,
		}

		if ag.HasUpdate() {
			agentItem.updateItem = item.AddSubMenuItem("Update", "Update this agent")
			go a.handleAgentUpdate(agentItem)
		}

		agentItem.infoItem = item.AddSubMenuItem("Info", "Show agent info")
		go a.handleAgentInfo(agentItem)

		a.agentItems = append(a.agentItems, agentItem)
	}
}

// handleAgentUpdate handles update clicks for an agent.
func (a *App) handleAgentUpdate(item *agentMenuItem) {
	if item.updateItem == nil {
		return
	}
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-item.updateItem.ClickedCh:
			go a.updateSingleAgent(item.installation)
		}
	}
}

// updateSingleAgent updates a single agent installation.
func (a *App) updateSingleAgent(inst agent.Installation) {
	a.platform.ShowNotification(
		"Update Started",
		fmt.Sprintf("Updating %s...", inst.AgentName),
	)

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
	defer cancel()

	// Get agent definition from catalog
	agentDef, err := a.catalog.GetAgent(ctx, inst.AgentID)
	if err != nil {
		a.platform.ShowNotification(
			"Update Failed",
			fmt.Sprintf("Failed to find %s in catalog: %v", inst.AgentName, err),
		)
		return
	}

	// Find the install method
	methodDef, ok := agentDef.GetInstallMethod(string(inst.Method))
	if !ok {
		a.platform.ShowNotification(
			"Update Failed",
			fmt.Sprintf("Install method %s not available for %s", inst.Method, inst.AgentName),
		)
		return
	}

	// Perform the update
	result, err := a.installer.Update(ctx, &inst, *agentDef, methodDef)
	if err != nil {
		a.platform.ShowNotification(
			"Update Failed",
			fmt.Sprintf("Failed to update %s: %v", inst.AgentName, err),
		)
		return
	}

	a.platform.ShowNotification(
		"Update Complete",
		fmt.Sprintf("%s updated to %s", inst.AgentName, result.Version.String()),
	)

	// Refresh agent list
	a.refreshAgents(ctx)
}

// handleAgentInfo handles info clicks for an agent.
func (a *App) handleAgentInfo(item *agentMenuItem) {
	if item.infoItem == nil {
		return
	}
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-item.infoItem.ClickedCh:
			info := fmt.Sprintf(
				"Name: %s\nVersion: %s\nMethod: %s\nPath: %s",
				item.installation.AgentName,
				item.installation.InstalledVersion.String(),
				item.installation.Method,
				item.installation.ExecutablePath,
			)
			fromVer := item.installation.InstalledVersion.String()
			toVer := ""
			if item.installation.HasUpdate() {
				toVer = item.installation.LatestVersion.String()
				info += fmt.Sprintf("\n\nUpdate Available: %s", toVer)
			}
			a.platform.ShowChangelogDialog(item.installation.AgentName, fromVer, toVer, info)
		}
	}
}

// updateAllAgents updates all agents with available updates.
func (a *App) updateAllAgents(ctx context.Context) {
	a.agentsMu.RLock()
	var toUpdate []agent.Installation
	for _, ag := range a.agents {
		if ag.HasUpdate() {
			toUpdate = append(toUpdate, ag)
		}
	}
	a.agentsMu.RUnlock()

	if len(toUpdate) == 0 {
		return
	}

	a.platform.ShowNotification(
		"Updating Agents",
		fmt.Sprintf("Updating %d agents...", len(toUpdate)),
	)

	// Update each agent sequentially
	var succeeded, failed int
	for _, inst := range toUpdate {
		updateCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)

		// Get agent definition from catalog
		agentDef, err := a.catalog.GetAgent(updateCtx, inst.AgentID)
		if err != nil {
			failed++
			cancel()
			continue
		}

		// Find the install method
		methodDef, ok := agentDef.GetInstallMethod(string(inst.Method))
		if !ok {
			failed++
			cancel()
			continue
		}

		// Perform the update
		_, err = a.installer.Update(updateCtx, &inst, *agentDef, methodDef)
		if err != nil {
			failed++
		} else {
			succeeded++
		}
		cancel()
	}

	// Show completion notification
	if failed == 0 {
		a.platform.ShowNotification(
			"Updates Complete",
			fmt.Sprintf("Successfully updated %d agents", succeeded),
		)
	} else {
		a.platform.ShowNotification(
			"Updates Complete",
			fmt.Sprintf("Updated %d agents, %d failed", succeeded, failed),
		)
	}

	// Refresh agent list
	a.refreshAgents(ctx)
}

// openTUI launches the TUI application in a new terminal window.
func (a *App) openTUI() {
	// Find the agentmgr binary
	agentmgrPath, err := findAgentMgrBinary()
	if err != nil {
		a.platform.ShowNotification("Error", "Could not find agentmgr binary")
		return
	}

	// Launch TUI based on platform
	var cmd *exec.Cmd
	switch a.platform.ID() {
	case platform.Darwin:
		// Use osascript to open Terminal with the TUI command
		script := fmt.Sprintf(`tell application "Terminal"
			activate
			do script "%s tui"
		end tell`, agentmgrPath)
		cmd = exec.Command("osascript", "-e", script)
	case platform.Linux:
		// Try common terminal emulators in order of preference
		terminals := []struct {
			name string
			args []string
		}{
			{"gnome-terminal", []string{"--", agentmgrPath, "tui"}},
			{"konsole", []string{"-e", agentmgrPath, "tui"}},
			{"xfce4-terminal", []string{"-e", agentmgrPath + " tui"}},
			{"xterm", []string{"-e", agentmgrPath, "tui"}},
		}
		for _, term := range terminals {
			if _, err := exec.LookPath(term.name); err == nil {
				cmd = exec.Command(term.name, term.args...) //nolint:gosec // Safe: iterating hardcoded terminal list
				break
			}
		}
		if cmd == nil {
			a.platform.ShowNotification("Error", "No supported terminal emulator found")
			return
		}
	case platform.Windows:
		// Use cmd.exe to open a new window with the TUI
		cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", agentmgrPath, "tui")
	default:
		a.platform.ShowNotification("Error", "Unsupported platform")
		return
	}

	// Start the command (don't wait for it)
	if err := cmd.Start(); err != nil {
		a.platform.ShowNotification("Error", fmt.Sprintf("Failed to launch TUI: %v", err))
		return
	}

	// Release the process so it runs independently
	if cmd.Process != nil {
		cmd.Process.Release()
	}
}

// findAgentMgrBinary locates the agentmgr binary.
func findAgentMgrBinary() (string, error) {
	// First check PATH
	if path, err := exec.LookPath("agentmgr"); err == nil {
		return path, nil
	}

	// Check same directory as current executable (for helper binary)
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		agentmgrPath := filepath.Join(dir, "agentmgr")
		if platform.IsWindows() {
			agentmgrPath += ".exe"
		}
		if _, err := os.Stat(agentmgrPath); err == nil {
			return agentmgrPath, nil
		}
	}

	// Check common paths
	paths := []string{
		"/usr/local/bin/agentmgr",
		"/usr/bin/agentmgr",
	}

	// Add home directory paths
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".local", "bin", "agentmgr"),
			filepath.Join(home, "go", "bin", "agentmgr"),
		)
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("agentmgr not found in PATH or common locations")
}

// toggleAutoStart toggles the auto-start setting.
func (a *App) toggleAutoStart() {
	enabled, err := a.platform.IsAutoStartEnabled(a.ctx)
	if err != nil {
		return
	}

	if enabled {
		if err := a.platform.DisableAutoStart(a.ctx); err == nil {
			a.mAutoStart.Uncheck()
		}
	} else {
		if err := a.platform.EnableAutoStart(a.ctx); err == nil {
			a.mAutoStart.Check()
		}
	}
}

// getIcon returns the systray icon.
func getIcon() []byte {
	// This would return an embedded icon
	// For now, return a placeholder (1x1 transparent PNG)
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
}
