package systray

import (
	"fmt"
	"path/filepath"
	"strings"
)

// shellQuote makes a single literal argument for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func terminalAppleScript(path string) string {
	return fmt.Sprintf(`tell application "Terminal"
activate
do script "%s"
end tell`, escapeAppleScript(shellQuote(path)+" tui"))
}

func powerShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func windowsTerminalScript(path string) string {
	return fmt.Sprintf("Start-Process -FilePath %s -ArgumentList 'tui'", powerShellQuote(path))
}

func installShellCommand(source, target string) string {
	return fmt.Sprintf("mkdir -p %s && cp %s %s && chmod +x %s", shellQuote(filepath.Dir(target)), shellQuote(source), shellQuote(target), shellQuote(target))
}

// escapeAppleScript escapes a string for use in an AppleScript literal.
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
