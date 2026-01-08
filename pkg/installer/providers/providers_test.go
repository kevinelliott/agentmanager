package providers

import (
	"context"
	"testing"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/platform"
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
