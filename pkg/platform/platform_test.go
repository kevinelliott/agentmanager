package platform

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIDs(t *testing.T) {
	// Test ID constants
	if Darwin != "darwin" {
		t.Errorf("Darwin = %q, want %q", Darwin, "darwin")
	}
	if Linux != "linux" {
		t.Errorf("Linux = %q, want %q", Linux, "linux")
	}
	if Windows != "windows" {
		t.Errorf("Windows = %q, want %q", Windows, "windows")
	}
}

func TestDialogResultConstants(t *testing.T) {
	if DialogResultCancel != 0 {
		t.Errorf("DialogResultCancel = %d, want 0", DialogResultCancel)
	}
	if DialogResultUpdate != 1 {
		t.Errorf("DialogResultUpdate = %d, want 1", DialogResultUpdate)
	}
	if DialogResultRemindLater != 2 {
		t.Errorf("DialogResultRemindLater = %d, want 2", DialogResultRemindLater)
	}
	if DialogResultViewDetails != 3 {
		t.Errorf("DialogResultViewDetails = %d, want 3", DialogResultViewDetails)
	}
}

func TestCurrent(t *testing.T) {
	plat := Current()
	if plat == nil {
		t.Fatal("Current() returned nil")
	}

	// Should return same instance on subsequent calls
	plat2 := Current()
	if plat != plat2 {
		t.Error("Current() should return same instance")
	}
}

func TestCurrentID(t *testing.T) {
	id := CurrentID()
	expected := ID(runtime.GOOS)

	if id != expected {
		t.Errorf("CurrentID() = %q, want %q", id, expected)
	}
}

func TestCurrentArch(t *testing.T) {
	arch := CurrentArch()
	expected := runtime.GOARCH

	if arch != expected {
		t.Errorf("CurrentArch() = %q, want %q", arch, expected)
	}
}

func TestIsDarwin(t *testing.T) {
	expected := runtime.GOOS == "darwin"
	if IsDarwin() != expected {
		t.Errorf("IsDarwin() = %v, want %v", IsDarwin(), expected)
	}
}

func TestIsLinux(t *testing.T) {
	expected := runtime.GOOS == "linux"
	if IsLinux() != expected {
		t.Errorf("IsLinux() = %v, want %v", IsLinux(), expected)
	}
}

func TestIsWindows(t *testing.T) {
	expected := runtime.GOOS == "windows"
	if IsWindows() != expected {
		t.Errorf("IsWindows() = %v, want %v", IsWindows(), expected)
	}
}

func TestSupports(t *testing.T) {
	tests := []struct {
		id       ID
		expected bool
	}{
		{Darwin, true},
		{Linux, true},
		{Windows, true},
		{"freebsd", false},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			if Supports(tt.id) != tt.expected {
				t.Errorf("Supports(%q) = %v, want %v", tt.id, Supports(tt.id), tt.expected)
			}
		})
	}
}

func TestExecutableExtension(t *testing.T) {
	ext := ExecutableExtension()
	if IsWindows() {
		if ext != ".exe" {
			t.Errorf("ExecutableExtension() = %q, want %q on Windows", ext, ".exe")
		}
	} else {
		if ext != "" {
			t.Errorf("ExecutableExtension() = %q, want %q on non-Windows", ext, "")
		}
	}
}

func TestPathSeparator(t *testing.T) {
	sep := PathSeparator()
	if IsWindows() {
		if sep != ";" {
			t.Errorf("PathSeparator() = %q, want %q on Windows", sep, ";")
		}
	} else {
		if sep != ":" {
			t.Errorf("PathSeparator() = %q, want %q on non-Windows", sep, ":")
		}
	}
}

func TestHomeDirEnv(t *testing.T) {
	env := HomeDirEnv()
	if IsWindows() {
		if env != "USERPROFILE" {
			t.Errorf("HomeDirEnv() = %q, want %q on Windows", env, "USERPROFILE")
		}
	} else {
		if env != "HOME" {
			t.Errorf("HomeDirEnv() = %q, want %q on non-Windows", env, "HOME")
		}
	}
}

func TestTempDir(t *testing.T) {
	dir := TempDir()
	if IsWindows() {
		if dir != "C:\\Windows\\Temp" {
			t.Errorf("TempDir() = %q, want %q on Windows", dir, "C:\\Windows\\Temp")
		}
	} else {
		if dir != "/tmp" {
			t.Errorf("TempDir() = %q, want %q on non-Windows", dir, "/tmp")
		}
	}
}

func TestPlatformInterface(t *testing.T) {
	plat := Current()

	// Test all interface methods don't panic
	t.Run("ID", func(t *testing.T) {
		id := plat.ID()
		if id != Darwin && id != Linux && id != Windows {
			t.Errorf("unexpected platform ID: %q", id)
		}
	})

	t.Run("Architecture", func(t *testing.T) {
		arch := plat.Architecture()
		if arch == "" {
			t.Error("Architecture() returned empty string")
		}
	})

	t.Run("Name", func(t *testing.T) {
		name := plat.Name()
		if name == "" {
			t.Error("Name() returned empty string")
		}
	})

	t.Run("GetDataDir", func(t *testing.T) {
		dir := plat.GetDataDir()
		if dir == "" {
			t.Error("GetDataDir() returned empty string")
		}
	})

	t.Run("GetConfigDir", func(t *testing.T) {
		dir := plat.GetConfigDir()
		if dir == "" {
			t.Error("GetConfigDir() returned empty string")
		}
	})

	t.Run("GetCacheDir", func(t *testing.T) {
		dir := plat.GetCacheDir()
		if dir == "" {
			t.Error("GetCacheDir() returned empty string")
		}
	})

	t.Run("GetLogDir", func(t *testing.T) {
		dir := plat.GetLogDir()
		if dir == "" {
			t.Error("GetLogDir() returned empty string")
		}
	})

	t.Run("GetIPCSocketPath", func(t *testing.T) {
		path := plat.GetIPCSocketPath()
		if path == "" {
			t.Error("GetIPCSocketPath() returned empty string")
		}
	})

	t.Run("GetPathDirs", func(t *testing.T) {
		dirs := plat.GetPathDirs()
		// May be empty in some environments, but shouldn't panic
		_ = dirs
	})

	t.Run("GetShell", func(t *testing.T) {
		shell := plat.GetShell()
		if shell == "" {
			t.Error("GetShell() returned empty string")
		}
	})

	t.Run("GetShellArg", func(t *testing.T) {
		arg := plat.GetShellArg()
		if arg == "" {
			t.Error("GetShellArg() returned empty string")
		}
	})

	t.Run("IsExecutableInPath", func(t *testing.T) {
		// Test with something that should exist
		// bash on Unix, cmd on Windows
		var execName string
		if IsWindows() {
			execName = "cmd"
		} else {
			execName = "sh"
		}
		result := plat.IsExecutableInPath(execName)
		// Just verify it doesn't panic
		_ = result
	})

	t.Run("FindExecutable", func(t *testing.T) {
		// Test with something unlikely to exist
		_, err := plat.FindExecutable("nonexistent-command-12345")
		// Should return error for non-existent command
		if err == nil {
			t.Log("FindExecutable found unexpected command")
		}
	})

	t.Run("FindExecutables", func(t *testing.T) {
		// Test with something unlikely to exist
		paths, _ := plat.FindExecutables("nonexistent-command-12345")
		// Should return empty or nil for non-existent command
		if len(paths) > 0 {
			t.Log("FindExecutables found unexpected commands")
		}
	})
}

func TestPlatformDirectoryPaths(t *testing.T) {
	plat := Current()
	home, _ := os.UserHomeDir()

	t.Run("GetDataDir contains expected path components", func(t *testing.T) {
		dir := plat.GetDataDir()
		if !strings.Contains(dir, home) && !strings.Contains(dir, "agentmgr") && !strings.Contains(dir, "AgentManager") {
			t.Errorf("GetDataDir() = %q, expected to contain home dir or app name", dir)
		}
	})

	t.Run("GetConfigDir contains expected path components", func(t *testing.T) {
		dir := plat.GetConfigDir()
		if !strings.Contains(dir, home) && !strings.Contains(dir, "agentmgr") && !strings.Contains(dir, "AgentManager") {
			t.Errorf("GetConfigDir() = %q, expected to contain home dir or app name", dir)
		}
	})

	t.Run("GetCacheDir contains expected path components", func(t *testing.T) {
		dir := plat.GetCacheDir()
		if !strings.Contains(dir, home) && !strings.Contains(dir, "agentmgr") && !strings.Contains(dir, "AgentManager") && !strings.Contains(dir, "Cache") {
			t.Errorf("GetCacheDir() = %q, expected to contain home dir or cache-related path", dir)
		}
	})

	t.Run("GetLogDir contains expected path components", func(t *testing.T) {
		dir := plat.GetLogDir()
		if !strings.Contains(dir, home) && !strings.Contains(dir, "log") && !strings.Contains(dir, "Log") {
			t.Errorf("GetLogDir() = %q, expected to contain home dir or log-related path", dir)
		}
	})

	t.Run("GetIPCSocketPath is not empty and has proper format", func(t *testing.T) {
		path := plat.GetIPCSocketPath()
		if path == "" {
			t.Error("GetIPCSocketPath() returned empty string")
		}
		if IsWindows() {
			if !strings.HasPrefix(path, `\\.\pipe\`) {
				t.Errorf("GetIPCSocketPath() = %q, expected Windows named pipe format", path)
			}
		} else {
			if !strings.HasSuffix(path, ".sock") {
				t.Errorf("GetIPCSocketPath() = %q, expected Unix socket format", path)
			}
		}
	})
}

func TestFindExecutableWithRealCommands(t *testing.T) {
	plat := Current()

	t.Run("FindExecutable with sh", func(t *testing.T) {
		if IsWindows() {
			t.Skip("sh not available on Windows")
		}
		path, err := plat.FindExecutable("sh")
		if err != nil {
			t.Errorf("FindExecutable(sh) failed: %v", err)
		}
		if path == "" {
			t.Error("FindExecutable(sh) returned empty path")
		}
		if !strings.Contains(path, "sh") {
			t.Errorf("FindExecutable(sh) = %q, expected to contain 'sh'", path)
		}
	})

	t.Run("FindExecutables with sh", func(t *testing.T) {
		if IsWindows() {
			t.Skip("sh not available on Windows")
		}
		paths, err := plat.FindExecutables("sh")
		if err != nil {
			t.Logf("FindExecutables(sh) returned error (may be expected): %v", err)
		}
		for _, p := range paths {
			if !strings.Contains(p, "sh") {
				t.Errorf("FindExecutables path %q doesn't contain 'sh'", p)
			}
		}
	})

	t.Run("IsExecutableInPath with sh returns true", func(t *testing.T) {
		if IsWindows() {
			t.Skip("sh not available on Windows")
		}
		if !plat.IsExecutableInPath("sh") {
			t.Error("IsExecutableInPath(sh) = false, expected true")
		}
	})

	t.Run("IsExecutableInPath with nonexistent returns false", func(t *testing.T) {
		if plat.IsExecutableInPath("nonexistent-cmd-xyz-12345") {
			t.Error("IsExecutableInPath(nonexistent) = true, expected false")
		}
	})
}

func TestGetPathDirs(t *testing.T) {
	plat := Current()
	dirs := plat.GetPathDirs()

	if len(dirs) == 0 {
		t.Log("GetPathDirs() returned empty slice (may be valid in some environments)")
	}

	for _, dir := range dirs {
		if IsWindows() {
			if strings.Contains(dir, ":") {
				continue
			}
		} else {
			if filepath.IsAbs(dir) || dir == "" {
				continue
			}
		}
	}
}

func TestGetShell(t *testing.T) {
	plat := Current()

	shell := plat.GetShell()
	if shell == "" {
		t.Error("GetShell() returned empty string")
	}

	if IsWindows() {
		if !strings.Contains(shell, "cmd") && !strings.Contains(shell, "powershell") && !strings.Contains(shell, "pwsh") {
			t.Errorf("GetShell() = %q, expected Windows shell", shell)
		}
	} else {
		if !strings.HasPrefix(shell, "/") {
			t.Errorf("GetShell() = %q, expected absolute path on Unix", shell)
		}
	}
}

func TestGetShellArg(t *testing.T) {
	plat := Current()

	arg := plat.GetShellArg()
	if arg == "" {
		t.Error("GetShellArg() returned empty string")
	}

	if IsWindows() {
		if arg != "-Command" && arg != "/c" {
			t.Errorf("GetShellArg() = %q, expected Windows shell arg", arg)
		}
	} else {
		if arg != "-c" {
			t.Errorf("GetShellArg() = %q, expected -c on Unix", arg)
		}
	}
}

func TestIsAutoStartEnabled(t *testing.T) {
	plat := Current()
	ctx := context.Background()

	enabled, err := plat.IsAutoStartEnabled(ctx)
	if err != nil {
		t.Logf("IsAutoStartEnabled returned error (may be expected): %v", err)
	}
	_ = enabled
}

func TestIDString(t *testing.T) {
	tests := []struct {
		id       ID
		expected string
	}{
		{Darwin, "darwin"},
		{Linux, "linux"},
		{Windows, "windows"},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			if string(tt.id) != tt.expected {
				t.Errorf("ID string = %q, want %q", string(tt.id), tt.expected)
			}
		})
	}
}

func TestPlatformIDMatchesRuntime(t *testing.T) {
	plat := Current()
	id := plat.ID()

	switch runtime.GOOS {
	case "darwin":
		if id != Darwin {
			t.Errorf("Platform ID = %q, expected %q", id, Darwin)
		}
	case "linux":
		if id != Linux {
			t.Errorf("Platform ID = %q, expected %q", id, Linux)
		}
	case "windows":
		if id != Windows {
			t.Errorf("Platform ID = %q, expected %q", id, Windows)
		}
	}
}

func TestArchitectureMatchesRuntime(t *testing.T) {
	plat := Current()
	arch := plat.Architecture()

	if arch != runtime.GOARCH {
		t.Errorf("Architecture() = %q, want %q", arch, runtime.GOARCH)
	}
}

func TestPlatformName(t *testing.T) {
	plat := Current()
	name := plat.Name()

	switch runtime.GOOS {
	case "darwin":
		if name != "macOS" {
			t.Errorf("Name() = %q, expected macOS", name)
		}
	case "linux":
		if name != "Linux" {
			t.Errorf("Name() = %q, expected Linux", name)
		}
	case "windows":
		if name != "Windows" {
			t.Errorf("Name() = %q, expected Windows", name)
		}
	}
}

func TestMutuallyExclusivePlatformDetection(t *testing.T) {
	darwinResult := IsDarwin()
	linuxResult := IsLinux()
	windowsResult := IsWindows()

	count := 0
	if darwinResult {
		count++
	}
	if linuxResult {
		count++
	}
	if windowsResult {
		count++
	}

	if count != 1 {
		t.Errorf("Expected exactly one platform detection to be true, got darwin=%v linux=%v windows=%v",
			darwinResult, linuxResult, windowsResult)
	}
}

func TestSupportsWithEmptyAndInvalid(t *testing.T) {
	invalidIDs := []ID{
		"",
		"freebsd",
		"netbsd",
		"openbsd",
		"solaris",
		"aix",
		"DARWIN",
		"Darwin",
		"LINUX",
		"WINDOWS",
		" darwin",
		"darwin ",
	}

	for _, id := range invalidIDs {
		t.Run(string(id), func(t *testing.T) {
			if Supports(id) {
				t.Errorf("Supports(%q) = true, expected false", id)
			}
		})
	}
}

func TestExecutableExtensionConsistency(t *testing.T) {
	ext := ExecutableExtension()

	if IsWindows() {
		if ext != ".exe" {
			t.Errorf("ExecutableExtension() = %q on Windows, expected .exe", ext)
		}
	} else {
		if ext != "" {
			t.Errorf("ExecutableExtension() = %q on Unix, expected empty string", ext)
		}
	}

	for i := 0; i < 3; i++ {
		if ExecutableExtension() != ext {
			t.Error("ExecutableExtension() is not consistent across calls")
		}
	}
}

func TestPathSeparatorConsistency(t *testing.T) {
	sep := PathSeparator()

	if IsWindows() {
		if sep != ";" {
			t.Errorf("PathSeparator() = %q on Windows, expected ;", sep)
		}
	} else {
		if sep != ":" {
			t.Errorf("PathSeparator() = %q on Unix, expected :", sep)
		}
	}

	for i := 0; i < 3; i++ {
		if PathSeparator() != sep {
			t.Error("PathSeparator() is not consistent across calls")
		}
	}
}

func TestHomeDirEnvConsistency(t *testing.T) {
	env := HomeDirEnv()

	if IsWindows() {
		if env != "USERPROFILE" {
			t.Errorf("HomeDirEnv() = %q on Windows, expected USERPROFILE", env)
		}
	} else {
		if env != "HOME" {
			t.Errorf("HomeDirEnv() = %q on Unix, expected HOME", env)
		}
	}

	if os.Getenv(env) == "" {
		t.Logf("Warning: %s environment variable is empty", env)
	}
}

func TestTempDirExists(t *testing.T) {
	dir := TempDir()

	if dir == "" {
		t.Error("TempDir() returned empty string")
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Logf("TempDir() = %q does not exist (may be expected in some environments)", dir)
	}
}

func TestCurrentReturnsSingleton(t *testing.T) {
	p1 := Current()
	p2 := Current()
	p3 := Current()

	if p1 != p2 || p2 != p3 {
		t.Error("Current() does not return the same singleton instance")
	}

	if p1 == nil {
		t.Fatal("Current() returned nil")
	}
}

func TestDirectoryPathsAreAbsolute(t *testing.T) {
	plat := Current()

	paths := map[string]string{
		"DataDir":   plat.GetDataDir(),
		"ConfigDir": plat.GetConfigDir(),
		"CacheDir":  plat.GetCacheDir(),
		"LogDir":    plat.GetLogDir(),
	}

	for name, path := range paths {
		t.Run(name, func(t *testing.T) {
			if !filepath.IsAbs(path) {
				t.Errorf("%s = %q is not an absolute path", name, path)
			}
		})
	}
}

func TestDialogResultValues(t *testing.T) {
	if DialogResultCancel != 0 {
		t.Errorf("DialogResultCancel = %d, expected 0", DialogResultCancel)
	}
	if DialogResultUpdate != 1 {
		t.Errorf("DialogResultUpdate = %d, expected 1", DialogResultUpdate)
	}
	if DialogResultRemindLater != 2 {
		t.Errorf("DialogResultRemindLater = %d, expected 2", DialogResultRemindLater)
	}
	if DialogResultViewDetails != 3 {
		t.Errorf("DialogResultViewDetails = %d, expected 3", DialogResultViewDetails)
	}

	if DialogResultCancel >= DialogResultUpdate {
		t.Error("DialogResultCancel should be less than DialogResultUpdate")
	}
	if DialogResultUpdate >= DialogResultRemindLater {
		t.Error("DialogResultUpdate should be less than DialogResultRemindLater")
	}
	if DialogResultRemindLater >= DialogResultViewDetails {
		t.Error("DialogResultRemindLater should be less than DialogResultViewDetails")
	}
}

func TestGetPathDirsNotEmpty(t *testing.T) {
	plat := Current()
	dirs := plat.GetPathDirs()

	if len(dirs) == 0 {
		t.Log("PATH appears to be empty")
		return
	}

	hasNonEmpty := false
	for _, dir := range dirs {
		if dir != "" {
			hasNonEmpty = true
			break
		}
	}

	if !hasNonEmpty {
		t.Log("All PATH directories are empty strings")
	}
}

func TestFindExecutableError(t *testing.T) {
	plat := Current()

	_, err := plat.FindExecutable("this-executable-definitely-does-not-exist-xyz123")
	if err == nil {
		t.Error("FindExecutable should return error for non-existent executable")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error message should contain 'not found', got: %v", err)
	}
}

func TestFindExecutablesError(t *testing.T) {
	plat := Current()

	paths, err := plat.FindExecutables("this-executable-definitely-does-not-exist-xyz123")
	if err == nil {
		t.Error("FindExecutables should return error for non-existent executable")
	}

	if len(paths) != 0 {
		t.Errorf("FindExecutables should return empty slice for non-existent executable, got %d paths", len(paths))
	}
}

func TestGetShellDefaultFallback(t *testing.T) {
	_ = Current()

	originalShell := os.Getenv("SHELL")
	os.Unsetenv("SHELL")
	defer os.Setenv("SHELL", originalShell)

	current = nil
	plat := Current()
	shell := plat.GetShell()

	if shell == "" {
		t.Error("GetShell() should return default when SHELL is unset")
	}

	if IsDarwin() {
		if shell != "/bin/zsh" {
			t.Errorf("GetShell() on macOS with unset SHELL = %q, expected /bin/zsh", shell)
		}
	}

	current = nil
}

func TestFindExecutablesMultipleResults(t *testing.T) {
	if IsWindows() {
		t.Skip("Test designed for Unix systems")
	}

	plat := Current()

	paths, err := plat.FindExecutables("ls")
	if err != nil {
		t.Logf("FindExecutables(ls) error: %v", err)
		return
	}

	if len(paths) == 0 {
		t.Log("FindExecutables(ls) returned no paths")
		return
	}

	for _, p := range paths {
		if !filepath.IsAbs(p) {
			t.Errorf("FindExecutables returned non-absolute path: %q", p)
		}
		if !strings.HasSuffix(p, "ls") {
			t.Errorf("FindExecutables path %q doesn't end with 'ls'", p)
		}
	}
}

func TestPlatformPathDirsContainsStandardDirs(t *testing.T) {
	if IsWindows() {
		t.Skip("Test designed for Unix systems")
	}

	plat := Current()
	dirs := plat.GetPathDirs()

	standardDirs := []string{"/usr/bin", "/bin"}
	for _, stdDir := range standardDirs {
		found := false
		for _, dir := range dirs {
			if dir == stdDir {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Standard directory %q not found in PATH (may be expected)", stdDir)
		}
	}
}

func TestCurrentArchValues(t *testing.T) {
	arch := CurrentArch()

	validArches := []string{"amd64", "arm64", "386", "arm"}
	found := false
	for _, valid := range validArches {
		if arch == valid {
			found = true
			break
		}
	}

	if !found {
		t.Logf("CurrentArch() = %q (unusual but may be valid)", arch)
	}
}

func TestIPCSocketPathFormat(t *testing.T) {
	plat := Current()
	path := plat.GetIPCSocketPath()

	if IsWindows() {
		if !strings.HasPrefix(path, `\\.\pipe\`) {
			t.Errorf("Windows IPC path should start with \\\\.\\pipe\\, got %q", path)
		}
	} else {
		if !strings.Contains(path, "agentmgr") {
			t.Errorf("Unix IPC path should contain 'agentmgr', got %q", path)
		}
		if !strings.HasSuffix(path, ".sock") {
			t.Errorf("Unix IPC path should end with .sock, got %q", path)
		}
	}
}

func TestDirectoryPathsContainAppName(t *testing.T) {
	plat := Current()

	paths := map[string]string{
		"DataDir":   plat.GetDataDir(),
		"ConfigDir": plat.GetConfigDir(),
		"CacheDir":  plat.GetCacheDir(),
		"LogDir":    plat.GetLogDir(),
	}

	for name, path := range paths {
		lowerPath := strings.ToLower(path)
		if !strings.Contains(lowerPath, "agentm") {
			t.Errorf("%s = %q should contain 'agentm' (case-insensitive)", name, path)
		}
	}
}

func TestPlatformNameHumanReadable(t *testing.T) {
	plat := Current()
	name := plat.Name()

	if len(name) < 3 {
		t.Errorf("Platform name %q is too short", name)
	}

	validNames := []string{"macOS", "Linux", "Windows"}
	found := false
	for _, valid := range validNames {
		if name == valid {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Platform name %q is not a recognized platform name", name)
	}
}

func TestShowChangelogDialogDoesNotPanic(t *testing.T) {
	// Skip in CI or headless environments - this test shows a real dialog
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping interactive dialog test in CI environment")
	}

	plat := Current()

	result := plat.ShowChangelogDialog("TestAgent", "1.0.0", "2.0.0", "- Bug fixes\n- New features")

	if result != DialogResultCancel && result != DialogResultUpdate &&
		result != DialogResultRemindLater && result != DialogResultViewDetails {
		t.Errorf("ShowChangelogDialog returned invalid result: %d", result)
	}
}

func TestShowNotificationDoesNotPanic(t *testing.T) {
	// Skip in CI - requires display/notification system
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping notification test in CI environment")
	}

	plat := Current()

	_ = plat.ShowNotification("Test Title", "Test Message")
}

func TestDarwinSpecificPaths(t *testing.T) {
	if !IsDarwin() {
		t.Skip("Test only for macOS")
	}

	plat := Current()

	dataDir := plat.GetDataDir()
	if !strings.Contains(dataDir, "Library") {
		t.Errorf("macOS data dir should contain Library, got %q", dataDir)
	}
	if !strings.Contains(dataDir, "Application Support") {
		t.Errorf("macOS data dir should contain 'Application Support', got %q", dataDir)
	}

	configDir := plat.GetConfigDir()
	if !strings.Contains(configDir, "Library") {
		t.Errorf("macOS config dir should contain Library, got %q", configDir)
	}
	if !strings.Contains(configDir, "Preferences") {
		t.Errorf("macOS config dir should contain 'Preferences', got %q", configDir)
	}

	cacheDir := plat.GetCacheDir()
	if !strings.Contains(cacheDir, "Library") {
		t.Errorf("macOS cache dir should contain Library, got %q", cacheDir)
	}
	if !strings.Contains(cacheDir, "Caches") {
		t.Errorf("macOS cache dir should contain 'Caches', got %q", cacheDir)
	}

	logDir := plat.GetLogDir()
	if !strings.Contains(logDir, "Library") {
		t.Errorf("macOS log dir should contain Library, got %q", logDir)
	}
	if !strings.Contains(logDir, "Logs") {
		t.Errorf("macOS log dir should contain 'Logs', got %q", logDir)
	}
}

func TestContextCancellation(t *testing.T) {
	plat := Current()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := plat.IsAutoStartEnabled(ctx)
	if err != nil {
		t.Logf("IsAutoStartEnabled with canceled context returned error (expected): %v", err)
	}
}
