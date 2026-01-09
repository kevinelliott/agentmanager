// Package platform provides OS-specific abstractions for cross-platform support.
package platform

import (
	"context"
	"os"
	"runtime"
)

// ID represents a platform identifier.
type ID string

const (
	Darwin  ID = "darwin"
	Linux   ID = "linux"
	Windows ID = "windows"
)

// Platform abstracts OS-specific operations.
type Platform interface {
	// Identity
	ID() ID
	Architecture() string
	Name() string

	// Paths
	GetDataDir() string
	GetConfigDir() string
	GetCacheDir() string
	GetLogDir() string
	GetIPCSocketPath() string

	// Auto-start
	EnableAutoStart(ctx context.Context) error
	DisableAutoStart(ctx context.Context) error
	IsAutoStartEnabled(ctx context.Context) (bool, error)

	// Executables
	FindExecutable(name string) (string, error)
	FindExecutables(name string) ([]string, error)
	IsExecutableInPath(name string) bool
	GetPathDirs() []string

	// Commands
	GetShell() string
	GetShellArg() string

	// Notifications
	ShowNotification(title, message string) error

	// Dialogs
	ShowChangelogDialog(agentName, fromVer, toVer, changelog string) DialogResult
}

// DialogResult represents the result of a dialog interaction.
type DialogResult int

const (
	DialogResultCancel DialogResult = iota
	DialogResultUpdate
	DialogResultRemindLater
	DialogResultViewDetails
)

// current holds the singleton platform instance.
var current Platform

// Current returns the Platform implementation for the current OS.
func Current() Platform {
	if current == nil {
		current = newPlatform()
	}
	return current
}

// CurrentID returns the current platform ID.
func CurrentID() ID {
	return ID(runtime.GOOS)
}

// CurrentArch returns the current architecture.
func CurrentArch() string {
	return runtime.GOARCH
}

// IsDarwin returns true if running on macOS.
func IsDarwin() bool {
	return runtime.GOOS == string(Darwin)
}

// IsLinux returns true if running on Linux.
func IsLinux() bool {
	return runtime.GOOS == string(Linux)
}

// IsWindows returns true if running on Windows.
func IsWindows() bool {
	return runtime.GOOS == string(Windows)
}

// Supports returns true if the given platform ID is supported.
func Supports(id ID) bool {
	return id == Darwin || id == Linux || id == Windows
}

// ExecutableExtension returns the executable file extension for the current platform.
func ExecutableExtension() string {
	if IsWindows() {
		return ".exe"
	}
	return ""
}

// PathSeparator returns the path separator for the current platform.
func PathSeparator() string {
	if IsWindows() {
		return ";"
	}
	return ":"
}

// HomeDirEnv returns the environment variable name for the home directory.
func HomeDirEnv() string {
	if IsWindows() {
		return "USERPROFILE"
	}
	return "HOME"
}

// TempDir returns the temp directory for the current platform.
func TempDir() string {
	if IsWindows() {
		return "C:\\Windows\\Temp"
	}
	return "/tmp"
}

// IsWSL returns true if running inside Windows Subsystem for Linux.
func IsWSL() bool {
	// Check for WSL-specific environment variables
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	return false
}

// IsNativeWindows returns true if running on native Windows (not WSL).
func IsNativeWindows() bool {
	return IsWindows() && !IsWSL()
}

// CheckWindowsSupport checks if the current platform is supported and returns an error message if not.
// Returns empty string if the platform is supported.
func CheckWindowsSupport() string {
	if IsNativeWindows() {
		return `AgentManager does not currently support native Windows.

Please use one of the following alternatives:
  - Windows Subsystem for Linux (WSL): https://docs.microsoft.com/en-us/windows/wsl/install
  - Docker with a Linux container
  - A Linux virtual machine

To install WSL, run this command in PowerShell as Administrator:
  wsl --install

After WSL is installed, you can run AgentManager inside your Linux distribution.`
	}
	return ""
}
