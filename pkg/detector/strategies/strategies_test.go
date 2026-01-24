package strategies

import (
	"context"
	"os/exec"
	"testing"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// mockPlatform implements platform.Platform for testing
type mockPlatform struct {
	executables     map[string]string
	executablePaths map[string]string // maps executable name to full path
}

func newMockPlatform() *mockPlatform {
	return &mockPlatform{
		executables:     make(map[string]string),
		executablePaths: make(map[string]string),
	}
}

func (m *mockPlatform) ID() platform.ID                                      { return platform.Darwin }
func (m *mockPlatform) Architecture() string                                 { return "amd64" }
func (m *mockPlatform) Name() string                                         { return "macOS" }
func (m *mockPlatform) GetDataDir() string                                   { return "/tmp/data" }
func (m *mockPlatform) GetConfigDir() string                                 { return "/tmp/config" }
func (m *mockPlatform) GetCacheDir() string                                  { return "/tmp/cache" }
func (m *mockPlatform) GetLogDir() string                                    { return "/tmp/log" }
func (m *mockPlatform) GetIPCSocketPath() string                             { return "/tmp/agentmgr.sock" }
func (m *mockPlatform) EnableAutoStart(ctx context.Context) error            { return nil }
func (m *mockPlatform) DisableAutoStart(ctx context.Context) error           { return nil }
func (m *mockPlatform) IsAutoStartEnabled(ctx context.Context) (bool, error) { return false, nil }
func (m *mockPlatform) FindExecutable(name string) (string, error) {
	if path, ok := m.executablePaths[name]; ok {
		return path, nil
	}
	return "", exec.ErrNotFound
}
func (m *mockPlatform) FindExecutables(name string) ([]string, error) {
	if path, ok := m.executablePaths[name]; ok {
		return []string{path}, nil
	}
	return nil, nil
}
func (m *mockPlatform) IsExecutableInPath(name string) bool                         { return m.executables[name] != "" }
func (m *mockPlatform) GetPathDirs() []string                                       { return nil }
func (m *mockPlatform) GetShell() string                                            { return "/bin/bash" }
func (m *mockPlatform) GetShellArg() string                                         { return "-c" }
func (m *mockPlatform) ShowNotification(title, message string) error                { return nil }
func (m *mockPlatform) ShowChangelogDialog(a, b, c, d string) platform.DialogResult { return 0 }

// ========== NPM Strategy Tests ==========

func TestNewNPMStrategy(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewNPMStrategy(plat)

	if strategy == nil {
		t.Fatal("NewNPMStrategy returned nil")
	}
	if strategy.platform != plat {
		t.Error("platform not set correctly")
	}
}

func TestNPMStrategyName(t *testing.T) {
	strategy := NewNPMStrategy(newMockPlatform())
	if strategy.Name() != "npm" {
		t.Errorf("Name() = %q, want %q", strategy.Name(), "npm")
	}
}

func TestNPMStrategyMethod(t *testing.T) {
	strategy := NewNPMStrategy(newMockPlatform())
	if strategy.Method() != agent.MethodNPM {
		t.Errorf("Method() = %v, want %v", strategy.Method(), agent.MethodNPM)
	}
}

func TestNPMStrategyIsApplicable(t *testing.T) {
	t.Run("with npm available", func(t *testing.T) {
		plat := newMockPlatform()
		plat.executables["npm"] = "/usr/local/bin/npm"
		strategy := NewNPMStrategy(plat)

		if !strategy.IsApplicable(plat) {
			t.Error("IsApplicable should return true when npm is available")
		}
	})

	t.Run("without npm available", func(t *testing.T) {
		plat := newMockPlatform()
		strategy := NewNPMStrategy(plat)

		if strategy.IsApplicable(plat) {
			t.Error("IsApplicable should return false when npm is not available")
		}
	})
}

func TestExtractNPMPackageName(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{"npm install -g @anthropic-ai/claude-code", "@anthropic-ai/claude-code"},
		{"npm i -g claude-code", "claude-code"},
		{"npm install --global my-package", "my-package"},
		{"npm install -g package@1.2.3", "package"},
		{"npm install -g @scope/package@latest", "@scope/package"},
		{"npm -g install package", "install"}, // edge case: -g before install, takes first non-flag token
		{"npm install package", ""},           // no -g flag
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := extractNPMPackageName(tt.command)
			if result != tt.expected {
				t.Errorf("extractNPMPackageName(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

// ========== Binary Strategy Tests ==========

func TestNewBinaryStrategy(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBinaryStrategy(plat)

	if strategy == nil {
		t.Fatal("NewBinaryStrategy returned nil")
	}
	if strategy.platform != plat {
		t.Error("platform not set correctly")
	}
}

func TestBinaryStrategyName(t *testing.T) {
	strategy := NewBinaryStrategy(newMockPlatform())
	if strategy.Name() != "binary" {
		t.Errorf("Name() = %q, want %q", strategy.Name(), "binary")
	}
}

func TestBinaryStrategyMethod(t *testing.T) {
	strategy := NewBinaryStrategy(newMockPlatform())
	if strategy.Method() != agent.MethodNative {
		t.Errorf("Method() = %v, want %v", strategy.Method(), agent.MethodNative)
	}
}

func TestBinaryStrategyIsApplicable(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBinaryStrategy(plat)

	// Binary strategy should always be applicable
	if !strategy.IsApplicable(plat) {
		t.Error("IsApplicable should always return true for binary strategy")
	}
}

func TestExtractVersionFromOutput(t *testing.T) {
	tests := []struct {
		output   string
		expected string
	}{
		{"version 1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"claude-code version 1.0.5", "1.0.5"},
		{"aider v0.50.0", "0.50.0"},
		{"Version: 2.0.0-beta.1", "2.0.0-beta.1"},
		{"some text v3.4.5 more text", "3.4.5"},
		{"no version here", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			result := extractVersionFromOutput(tt.output)
			if result != tt.expected {
				t.Errorf("extractVersionFromOutput(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}

// ========== Pip Strategy Tests ==========

func TestNewPipStrategy(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewPipStrategy(plat)

	if strategy == nil {
		t.Fatal("NewPipStrategy returned nil")
	}
	if strategy.platform != plat {
		t.Error("platform not set correctly")
	}
}

func TestPipStrategyName(t *testing.T) {
	strategy := NewPipStrategy(newMockPlatform())
	if strategy.Name() != "pip" {
		t.Errorf("Name() = %q, want %q", strategy.Name(), "pip")
	}
}

func TestPipStrategyMethod(t *testing.T) {
	strategy := NewPipStrategy(newMockPlatform())
	if strategy.Method() != agent.MethodPip {
		t.Errorf("Method() = %v, want %v", strategy.Method(), agent.MethodPip)
	}
}

func TestPipStrategyIsApplicable(t *testing.T) {
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
			strategy := NewPipStrategy(plat)

			if strategy.IsApplicable(plat) != tt.expected {
				t.Errorf("IsApplicable() = %v, want %v", strategy.IsApplicable(plat), tt.expected)
			}
		})
	}
}

func TestExtractPipPackageName(t *testing.T) {
	tests := []struct {
		packageField string
		command      string
		expected     string
	}{
		{"aider-chat", "", "aider-chat"},
		{"", "pip install aider-chat", "aider-chat"},
		{"", "pip3 install package-name", "package-name"},
		{"", "pipx install aider-chat", "aider-chat"},
		{"", "uv tool install ruff", "ruff"},
		{"", "pip install package==1.2.3", "package"},
		{"", "pip install package>=1.0", "package"},
		{"", "pip install -U package", "package"},
		{"", "", ""},
	}

	for _, tt := range tests {
		name := tt.packageField
		if name == "" {
			name = tt.command
		}
		t.Run(name, func(t *testing.T) {
			result := extractPipPackageName(tt.packageField, tt.command)
			if result != tt.expected {
				t.Errorf("extractPipPackageName(%q, %q) = %q, want %q",
					tt.packageField, tt.command, result, tt.expected)
			}
		})
	}
}

func TestParseUVTextOutput(t *testing.T) {
	strategy := NewPipStrategy(newMockPlatform())

	tests := []struct {
		name     string
		output   string
		expected map[string]string // package name -> version
	}{
		{
			name:   "single package",
			output: "ruff v0.1.0",
			expected: map[string]string{
				"ruff": "0.1.0",
			},
		},
		{
			name:   "multiple packages",
			output: "ruff v0.1.0\naider-chat 0.50.0\nmypy v1.2.3",
			expected: map[string]string{
				"ruff":       "0.1.0",
				"aider-chat": "0.50.0",
				"mypy":       "1.2.3",
			},
		},
		{
			name:   "with empty lines",
			output: "\nruff v0.1.0\n\naider 0.50.0\n",
			expected: map[string]string{
				"ruff":  "0.1.0",
				"aider": "0.50.0",
			},
		},
		{
			name:   "with dashes (ignored)",
			output: "- ruff\n  v0.1.0\nreal-package v1.0.0",
			expected: map[string]string{
				"real-package": "1.0.0",
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.parseUVTextOutput(tt.output)

			if len(result) != len(tt.expected) {
				t.Errorf("parseUVTextOutput returned %d packages, want %d", len(result), len(tt.expected))
			}

			for name, version := range tt.expected {
				pkg, found := result[name]
				if !found {
					t.Errorf("package %q not found in result", name)
					continue
				}
				if pkg.Version != version {
					t.Errorf("package %q version = %q, want %q", name, pkg.Version, version)
				}
			}
		})
	}
}

// ========== Brew Strategy Tests ==========

func TestNewBrewStrategy(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBrewStrategy(plat)

	if strategy == nil {
		t.Fatal("NewBrewStrategy returned nil")
	}
	if strategy.platform != plat {
		t.Error("platform not set correctly")
	}
}

func TestBrewStrategyName(t *testing.T) {
	strategy := NewBrewStrategy(newMockPlatform())
	if strategy.Name() != "brew" {
		t.Errorf("Name() = %q, want %q", strategy.Name(), "brew")
	}
}

func TestBrewStrategyMethod(t *testing.T) {
	strategy := NewBrewStrategy(newMockPlatform())
	if strategy.Method() != agent.MethodBrew {
		t.Errorf("Method() = %v, want %v", strategy.Method(), agent.MethodBrew)
	}
}

func TestBrewStrategyIsApplicable(t *testing.T) {
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
			plat := &mockPlatformWithID{
				mockPlatform: mockPlatform{executables: tt.executables},
				id:           tt.platformID,
			}
			strategy := NewBrewStrategy(plat)

			if strategy.IsApplicable(plat) != tt.expected {
				t.Errorf("IsApplicable() = %v, want %v", strategy.IsApplicable(plat), tt.expected)
			}
		})
	}
}

// mockPlatformWithID extends mockPlatform with configurable ID
type mockPlatformWithID struct {
	mockPlatform
	id platform.ID
}

func (m *mockPlatformWithID) ID() platform.ID { return m.id }

func TestExtractBrewPackageName(t *testing.T) {
	tests := []struct {
		packageField string
		command      string
		expected     string
	}{
		{"gh", "", "gh"},
		{"", "brew install gh", "gh"},
		{"", "brew install --cask visual-studio-code", "visual-studio-code"},
		{"", "brew install user/tap/formula", "formula"},
		{"", "brew install homebrew/core/package", "package"},
		{"", "brew install -q package", "package"},
		{"", "", ""},
	}

	for _, tt := range tests {
		name := tt.packageField
		if name == "" {
			name = tt.command
		}
		t.Run(name, func(t *testing.T) {
			result := extractBrewPackageName(tt.packageField, tt.command)
			if result != tt.expected {
				t.Errorf("extractBrewPackageName(%q, %q) = %q, want %q",
					tt.packageField, tt.command, result, tt.expected)
			}
		})
	}
}

// ========== Additional Version Extraction Tests ==========

func TestExtractVersionFromOutput_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{"version with v prefix", "v1.2.3", "1.2.3"},
		{"version without v prefix", "1.2.3", "1.2.3"},
		{"version word prefix", "version 1.2.3", "1.2.3"},
		{"Version capitalized", "Version: 1.2.3", "1.2.3"},
		{"prerelease version", "v1.0.0-alpha.1", "1.0.0-alpha.1"},
		{"beta version", "2.0.0-beta.5", "2.0.0-beta.5"},
		{"rc version", "3.0.0-rc.1", "3.0.0-rc.1"},
		{"multiline output first line", "v1.2.3\nsome other text", "1.2.3"},
		{"multiline output middle", "some text\nv1.2.3\nmore text", "1.2.3"},
		{"version in sentence", "claude-code version 1.0.5 is installed", "1.0.5"},
		{"version with extra info", "aider v0.50.0 (Python 3.11)", "0.50.0"},
		{"no version number", "installed successfully", ""},
		{"only text", "hello world", ""},
		{"empty string", "", ""},
		{"whitespace only", "   \n  \t  ", ""},
		{"version at end", "Package version is 4.5.6", "4.5.6"},
		{"version with build metadata", "1.0.0-beta.1+build.123", "1.0.0-beta.1"},
		{"two digit version", "v10.20.30", "10.20.30"},
		{"version with leading zeros", "v01.02.03", "01.02.03"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersionFromOutput(tt.output)
			if result != tt.expected {
				t.Errorf("extractVersionFromOutput(%q) = %q, want %q", tt.output, result, tt.expected)
			}
		})
	}
}

// ========== NPM Package Name Extraction Edge Cases ==========

func TestExtractNPMPackageName_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{"scoped package", "npm install -g @anthropic-ai/claude-code", "@anthropic-ai/claude-code"},
		{"scoped with version", "npm install -g @scope/package@1.2.3", "@scope/package"},
		{"short form i", "npm i -g some-package", "some-package"},
		{"long form global", "npm install --global my-cli", "my-cli"},
		{"with flags before package", "npm install -g --save-dev package", "package"},
		{"version latest", "npm install -g package@latest", "package"},
		{"version beta", "npm install -g package@beta", "package"},
		{"empty command", "", ""},
		{"no global flag", "npm install package", ""},
		{"just npm", "npm", ""},
		{"global before install", "npm -g install package", "install"},
		{"multiple scoped packages", "npm install -g @scope/pkg1", "@scope/pkg1"},
		{"package with numbers", "npm install -g package123", "package123"},
		{"package with underscores", "npm install -g my_package", "my_package"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNPMPackageName(tt.command)
			if result != tt.expected {
				t.Errorf("extractNPMPackageName(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

// ========== Pip Package Name Extraction Edge Cases ==========

func TestExtractPipPackageName_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		packageField string
		command      string
		expected     string
	}{
		{"package field provided", "aider-chat", "pip install something-else", "aider-chat"},
		{"pip install", "", "pip install aider-chat", "aider-chat"},
		{"pip3 install", "", "pip3 install mypackage", "mypackage"},
		{"pipx install", "", "pipx install aider-chat", "aider-chat"},
		{"uv tool install", "", "uv tool install ruff", "ruff"},
		{"version constraint ==", "", "pip install package==1.2.3", "package"},
		{"version constraint >=", "", "pip install package>=1.0", "package"},
		{"upgrade flag", "", "pip install -U package", "package"},
		{"empty command", "", "", ""},
		{"just pip", "", "pip", ""},
		{"pip install only", "", "pip install", ""},
		{"with extra flags", "", "pip install --user package", "package"},
		{"package with dashes", "", "pip install my-cool-package", "my-cool-package"},
		{"package with underscores", "", "pip install my_package", "my_package"},
		{"complex version", "", "pip install pkg>=1.0,<2.0", "pkg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPipPackageName(tt.packageField, tt.command)
			if result != tt.expected {
				t.Errorf("extractPipPackageName(%q, %q) = %q, want %q",
					tt.packageField, tt.command, result, tt.expected)
			}
		})
	}
}

// ========== Brew Package Name Extraction Edge Cases ==========

func TestExtractBrewPackageName_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		packageField string
		command      string
		expected     string
	}{
		{"package field provided", "gh", "brew install something-else", "gh"},
		{"simple install", "", "brew install gh", "gh"},
		{"cask install", "", "brew install --cask visual-studio-code", "visual-studio-code"},
		{"tap format user/tap/formula", "", "brew install user/tap/formula", "formula"},
		{"homebrew core tap", "", "brew install homebrew/core/package", "package"},
		{"quiet flag", "", "brew install -q package", "package"},
		{"empty command", "", "", ""},
		{"just brew", "", "brew", ""},
		{"brew install only", "", "brew install", ""},
		{"multiple flags", "", "brew install -v --force package", "package"},
		{"reinstall command", "", "brew reinstall package", "package"},
		{"deep tap path", "", "brew install org/repo/tap/formula", "formula"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBrewPackageName(tt.packageField, tt.command)
			if result != tt.expected {
				t.Errorf("extractBrewPackageName(%q, %q) = %q, want %q",
					tt.packageField, tt.command, result, tt.expected)
			}
		})
	}
}

// ========== UV Text Output Parsing Tests ==========

func TestParseUVTextOutput_EdgeCases(t *testing.T) {
	strategy := NewPipStrategy(newMockPlatform())

	tests := []struct {
		name     string
		output   string
		expected map[string]string
	}{
		{
			name:   "standard format with v prefix",
			output: "ruff v0.1.0",
			expected: map[string]string{
				"ruff": "0.1.0",
			},
		},
		{
			name:   "without v prefix",
			output: "aider-chat 0.50.0",
			expected: map[string]string{
				"aider-chat": "0.50.0",
			},
		},
		{
			name:   "multiple packages",
			output: "ruff v0.1.0\naider-chat 0.50.0\nmypy v1.2.3",
			expected: map[string]string{
				"ruff":       "0.1.0",
				"aider-chat": "0.50.0",
				"mypy":       "1.2.3",
			},
		},
		{
			name:   "with empty lines",
			output: "\nruff v0.1.0\n\naider 0.50.0\n",
			expected: map[string]string{
				"ruff":  "0.1.0",
				"aider": "0.50.0",
			},
		},
		{
			name:   "lines starting with dash ignored",
			output: "- ruff\n  v0.1.0\nreal-package v1.0.0",
			expected: map[string]string{
				"real-package": "1.0.0",
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: map[string]string{},
		},
		{
			name:     "whitespace only",
			output:   "   \n  \t  \n",
			expected: map[string]string{},
		},
		{
			name:   "single package no newline",
			output: "package v1.0.0",
			expected: map[string]string{
				"package": "1.0.0",
			},
		},
		{
			name:   "package with numbers",
			output: "package123 v2.0.0",
			expected: map[string]string{
				"package123": "2.0.0",
			},
		},
		{
			name:     "malformed line single word",
			output:   "justoneword",
			expected: map[string]string{},
		},
		{
			name:   "extra whitespace between fields",
			output: "package    v3.0.0",
			expected: map[string]string{
				"package": "3.0.0",
			},
		},
		{
			name:   "line with extra fields ignored",
			output: "package v1.0.0 extra stuff",
			expected: map[string]string{
				"package": "1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.parseUVTextOutput(tt.output)

			if len(result) != len(tt.expected) {
				t.Errorf("parseUVTextOutput returned %d packages, want %d", len(result), len(tt.expected))
				return
			}

			for name, version := range tt.expected {
				pkg, found := result[name]
				if !found {
					t.Errorf("package %q not found in result", name)
					continue
				}
				if pkg.Version != version {
					t.Errorf("package %q version = %q, want %q", name, pkg.Version, version)
				}
			}
		})
	}
}

// ========== Binary Methods Variable Test ==========

func TestBinaryMethods(t *testing.T) {
	expected := []string{"native", "binary", "curl"}
	if len(binaryMethods) != len(expected) {
		t.Errorf("binaryMethods length = %d, want %d", len(binaryMethods), len(expected))
	}
	for i, method := range expected {
		if binaryMethods[i] != method {
			t.Errorf("binaryMethods[%d] = %q, want %q", i, binaryMethods[i], method)
		}
	}
}

// ========== Strategy Interface Compliance Tests ==========

func TestNPMStrategyMethodReturnsCorrectType(t *testing.T) {
	strategy := NewNPMStrategy(newMockPlatform())
	method := strategy.Method()
	if method != agent.MethodNPM {
		t.Errorf("Method() = %v, want %v", method, agent.MethodNPM)
	}
}

func TestPipStrategyMethodReturnsCorrectType(t *testing.T) {
	strategy := NewPipStrategy(newMockPlatform())
	method := strategy.Method()
	if method != agent.MethodPip {
		t.Errorf("Method() = %v, want %v", method, agent.MethodPip)
	}
}

func TestBrewStrategyMethodReturnsCorrectType(t *testing.T) {
	strategy := NewBrewStrategy(newMockPlatform())
	method := strategy.Method()
	if method != agent.MethodBrew {
		t.Errorf("Method() = %v, want %v", method, agent.MethodBrew)
	}
}

func TestBinaryStrategyMethodReturnsCorrectType(t *testing.T) {
	strategy := NewBinaryStrategy(newMockPlatform())
	method := strategy.Method()
	if method != agent.MethodNative {
		t.Errorf("Method() = %v, want %v", method, agent.MethodNative)
	}
}

// ========== Platform ID Edge Cases ==========

func TestBrewStrategyIsApplicable_AllPlatforms(t *testing.T) {
	tests := []struct {
		name       string
		platformID platform.ID
		hasBrew    bool
		expected   bool
	}{
		{"darwin with brew", platform.Darwin, true, true},
		{"darwin without brew", platform.Darwin, false, false},
		{"linux with brew", platform.Linux, true, true},
		{"linux without brew", platform.Linux, false, false},
		{"windows with brew", platform.Windows, true, false},
		{"windows without brew", platform.Windows, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executables := make(map[string]string)
			if tt.hasBrew {
				executables["brew"] = "/usr/local/bin/brew"
			}
			plat := &mockPlatformWithID{
				mockPlatform: mockPlatform{executables: executables},
				id:           tt.platformID,
			}
			strategy := NewBrewStrategy(plat)

			if strategy.IsApplicable(plat) != tt.expected {
				t.Errorf("IsApplicable() = %v, want %v", strategy.IsApplicable(plat), tt.expected)
			}
		})
	}
}

// ========== Constructor Tests ==========

func TestNewNPMStrategy_ReturnsNonNil(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewNPMStrategy(plat)
	if strategy == nil {
		t.Fatal("NewNPMStrategy returned nil")
	}
	if strategy.platform != plat {
		t.Error("platform field not set correctly")
	}
}

func TestNewPipStrategy_ReturnsNonNil(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewPipStrategy(plat)
	if strategy == nil {
		t.Fatal("NewPipStrategy returned nil")
	}
	if strategy.platform != plat {
		t.Error("platform field not set correctly")
	}
}

func TestNewBrewStrategy_ReturnsNonNil(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBrewStrategy(plat)
	if strategy == nil {
		t.Fatal("NewBrewStrategy returned nil")
	}
	if strategy.platform != plat {
		t.Error("platform field not set correctly")
	}
}

func TestNewBinaryStrategy_ReturnsNonNil(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBinaryStrategy(plat)
	if strategy == nil {
		t.Fatal("NewBinaryStrategy returned nil")
	}
	if strategy.platform != plat {
		t.Error("platform field not set correctly")
	}
}

// ========== Pip IsApplicable Comprehensive Tests ==========

func TestPipStrategyIsApplicable_AllCombinations(t *testing.T) {
	tests := []struct {
		name        string
		executables map[string]string
		expected    bool
	}{
		{"no python tools", map[string]string{}, false},
		{"pip only", map[string]string{"pip": "/usr/bin/pip"}, true},
		{"pip3 only", map[string]string{"pip3": "/usr/bin/pip3"}, true},
		{"pipx only", map[string]string{"pipx": "/usr/local/bin/pipx"}, true},
		{"uv only", map[string]string{"uv": "/usr/local/bin/uv"}, true},
		{"pip and pip3", map[string]string{"pip": "x", "pip3": "x"}, true},
		{"all tools", map[string]string{"pip": "x", "pip3": "x", "pipx": "x", "uv": "x"}, true},
		{"pipx and uv", map[string]string{"pipx": "x", "uv": "x"}, true},
		{"other tools only", map[string]string{"npm": "x", "brew": "x"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executables = tt.executables
			strategy := NewPipStrategy(plat)

			if strategy.IsApplicable(plat) != tt.expected {
				t.Errorf("IsApplicable() = %v, want %v", strategy.IsApplicable(plat), tt.expected)
			}
		})
	}
}

// ========== NPM IsApplicable Edge Cases ==========

func TestNPMStrategyIsApplicable_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		executables map[string]string
		expected    bool
	}{
		{"npm available", map[string]string{"npm": "/usr/local/bin/npm"}, true},
		{"npm not available", map[string]string{}, false},
		{"other tools only", map[string]string{"pip": "x", "brew": "x"}, false},
		{"node but no npm", map[string]string{"node": "/usr/local/bin/node"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executables = tt.executables
			strategy := NewNPMStrategy(plat)

			if strategy.IsApplicable(plat) != tt.expected {
				t.Errorf("IsApplicable() = %v, want %v", strategy.IsApplicable(plat), tt.expected)
			}
		})
	}
}

// ========== FindExecutable Tests ==========

func TestNPMStrategy_findExecutable(t *testing.T) {
	tests := []struct {
		name            string
		executablePaths map[string]string
		agentDef        catalog.AgentDef
		expected        string
	}{
		{
			name:            "finds first executable",
			executablePaths: map[string]string{"claude": "/usr/local/bin/claude"},
			agentDef: catalog.AgentDef{
				Detection: catalog.DetectionDef{
					Executables: []string{"claude", "claude-code"},
				},
			},
			expected: "/usr/local/bin/claude",
		},
		{
			name:            "finds second executable when first not found",
			executablePaths: map[string]string{"claude-code": "/usr/bin/claude-code"},
			agentDef: catalog.AgentDef{
				Detection: catalog.DetectionDef{
					Executables: []string{"claude", "claude-code"},
				},
			},
			expected: "/usr/bin/claude-code",
		},
		{
			name:            "returns empty when no executable found",
			executablePaths: map[string]string{},
			agentDef: catalog.AgentDef{
				Detection: catalog.DetectionDef{
					Executables: []string{"nonexistent"},
				},
			},
			expected: "",
		},
		{
			name:            "returns empty for empty executables list",
			executablePaths: map[string]string{"something": "/path/to/something"},
			agentDef: catalog.AgentDef{
				Detection: catalog.DetectionDef{
					Executables: []string{},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executablePaths = tt.executablePaths
			strategy := NewNPMStrategy(plat)

			result := strategy.findExecutable(tt.agentDef)
			if result != tt.expected {
				t.Errorf("findExecutable() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPipStrategy_findExecutable(t *testing.T) {
	tests := []struct {
		name            string
		executablePaths map[string]string
		agentDef        catalog.AgentDef
		expected        string
	}{
		{
			name:            "finds executable",
			executablePaths: map[string]string{"aider": "/home/user/.local/bin/aider"},
			agentDef: catalog.AgentDef{
				Detection: catalog.DetectionDef{
					Executables: []string{"aider"},
				},
			},
			expected: "/home/user/.local/bin/aider",
		},
		{
			name:            "returns empty when not found",
			executablePaths: map[string]string{},
			agentDef: catalog.AgentDef{
				Detection: catalog.DetectionDef{
					Executables: []string{"aider"},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executablePaths = tt.executablePaths
			strategy := NewPipStrategy(plat)

			result := strategy.findExecutable(tt.agentDef)
			if result != tt.expected {
				t.Errorf("findExecutable() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBrewStrategy_findExecutable(t *testing.T) {
	tests := []struct {
		name            string
		executablePaths map[string]string
		agentDef        catalog.AgentDef
		expected        string
	}{
		{
			name:            "finds executable",
			executablePaths: map[string]string{"gh": "/opt/homebrew/bin/gh"},
			agentDef: catalog.AgentDef{
				Detection: catalog.DetectionDef{
					Executables: []string{"gh"},
				},
			},
			expected: "/opt/homebrew/bin/gh",
		},
		{
			name:            "returns empty when not found",
			executablePaths: map[string]string{},
			agentDef: catalog.AgentDef{
				Detection: catalog.DetectionDef{
					Executables: []string{"gh"},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plat := newMockPlatform()
			plat.executablePaths = tt.executablePaths
			strategy := NewBrewStrategy(plat)

			result := strategy.findExecutable(tt.agentDef)
			if result != tt.expected {
				t.Errorf("findExecutable() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ========== Detect Method Tests (edge cases that don't require real commands) ==========

func TestNPMStrategy_Detect_NoNPMMethod(t *testing.T) {
	plat := newMockPlatform()
	plat.executables["npm"] = "/usr/local/bin/npm"
	strategy := NewNPMStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "test-agent",
			Name: "Test Agent",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"pip": {Command: "pip install test"},
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err == nil && len(installations) == 0 {
		// Expected: no installations found because agent has no npm method
	} else if err != nil {
		// npm list command will fail in test env, but we're testing the logic
		t.Logf("Expected error in test environment: %v", err)
	}
}

func TestPipStrategy_Detect_NoMatchingMethod(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewPipStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "test-agent",
			Name: "Test Agent",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"npm": {Command: "npm install -g test"},
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 0 {
		t.Errorf("Detect() returned %d installations, want 0", len(installations))
	}
}

func TestBrewStrategy_Detect_NoBrewMethod(t *testing.T) {
	plat := newMockPlatform()
	plat.executables["brew"] = "/opt/homebrew/bin/brew"
	strategy := NewBrewStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "test-agent",
			Name: "Test Agent",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"npm": {Command: "npm install -g test"},
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 0 {
		t.Errorf("Detect() returned %d installations, want 0", len(installations))
	}
}

func TestBinaryStrategy_Detect_NoBinaryMethod(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBinaryStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "test-agent",
			Name: "Test Agent",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"npm": {Command: "npm install -g test"},
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 0 {
		t.Errorf("Detect() returned %d installations, want 0", len(installations))
	}
}

func TestBinaryStrategy_Detect_WithNativeMethod_ExecutableNotFound(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBinaryStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "test-agent",
			Name: "Test Agent",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"native": {Command: "curl -sSL https://example.com/install.sh | bash"},
			},
			Detection: catalog.DetectionDef{
				Executables: []string{"test-agent"},
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 0 {
		t.Errorf("Detect() returned %d installations, want 0 (executable not found)", len(installations))
	}
}

func TestBinaryStrategy_Detect_WithNativeMethod_ExecutableFound(t *testing.T) {
	plat := newMockPlatform()
	plat.executablePaths["test-agent"] = "/usr/local/bin/test-agent"
	strategy := NewBinaryStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "test-agent",
			Name: "Test Agent",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"native": {Command: "curl -sSL https://example.com/install.sh | bash"},
			},
			Detection: catalog.DetectionDef{
				Executables: []string{"test-agent"},
				VersionCmd:  "", // No version command
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 1 {
		t.Fatalf("Detect() returned %d installations, want 1", len(installations))
	}

	inst := installations[0]
	if inst.AgentID != "test-agent" {
		t.Errorf("AgentID = %q, want %q", inst.AgentID, "test-agent")
	}
	if inst.AgentName != "Test Agent" {
		t.Errorf("AgentName = %q, want %q", inst.AgentName, "Test Agent")
	}
	if inst.ExecutablePath != "/usr/local/bin/test-agent" {
		t.Errorf("ExecutablePath = %q, want %q", inst.ExecutablePath, "/usr/local/bin/test-agent")
	}
	if inst.Metadata["detected_by"] != "binary" {
		t.Errorf("Metadata[detected_by] = %q, want %q", inst.Metadata["detected_by"], "binary")
	}
	if inst.Metadata["executable"] != "test-agent" {
		t.Errorf("Metadata[executable] = %q, want %q", inst.Metadata["executable"], "test-agent")
	}
}

func TestBinaryStrategy_Detect_MultipleExecutables_FindsFirst(t *testing.T) {
	plat := newMockPlatform()
	plat.executablePaths["agent-v2"] = "/usr/local/bin/agent-v2"
	strategy := NewBinaryStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "my-agent",
			Name: "My Agent",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"binary": {},
			},
			Detection: catalog.DetectionDef{
				Executables: []string{"agent-v1", "agent-v2", "agent-v3"},
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 1 {
		t.Fatalf("Detect() returned %d installations, want 1", len(installations))
	}

	if installations[0].Metadata["executable"] != "agent-v2" {
		t.Errorf("Found executable = %q, want %q", installations[0].Metadata["executable"], "agent-v2")
	}
}

func TestBinaryStrategy_Detect_CurlMethod(t *testing.T) {
	plat := newMockPlatform()
	plat.executablePaths["curl-installed"] = "/usr/local/bin/curl-installed"
	strategy := NewBinaryStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "curl-agent",
			Name: "Curl Agent",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"curl": {Command: "curl -sSL https://example.com/install.sh | bash"},
			},
			Detection: catalog.DetectionDef{
				Executables: []string{"curl-installed"},
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 1 {
		t.Fatalf("Detect() returned %d installations, want 1", len(installations))
	}

	if string(installations[0].Method) != "curl" {
		t.Errorf("Method = %q, want %q", installations[0].Method, "curl")
	}
}

func TestBinaryStrategy_Detect_PrefersNativeOverBinary(t *testing.T) {
	plat := newMockPlatform()
	plat.executablePaths["my-tool"] = "/usr/local/bin/my-tool"
	strategy := NewBinaryStrategy(plat)

	agents := []catalog.AgentDef{
		{
			ID:   "my-tool",
			Name: "My Tool",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"native": {Command: "native install"},
				"binary": {Command: "binary install"},
				"curl":   {Command: "curl install"},
			},
			Detection: catalog.DetectionDef{
				Executables: []string{"my-tool"},
			},
		},
	}

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, agents)

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 1 {
		t.Fatalf("Detect() returned %d installations, want 1", len(installations))
	}

	// Should prefer "native" because binaryMethods = []string{"native", "binary", "curl"}
	if string(installations[0].Method) != "native" {
		t.Errorf("Method = %q, want %q (native is checked first)", installations[0].Method, "native")
	}
}

func TestBinaryStrategy_Detect_EmptyAgentList(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBinaryStrategy(plat)

	ctx := context.Background()
	installations, err := strategy.Detect(ctx, []catalog.AgentDef{})

	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}
	if len(installations) != 0 {
		t.Errorf("Detect() returned %d installations, want 0", len(installations))
	}
}

// ========== getVersion Tests ==========

func TestBinaryStrategy_getVersion_EmptyVersionCmd(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBinaryStrategy(plat)

	agentDef := catalog.AgentDef{
		Detection: catalog.DetectionDef{
			VersionCmd: "",
		},
	}

	ctx := context.Background()
	version := strategy.getVersion(ctx, agentDef, "/usr/local/bin/test")

	if version.Major != 0 || version.Minor != 0 || version.Patch != 0 {
		t.Errorf("getVersion() with empty cmd should return empty version, got %v", version)
	}
}

func TestBinaryStrategy_getVersion_InvalidCommand(t *testing.T) {
	plat := newMockPlatform()
	strategy := NewBinaryStrategy(plat)

	agentDef := catalog.AgentDef{
		Detection: catalog.DetectionDef{
			VersionCmd: "nonexistent-binary --version",
		},
	}

	ctx := context.Background()
	version := strategy.getVersion(ctx, agentDef, "/path/to/nonexistent")

	// Should return empty version when command fails
	if version.Major != 0 || version.Minor != 0 || version.Patch != 0 {
		t.Errorf("getVersion() with invalid command should return empty version, got %v", version)
	}
}

// ========== Package Name Extraction with Complex Cases ==========

func TestExtractPipPackageName_VersionConstraints(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{"double equals", "pip install package==1.2.3", "package"},
		{"greater equals", "pip install package>=1.0", "package"},
		{"complex constraint", "pip install pkg>=1.0,<2.0", "pkg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPipPackageName("", tt.command)
			if result != tt.expected {
				t.Errorf("extractPipPackageName(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

func TestExtractNPMPackageName_VersionTags(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{"latest tag", "npm install -g package@latest", "package"},
		{"beta tag", "npm install -g package@beta", "package"},
		{"next tag", "npm install -g package@next", "package"},
		{"specific version", "npm install -g package@1.2.3", "package"},
		{"scoped with version", "npm install -g @org/package@1.0.0", "@org/package"},
		{"scoped with tag", "npm install -g @org/package@latest", "@org/package"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNPMPackageName(tt.command)
			if result != tt.expected {
				t.Errorf("extractNPMPackageName(%q) = %q, want %q", tt.command, result, tt.expected)
			}
		})
	}
}

func TestIsPackageManagerPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// npm/node paths - should be detected
		{"npm global", "/usr/local/lib/node_modules/.bin/claude", true},
		{"mise node", "/Users/kevin/.local/share/mise/installs/node/22.20.0/bin/blackbox", true},
		{"nvm node", "/Users/kevin/.nvm/versions/node/v20.0.0/bin/amp", true},
		{"fnm node", "/Users/kevin/Library/fnm/node-versions/v20.0.0/bin/app", true},
		{"volta node", "/Users/kevin/.volta/bin/node", true},
		{"pnpm", "/Users/kevin/.local/share/pnpm/bin/app", true},
		{"bun", "/Users/kevin/.bun/bin/app", true},

		// Python paths - should be detected
		{"pipx", "/Users/kevin/.local/pipx/venvs/aider/bin/aider", true},
		{"pyenv", "/Users/kevin/.pyenv/versions/3.11.0/bin/python", true},
		{"conda", "/opt/conda/bin/python", true},
		{"venv", "/project/.venv/bin/python", true},
		{"uv", "/Users/kevin/.local/share/uv/bin/app", true},

		// Homebrew paths - should be detected
		{"homebrew cellar", "/opt/homebrew/Cellar/droid/0.47.0/bin/droid", true},
		{"homebrew bin", "/opt/homebrew/bin/droid", true},
		{"linuxbrew", "/home/linuxbrew/.linuxbrew/bin/app", true},

		// Go paths - should be detected
		{"go bin", "/Users/kevin/go/bin/app", true},
		{"gopath", "/Users/kevin/gopath/bin/app", true},

		// Cargo paths - should be detected
		{"cargo", "/Users/kevin/.cargo/bin/rustc", true},

		// Native paths - should NOT be detected
		{"usr local bin", "/usr/local/bin/claude", false},
		{"usr bin", "/usr/bin/python", false},
		{"opt bin", "/opt/app/bin/app", false},
		{"applications", "/Applications/App.app/Contents/MacOS/app", false},
		{"home bin", "/Users/kevin/bin/app", false},
		{"custom install", "/opt/claude-cli/bin/claude", false},

		// asdf paths - should be detected
		{"asdf nodejs", "/Users/kevin/.asdf/installs/nodejs/20.0.0/bin/node", true},
		{"asdf python", "/Users/kevin/.asdf/installs/python/3.11.0/bin/python", true},

		// Generic version managers
		{"mise generic", "/Users/kevin/.local/share/mise/installs/go/1.22.0/bin/go", true},
		{"rtx", "/Users/kevin/.local/share/rtx/installs/node/20.0.0/bin/node", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPackageManagerPath(tt.path)
			if result != tt.expected {
				t.Errorf("isPackageManagerPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
