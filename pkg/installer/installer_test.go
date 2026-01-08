package installer

import (
	"testing"

	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

func TestNewManager(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m == nil {
		t.Fatal("NewManager() returned nil")
	}

	// Verify providers are initialized
	if m.npm == nil {
		t.Error("npm provider should not be nil")
	}
	if m.pip == nil {
		t.Error("pip provider should not be nil")
	}
	if m.brew == nil {
		t.Error("brew provider should not be nil")
	}
	if m.native == nil {
		t.Error("native provider should not be nil")
	}
	if m.plat == nil {
		t.Error("platform should not be nil")
	}
}

func TestIsMethodAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	tests := []struct {
		method   string
		expected bool // Expected minimum - native/curl/binary always available
	}{
		{"native", true},
		{"curl", true},
		{"binary", true},
		{"unknown", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := m.IsMethodAvailable(tt.method)
			// For native/curl/binary, they should always be true
			// For others (npm, pip, brew), it depends on system
			if tt.method == "native" || tt.method == "curl" || tt.method == "binary" {
				if got != tt.expected {
					t.Errorf("IsMethodAvailable(%q) = %v, want %v", tt.method, got, tt.expected)
				}
			}
			// For "unknown" and "invalid", should always be false
			if tt.method == "unknown" || tt.method == "invalid" {
				if got != tt.expected {
					t.Errorf("IsMethodAvailable(%q) = %v, want %v", tt.method, got, tt.expected)
				}
			}
		})
	}
}

func TestGetAvailableMethods(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	platformID := string(p.ID())

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"native": {
				Method:    "native",
				Command:   "curl -fsSL https://example.com/install.sh | sh",
				Platforms: []string{platformID},
			},
			"npm": {
				Method:    "npm",
				Package:   "test-agent",
				Command:   "npm install -g test-agent",
				Platforms: []string{platformID},
			},
			"other-platform": {
				Method:    "native",
				Command:   "some command",
				Platforms: []string{"unsupported-platform"},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	// At minimum, native should always be available
	hasNative := false
	for _, method := range methods {
		if method.Method == "native" {
			hasNative = true
			break
		}
	}

	if !hasNative {
		t.Error("GetAvailableMethods() should include native method for current platform")
	}

	// Should not include methods for unsupported platforms
	for _, method := range methods {
		platformSupported := false
		for _, p := range method.Platforms {
			if p == platformID {
				platformSupported = true
				break
			}
		}
		if !platformSupported {
			t.Errorf("GetAvailableMethods() included method for unsupported platform: %v", method)
		}
	}
}

func TestGetAvailableMethodsEmpty(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	// Agent with no methods for current platform
	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"native": {
				Method:    "native",
				Command:   "some command",
				Platforms: []string{"unsupported-platform"},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	if len(methods) != 0 {
		t.Errorf("GetAvailableMethods() should return empty for unsupported platform, got %d", len(methods))
	}
}

func TestInstallUnsupportedMethod(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method: "unsupported-method",
	}

	_, err := m.Install(nil, agentDef, method, false)
	if err == nil {
		t.Error("Install() should return error for unsupported method")
	}
}

func TestUpdateUnsupportedMethod(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method: "unsupported-method",
	}

	_, err := m.Update(nil, nil, agentDef, method)
	if err == nil {
		t.Error("Update() should return error for unsupported method")
	}
}

func TestUninstallUnsupportedMethod(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	method := catalog.InstallMethodDef{
		Method: "unsupported-method",
	}

	err := m.Uninstall(nil, nil, method)
	if err == nil {
		t.Error("Uninstall() should return error for unsupported method")
	}
}
