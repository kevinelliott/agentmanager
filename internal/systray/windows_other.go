//go:build !darwin

package systray

import "github.com/kevinelliott/agentmanager/pkg/agent"

// showNativeSettingsWindow is not available on this platform.
func (a *App) showNativeSettingsWindow() {
	// Fall back to platform-specific dialog
	a.showSettings()
}

// showNativeAgentDetailsWindow is not available on this platform.
func (a *App) showNativeAgentDetailsWindow(inst agent.Installation) {
	// Fall back to platform-specific dialog
	a.showAgentDetails(inst)
}

// closeAllNativeWindows is a no-op on non-darwin platforms.
func closeAllNativeWindows() {}

// hasNativeWindowSupport returns false on non-darwin platforms.
func hasNativeWindowSupport() bool {
	return false
}
