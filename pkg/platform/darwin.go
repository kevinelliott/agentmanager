//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// newPlatform creates the platform implementation for the current OS.
func newPlatform() Platform {
	return &darwinPlatform{}
}

type darwinPlatform struct{}

func (d *darwinPlatform) ID() ID {
	return Darwin
}

// homeDir returns the user's home directory or a fallback.
func (d *darwinPlatform) homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to common macOS home directory pattern
		return "/Users/" + os.Getenv("USER")
	}
	return home
}

func (d *darwinPlatform) Architecture() string {
	return CurrentArch()
}

func (d *darwinPlatform) Name() string {
	return "macOS"
}

func (d *darwinPlatform) GetDataDir() string {
	return filepath.Join(d.homeDir(), "Library", "Application Support", "AgentManager")
}

func (d *darwinPlatform) GetConfigDir() string {
	return filepath.Join(d.homeDir(), "Library", "Preferences", "AgentManager")
}

func (d *darwinPlatform) GetCacheDir() string {
	return filepath.Join(d.homeDir(), "Library", "Caches", "AgentManager")
}

func (d *darwinPlatform) GetLogDir() string {
	return filepath.Join(d.homeDir(), "Library", "Logs", "AgentManager")
}

func (d *darwinPlatform) GetIPCSocketPath() string {
	return filepath.Join(os.TempDir(), "agentmgr.sock")
}

func (d *darwinPlatform) EnableAutoStart(ctx context.Context) error {
	plistPath := d.getLaunchAgentPath()
	plistContent := d.generateLaunchAgentPlist()

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Write plist file
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Load the agent
	cmd := exec.CommandContext(ctx, "launchctl", "load", plistPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load launch agent: %w", err)
	}

	return nil
}

func (d *darwinPlatform) DisableAutoStart(ctx context.Context) error {
	plistPath := d.getLaunchAgentPath()

	// Unload the agent (ignore errors if not loaded)
	_ = exec.CommandContext(ctx, "launchctl", "unload", plistPath).Run()

	// Remove plist file
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

func (d *darwinPlatform) IsAutoStartEnabled(ctx context.Context) (bool, error) {
	plistPath := d.getLaunchAgentPath()
	_, err := os.Stat(plistPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *darwinPlatform) getLaunchAgentPath() string {
	return filepath.Join(d.homeDir(), "Library", "LaunchAgents", "com.agentmgr.helper.plist")
}

func (d *darwinPlatform) generateLaunchAgentPlist() string {
	logDir := d.GetLogDir()
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.agentmgr.helper</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/agentmgr-helper</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s/helper.log</string>
    <key>StandardErrorPath</key>
    <string>%s/helper.error.log</string>
</dict>
</plist>`, logDir, logDir)
}

func (d *darwinPlatform) FindExecutable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("executable %q not found: %w", name, err)
	}
	return path, nil
}

func (d *darwinPlatform) FindExecutables(name string) ([]string, error) {
	var paths []string
	pathDirs := d.GetPathDirs()

	for _, dir := range pathDirs {
		fullPath := filepath.Join(dir, name)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			paths = append(paths, fullPath)
		}
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("executable %q not found", name)
	}

	return paths, nil
}

func (d *darwinPlatform) IsExecutableInPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (d *darwinPlatform) GetPathDirs() []string {
	pathEnv := os.Getenv("PATH")
	return strings.Split(pathEnv, ":")
}

func (d *darwinPlatform) GetShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	return shell
}

func (d *darwinPlatform) GetShellArg() string {
	return "-c"
}

func (d *darwinPlatform) ShowNotification(title, message string) error {
	script := fmt.Sprintf(`display notification "%s" with title "%s"`,
		escapeAppleScript(message),
		escapeAppleScript(title))
	return exec.Command("osascript", "-e", script).Run()
}

func (d *darwinPlatform) ShowChangelogDialog(agentName, fromVer, toVer, changelog string) DialogResult {
	// Use osascript to show a dialog
	script := fmt.Sprintf(`
set theMessage to "%s"
set theResult to display dialog theMessage with title "%s Update Available" buttons {"Cancel", "Remind Later", "Update Now"} default button "Update Now"
return button returned of theResult
`,
		escapeAppleScript(fmt.Sprintf("%s → %s\n\n%s", fromVer, toVer, changelog)),
		escapeAppleScript(agentName))

	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.Output()
	if err != nil {
		return DialogResultCancel
	}

	result := strings.TrimSpace(string(output))
	switch result {
	case "Update Now":
		return DialogResultUpdate
	case "Remind Later":
		return DialogResultRemindLater
	default:
		return DialogResultCancel
	}
}

func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
