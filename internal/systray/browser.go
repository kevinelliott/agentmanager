package systray

import (
	"os/exec"
	"runtime"
)

// openInDefaultBrowser opens a URL with the system's default handler.
func openInDefaultBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// #nosec G204 -- Fixed program; the URL is passed as a single argument, not through a shell.
		cmd = exec.Command("open", url)
	case "windows":
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
