// Package platform provides OS-specific abstractions for cross-platform support.
package platform

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

// ID represents a platform identifier.
type ID string

const (
	Darwin  ID = "darwin"
	Linux   ID = "linux"
	Windows ID = "windows"
)

const (
	windowsExeSuffix = ".exe"
	envHome          = "HOME"
	envUserProfile   = "USERPROFILE"
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
		return windowsExeSuffix
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
		return envUserProfile
	}
	return envHome
}

// TempDir returns the temp directory for the current platform.
func TempDir() string {
	if IsWindows() {
		return "C:\\Windows\\Temp"
	}
	return "/tmp"
}

// lookPathCache memoizes exec.LookPath results keyed by PATH + executable name.
// It is process-global because PATH resolution is also process-global. Entries
// are invalidated automatically when PATH changes (the key embeds PATH).
var lookPathCache sync.Map // key: "<PATH>\x00<name>" -> lookPathResult

type lookPathResult struct {
	path string
	err  error
}

// cachedLookPath returns a memoized exec.LookPath result. It is safe for
// concurrent use. Callers should not retain the returned error across
// environment changes other than PATH (which is part of the cache key).
func cachedLookPath(name string) (string, error) {
	pathEnv := os.Getenv("PATH")
	key := pathEnv + "\x00" + name

	if v, ok := lookPathCache.Load(key); ok {
		if r, ok := v.(lookPathResult); ok {
			return r.path, r.err
		}
	}

	path, err := exec.LookPath(name)
	// LoadOrStore ensures only one entry wins under races.
	actual, _ := lookPathCache.LoadOrStore(key, lookPathResult{path: path, err: err})
	if r, ok := actual.(lookPathResult); ok {
		return r.path, r.err
	}
	return path, err
}

// resetLookPathCache clears the memoization cache. Intended for tests.
//
// Uses Range+Delete rather than reassigning the package-level sync.Map —
// struct assignment is not atomic with respect to concurrent readers, so
// reassigning while cachedLookPath is running from another goroutine would
// race on the zero value.
func resetLookPathCache() {
	lookPathCache.Range(func(k, _ any) bool {
		lookPathCache.Delete(k)
		return true
	})
}
