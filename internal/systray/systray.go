// Package systray provides the system tray helper application.
package systray

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/api/rest"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/detector"
	"github.com/kevinelliott/agentmanager/pkg/installer"
	"github.com/kevinelliott/agentmanager/pkg/ipc"
	"github.com/kevinelliott/agentmanager/pkg/platform"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

// String constants for repeated values.
const (
	statusNotFound = "(not found)"
)

// agentMenuItem tracks a menu item for an agent.
type agentMenuItem struct {
	item    *systray.MenuItem
	agentID string
	method  agent.InstallMethod
}

// App represents the systray helper application.
type App struct {
	config       *config.Config
	configLoader *config.Loader
	platform     platform.Platform
	store        storage.Store
	detector     *detector.Detector
	catalog      *catalog.Manager
	installer    *installer.Manager
	version      string

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
	mStatus        *systray.MenuItem
	mAgentsMenu    *systray.MenuItem
	mManageAgents  *systray.MenuItem
	mAgentsLoading *systray.MenuItem
	agentItems     []*agentMenuItem
	agentItemsMu   sync.Mutex
	mRefresh       *systray.MenuItem
	mUpdateAll     *systray.MenuItem
	mOpenTUI       *systray.MenuItem
	mSettings      *systray.MenuItem
	mAutoStart     *systray.MenuItem
	mQuit          *systray.MenuItem

	// Track spawned dialog processes to kill on exit
	dialogProcs   []*exec.Cmd
	dialogProcsMu sync.Mutex

	// Channels
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// Silence unused warning - reserved for future settings UI
var _ = (*App).changeCLIPath

// New creates a new systray application.
func New(cfg *config.Config, cfgLoader *config.Loader, plat platform.Platform, store storage.Store, det *detector.Detector, cat *catalog.Manager, inst *installer.Manager, version string) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		config:       cfg,
		configLoader: cfgLoader,
		platform:     plat,
		store:        store,
		detector:     det,
		catalog:      cat,
		installer:    inst,
		version:      version,
		startTime:    time.Now(),
		ctx:          ctx,
		cancel:       cancel,
		done:         make(chan struct{}),
	}
}

// Run starts the systray application.
// This must be called from the main goroutine on macOS.
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
	// On macOS, this must run on the main thread
	systray.Run(a.onReady, a.onExit)
	return nil
}

// Quit triggers a graceful shutdown of the systray application.
func (a *App) Quit() {
	systray.Quit()
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
		PID:                os.Getpid(),
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
	icon := getIcon()
	systray.SetTemplateIcon(icon, icon)
	systray.SetTooltip("AgentManager")
	// Note: Not setting a title keeps it icon-only in menu bar

	// Status line - use fixed text to avoid menu resizing
	a.mStatus = systray.AddMenuItem("Loading...", "")
	a.mStatus.Disable()

	// Agents submenu
	a.mAgentsMenu = systray.AddMenuItem("Agents", "View detected agents")
	a.mManageAgents = a.mAgentsMenu.AddSubMenuItem("Manage Agents", "Manage all agents")
	separatorItem := a.mAgentsMenu.AddSubMenuItem("─────────────────────", "")
	separatorItem.Disable() // Disable to make it non-clickable
	// Loading item shown while agents are being detected
	a.mAgentsLoading = a.mAgentsMenu.AddSubMenuItem("Loading...", "")
	a.mAgentsLoading.Disable()

	a.mUpdateAll = systray.AddMenuItem("Updates", "")
	a.mUpdateAll.Disable()

	systray.AddSeparator()

	a.mOpenTUI = systray.AddMenuItem("Open TUI", "Launch terminal interface")
	a.mRefresh = systray.AddMenuItem("Refresh", "Re-detect agents")
	a.mAutoStart = systray.AddMenuItem("Start at Login", "Toggle auto-start on login")

	systray.AddSeparator()

	a.mSettings = systray.AddMenuItem("Settings", "Configure AgentManager")
	a.mQuit = systray.AddMenuItem("Quit", "")

	// Check auto-start status
	if enabled, err := a.platform.IsAutoStartEnabled(a.ctx); err == nil && enabled {
		a.mAutoStart.Check()
	}

	// Initial refresh
	go a.refreshAgents(a.ctx)

	// Start background tasks
	go a.backgroundLoop()

	// Handle menu clicks (all menu items in one goroutine)
	go a.handleMenuClicks()
}

// onExit is called when systray is exiting.
func (a *App) onExit() {
	a.cancel()

	// Close all native windows
	closeAllNativeWindows()

	// Kill any open dialog processes (fallback osascript)
	a.killAllDialogs()

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

// trackDialog adds a dialog process to be killed on exit.
func (a *App) trackDialog(cmd *exec.Cmd) {
	a.dialogProcsMu.Lock()
	a.dialogProcs = append(a.dialogProcs, cmd)
	a.dialogProcsMu.Unlock()
}

// untrackDialog removes a dialog process from tracking.
func (a *App) untrackDialog(cmd *exec.Cmd) {
	a.dialogProcsMu.Lock()
	defer a.dialogProcsMu.Unlock()
	for i, c := range a.dialogProcs {
		if c == cmd {
			a.dialogProcs = append(a.dialogProcs[:i], a.dialogProcs[i+1:]...)
			return
		}
	}
}

// killAllDialogs kills all tracked dialog processes.
func (a *App) killAllDialogs() {
	a.dialogProcsMu.Lock()
	procs := make([]*exec.Cmd, len(a.dialogProcs))
	copy(procs, a.dialogProcs)
	a.dialogProcs = nil
	a.dialogProcsMu.Unlock()

	for _, cmd := range procs {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

// handleMenuClicks handles menu item clicks.
func (a *App) handleMenuClicks() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.mManageAgents.ClickedCh:
			go a.showManageAgentsWindow()
		case <-a.mRefresh.ClickedCh:
			go a.forceRefreshAgents(a.ctx) // Manual refresh bypasses cache
		case <-a.mUpdateAll.ClickedCh:
			go a.updateAllAgents(a.ctx)
		case <-a.mOpenTUI.ClickedCh:
			go a.openTUI()
		case <-a.mSettings.ClickedCh:
			go a.showSettings()
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

// refreshAgents refreshes the list of detected agents (uses cache if available).
func (a *App) refreshAgents(ctx context.Context) error {
	return a.refreshAgentsWithCache(ctx, false)
}

// forceRefreshAgents forces a refresh, bypassing the cache.
func (a *App) forceRefreshAgents(ctx context.Context) error {
	return a.refreshAgentsWithCache(ctx, true)
}

// refreshAgentsWithCache refreshes agents, optionally bypassing the cache.
func (a *App) refreshAgentsWithCache(ctx context.Context, forceRefresh bool) error {
	// Get agent definitions from catalog
	agentDefs, err := a.catalog.GetAgentsForPlatform(ctx, string(a.platform.ID()))
	if err != nil {
		// If catalog fails, use empty list for detection
		agentDefs = nil
	}

	// Build a map of agent ID -> AgentDef for quick lookup
	agentDefMap := make(map[string]catalog.AgentDef)
	for _, def := range agentDefs {
		agentDefMap[def.ID] = def
	}

	var detected []*agent.Installation
	var usedDetectionCache bool
	var needUpdateCheck bool

	// Check if we can use cached detection results
	if a.config.Detection.CacheEnabled && !forceRefresh {
		cachedInstallations, cachedAt, err := a.store.GetDetectionCache(ctx)
		if err == nil && cachedInstallations != nil {
			// Check if detection cache is still valid
			if time.Since(cachedAt) < a.config.Detection.CacheDuration {
				detected = cachedInstallations
				usedDetectionCache = true

				// Check if we need to refresh update check
				lastUpdateCheck, err := a.store.GetLastUpdateCheckTime(ctx)
				if err != nil || lastUpdateCheck.IsZero() || time.Since(lastUpdateCheck) >= a.config.Detection.UpdateCheckCacheDuration {
					needUpdateCheck = true
				}
			}
		}
	}

	// If no cache or cache expired, detect agents
	if !usedDetectionCache {
		// Clear cache if forcing refresh
		if forceRefresh {
			_ = a.store.ClearDetectionCache(ctx)
		}

		// Detect agents
		detected, err = a.detector.DetectAll(ctx, agentDefs)
		if err != nil {
			return err
		}

		// Always check for updates on fresh detection
		needUpdateCheck = true
	}

	// Check for updates if needed
	if needUpdateCheck {
		// Check for latest versions
		for _, inst := range detected {
			if agentDef, ok := agentDefMap[inst.AgentID]; ok {
				methodStr := string(inst.Method)
				if method, ok := agentDef.InstallMethods[methodStr]; ok {
					latestVer, err := a.installer.GetLatestVersion(ctx, method)
					if err == nil {
						inst.LatestVersion = &latestVer
					}
				}
			}
		}

		// Save last update check time
		_ = a.store.SetLastUpdateCheckTime(ctx, time.Now())

		// Save to cache if enabled (with updated version info)
		if a.config.Detection.CacheEnabled {
			_ = a.store.SaveDetectionCache(ctx, detected)
		}
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

	// Only update the counts, not the full menu (to avoid menu jumping)
	a.updateMenuCounts()

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
	agents := make([]agent.Installation, len(a.agents))
	copy(agents, a.agents)
	agentCount := len(agents)
	updatesAvailable := 0
	for _, ag := range agents {
		if ag.HasUpdate() {
			updatesAvailable++
		}
	}
	a.agentsMu.RUnlock()

	// Update status line (keep text short to minimize menu resizing)
	a.mStatus.SetTitle(fmt.Sprintf("%d Agents", agentCount))

	// Update agents submenu
	a.updateAgentsSubmenu(agents)

	// Update Agents menu state
	if agentCount > 0 {
		a.mAgentsMenu.Enable()
	} else {
		a.mAgentsMenu.Disable()
	}

	// Update updates line (keep text short to minimize menu resizing)
	if updatesAvailable > 0 {
		a.mUpdateAll.SetTitle(fmt.Sprintf("⬆ %d Updates", updatesAvailable))
		a.mUpdateAll.Enable()
		systray.SetTooltip(fmt.Sprintf("AgentManager (%d updates)", updatesAvailable))
	} else {
		a.mUpdateAll.SetTitle("Up to date")
		a.mUpdateAll.Disable()
		systray.SetTooltip("AgentManager")
	}
}

// updateMenuCounts updates only the status and update counts without modifying the agents submenu.
// This is used for background updates to avoid menu jumping.
func (a *App) updateMenuCounts() {
	a.agentsMu.RLock()
	agentCount := len(a.agents)
	updatesAvailable := 0
	for _, ag := range a.agents {
		if ag.HasUpdate() {
			updatesAvailable++
		}
	}
	a.agentsMu.RUnlock()

	// Update status line (keep text short to minimize menu resizing)
	a.mStatus.SetTitle(fmt.Sprintf("%d Agents", agentCount))

	// Update updates line (keep text short to minimize menu resizing)
	if updatesAvailable > 0 {
		a.mUpdateAll.SetTitle(fmt.Sprintf("⬆ %d Updates", updatesAvailable))
		a.mUpdateAll.Enable()
		systray.SetTooltip(fmt.Sprintf("AgentManager (%d updates)", updatesAvailable))
	} else {
		a.mUpdateAll.SetTitle("Up to date")
		a.mUpdateAll.Disable()
		systray.SetTooltip("AgentManager")
	}
}

// updateAgentsSubmenu updates the agents submenu with current agents.
func (a *App) updateAgentsSubmenu(agents []agent.Installation) {
	a.agentItemsMu.Lock()
	defer a.agentItemsMu.Unlock()

	// Hide loading indicator once agents are loaded
	if a.mAgentsLoading != nil {
		a.mAgentsLoading.Hide()
	}

	// Sort agents alphabetically by name (case-insensitive)
	sort.Slice(agents, func(i, j int) bool {
		return strings.ToLower(agents[i].AgentName) < strings.ToLower(agents[j].AgentName)
	})

	// Hide existing items that are no longer needed
	for i, item := range a.agentItems {
		if i >= len(agents) {
			item.item.Hide()
		}
	}

	// Create or update items for each agent
	for i, ag := range agents {
		// Build the menu item title
		title := a.formatAgentMenuTitle(ag)

		if i < len(a.agentItems) {
			// Update existing item
			a.agentItems[i].item.SetTitle(title)
			a.agentItems[i].item.Show()
			a.agentItems[i].agentID = ag.AgentID
			a.agentItems[i].method = ag.Method
		} else {
			// Create new submenu item (no tooltip)
			item := a.mAgentsMenu.AddSubMenuItem(title, "")
			menuItem := &agentMenuItem{
				item:    item,
				agentID: ag.AgentID,
				method:  ag.Method,
			}
			a.agentItems = append(a.agentItems, menuItem)

			// Start click handler for this item
			go a.handleAgentItemClick(menuItem)
		}
	}
}

// formatAgentMenuTitle formats the menu title for an agent.
// Format: "● Name (method) — version" with em-dash separator
// Note: Tab-based right-alignment requires NSAttributedString with paragraph styles,
// which isn't supported by getlantern/systray. Using em-dash separator for clean layout.
func (a *App) formatAgentMenuTitle(ag agent.Installation) string {
	version := ag.InstalledVersion.String()
	if version == "" {
		version = "unknown"
	}

	// Method in lowercase parentheses
	methodStr := ""
	if ag.Method != "" {
		methodStr = fmt.Sprintf(" (%s)", strings.ToLower(string(ag.Method)))
	}

	// Use em-dash separator for clean visual separation
	if ag.HasUpdate() {
		return fmt.Sprintf("⬆ %s%s — %s → %s", ag.AgentName, methodStr, version, ag.LatestVersion.String())
	}
	return fmt.Sprintf("● %s%s — %s", ag.AgentName, methodStr, version)
}

// handleAgentItemClick handles clicks on an agent menu item.
func (a *App) handleAgentItemClick(item *agentMenuItem) {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-item.item.ClickedCh:
			// Find the current agent state
			a.agentsMu.RLock()
			var foundAgent *agent.Installation
			for _, ag := range a.agents {
				if ag.AgentID == item.agentID && ag.Method == item.method {
					agCopy := ag
					foundAgent = &agCopy
					break
				}
			}
			a.agentsMu.RUnlock()

			if foundAgent != nil {
				go a.showAgentDetails(*foundAgent)
			}
		}
	}
}

// showAgentDetails shows a dialog with agent details and an optional update button.
func (a *App) showAgentDetails(inst agent.Installation) {
	// Use native windows if available
	if hasNativeWindowSupport() {
		a.showNativeAgentDetailsWindow(inst)
		return
	}

	// Build the details message for fallback dialogs
	version := inst.InstalledVersion.String()
	if version == "" {
		version = "unknown"
	}

	details := fmt.Sprintf("Name: %s\nVersion: %s\nInstall Method: %s",
		inst.AgentName, version, inst.Method)

	if inst.ExecutablePath != "" {
		details += fmt.Sprintf("\nPath: %s", inst.ExecutablePath)
	}

	if !inst.DetectedAt.IsZero() {
		details += fmt.Sprintf("\nDetected: %s", inst.DetectedAt.Format("2006-01-02 15:04"))
	}

	hasUpdate := inst.HasUpdate()
	if hasUpdate {
		details += fmt.Sprintf("\n\nUpdate Available: %s → %s",
			version, inst.LatestVersion.String())
	}

	// Show dialog based on platform
	switch a.platform.ID() {
	case platform.Darwin:
		a.showMacOSAgentDialog(inst, details, hasUpdate)
	case platform.Linux:
		a.showLinuxAgentDialog(inst, details, hasUpdate)
	case platform.Windows:
		a.showWindowsAgentDialog(inst, details, hasUpdate)
	default:
		// Fallback: show notification
		a.platform.ShowNotification(inst.AgentName, details)
	}
}

// showMacOSAgentDialog shows an agent details dialog on macOS using osascript.
func (a *App) showMacOSAgentDialog(inst agent.Installation, details string, hasUpdate bool) {
	var script string
	if hasUpdate {
		// Dialog with Update and Close buttons
		script = fmt.Sprintf(`
tell application "System Events"
	set dialogResult to display dialog %q with title %q buttons {"Close", "Update"} default button "Close" with icon note
	if button returned of dialogResult is "Update" then
		return "update"
	end if
end tell
return "close"
`, details, inst.AgentName)
	} else {
		// Dialog with only Close button
		script = fmt.Sprintf(`
tell application "System Events"
	display dialog %q with title %q buttons {"Close"} default button "Close" with icon note
end tell
return "close"
`, details, inst.AgentName)
	}

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		// Dialog was canceled or errored, ignore
		return
	}

	result := string(output)
	if hasUpdate && (result == "update\n" || result == "update") {
		go a.updateSingleAgent(inst)
	}
}

// showLinuxAgentDialog shows an agent details dialog on Linux using zenity or kdialog.
func (a *App) showLinuxAgentDialog(inst agent.Installation, details string, hasUpdate bool) {
	// Try zenity first
	if _, err := exec.LookPath("zenity"); err == nil {
		var cmd *exec.Cmd
		if hasUpdate {
			cmd = exec.Command("zenity", "--question", //nolint:gosec // G204: agent names from trusted catalog
				"--title="+inst.AgentName,
				"--text="+details+"\n\nDo you want to update?",
				"--ok-label=Update",
				"--cancel-label=Close")
		} else {
			cmd = exec.Command("zenity", "--info", //nolint:gosec // G204: agent names from trusted catalog
				"--title="+inst.AgentName,
				"--text="+details)
		}
		err := cmd.Run()
		if hasUpdate && err == nil {
			go a.updateSingleAgent(inst)
		}
		return
	}

	// Try kdialog
	if _, err := exec.LookPath("kdialog"); err == nil {
		if hasUpdate {
			//nolint:gosec // G204: agent names come from trusted catalog, not user input
			cmd := exec.Command("kdialog", "--yesno", details+"\n\nDo you want to update?", "--title", inst.AgentName)
			err := cmd.Run()
			if err == nil {
				go a.updateSingleAgent(inst)
			}
		} else {
			//nolint:gosec // G204: agent names come from trusted catalog, not user input
			cmd := exec.Command("kdialog", "--msgbox", details, "--title", inst.AgentName)
			_ = cmd.Run()
		}
		return
	}

	// Fallback to notification
	a.platform.ShowNotification(inst.AgentName, details)
}

// showWindowsAgentDialog shows an agent details dialog on Windows using PowerShell.
func (a *App) showWindowsAgentDialog(inst agent.Installation, details string, hasUpdate bool) {
	var script string
	if hasUpdate {
		script = fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$result = [System.Windows.Forms.MessageBox]::Show('%s', '%s', 'YesNo', 'Information')
if ($result -eq 'Yes') { Write-Output 'update' } else { Write-Output 'close' }
`, details+"\n\nDo you want to update?", inst.AgentName)
	} else {
		script = fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.MessageBox]::Show('%s', '%s', 'OK', 'Information')
Write-Output 'close'
`, details, inst.AgentName)
	}

	cmd := exec.Command("powershell", "-Command", script)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	result := string(output)
	if hasUpdate && (result == "update\r\n" || result == "update\n" || result == "update") {
		go a.updateSingleAgent(inst)
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

// showManageAgentsWindow displays the agent management window.
func (a *App) showManageAgentsWindow() {
	if hasNativeWindowSupport() {
		// Get all agents from catalog for this platform
		agentDefs, err := a.catalog.GetAgentsForPlatform(a.ctx, string(a.platform.ID()))
		if err != nil {
			a.platform.ShowNotification("Error", "Could not load agent catalog")
			return
		}

		// Get currently installed agents
		a.agentsMu.RLock()
		installedAgents := make([]agent.Installation, len(a.agents))
		copy(installedAgents, a.agents)
		a.agentsMu.RUnlock()

		a.showNativeManageAgentsWindow(agentDefs, installedAgents)
		return
	}

	// Fallback to notification
	a.platform.ShowNotification("Manage Agents", "Use the TUI for full agent management")
}

// showSettings displays the settings dialog.
func (a *App) showSettings() {
	// Use native windows if available
	if hasNativeWindowSupport() {
		a.showNativeSettingsWindow()
		return
	}

	// Fall back to osascript/platform dialogs
	switch a.platform.ID() {
	case platform.Darwin:
		a.showMacOSSettings()
	case platform.Linux:
		a.showLinuxSettings()
	case platform.Windows:
		a.showWindowsSettings()
	default:
		a.platform.ShowNotification("Settings", "Settings panel not available on this platform")
	}
}

// showMacOSSettings shows the settings dialog on macOS using osascript.
func (a *App) showMacOSSettings() {
	// Find current CLI path
	currentPath := a.config.Helper.CLIPath
	if currentPath == "" {
		if path, err := findAgentMgrBinary(); err == nil {
			currentPath = path
		} else {
			currentPath = statusNotFound
		}
	}

	notifyStatus := "ON"
	if !a.config.Updates.Notify {
		notifyStatus = "OFF"
	}

	// Use a simple button dialog instead of choose from list
	// This doesn't require System Events accessibility permissions
	script := fmt.Sprintf(`
set theMessage to "AgentManager Settings

CLI Path: %s

Notifications: %s"

display dialog theMessage with title "AgentManager Settings" buttons {"Install CLI", "Toggle Notifications", "Done"} default button "Done"
return button returned of result
`, escapeAppleScript(currentPath), notifyStatus)

	for {
		// Check if context is canceled (helper is shutting down)
		select {
		case <-a.ctx.Done():
			return
		default:
		}

		cmd := exec.Command("osascript", "-e", script)
		a.trackDialog(cmd)
		output, err := cmd.CombinedOutput()
		a.untrackDialog(cmd)

		if err != nil {
			// Dialog was canceled, killed, or errored
			return
		}

		result := strings.TrimSpace(string(output))
		switch result {
		case "Done":
			return
		case "Install CLI":
			a.installOrUpdateCLI()
			// Refresh path after potential installation
			if path, err := findAgentMgrBinary(); err == nil {
				currentPath = path
			}
		case "Toggle Notifications":
			a.config.Updates.Notify = !a.config.Updates.Notify
			if a.configLoader != nil {
				_ = a.configLoader.SetAndSave("updates.notify", a.config.Updates.Notify)
			}
			notifyStatus = "ON"
			if !a.config.Updates.Notify {
				notifyStatus = "OFF"
			}
		}

		// Regenerate script with updated values
		script = fmt.Sprintf(`
set theMessage to "AgentManager Settings

CLI Path: %s

Notifications: %s"

display dialog theMessage with title "AgentManager Settings" buttons {"Install CLI", "Toggle Notifications", "Done"} default button "Done"
return button returned of result
`, escapeAppleScript(currentPath), notifyStatus)
	}
}

// changeCLIPath prompts the user to change the CLI path.
func (a *App) changeCLIPath() {
	switch a.platform.ID() {
	case platform.Darwin:
		// Use osascript to show file chooser
		script := `
tell application "System Events"
	set selectedFile to choose file with prompt "Select the agentmgr CLI binary:" of type {"public.unix-executable", "public.item"}
	return POSIX path of selectedFile
end tell
`
		cmd := exec.Command("osascript", "-e", script)
		output, err := cmd.Output()
		if err != nil {
			return
		}

		newPath := strings.TrimSpace(string(output))
		if newPath != "" {
			a.config.Helper.CLIPath = newPath
			if a.configLoader != nil {
				_ = a.configLoader.SetAndSave("helper.cli_path", newPath)
			}
			a.platform.ShowNotification("Settings", fmt.Sprintf("CLI path set to: %s", newPath))
		}
	default:
		a.platform.ShowNotification("Settings", "Use the config file to change CLI path")
	}
}

// installOrUpdateCLI installs or updates the agentmgr CLI. Returns true on success.
func (a *App) installOrUpdateCLI() bool {
	// Find the bundled agentmgr binary (should be in same directory as helper)
	bundledPath, err := findBundledAgentMgr()
	if err != nil {
		a.platform.ShowNotification("Install Failed", "Could not find bundled agentmgr binary")
		return false
	}

	// Target path for installation
	targetPath := "/usr/local/bin/agentmgr"

	switch a.platform.ID() {
	case platform.Darwin, platform.Linux:
		// Copy with sudo (will prompt for password via macOS native dialog)
		return a.installCLIToPath(bundledPath, targetPath)

	case platform.Windows:
		targetPath = filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "agentmgr", "agentmgr.exe")
		a.installCLIToPathWindows(bundledPath, targetPath)
		return true // Windows version doesn't return status yet

	default:
		a.platform.ShowNotification("Install CLI", "Unsupported platform")
		return false
	}
}

// findBundledAgentMgr finds the agentmgr binary bundled with the helper.
func findBundledAgentMgr() (string, error) {
	// Get the path of the current executable (the helper)
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not get executable path: %w", err)
	}

	// Resolve any symlinks
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("could not resolve symlinks: %w", err)
	}

	dir := filepath.Dir(exe)

	// Check for agentmgr in the same directory
	agentmgrPath := filepath.Join(dir, "agentmgr")
	if platform.IsWindows() {
		agentmgrPath += ".exe"
	}

	if _, err := os.Stat(agentmgrPath); err == nil {
		return agentmgrPath, nil
	}

	// On macOS, check inside the app bundle's MacOS directory
	// Structure: AgentManager.app/Contents/MacOS/agentmgr-helper
	// The agentmgr should also be in MacOS/
	if strings.Contains(dir, ".app/Contents/MacOS") {
		agentmgrPath = filepath.Join(dir, "agentmgr")
		if _, err := os.Stat(agentmgrPath); err == nil {
			return agentmgrPath, nil
		}
	}

	return "", fmt.Errorf("agentmgr binary not found in %s", dir)
}

// installCLIToPath installs the CLI binary to the target path using sudo.
func (a *App) installCLIToPath(sourcePath, targetPath string) bool {
	// Ensure target directory exists
	targetDir := filepath.Dir(targetPath)

	// Use osascript to run sudo with password prompt
	script := fmt.Sprintf(`
do shell script "mkdir -p '%s' && cp '%s' '%s' && chmod +x '%s'" with administrator privileges
`, targetDir, sourcePath, targetPath, targetPath)

	cmd := exec.Command("osascript", "-e", script)
	err := cmd.Run()
	if err != nil {
		return false
	}

	// Update config with the new path
	a.config.Helper.CLIPath = targetPath
	if a.configLoader != nil {
		_ = a.configLoader.SetAndSave("helper.cli_path", targetPath)
	}
	return true
}

// installCLIToPathWindows installs the CLI binary on Windows.
func (a *App) installCLIToPathWindows(sourcePath, targetPath string) {
	// Create target directory
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		a.platform.ShowNotification("Install Failed", fmt.Sprintf("Could not create directory: %v", err))
		return
	}

	// Copy the file
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		a.platform.ShowNotification("Install Failed", fmt.Sprintf("Could not read source: %v", err))
		return
	}

	if err := os.WriteFile(targetPath, sourceData, 0755); err != nil {
		a.platform.ShowNotification("Install Failed", fmt.Sprintf("Could not write target: %v", err))
		return
	}

	a.platform.ShowNotification("Install Complete", fmt.Sprintf("agentmgr installed to %s\n\nAdd this to your PATH to use from command line.", targetPath))

	// Update config
	a.config.Helper.CLIPath = targetPath
	if a.configLoader != nil {
		_ = a.configLoader.SetAndSave("helper.cli_path", targetPath)
	}
}

// uninstallCLI removes the CLI binary from the system path.
func (a *App) uninstallCLI() bool { //nolint:unused // Reserved for future use
	targetPath := "/usr/local/bin/agentmgr"

	switch a.platform.ID() {
	case platform.Darwin, platform.Linux:
		// Use osascript to run sudo with password prompt
		script := fmt.Sprintf(`
do shell script "rm -f '%s'" with administrator privileges
`, targetPath)

		cmd := exec.Command("osascript", "-e", script)
		err := cmd.Run()
		if err != nil {
			return false
		}

		// Clear config path
		a.config.Helper.CLIPath = ""
		if a.configLoader != nil {
			_ = a.configLoader.SetAndSave("helper.cli_path", "")
		}
		return true

	case platform.Windows:
		// Try to remove directly on Windows
		targetPath = filepath.Join(os.Getenv("LOCALAPPDATA"), "agentmgr", "agentmgr.exe")
		if err := os.Remove(targetPath); err != nil {
			return false
		}

		// Clear config path
		a.config.Helper.CLIPath = ""
		if a.configLoader != nil {
			_ = a.configLoader.SetAndSave("helper.cli_path", "")
		}
		return true
	}

	return false
}

// showLinuxSettings shows the settings dialog on Linux.
func (a *App) showLinuxSettings() {
	// Try zenity first
	if _, err := exec.LookPath("zenity"); err == nil {
		currentPath := a.config.Helper.CLIPath
		if currentPath == "" {
			if path, err := findAgentMgrBinary(); err == nil {
				currentPath = path
			} else {
				currentPath = statusNotFound
			}
		}

		details := fmt.Sprintf("CLI Path: %s\nNotifications: %s\n\nEdit config file to change settings:\n%s",
			currentPath, boolToOnOff(a.config.Updates.Notify), config.GetConfigPath())

		//nolint:gosec // G204: uses fixed zenity command with app-controlled content
		cmd := exec.Command("zenity", "--info", "--title=AgentManager Settings", "--text="+details)
		_ = cmd.Run()
		return
	}

	// Fallback to notification
	a.platform.ShowNotification("Settings", fmt.Sprintf("Edit config file: %s", config.GetConfigPath()))
}

// showWindowsSettings shows the settings dialog on Windows.
func (a *App) showWindowsSettings() {
	currentPath := a.config.Helper.CLIPath
	if currentPath == "" {
		if path, err := findAgentMgrBinary(); err == nil {
			currentPath = path
		} else {
			currentPath = statusNotFound
		}
	}

	details := fmt.Sprintf("CLI Path: %s\nNotifications: %s\n\nEdit config file to change settings:\n%s",
		currentPath, boolToOnOff(a.config.Updates.Notify), config.GetConfigPath())

	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.MessageBox]::Show('%s', 'AgentManager Settings', 'OK', 'Information')
`, strings.ReplaceAll(details, "'", "''"))

	cmd := exec.Command("powershell", "-Command", script)
	_ = cmd.Run()
}

// escapeAppleScript escapes a string for use in AppleScript.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// boolToOnOff converts a bool to "On" or "Off".
func boolToOnOff(b bool) string {
	if b {
		return "On"
	}
	return "Off"
}

// getIcon returns the systray icon.
// 16x16 ring icon (template image for macOS menu bar).
func getIcon() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x10,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0xF3, 0xFF, 0x61, 0x00, 0x00, 0x00,
		0x2B, 0x49, 0x44, 0x41, 0x54, 0x78, 0xDA, 0x63, 0x60, 0x18, 0xAC, 0xE0,
		0x3F, 0x0E, 0x4C, 0xB6, 0x46, 0xA2, 0x0D, 0xA2, 0xC8, 0x80, 0xFF, 0x24,
		0x62, 0x82, 0x06, 0x90, 0x2A, 0x3F, 0x1C, 0x0D, 0xA0, 0x69, 0x4C, 0xD0,
		0x2E, 0x21, 0x51, 0x9C, 0x94, 0xE9, 0x0F, 0x00, 0xF4, 0x09, 0x6B, 0x95,
		0x94, 0x7F, 0x2F, 0x72, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}
}
