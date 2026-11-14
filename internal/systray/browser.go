package systray

import (
	"os/exec"

	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// openInDefaultBrowser opens a URL with the system's default handler.
func openInDefaultBrowser(url string) error {
	var cmd *exec.Cmd
	switch platform.Current().ID() {
	case platform.Darwin:
		// #nosec G204 -- Fixed program; the URL is passed as a single argument, not through a shell.
		cmd = exec.Command("open", url)
	case platform.Windows:
		// #nosec G204 -- Fixed program; the URL is passed as a single argument, not through a shell.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		// #nosec G204 -- Fixed program; the URL is passed as a single argument, not through a shell.
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
