//go:build windows

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// newPlatform creates the platform implementation for the current OS.
func newPlatform() Platform {
	return &windowsPlatform{}
}

type windowsPlatform struct{}

func (w *windowsPlatform) ID() ID {
	return Windows
}

func (w *windowsPlatform) Architecture() string {
	return CurrentArch()
}

func (w *windowsPlatform) Name() string {
	return "Windows"
}

func (w *windowsPlatform) GetDataDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(localAppData, "AgentManager")
}

func (w *windowsPlatform) GetConfigDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
	}
	return filepath.Join(appData, "AgentManager")
}

func (w *windowsPlatform) GetCacheDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(localAppData, "AgentManager", "Cache")
}

func (w *windowsPlatform) GetLogDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(localAppData, "AgentManager", "Logs")
}

func (w *windowsPlatform) GetIPCSocketPath() string {
	return `\\.\pipe\agentmgr`
}

func (w *windowsPlatform) EnableAutoStart(ctx context.Context) error {
	key, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	exePath := filepath.Join(w.GetDataDir(), "agentmgr-helper.exe")

	// Try to find the executable in common locations
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		// Try Program Files
		programFiles := os.Getenv("PROGRAMFILES")
		if programFiles != "" {
			altPath := filepath.Join(programFiles, "AgentManager", "agentmgr-helper.exe")
			if _, err := os.Stat(altPath); err == nil {
				exePath = altPath
			}
		}
	}

	return key.SetStringValue("AgentManager", exePath)
}

func (w *windowsPlatform) DisableAutoStart(ctx context.Context) error {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.ALL_ACCESS,
	)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	err = key.DeleteValue("AgentManager")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("failed to delete registry value: %w", err)
	}
	return nil
}

func (w *windowsPlatform) IsAutoStartEnabled(ctx context.Context) (bool, error) {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false, nil
	}
	defer key.Close()

	_, _, err = key.GetStringValue("AgentManager")
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (w *windowsPlatform) FindExecutable(name string) (string, error) {
	// Add .exe if not present
	if !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name = name + ".exe"
	}

	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("executable %q not found: %w", name, err)
	}
	return path, nil
}

func (w *windowsPlatform) FindExecutables(name string) ([]string, error) {
	// Add .exe if not present
	if !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name = name + ".exe"
	}

	var paths []string
	pathDirs := w.GetPathDirs()

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

func (w *windowsPlatform) IsExecutableInPath(name string) bool {
	if !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name = name + ".exe"
	}
	_, err := exec.LookPath(name)
	return err == nil
}

func (w *windowsPlatform) GetPathDirs() []string {
	pathEnv := os.Getenv("PATH")
	return strings.Split(pathEnv, ";")
}

func (w *windowsPlatform) GetShell() string {
	// Prefer PowerShell if available
	if _, err := exec.LookPath("pwsh"); err == nil {
		return "pwsh"
	}
	if _, err := exec.LookPath("powershell"); err == nil {
		return "powershell"
	}
	return "cmd"
}

func (w *windowsPlatform) GetShellArg() string {
	shell := w.GetShell()
	if strings.Contains(shell, "powershell") || strings.Contains(shell, "pwsh") {
		return "-Command"
	}
	return "/c"
}

func (w *windowsPlatform) ShowNotification(title, message string) error {
	// Use PowerShell to show a toast notification
	script := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$template = @"
<toast>
    <visual>
        <binding template="ToastText02">
            <text id="1">%s</text>
            <text id="2">%s</text>
        </binding>
    </visual>
</toast>
"@
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("AgentManager").Show($toast)
`, escapeForPowerShell(title), escapeForPowerShell(message))

	return exec.Command("powershell", "-Command", script).Run()
}

func (w *windowsPlatform) ShowChangelogDialog(agentName, fromVer, toVer, changelog string) DialogResult {
	// Use PowerShell to show a dialog
	message := fmt.Sprintf("%s Update Available\n\n%s → %s\n\n%s",
		agentName, fromVer, toVer, changelog)

	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$result = [System.Windows.Forms.MessageBox]::Show(
    '%s',
    'AgentManager Update',
    [System.Windows.Forms.MessageBoxButtons]::YesNoCancel,
    [System.Windows.Forms.MessageBoxIcon]::Information
)
Write-Output $result
`, escapeForPowerShell(message))

	cmd := exec.Command("powershell", "-Command", script)
	output, err := cmd.Output()
	if err != nil {
		return DialogResultCancel
	}

	result := strings.TrimSpace(string(output))
	switch result {
	case "Yes":
		return DialogResultUpdate
	case "No":
		return DialogResultRemindLater
	default:
		return DialogResultCancel
	}
}

func escapeForPowerShell(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "`", "``")
	return s
}
