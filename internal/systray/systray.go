// Package systray provides the system tray helper application.
package systray

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"

	"github.com/kevinelliott/agentmanager/internal/orchestrator"
	"github.com/kevinelliott/agentmanager/pkg/agent"
	grpcapi "github.com/kevinelliott/agentmanager/pkg/api/grpc"
	"github.com/kevinelliott/agentmanager/pkg/api/rest"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/detector"
	"github.com/kevinelliott/agentmanager/pkg/installer"
	"github.com/kevinelliott/agentmanager/pkg/ipc"
	"github.com/kevinelliott/agentmanager/pkg/platform"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

// Actions the window and tray can request for an agent.
const (
	actionInstall   = "install"
	actionUninstall = "uninstall"
	actionUpdate    = "update"
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

	// Web UI server backing the desktop window
	ui *uiServer

	// REST API server (optional)
	restServer *rest.Server

	// gRPC API server (optional)
	grpcServer *grpcapi.Server

	// State
	agents      []agent.Installation
	agentsMu    sync.RWMutex
	startTime   time.Time
	lastRefresh time.Time
	lastCheck   time.Time

	// Menu items
	mOpen          *systray.MenuItem
	mStatus        *systray.MenuItem
	mAgentsMenu    *systray.MenuItem
	mBrowse        *systray.MenuItem
	mAgentsLoading *systray.MenuItem
	agentItems     []*agentMenuItem
	agentItemsMu   sync.Mutex
	mRefresh       *systray.MenuItem
	mUpdateAll     *systray.MenuItem
	mOpenTUI       *systray.MenuItem
	mSettings      *systray.MenuItem
	mAutoStart     *systray.MenuItem
	mQuit          *systray.MenuItem

	// Channels
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	// shutdown coordination. shutdownCh is closed exactly once by
	// requestShutdown(); a background goroutine launched in onReady watches
	// it (and a.ctx.Done()) and calls systray.Quit() when either fires.
	// This replaces an earlier time.Sleep(100ms) in the IPC shutdown handler
	// with a deterministic context-cancel handshake. bgWG tracks background
	// goroutines so onExit can wait for them to drain.
	shutdownCh   chan struct{}
	shutdownOnce sync.Once
	bgWG         sync.WaitGroup
}

// New creates a new systray application.
func New(cfg *config.Config, cfgLoader *config.Loader, plat platform.Platform, store storage.Store, det *detector.Detector, cat *catalog.Manager, inst *installer.Manager, version string) *App {
	// The cancel func is stashed on the App struct (a.cancel) and called
	// from onExit during graceful shutdown. gosec G118 cannot see that.
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel stored on struct + invoked in onExit
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
		shutdownCh:   make(chan struct{}),
	}
}

// Run starts the systray application.
// This must be called from the main goroutine on macOS.
func (a *App) Run() error {
	// Start IPC server
	if err := a.startIPCServer(); err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}

	// Start the local web UI that the desktop window shows
	if err := a.startUIServer(); err != nil {
		return fmt.Errorf("failed to start UI server: %w", err)
	}

	// Start REST API server if enabled
	if a.config.API.EnableREST {
		if err := a.startRESTServer(); err != nil {
			return fmt.Errorf("failed to start REST server: %w", err)
		}
	}

	// Start gRPC API server if enabled
	if a.config.API.EnableGRPC {
		if err := a.startGRPCServer(); err != nil {
			return fmt.Errorf("failed to start gRPC server: %w", err)
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
	a.restServer = rest.NewServer(
		a.config, a.platform, a.store, a.detector, a.catalog, a.installer,
		rest.WithVersion(a.version),
		rest.WithStartTime(a.startTime),
	)
	return a.restServer.Start(a.ctx, rest.ServerConfig{
		Address: fmt.Sprintf(":%d", a.config.API.RESTPort),
	})
}

// startGRPCServer starts the gRPC API server.
func (a *App) startGRPCServer() error {
	a.grpcServer = grpcapi.NewServer(
		a.config, a.platform, a.store, a.detector, a.catalog, a.installer,
		grpcapi.WithVersion(a.version),
		grpcapi.WithStartTime(a.startTime),
	)
	return a.grpcServer.Start(a.ctx, grpcapi.ServerConfig{
		Address: fmt.Sprintf(":%d", a.config.API.GRPCPort),
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
	case ipc.MessageTypeShowWindow:
		var req ipc.ShowWindowRequest
		if len(msg.Payload) > 0 {
			if err := msg.DecodePayload(&req); err != nil {
				return ipc.NewMessage(ipc.MessageTypeError, ipc.ErrorResponse{
					Code:    "invalid_payload",
					Message: err.Error(),
				})
			}
		}
		if a.ui == nil {
			return ipc.NewMessage(ipc.MessageTypeError, ipc.ErrorResponse{
				Code:    "window_unavailable",
				Message: "the helper has no window to show",
			})
		}
		a.showMainWindow(req.Route)
		return ipc.NewMessage(ipc.MessageTypeSuccess, nil)
	case ipc.MessageTypeShutdown:
		// Signal the background shutdown watcher. The response below is sent
		// by the IPC framework before the watcher observes shutdownCh and
		// triggers systray.Quit(), which avoids racing the caller's read.
		a.requestShutdown()
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

	a.mOpen = systray.AddMenuItem("Open AgentManager", "Open the AgentManager window")

	systray.AddSeparator()

	// Status line - use fixed text to avoid menu resizing
	a.mStatus = systray.AddMenuItem("Looking for agents…", "")
	a.mStatus.Disable()

	// Installed agents; choosing one opens it in the window
	a.mAgentsMenu = systray.AddMenuItem("Installed Agents", "Agents found on this computer")
	a.mAgentsLoading = a.mAgentsMenu.AddSubMenuItem("Looking for agents…", "")
	a.mAgentsLoading.Disable()

	a.mUpdateAll = systray.AddMenuItem("Up to date", "Update every agent that has a newer version")
	a.mUpdateAll.Disable()

	a.mBrowse = systray.AddMenuItem("Browse Catalog…", "Find and install more agents")

	systray.AddSeparator()

	a.mRefresh = systray.AddMenuItem("Check Now", "Look for agents and updates again")
	a.mOpenTUI = systray.AddMenuItem("Open in Terminal", "Launch the terminal interface")
	a.mAutoStart = systray.AddMenuItem("Start at Login", "Start AgentManager when you log in")

	systray.AddSeparator()

	a.mSettings = systray.AddMenuItem("Settings…", "Configure AgentManager")
	a.mQuit = systray.AddMenuItem("Quit AgentManager", "")

	// Check auto-start status
	if enabled, err := a.platform.IsAutoStartEnabled(a.ctx); err == nil && enabled {
		a.mAutoStart.Check()
	}

	// Initial refresh
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		_ = a.refreshAgents(a.ctx)
	}()

	// Start background tasks (tracked for clean shutdown).
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		a.backgroundLoop()
	}()

	// Handle menu clicks (all menu items in one goroutine).
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		a.handleMenuClicks()
	}()

	// Shutdown watcher: triggers systray.Quit() on request or ctx cancellation.
	a.bgWG.Add(1)
	go func() {
		defer a.bgWG.Done()
		a.shutdownWatcher()
	}()
}

// requestShutdown signals the shutdown watcher to tear the helper down.
// Safe to call from any goroutine and idempotent.
func (a *App) requestShutdown() {
	a.shutdownOnce.Do(func() {
		close(a.shutdownCh)
	})
}

// shutdownWatcher waits for either an explicit shutdown request or the app
// context to be canceled, then triggers systray.Quit(). Running Quit via a
// dedicated goroutine instead of a time.Sleep gives the IPC framework a
// deterministic handshake: the shutdown IPC handler returns its response
// first, the IPC framework writes it on the wire, and this watcher then
// trips Quit — ordered solely by goroutine scheduling, not a magic delay.
func (a *App) shutdownWatcher() {
	select {
	case <-a.shutdownCh:
	case <-a.ctx.Done():
	}
	systray.Quit()
}

// reportError records asynchronous failures that cannot be returned to a caller.
func (a *App) reportError(operation string, err error) {
	if err != nil {
		slog.Error(operation, "error", err)
	}
}

func (a *App) notify(title, message string) {
	a.reportError("show desktop notification", a.platform.ShowNotification(title, message))
}

// onExit is called when systray is exiting.
func (a *App) onExit() {
	// Make sure the shutdown signal is set so any goroutines selecting on
	// shutdownCh can exit even if Quit was invoked outside requestShutdown
	// (e.g. via the Quit menu or a.Quit()).
	a.requestShutdown()

	// Cancel the app context so long-running goroutines (refresh, menu clicks,
	// IPC/REST handlers) can unwind.
	a.cancel()

	// Close the desktop window
	closeMainWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop the web UI server
	if a.ui != nil {
		a.reportError("stop UI server", a.ui.stop(ctx))
	}

	// Stop REST server
	if a.restServer != nil {
		a.reportError("stop REST server", a.restServer.Stop(ctx))
	}

	// Stop gRPC server: prefer GracefulStop with a short timeout; fall back
	// to a hard Stop if graceful shutdown does not complete in time so a
	// wedged in-flight RPC cannot block helper exit.
	if a.grpcServer != nil {
		graceCtx, graceCancel := context.WithTimeout(context.Background(), 2*time.Second)
		gracefulDone := make(chan struct{})
		go func() {
			_ = a.grpcServer.Stop(graceCtx)
			close(gracefulDone)
		}()
		select {
		case <-gracefulDone:
		case <-graceCtx.Done():
			a.grpcServer.ForceStop()
		}
		graceCancel()
	}

	// Stop IPC server (drains in-flight handlers before returning).
	if a.ipcServer != nil {
		a.reportError("stop IPC server", a.ipcServer.Stop(ctx))
	}

	// Wait for our own background goroutines to exit, but bound the wait so
	// a stuck goroutine cannot prevent the process from exiting.
	waitDone := make(chan struct{})
	go func() {
		a.bgWG.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-ctx.Done():
	}

	close(a.done)
}

// handleMenuClicks handles menu item clicks.
func (a *App) handleMenuClicks() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-a.mOpen.ClickedCh:
			a.showMainWindow("")
		case <-a.mBrowse.ClickedCh:
			a.showMainWindow("/catalog")
		case <-a.mRefresh.ClickedCh:
			a.ui.refreshInBackground(true) // Manual refresh bypasses cache
		case <-a.mUpdateAll.ClickedCh:
			// Queue the updates and show them running, so failures are
			// visible with their reason rather than as a bare count.
			a.ui.enqueueAllUpdates()
			a.showMainWindow("/activity")
		case <-a.mOpenTUI.ClickedCh:
			go a.openTUI()
		case <-a.mSettings.ClickedCh:
			a.showMainWindow("/settings")
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
			a.reportError("refresh agents", a.refreshAgents(a.ctx))
		case <-checkTicker.C:
			if a.config.Updates.AutoCheck {
				a.reportError("check updates", a.checkUpdates(a.ctx))
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
	// Run the shared detect+version-check pipeline. Per-index version-check
	// errors are intentionally ignored here; the systray surfaces failures
	// implicitly via HasUpdate() returning false for agents whose registries
	// could not be reached. TolerateCatalogError preserves the historical
	// systray behavior of starting up even when the catalog fetch fails.
	pipeline := orchestrator.NewFromManagers(a.config, a.platform, a.store, a.catalog, a.detector, a.installer)
	res, err := pipeline.DetectAndCheckVersions(ctx, orchestrator.Options{
		ForceRefresh:         forceRefresh,
		TolerateCatalogError: true,
	})
	if err != nil {
		return err
	}
	detected := res.Installations

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

// checkUpdates checks for available updates by re-querying each installed
// agent's package registry for the latest version.
//
// When cfg.Updates.AutoCheck is false, this reverts to the previous behavior
// of simply recounting HasUpdate() on the already-cached snapshot (used for
// user-initiated triggers that should not hit the network).
func (a *App) checkUpdates(ctx context.Context) error {
	a.agentsMu.Lock()
	a.lastCheck = time.Now()
	a.agentsMu.Unlock()

	if a.config.Updates.AutoCheck {
		// Snapshot current agents under the read lock so we can mutate copies
		// off-lock while the parallel fetcher runs.
		a.agentsMu.RLock()
		snapshot := make([]*agent.Installation, len(a.agents))
		for i := range a.agents {
			agCopy := a.agents[i]
			snapshot[i] = &agCopy
		}
		a.agentsMu.RUnlock()

		pipeline := orchestrator.NewFromManagers(a.config, a.platform, a.store, a.catalog, a.detector, a.installer)
		result, err := pipeline.CheckVersions(ctx, snapshot)
		if err == nil {
			latest := make(map[string]*agent.Version, len(result.Installations))
			for _, inst := range result.Installations {
				latest[inst.Key()] = inst.LatestVersion
			}
			a.agentsMu.Lock()
			for i := range a.agents {
				if version, ok := latest[a.agents[i].Key()]; ok {
					a.agents[i].LatestVersion = version
				}
			}
			a.agentsMu.Unlock()
		}
	}

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
		a.notify(
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
	a.mStatus.SetTitle(statusTitle(agentCount))

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
		a.mUpdateAll.SetTitle(updateAllTitle(updatesAvailable))
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
	a.mStatus.SetTitle(statusTitle(agentCount))

	// Update updates line (keep text short to minimize menu resizing)
	if updatesAvailable > 0 {
		a.mUpdateAll.SetTitle(updateAllTitle(updatesAvailable))
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
				a.showMainWindow("/agent/" + foundAgent.AgentID)
			}
		}
	}
}

// openTUI launches the TUI application in a new terminal window.
func (a *App) openTUI() {
	// Find the agentmgr binary
	agentmgrPath, err := findAgentMgrBinary()
	if err != nil {
		a.notify("Error", "Could not find agentmgr binary")
		return
	}

	// Launch TUI based on platform
	var cmd *exec.Cmd
	switch a.platform.ID() {
	case platform.Darwin:
		// Use osascript to open Terminal with the TUI command
		script := terminalAppleScript(agentmgrPath)
		// #nosec G204 -- Fixed desktop command; text and paths are passed as arguments or explicitly quoted script literals.
		cmd = exec.Command("osascript", "-e", script)
	case platform.Linux:
		// Try common terminal emulators in order of preference
		terminals := []struct {
			name string
			args []string
		}{
			{"gnome-terminal", []string{"--", agentmgrPath, "tui"}},
			{"konsole", []string{"-e", agentmgrPath, "tui"}},
			{"xfce4-terminal", []string{"-e", shellQuote(agentmgrPath) + " tui"}},
			{"xterm", []string{"-e", agentmgrPath, "tui"}},
		}
		for _, term := range terminals {
			if _, err := exec.LookPath(term.name); err == nil {
				// #nosec G204 -- Fixed desktop command; text and paths are passed as arguments or explicitly quoted script literals.
				cmd = exec.Command(term.name, term.args...)
				break
			}
		}
		if cmd == nil {
			a.notify("Error", "No supported terminal emulator found")
			return
		}
	case platform.Windows:
		// PowerShell single-quoted literals keep path metacharacters out of code.
		// #nosec G204 -- Executable path is escaped as a PowerShell literal; arguments are fixed.
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", windowsTerminalScript(agentmgrPath))
	default:
		a.notify("Error", "Unsupported platform")
		return
	}

	// Start the command (don't wait for it)
	if err := cmd.Start(); err != nil {
		a.notify("Error", fmt.Sprintf("Failed to launch TUI: %v", err))
		return
	}

	// Release the process so it runs independently
	if cmd.Process != nil {
		a.reportError("release terminal process", cmd.Process.Release())
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

// syncAutoStartMenu makes the tray checkmark match a change made elsewhere.
func (a *App) syncAutoStartMenu(enabled bool) {
	if a.mAutoStart == nil {
		return
	}
	if enabled {
		a.mAutoStart.Check()
	} else {
		a.mAutoStart.Uncheck()
	}
}

// installOrUpdateCLI installs or updates the agentmgr CLI. Returns true on success.
func (a *App) installOrUpdateCLI() bool {
	// Find the bundled agentmgr binary (should be in same directory as helper)
	bundledPath, err := findBundledAgentMgr()
	if err != nil {
		a.notify("Install Failed", "Could not find bundled agentmgr binary")
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
		a.notify("Install CLI", "Unsupported platform")
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
	var cmd *exec.Cmd
	if a.platform.ID() == platform.Linux {
		// #nosec G204 -- Explicit user-requested install; fixed program and mode with paths as argv, without a shell.
		cmd = exec.Command("pkexec", "install", "-D", "-m", "0755", sourcePath, targetPath)
	} else {
		command := installShellCommand(sourcePath, targetPath)
		script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapeAppleScript(command))
		// #nosec G204 -- Paths are shell-quoted, then escaped for the enclosing AppleScript literal.
		cmd = exec.Command("osascript", "-e", script)
	}
	err := cmd.Run()
	if err != nil {
		return false
	}

	// Update config with the new path
	a.config.Helper.CLIPath = targetPath
	if a.configLoader != nil {
		a.reportError("save CLI path", a.configLoader.SetAndSave("helper.cli_path", targetPath))
	}
	return true
}

// installCLIToPathWindows installs the CLI binary on Windows.
func (a *App) installCLIToPathWindows(sourcePath, targetPath string) {
	if err := copyCLIForWindows(sourcePath, targetPath); err != nil {
		a.notify("Install Failed", fmt.Sprintf("Could not install CLI: %v", err))
		return
	}

	a.notify("Install Complete", fmt.Sprintf("agentmgr installed to %s\n\nAdd this to your PATH to use from command line.", targetPath))

	// Update config
	a.config.Helper.CLIPath = targetPath
	if a.configLoader != nil {
		a.reportError("save CLI path", a.configLoader.SetAndSave("helper.cli_path", targetPath))
	}
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

// statusTitle is the tray's one-line summary of what was found.
func statusTitle(agentCount int) string {
	switch agentCount {
	case 0:
		return "No agents installed"
	case 1:
		return "1 agent installed"
	default:
		return fmt.Sprintf("%d agents installed", agentCount)
	}
}

// updateAllTitle labels the tray's update action.
func updateAllTitle(updates int) string {
	if updates == 1 {
		return "Update 1 Agent"
	}
	return fmt.Sprintf("Update %d Agents", updates)
}
