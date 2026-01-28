//go:build linux

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
	return &linuxPlatform{}
}

type linuxPlatform struct{}

func (l *linuxPlatform) ID() ID {
	return Linux
}

func (l *linuxPlatform) Architecture() string {
	return CurrentArch()
}

func (l *linuxPlatform) Name() string {
	return "Linux"
}

func (l *linuxPlatform) GetDataDir() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "agentmgr")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir() + "/agentmgr"
	}
	return filepath.Join(home, ".local", "share", "agentmgr")
}

func (l *linuxPlatform) GetConfigDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "agentmgr")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir() + "/agentmgr"
	}
	return filepath.Join(home, ".config", "agentmgr")
}

func (l *linuxPlatform) GetCacheDir() string {
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "agentmgr")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir() + "/agentmgr/cache"
	}
	return filepath.Join(home, ".cache", "agentmgr")
}

func (l *linuxPlatform) GetLogDir() string {
	dataDir := l.GetDataDir()
	return filepath.Join(dataDir, "logs")
}

func (l *linuxPlatform) GetIPCSocketPath() string {
	if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
		return filepath.Join(xdgRuntime, "agentmgr.sock")
	}
	return filepath.Join(os.TempDir(), "agentmgr.sock")
}

func (l *linuxPlatform) EnableAutoStart(ctx context.Context) error {
	// Try systemd user service first
	if l.hasSystemd() {
		return l.enableSystemdAutoStart(ctx)
	}
	// Fall back to XDG autostart
	return l.enableXDGAutoStart()
}

func (l *linuxPlatform) DisableAutoStart(ctx context.Context) error {
	// Try both methods - errors are intentionally ignored as we want to attempt both
	//nolint:errcheck
	l.disableSystemdAutoStart(ctx)
	//nolint:errcheck
	l.disableXDGAutoStart()
	return nil
}

func (l *linuxPlatform) IsAutoStartEnabled(ctx context.Context) (bool, error) {
	// Check systemd
	if l.hasSystemd() {
		enabled, err := l.isSystemdEnabled(ctx)
		if err == nil && enabled {
			return true, nil
		}
	}
	// Check XDG
	return l.isXDGEnabled()
}

func (l *linuxPlatform) hasSystemd() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func (l *linuxPlatform) enableSystemdAutoStart(ctx context.Context) error {
	serviceDir := l.getSystemdUserDir()
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return err
	}

	servicePath := filepath.Join(serviceDir, "agentmgr-helper.service")
	serviceContent := `[Unit]
Description=AgentManager Helper
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/agentmgr-helper
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return err
	}

	// Reload and enable
	exec.CommandContext(ctx, "systemctl", "--user", "daemon-reload").Run()
	if err := exec.CommandContext(ctx, "systemctl", "--user", "enable", "agentmgr-helper.service").Run(); err != nil {
		return err
	}
	return exec.CommandContext(ctx, "systemctl", "--user", "start", "agentmgr-helper.service").Run()
}

func (l *linuxPlatform) disableSystemdAutoStart(ctx context.Context) error {
	exec.CommandContext(ctx, "systemctl", "--user", "stop", "agentmgr-helper.service").Run()
	exec.CommandContext(ctx, "systemctl", "--user", "disable", "agentmgr-helper.service").Run()

	servicePath := filepath.Join(l.getSystemdUserDir(), "agentmgr-helper.service")
	os.Remove(servicePath)
	return nil
}

func (l *linuxPlatform) isSystemdEnabled(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "--user", "is-enabled", "agentmgr-helper.service")
	err := cmd.Run()
	return err == nil, nil
}

func (l *linuxPlatform) getSystemdUserDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "systemd", "user")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir() + "/systemd/user"
	}
	return filepath.Join(home, ".config", "systemd", "user")
}

func (l *linuxPlatform) enableXDGAutoStart() error {
	autostartDir := l.getXDGAutostartDir()
	if err := os.MkdirAll(autostartDir, 0755); err != nil {
		return err
	}

	desktopEntry := `[Desktop Entry]
Type=Application
Name=AgentManager Helper
Exec=/usr/local/bin/agentmgr-helper
Hidden=false
NoDisplay=true
X-GNOME-Autostart-enabled=true
`

	desktopPath := filepath.Join(autostartDir, "agentmgr-helper.desktop")
	return os.WriteFile(desktopPath, []byte(desktopEntry), 0644)
}

func (l *linuxPlatform) disableXDGAutoStart() error {
	desktopPath := filepath.Join(l.getXDGAutostartDir(), "agentmgr-helper.desktop")
	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (l *linuxPlatform) isXDGEnabled() (bool, error) {
	desktopPath := filepath.Join(l.getXDGAutostartDir(), "agentmgr-helper.desktop")
	_, err := os.Stat(desktopPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (l *linuxPlatform) getXDGAutostartDir() string {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "autostart")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir() + "/autostart"
	}
	return filepath.Join(home, ".config", "autostart")
}

func (l *linuxPlatform) FindExecutable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("executable %q not found: %w", name, err)
	}
	return path, nil
}

func (l *linuxPlatform) FindExecutables(name string) ([]string, error) {
	var paths []string
	pathDirs := l.GetPathDirs()

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

func (l *linuxPlatform) IsExecutableInPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (l *linuxPlatform) GetPathDirs() []string {
	pathEnv := os.Getenv("PATH")
	return strings.Split(pathEnv, ":")
}

func (l *linuxPlatform) GetShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return shell
}

func (l *linuxPlatform) GetShellArg() string {
	return "-c"
}

func (l *linuxPlatform) ShowNotification(title, message string) error {
	// Try notify-send first (most common)
	if _, err := exec.LookPath("notify-send"); err == nil {
		return exec.Command("notify-send", title, message).Run()
	}
	// Try zenity
	if _, err := exec.LookPath("zenity"); err == nil {
		return exec.Command("zenity", "--notification", "--text="+title+"\n"+message).Run() //nolint:gosec // User-provided notification text is intentional
	}
	return fmt.Errorf("no notification system available")
}

func (l *linuxPlatform) ShowChangelogDialog(agentName, fromVer, toVer, changelog string) DialogResult {
	// Try zenity first
	if _, err := exec.LookPath("zenity"); err == nil {
		return l.showZenityDialog(agentName, fromVer, toVer, changelog)
	}
	// Try kdialog
	if _, err := exec.LookPath("kdialog"); err == nil {
		return l.showKDialogDialog(agentName, fromVer, toVer, changelog)
	}
	return DialogResultCancel
}

func (l *linuxPlatform) showZenityDialog(agentName, fromVer, toVer, changelog string) DialogResult {
	text := fmt.Sprintf("%s\n\n%s → %s\n\n%s", agentName, fromVer, toVer, changelog)

	cmd := exec.Command("zenity", "--question", //nolint:gosec // User-provided dialog text is intentional
		"--title=Update Available",
		"--text="+text,
		"--ok-label=Update Now",
		"--cancel-label=Cancel",
		"--extra-button=Remind Later")

	output, err := cmd.Output()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 1 {
			result := strings.TrimSpace(string(output))
			if result == "Remind Later" {
				return DialogResultRemindLater
			}
		}
		return DialogResultCancel
	}
	return DialogResultUpdate
}

func (l *linuxPlatform) showKDialogDialog(agentName, fromVer, toVer, changelog string) DialogResult {
	text := fmt.Sprintf("%s\n\n%s → %s\n\n%s", agentName, fromVer, toVer, changelog)

	cmd := exec.Command("kdialog",
		"--yesnocancel", text,
		"--title", "Update Available",
		"--yes-label", "Update Now",
		"--no-label", "Remind Later",
		"--cancel-label", "Cancel")

	err := cmd.Run()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			switch exitErr.ExitCode() {
			case 1: // No
				return DialogResultRemindLater
			default:
				return DialogResultCancel
			}
		}
		return DialogResultCancel
	}
	return DialogResultUpdate
}
