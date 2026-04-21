//go:build darwin

package systray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// uninstallCLI removes the CLI binary from the system path.
//
// Only invoked by the darwin systray menu; other platforms have no caller and
// the function is omitted from their builds via the build tag on this file.
func (a *App) uninstallCLI() bool {
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
