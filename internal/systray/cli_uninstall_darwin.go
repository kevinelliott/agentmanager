//go:build darwin

package systray

import (
	"fmt"
	"os/exec"

	"github.com/kevinelliott/agentmanager/pkg/config"
)

// uninstallCLI removes the CLI binary from the system path on darwin.
//
// Only invoked by the darwin systray menu. The function is omitted from
// non-darwin builds via the build tag on this file, so platform.ID() is
// always platform.Darwin at runtime — no need to branch on it.
func (a *App) uninstallCLI() bool {
	const targetPath = "/usr/local/bin/agentmgr"

	// Use osascript to run sudo with password prompt.
	script := fmt.Sprintf(`
do shell script "rm -f '%s'" with administrator privileges
`, targetPath)

	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return false
	}

	// Clear config path.
	a.reportError("clear CLI path", a.saveSetting("helper.cli_path", "", func(c *config.Config) {
		c.Helper.CLIPath = ""
	}))
	return true
}
