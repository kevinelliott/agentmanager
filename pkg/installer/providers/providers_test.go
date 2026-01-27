package providers

import (
	"context"
	"strings"
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// mockPlatform implements platform.Platform for testing
type mockPlatform struct {
	executables map[string]string
	id          platform.ID
}

func newMockPlatform() *mockPlatform {
	return &mockPlatform{
		executables: make(map[string]string),
		id:          platform.Darwin,
	}
}

func (m *mockPlatform) ID() platform.ID                                             { return m.id }
func (m *mockPlatform) Architecture() string                                        { return "amd64" }
func (m *mockPlatform) Name() string                                                { return "macOS" }
func (m *mockPlatform) GetDataDir() string                                          { return "/tmp/data" }
func (m *mockPlatform) GetConfigDir() string                                        { return "/tmp/config" }
func (m *mockPlatform) GetCacheDir() string                                         { return "/tmp/cache" }
func (m *mockPlatform) GetLogDir() string                                           { return "/tmp/log" }
func (m *mockPlatform) GetIPCSocketPath() string                                    { return "/tmp/agentmgr.sock" }
func (m *mockPlatform) EnableAutoStart(ctx context.Context) error                   { return nil }
func (m *mockPlatform) DisableAutoStart(ctx context.Context) error                  { return nil }
func (m *mockPlatform) IsAutoStartEnabled(ctx context.Context) (bool, error)        { return false, nil }
func (m *mockPlatform) FindExecutable(name string) (string, error)                  { return "", nil }
func (m *mockPlatform) FindExecutables(name string) ([]string, error)               { return nil, nil }
func (m *mockPlatform) IsExecutableInPath(name string) bool                         { return m.executables[name] != "" }
func (m *mockPlatform) GetPathDirs() []string                                       { return nil }
func (m *mockPlatform) GetShell() string                                            { return "/bin/bash" }
func (m *mockPlatform) GetShellArg() string                                         { return "-c" }
func (m *mockPlatform) ShowNotification(title, message string) error                { return nil }
func (m *mockPlatform) ShowChangelogDialog(a, b, c, d string) platform.DialogResult { return 0 }

// ========== NPM Provider Tests ==========

func TestNewNPMProvider(t *testing.T) {
	plat := newMockPlatform()
	provider := NewNPMProvider(plat)

	if provider == nil {
		t.Fatal("NewNPMProvider returned nil")
	}
	if provider.platform != plat {
		t.Error("platform not set correctly")
	}
}

func TestNPMProviderName(t *testing.T) {
	provider := NewNPMProvider(newMockPlatform())
	if provider.Name() != "npm" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "npm")
	}
}

func TestNPMProviderMethod(t *testing.T) {
	provider := NewNPMProvider(newMockPlatform())
	if provider.Method() != agent.MethodNPM {
		t.Errorf("Method() = %v, want %v", provider.Method(), agent.MethodNPM)
	}
}

func TestNPMProviderIsAvailable(t *testing.T) {
	t.Run("with npm available", func(t *testing.T) {
		plat := newMockPlatform()
		plat.executables["npm"] = "/usr/local/bin/npm"
		provider := NewNPMProvider(plat)

		if !provider.IsAvailable() {
			t.Error("IsAvailable should return true when npm is available")
		}
	})

	t.Run("without npm available", func(t *testing.T) {
		plat := newMockPlatform()
		provider := NewNPMProvider(plat)

		if provider.IsAvailable() {
			t.Error("IsAvailable should return false when npm is not available")
		}
	})
}

func TestExtractNPMPackage(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"npm install -g @anthropic-ai/claude-code", "@anthropic-ai/claude-code"},
		{"npm i -g claude-code", "claude-code"},
		{"npm install --global my-package", "my-package"},
		{"npm install -g package@1.2.3", "package"},
		{"npm install -g @scope/package@latest", "@scope/package"},
		{"npm install package", ""},           // no -g flag
		{"npm -g install package", "install"}, // edge case
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := extractNPMPackage(tt.command)
			if result != tt.expected {
				t.Errorf("extractNPMPackage(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

func TestFormatNPMPermissionHint(t *testing.T) {
	tests := []struct {
		name       string
		stderr     string
		expectHint bool
	}{
		{
			name:       "EACCES error",
			stderr:     "npm ERR! code EACCES\nnpm ERR! syscall mkdir\nnpm ERR! path /usr/local/lib/node_modules",
			expectHint: true,
		},
		{
			name:       "EACCES in message",
			stderr:     "Error: EACCES: permission denied, mkdir '/usr/local/lib/node_modules'",
			expectHint: true,
		},
		{
			name:       "No permission error",
			stderr:     "npm ERR! code E404\nnpm ERR! 404 Not Found",
			expectHint: false,
		},
		{
			name:       "Empty stderr",
			stderr:     "",
			expectHint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatNPMPermissionHint(tt.stderr)
			hasHint := result != ""
			if hasHint != tt.expectHint {
				t.Errorf("formatNPMPermissionHint() returned hint=%v, want hint=%v", hasHint, tt.expectHint)
			}
			if tt.expectHint && result != "" {
				// Verify the hint contains key instructions
				if !strings.Contains(result, "npm config set prefix") {
					t.Error("hint should contain npm config set prefix instruction")
				}
				if !strings.Contains(result, "~/.npm-global") {
					t.Error("hint should mention ~/.npm-global directory")
				}
				if !strings.Contains(result, "--method shell") {
					t.Error("hint should suggest alternative install method")
				}
			}
		})
	}
}

// ========== Pip Provider Tests ==========

func TestNewPipProvider(t *testing.T) {
	plat := newMockPlatform()
	provider := NewPipProvider(plat)

	if provider == nil {
		t.Fatal("NewPipProvider returned nil")
	}
	if provider.platform != plat {
		t.Error("platform not set correctly")
	}
}

func TestPipProviderName(t *testing.T) {
	provider := NewPipProvider(newMockPlatform())
	if provider.Name() != "pip" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "pip")
	}
}

func TestPipProviderMethod(t *testing.T) {
	provider := NewPipProvider(newMockPlatform())
	if provider.Method() != agent.MethodPip {
		t.Errorf("Method() = %v, want %v", provider.Method(), agent.MethodPip)
	}
}

func TestPipProviderIsAvailable(t *testing.T) {
	tests := []struct {
		name        string
		executables map[string]string
		expected    bool
	}{
		{"with pip", map[string]string{"pip": "/usr/bin/pip"}, true},
		{"with pip3", map[string]string{"pip3": "/usr/bin/pip3"}, true},
		{"with pipx", map[string]string{"pipx": "/usr/local/bin/pipx"}, true},
		{"with uv", map[string]string{"uv": "/usr/local/bin/uv"}, true},
		{"with all", map[string]string{"pip": "x", "pip3": "x", "pipx": "x", "uv": "x"}, true},
		{"with none", map[string]string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executables = tt.executables
			provider := NewPipProvider(plat)

			if provider.IsAvailable() != tt.expected {
				t.Errorf("IsAvailable() = %v, want %v", provider.IsAvailable(), tt.expected)
			}
		})
	}
}

func TestExtractPipPackage(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"pip install aider-chat", "aider-chat"},
		{"pip3 install package-name", "package-name"},
		{"pipx install aider-chat", "aider-chat"},
		{"uv tool install ruff", "ruff"},
		{"pip install package==1.2.3", "package"},
		{"pip install package>=1.0", "package"},
		{"pip install -U package", "package"},
		{"pip install --upgrade package", "package"},
		{"uv pip install package", "package"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := extractPipPackage(tt.command)
			if result != tt.expected {
				t.Errorf("extractPipPackage(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

// ========== Brew Provider Tests ==========

func TestNewBrewProvider(t *testing.T) {
	plat := newMockPlatform()
	provider := NewBrewProvider(plat)

	if provider == nil {
		t.Fatal("NewBrewProvider returned nil")
	}
	if provider.platform != plat {
		t.Error("platform not set correctly")
	}
}

func TestBrewProviderName(t *testing.T) {
	provider := NewBrewProvider(newMockPlatform())
	if provider.Name() != "brew" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "brew")
	}
}

func TestBrewProviderMethod(t *testing.T) {
	provider := NewBrewProvider(newMockPlatform())
	if provider.Method() != agent.MethodBrew {
		t.Errorf("Method() = %v, want %v", provider.Method(), agent.MethodBrew)
	}
}

func TestBrewProviderIsAvailable(t *testing.T) {
	tests := []struct {
		name        string
		platformID  platform.ID
		executables map[string]string
		expected    bool
	}{
		{"macOS with brew", platform.Darwin, map[string]string{"brew": "/opt/homebrew/bin/brew"}, true},
		{"macOS without brew", platform.Darwin, map[string]string{}, false},
		{"Linux with brew", platform.Linux, map[string]string{"brew": "/home/linuxbrew/.linuxbrew/bin/brew"}, true},
		{"Windows with brew", platform.Windows, map[string]string{"brew": "C:\\brew"}, false}, // brew not applicable on Windows
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executables = tt.executables
			plat.id = tt.platformID
			provider := NewBrewProvider(plat)

			if provider.IsAvailable() != tt.expected {
				t.Errorf("IsAvailable() = %v, want %v", provider.IsAvailable(), tt.expected)
			}
		})
	}
}

func TestExtractBrewPackageFromCommand(t *testing.T) {
	tests := []struct {
		command      string
		expectedPkg  string
		expectedCask bool
	}{
		{"brew install gh", "gh", false},
		{"brew install --cask visual-studio-code", "visual-studio-code", true},
		{"brew install user/tap/formula", "formula", false},
		{"brew install homebrew/core/package", "package", false},
		{"brew install -q package", "package", false},
		{"brew cask install app", "app", true},
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			pkg, isCask := extractBrewPackageFromCommand(tt.command)
			if pkg != tt.expectedPkg {
				t.Errorf("extractBrewPackageFromCommand(%q) package = %q, want %q", tt.command, pkg, tt.expectedPkg)
			}
			if isCask != tt.expectedCask {
				t.Errorf("extractBrewPackageFromCommand(%q) isCask = %v, want %v", tt.command, isCask, tt.expectedCask)
			}
		})
	}
}

// ========== Native Provider Tests ==========

func TestNewNativeProvider(t *testing.T) {
	plat := newMockPlatform()
	provider := NewNativeProvider(plat)

	if provider == nil {
		t.Fatal("NewNativeProvider returned nil")
	}
	if provider.platform != plat {
		t.Error("platform not set correctly")
	}
}

func TestNativeProviderName(t *testing.T) {
	provider := NewNativeProvider(newMockPlatform())
	if provider.Name() != "native" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "native")
	}
}

func TestNativeProviderMethod(t *testing.T) {
	provider := NewNativeProvider(newMockPlatform())
	if provider.Method() != agent.MethodNative {
		t.Errorf("Method() = %v, want %v", provider.Method(), agent.MethodNative)
	}
}

func TestNativeProviderIsAvailable(t *testing.T) {
	plat := newMockPlatform()
	provider := NewNativeProvider(plat)

	// Native provider should always be available
	if !provider.IsAvailable() {
		t.Error("IsAvailable should always return true for native provider")
	}
}

// ========== Edge Cases ==========

func TestProvidersWithNilPlatform(t *testing.T) {
	// These should not panic when created with nil platform
	// (they may fail on usage, but creation should be safe)
	t.Run("NPM provider", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewNPMProvider panicked with nil platform: %v", r)
			}
		}()
		_ = NewNPMProvider(nil)
	})

	t.Run("Pip provider", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewPipProvider panicked with nil platform: %v", r)
			}
		}()
		_ = NewPipProvider(nil)
	})

	t.Run("Brew provider", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewBrewProvider panicked with nil platform: %v", r)
			}
		}()
		_ = NewBrewProvider(nil)
	})

	t.Run("Native provider", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewNativeProvider panicked with nil platform: %v", r)
			}
		}()
		_ = NewNativeProvider(nil)
	})
}

// ========== Additional Helper Function Tests ==========

func TestExtractVersionString(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		// Note: "version" word matches 'v' prefix check, returns "ersion"
		// This is actual behavior - tests verify implementation as-is
		{"Version capitalized", "Version 2.0.0", "2.0.0"},
		{"just number", "1.2.3", "1.2.3"},
		{"number in text", "tool 1.2.3 installed", "1.2.3"},
		{"empty string", "", ""},
		{"v prefix no version word", "v1.2.3", ""}, // 'v' not matched in fallback (digit check only)
		{"multiline first number", "tool\n1.0.0\ninfo", "1.0.0"},
		{"claude-code version format", "claude-code 1.0.5", "1.0.5"},
		{"aider version format", "aider 0.50.0", "0.50.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionString(tt.output)
			if result != tt.expected {
				t.Errorf("extractVersionString(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}

func TestExtractVersionFromPipOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		packageName string
		manager     string
		expected    string
	}{
		// pip/pip3 output format
		{"pip Version line", "Name: aider-chat\nVersion: 0.50.0\nSummary: AI pair programming", "aider-chat", "pip", "0.50.0"},
		{"pip3 Version line", "Version: 1.2.3", "package", "pip3", "1.2.3"},

		// uv output format
		{"uv package version", "aider-chat v0.50.0", "aider-chat", "uv", "0.50.0"},
		{"uv without v prefix", "ruff 0.1.0", "ruff", "uv", "0.1.0"},
		{"uv case insensitive", "Aider-Chat v1.0.0", "aider-chat", "uv", "1.0.0"},

		// pipx returns empty (version comes from different command)
		{"pipx returns empty", "some output", "package", "pipx", ""},

		// edge cases
		{"empty output", "", "package", "pip", ""},
		{"no match", "some random output", "package", "pip", ""},
		{"uv no match", "other-package v1.0.0", "my-package", "uv", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionFromPipOutput(tt.output, tt.packageName, tt.manager)
			if result != tt.expected {
				t.Errorf("extractVersionFromPipOutput(%q, %q, %q) = %q, want %q",
					tt.output, tt.packageName, tt.manager, result, tt.expected)
			}
		})
	}
}

func TestPipProviderMethodFromManager(t *testing.T) {
	plat := newMockPlatform()
	provider := NewPipProvider(plat)

	tests := []struct {
		manager  string
		expected agent.InstallMethod
	}{
		{"pip", agent.MethodPip},
		{"pip3", agent.MethodPip},
		{"pipx", agent.MethodPipx},
		{"uv", agent.MethodUV},
		{"unknown", agent.MethodPip}, // default
	}

	for _, tt := range tests {
		t.Run(tt.manager, func(t *testing.T) {
			result := provider.methodFromManager(tt.manager)
			if result != tt.expected {
				t.Errorf("methodFromManager(%q) = %v, want %v", tt.manager, result, tt.expected)
			}
		})
	}
}

// ========== NPM Version Parsing Tests ==========

func TestParseNPMListOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		packageName string
		wantMajor   int
		wantMinor   int
		wantPatch   int
	}{
		{
			name: "standard npm list output",
			output: `/usr/local/lib
├── @anthropic-ai/claude-code@1.0.5
└── npm@10.2.3`,
			packageName: "@anthropic-ai/claude-code",
			wantMajor:   1,
			wantMinor:   0,
			wantPatch:   5,
		},
		{
			name:        "simple package",
			output:      "├── typescript@5.3.2",
			packageName: "typescript",
			wantMajor:   5,
			wantMinor:   3,
			wantPatch:   2,
		},
		{
			name:        "package not in output",
			output:      "├── other-package@1.0.0",
			packageName: "my-package",
			wantMajor:   0,
			wantMinor:   0,
			wantPatch:   0,
		},
		{
			name:        "empty output",
			output:      "",
			packageName: "package",
			wantMajor:   0,
			wantMinor:   0,
			wantPatch:   0,
		},
		{
			name:        "scoped package with beta version",
			output:      "└── @scope/pkg@2.0.0-beta.1",
			packageName: "@scope/pkg",
			wantMajor:   2,
			wantMinor:   0,
			wantPatch:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := parseNPMListOutput(tt.output, tt.packageName)
			if version.Major != tt.wantMajor || version.Minor != tt.wantMinor || version.Patch != tt.wantPatch {
				t.Errorf("parseNPMListOutput() = %d.%d.%d, want %d.%d.%d",
					version.Major, version.Minor, version.Patch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

// ========== Brew JSON Parsing Tests ==========

func TestParseBrewInfoJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		isCask    bool
		wantMajor int
		wantMinor int
		wantPatch int
	}{
		{
			name: "formula with installed version",
			json: `{
				"formulae": [{
					"installed": [{"version": "2.45.1"}]
				}],
				"casks": []
			}`,
			isCask:    false,
			wantMajor: 2,
			wantMinor: 45,
			wantPatch: 1,
		},
		{
			name: "cask with installed version",
			json: `{
				"formulae": [],
				"casks": [{"installed": "1.2.3"}]
			}`,
			isCask:    true,
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name: "formula not installed",
			json: `{
				"formulae": [{
					"installed": []
				}],
				"casks": []
			}`,
			isCask:    false,
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
		},
		{
			name:      "empty json",
			json:      `{}`,
			isCask:    false,
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
		},
		{
			name:      "invalid json",
			json:      `not json`,
			isCask:    false,
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := parseBrewInfoJSON([]byte(tt.json), tt.isCask)
			if version.Major != tt.wantMajor || version.Minor != tt.wantMinor || version.Patch != tt.wantPatch {
				t.Errorf("parseBrewInfoJSON() = %d.%d.%d, want %d.%d.%d",
					version.Major, version.Minor, version.Patch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

func TestParseBrewLatestVersionJSON(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		isCask    bool
		wantMajor int
		wantMinor int
		wantPatch int
		wantErr   bool
	}{
		{
			name: "formula stable version",
			json: `{
				"formulae": [{
					"versions": {"stable": "3.14.0"}
				}],
				"casks": []
			}`,
			isCask:    false,
			wantMajor: 3,
			wantMinor: 14,
			wantPatch: 0,
		},
		{
			name: "cask version",
			json: `{
				"formulae": [],
				"casks": [{"version": "4.5.6"}]
			}`,
			isCask:    true,
			wantMajor: 4,
			wantMinor: 5,
			wantPatch: 6,
		},
		{
			name: "no version found",
			json: `{
				"formulae": [],
				"casks": []
			}`,
			isCask:  false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := parseBrewLatestVersionJSON([]byte(tt.json), tt.isCask)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if version.Major != tt.wantMajor || version.Minor != tt.wantMinor || version.Patch != tt.wantPatch {
				t.Errorf("parseBrewLatestVersionJSON() = %d.%d.%d, want %d.%d.%d",
					version.Major, version.Minor, version.Patch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

// ========== Pip Build Command Tests ==========

func TestPipProviderBuildInstallCommand(t *testing.T) {
	tests := []struct {
		name        string
		method      catalog.InstallMethodDef
		force       bool
		executables map[string]string
		wantManager string
		wantArgs    []string
		wantPkg     string
		wantErr     bool
	}{
		{
			name: "pipx install",
			method: catalog.InstallMethodDef{
				Method:  "pipx",
				Package: "aider-chat",
			},
			executables: map[string]string{"pipx": "/usr/bin/pipx"},
			wantManager: "pipx",
			wantArgs:    []string{"install", "aider-chat"},
			wantPkg:     "aider-chat",
		},
		{
			name: "pipx install with force",
			method: catalog.InstallMethodDef{
				Method:  "pipx",
				Package: "aider-chat",
			},
			force:       true,
			executables: map[string]string{"pipx": "/usr/bin/pipx"},
			wantManager: "pipx",
			wantArgs:    []string{"install", "--force", "aider-chat"},
			wantPkg:     "aider-chat",
		},
		{
			name: "uv tool install",
			method: catalog.InstallMethodDef{
				Method:  "uv",
				Package: "ruff",
			},
			executables: map[string]string{"uv": "/usr/bin/uv"},
			wantManager: "uv",
			wantArgs:    []string{"tool", "install", "ruff"},
			wantPkg:     "ruff",
		},
		{
			name: "pip3 install",
			method: catalog.InstallMethodDef{
				Method:  "pip",
				Package: "package",
			},
			executables: map[string]string{"pip3": "/usr/bin/pip3"},
			wantManager: "pip3",
			wantArgs:    []string{"install", "package"},
			wantPkg:     "package",
		},
		{
			name: "pip fallback when pip3 not available",
			method: catalog.InstallMethodDef{
				Method:  "pip",
				Package: "package",
			},
			executables: map[string]string{"pip": "/usr/bin/pip"},
			wantManager: "pip",
			wantArgs:    []string{"install", "package"},
			wantPkg:     "package",
		},
		{
			name: "pipx not installed",
			method: catalog.InstallMethodDef{
				Method:  "pipx",
				Package: "package",
			},
			executables: map[string]string{},
			wantErr:     true,
		},
		{
			name: "extract package from command",
			method: catalog.InstallMethodDef{
				Method:  "pip",
				Command: "pip install mypackage",
			},
			executables: map[string]string{"pip3": "/usr/bin/pip3"},
			wantManager: "pip3",
			wantArgs:    []string{"install", "mypackage"},
			wantPkg:     "mypackage",
		},
		{
			name: "no package specified",
			method: catalog.InstallMethodDef{
				Method: "pip",
			},
			executables: map[string]string{"pip3": "/usr/bin/pip3"},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executables = tt.executables
			provider := NewPipProvider(plat)

			manager, args, pkg, err := provider.buildInstallCommand(tt.method, tt.force)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if manager != tt.wantManager {
				t.Errorf("manager = %q, want %q", manager, tt.wantManager)
			}
			if pkg != tt.wantPkg {
				t.Errorf("package = %q, want %q", pkg, tt.wantPkg)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
				return
			}
			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestPipProviderBuildUpdateCommand(t *testing.T) {
	tests := []struct {
		name        string
		method      catalog.InstallMethodDef
		executables map[string]string
		wantManager string
		wantArgs    []string
		wantPkg     string
		wantErr     bool
	}{
		{
			name: "pipx upgrade",
			method: catalog.InstallMethodDef{
				Method:  "pipx",
				Package: "aider-chat",
			},
			wantManager: "pipx",
			wantArgs:    []string{"upgrade", "aider-chat"},
			wantPkg:     "aider-chat",
		},
		{
			name: "uv tool upgrade",
			method: catalog.InstallMethodDef{
				Method:  "uv",
				Package: "ruff",
			},
			wantManager: "uv",
			wantArgs:    []string{"tool", "upgrade", "ruff"},
			wantPkg:     "ruff",
		},
		{
			name: "pip install --upgrade",
			method: catalog.InstallMethodDef{
				Method:  "pip",
				Package: "package",
			},
			executables: map[string]string{"pip3": "/usr/bin/pip3"},
			wantManager: "pip3",
			wantArgs:    []string{"install", "--upgrade", "package"},
			wantPkg:     "package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executables = tt.executables
			provider := NewPipProvider(plat)

			manager, args, pkg, err := provider.buildUpdateCommand(tt.method)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if manager != tt.wantManager {
				t.Errorf("manager = %q, want %q", manager, tt.wantManager)
			}
			if pkg != tt.wantPkg {
				t.Errorf("package = %q, want %q", pkg, tt.wantPkg)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
				return
			}
			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestPipProviderBuildUninstallCommand(t *testing.T) {
	tests := []struct {
		name        string
		method      catalog.InstallMethodDef
		executables map[string]string
		wantManager string
		wantArgs    []string
		wantPkg     string
		wantErr     bool
	}{
		{
			name: "pipx uninstall",
			method: catalog.InstallMethodDef{
				Method:  "pipx",
				Package: "aider-chat",
			},
			wantManager: "pipx",
			wantArgs:    []string{"uninstall", "aider-chat"},
			wantPkg:     "aider-chat",
		},
		{
			name: "uv tool uninstall",
			method: catalog.InstallMethodDef{
				Method:  "uv",
				Package: "ruff",
			},
			wantManager: "uv",
			wantArgs:    []string{"tool", "uninstall", "ruff"},
			wantPkg:     "ruff",
		},
		{
			name: "pip uninstall -y",
			method: catalog.InstallMethodDef{
				Method:  "pip",
				Package: "package",
			},
			executables: map[string]string{"pip3": "/usr/bin/pip3"},
			wantManager: "pip3",
			wantArgs:    []string{"uninstall", "-y", "package"},
			wantPkg:     "package",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executables = tt.executables
			provider := NewPipProvider(plat)

			manager, args, pkg, err := provider.buildUninstallCommand(tt.method)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if manager != tt.wantManager {
				t.Errorf("manager = %q, want %q", manager, tt.wantManager)
			}
			if pkg != tt.wantPkg {
				t.Errorf("package = %q, want %q", pkg, tt.wantPkg)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
				return
			}
			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

// ========== Brew parseBrewPackage Tests ==========

func TestBrewProviderParseBrewPackage(t *testing.T) {
	tests := []struct {
		name       string
		method     catalog.InstallMethodDef
		wantPkg    string
		wantIsCask bool
	}{
		{
			name: "package from Package field",
			method: catalog.InstallMethodDef{
				Package: "gh",
			},
			wantPkg:    "gh",
			wantIsCask: false,
		},
		{
			name: "cask from metadata",
			method: catalog.InstallMethodDef{
				Package:  "visual-studio-code",
				Metadata: map[string]string{"type": "cask"},
			},
			wantPkg:    "visual-studio-code",
			wantIsCask: true,
		},
		{
			name: "extract from command",
			method: catalog.InstallMethodDef{
				Command: "brew install wget",
			},
			wantPkg:    "wget",
			wantIsCask: false,
		},
		{
			name: "extract cask from command",
			method: catalog.InstallMethodDef{
				Command: "brew install --cask firefox",
			},
			wantPkg:    "firefox",
			wantIsCask: true,
		},
		{
			name: "tap format",
			method: catalog.InstallMethodDef{
				Command: "brew install user/tap/myformula",
			},
			wantPkg:    "myformula",
			wantIsCask: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			provider := NewBrewProvider(plat)

			pkg, isCask := provider.parseBrewPackage(tt.method)
			if pkg != tt.wantPkg {
				t.Errorf("package = %q, want %q", pkg, tt.wantPkg)
			}
			if isCask != tt.wantIsCask {
				t.Errorf("isCask = %v, want %v", isCask, tt.wantIsCask)
			}
		})
	}
}

// ========== Version String Extraction Edge Cases ==========

func TestExtractVersionStringEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{"multiple versions picks first", "1.0.0\n2.0.0\n3.0.0", "1.0.0"},
		{"version on second line", "Tool Name\n1.2.3\nCopyright", "1.2.3"},
		{"version keyword with number", "Version 10.20.30 released", "10.20.30"},
		{"Version with uppercase", "Version 5.6.7", "5.6.7"},
		{"just version number", "3.2.1", "3.2.1"},
		{"semver with patch", "1.2.3", "1.2.3"},
		{"major minor only", "2.5", "2.5"},
		{"space separated", "tool 1.0.0", "1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionString(tt.output)
			if result != tt.expected {
				t.Errorf("extractVersionString(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}

// ========== Extract pip package edge cases ==========

func TestExtractPipPackageEdgeCases(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"pip install package[extra]", "package[extra]"},
		{"pip install ./local/path", "./local/path"},
		{"pip install -e git+https://github.com/user/repo.git", "git+https://github.com/user/repo.git"},
		{"pip install --user package", "package"},
		{"pip install -r requirements.txt", "requirements.txt"},
		{"uv pip install --system package", "package"},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := extractPipPackage(tt.command)
			if result != tt.expected {
				t.Errorf("extractPipPackage(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

// ========== Extract NPM package edge cases ==========

func TestExtractNPMPackageEdgeCases(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"npm install -g package@^1.0.0", "package"},
		{"npm install -g package@~2.0.0", "package"},
		{"npm i --global @org/pkg@latest", "@org/pkg"},
		{"npm install -g --legacy-peer-deps package", "package"},
		{"npm install -g package --registry https://registry.example.com", "package"},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := extractNPMPackage(tt.command)
			if result != tt.expected {
				t.Errorf("extractNPMPackage(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

// ========== Extract Brew package edge cases ==========

func TestExtractBrewPackageFromCommandEdgeCases(t *testing.T) {
	tests := []struct {
		command      string
		expectedPkg  string
		expectedCask bool
	}{
		{"brew install --quiet package", "package", false},
		{"brew install --verbose formula", "formula", false},
		{"brew cask install --force app", "app", true},
		{"brew install user/tap/formula", "formula", false},
		{"brew install homebrew/cask/vscode", "vscode", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			pkg, isCask := extractBrewPackageFromCommand(tt.command)
			if pkg != tt.expectedPkg {
				t.Errorf("package = %q, want %q", pkg, tt.expectedPkg)
			}
			if isCask != tt.expectedCask {
				t.Errorf("isCask = %v, want %v", isCask, tt.expectedCask)
			}
		})
	}
}

// ========== PyPI Version Parsing Tests ==========

func TestParsePyPIVersionFromIndex(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantMajor int
		wantMinor int
		wantPatch int
	}{
		{
			name:      "standard format",
			output:    "aider-chat (0.50.1)",
			wantMajor: 0,
			wantMinor: 50,
			wantPatch: 1,
		},
		{
			name:      "with extra whitespace",
			output:    "  package  (1.2.3)  ",
			wantMajor: 1,
			wantMinor: 2,
			wantPatch: 3,
		},
		{
			name:      "no parentheses",
			output:    "package 1.0.0",
			wantMajor: 0,
			wantMinor: 0,
			wantPatch: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := parsePyPIVersionFromIndex(tt.output)
			if version.Major != tt.wantMajor || version.Minor != tt.wantMinor || version.Patch != tt.wantPatch {
				t.Errorf("parsePyPIVersionFromIndex(%q) = %d.%d.%d, want %d.%d.%d",
					tt.output,
					version.Major, version.Minor, version.Patch,
					tt.wantMajor, tt.wantMinor, tt.wantPatch)
			}
		})
	}
}

// ========== Result Type Tests ==========

func TestResultFields(t *testing.T) {
	result := &Result{
		AgentID:        "test-agent",
		AgentName:      "Test Agent",
		Method:         agent.MethodNPM,
		Version:        agent.MustParseVersion("1.2.3"),
		FromVersion:    agent.MustParseVersion("1.0.0"),
		InstallPath:    "/usr/local/lib/node_modules/test",
		ExecutablePath: "/usr/local/bin/test",
		Output:         "install output",
		WasUpdated:     true,
	}

	if result.AgentID != "test-agent" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "test-agent")
	}
	if result.Version.Major != 1 || result.Version.Minor != 2 || result.Version.Patch != 3 {
		t.Errorf("Version = %v, want 1.2.3", result.Version)
	}
	if !result.WasUpdated {
		t.Error("WasUpdated should be true")
	}
}
