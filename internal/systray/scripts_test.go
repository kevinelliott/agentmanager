package systray

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallShellCommandTreatsPathsAsLiterals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "bundle'$(touch injected)`touch injected`")
	target := filepath.Join(dir, "target folder", "agent'mgr")
	if err := os.WriteFile(source, []byte("executable"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", installShellCommand(source, target))
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("literal-path installation failed: %v %s", err, output)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "executable" {
		t.Fatalf("installed content = %q %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "injected")); !os.IsNotExist(err) {
		t.Fatalf("path executed as shell code: %v", err)
	}
}

func TestAppleScriptLiteralRoundTrip(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("AppleScript is only available on macOS")
	}
	value := "path with \"quotes\", \\slashes and\na newline"
	output, err := exec.Command("osascript", "-e", `return "`+escapeAppleScript(value)+`"`).Output()
	if err != nil || strings.TrimSuffix(string(output), "\n") != value {
		t.Fatalf("AppleScript literal = %q %v, want %q", output, err, value)
	}
}

func TestPowerShellLiteralRoundTrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell is verified on Windows")
	}
	value := `C:\agent's path\$(throw 'injected'); & agentmgr.exe`
	output, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "[Console]::Write("+powerShellQuote(value)+")").Output()
	if err != nil || string(output) != value {
		t.Fatalf("PowerShell literal = %q %v, want %q", output, err, value)
	}
}

func TestTerminalScriptsQuoteExecutablePaths(t *testing.T) {
	path := `a'b"c\d;$(exit 1)`
	if script := terminalAppleScript(path); !strings.Contains(script, escapeAppleScript(shellQuote(path)+" tui")) {
		t.Fatalf("Terminal command missing quoted path: %s", script)
	}
	if script := windowsTerminalScript(path); !strings.Contains(script, powerShellQuote(path)) {
		t.Fatalf("PowerShell command missing quoted path: %s", script)
	}
}
