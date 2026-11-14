//go:build !darwin

package systray

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// canUninstallCLI reports whether this platform can remove the CLI it installed.
const canUninstallCLI = false

// uninstallCLI is only implemented on macOS.
func (a *App) uninstallCLI() bool { return false }

// showMainWindow opens the AgentManager window. On Windows and Linux the UI
// is shown in a chromeless browser window when Edge, Chrome or Chromium is
// installed, and in the default browser otherwise.
func (a *App) showMainWindow(route string) {
	url := a.ui.entryURL(route)

	if browser := findAppModeBrowser(); browser != "" {
		// #nosec G204 -- Browser path comes from a fixed candidate list; the URL is our own loopback address.
		cmd := exec.Command(browser, "--app="+url, "--window-size=1120,740")
		if err := cmd.Start(); err == nil {
			a.reportError("release browser process", cmd.Process.Release())
			return
		}
	}

	if err := openInDefaultBrowser(url); err != nil {
		a.reportError("open AgentManager window", err)
		a.notify("AgentManager", "Could not open the window. No web browser was found.")
	}
}

// findAppModeBrowser returns a Chromium-family browser that supports --app
// windows, or "" if none is installed.
func findAppModeBrowser() string {
	if platform.IsWindows() {
		var roots []string
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
			if dir := os.Getenv(env); dir != "" {
				roots = append(roots, dir)
			}
		}
		for _, rel := range []string{
			filepath.Join("Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join("Google", "Chrome", "Application", "chrome.exe"),
		} {
			for _, root := range roots {
				candidate := filepath.Join(root, rel)
				// #nosec G703 -- Only probes fixed browser install locations with Stat; nothing is read or written.
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
		}
		return ""
	}

	for _, name := range []string{
		"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
		"microsoft-edge", "brave-browser",
	} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// closeMainWindow is a no-op here: the window belongs to the browser.
func closeMainWindow() {}
