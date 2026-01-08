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

func TestIsMethodAvailablePipVariants(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	// pip, pipx, and uv should all check the same pip provider
	pipAvail := m.IsMethodAvailable("pip")
	pipxAvail := m.IsMethodAvailable("pipx")
	uvAvail := m.IsMethodAvailable("uv")

	// All three should have the same availability (pip provider)
	if pipAvail != pipxAvail || pipxAvail != uvAvail {
		t.Errorf("pip variants should have same availability: pip=%v, pipx=%v, uv=%v",
			pipAvail, pipxAvail, uvAvail)
	}
}

func TestIsMethodAvailableNativeVariants(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	// native, curl, and binary should all be available
	tests := []string{"native", "curl", "binary"}
	for _, method := range tests {
		if !m.IsMethodAvailable(method) {
			t.Errorf("IsMethodAvailable(%q) should always be true", method)
		}
	}
}

func TestGetAvailableMethodsFiltersByProvider(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	platformID := string(p.ID())

	// Create an agent with a mix of methods
	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"native": {
				Method:    "native",
				Command:   "curl -fsSL https://example.com/install.sh | sh",
				Platforms: []string{platformID},
			},
			"curl": {
				Method:    "curl",
				Command:   "curl -fsSL https://example.com/binary -o /usr/local/bin/test",
				Platforms: []string{platformID},
			},
			"binary": {
				Method:    "binary",
				Command:   "download from releases",
				Platforms: []string{platformID},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	// All three native-type methods should be available
	if len(methods) != 3 {
		t.Errorf("GetAvailableMethods() returned %d methods, want 3", len(methods))
	}

	foundMethods := make(map[string]bool)
	for _, method := range methods {
		foundMethods[method.Method] = true
	}

	for _, expected := range []string{"native", "curl", "binary"} {
		if !foundMethods[expected] {
			t.Errorf("GetAvailableMethods() should include %q method", expected)
		}
	}
}

func TestInstallPipVariantsNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	// Skip if pip is available - we only test the unavailable case
	if m.pip.IsAvailable() {
		t.Skip("pip is available, skipping unavailable test")
	}

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	// Test all pip variants have same behavior (check availability)
	variants := []string{"pip", "pipx", "uv"}

	for _, variant := range variants {
		method := catalog.InstallMethodDef{
			Method:  variant,
			Package: "test-package",
		}

		_, err := m.Install(nil, agentDef, method, false)
		if err == nil {
			t.Errorf("Install(%q) should fail when pip is not available", variant)
		}
		if err.Error() != "pip/pipx/uv is not available" {
			t.Errorf("Install(%q) error = %q, want %q", variant, err.Error(), "pip/pipx/uv is not available")
		}
	}
}

func TestUpdatePipVariantsNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.pip.IsAvailable() {
		t.Skip("pip is available, skipping unavailable test")
	}

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	variants := []string{"pip", "pipx", "uv"}

	for _, variant := range variants {
		method := catalog.InstallMethodDef{
			Method:  variant,
			Package: "test-package",
		}

		_, err := m.Update(nil, nil, agentDef, method)
		if err == nil {
			t.Errorf("Update(%q) should fail when pip is not available", variant)
		}
		if err.Error() != "pip/pipx/uv is not available" {
			t.Errorf("Update(%q) error = %q, want %q", variant, err.Error(), "pip/pipx/uv is not available")
		}
	}
}

func TestUninstallPipVariantsNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.pip.IsAvailable() {
		t.Skip("pip is available, skipping unavailable test")
	}

	variants := []string{"pip", "pipx", "uv"}

	for _, variant := range variants {
		method := catalog.InstallMethodDef{
			Method:  variant,
			Package: "test-package",
		}

		err := m.Uninstall(nil, nil, method)
		if err == nil {
			t.Errorf("Uninstall(%q) should fail when pip is not available", variant)
		}
		if err.Error() != "pip/pipx/uv is not available" {
			t.Errorf("Uninstall(%q) error = %q, want %q", variant, err.Error(), "pip/pipx/uv is not available")
		}
	}
}

func TestInstallNpmNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	// Skip if npm is actually available
	if m.npm.IsAvailable() {
		t.Skip("npm is available, skipping unavailable test")
	}

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method:  "npm",
		Package: "test-package",
	}

	_, err := m.Install(nil, agentDef, method, false)
	if err == nil {
		t.Error("Install(npm) should fail when npm is not available")
	}
	if err.Error() != "npm is not available" {
		t.Errorf("Install(npm) error = %q, want %q", err.Error(), "npm is not available")
	}
}

func TestUpdateNpmNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.npm.IsAvailable() {
		t.Skip("npm is available, skipping unavailable test")
	}

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method:  "npm",
		Package: "test-package",
	}

	_, err := m.Update(nil, nil, agentDef, method)
	if err == nil {
		t.Error("Update(npm) should fail when npm is not available")
	}
	if err.Error() != "npm is not available" {
		t.Errorf("Update(npm) error = %q, want %q", err.Error(), "npm is not available")
	}
}

func TestUninstallNpmNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.npm.IsAvailable() {
		t.Skip("npm is available, skipping unavailable test")
	}

	method := catalog.InstallMethodDef{
		Method:  "npm",
		Package: "test-package",
	}

	err := m.Uninstall(nil, nil, method)
	if err == nil {
		t.Error("Uninstall(npm) should fail when npm is not available")
	}
	if err.Error() != "npm is not available" {
		t.Errorf("Uninstall(npm) error = %q, want %q", err.Error(), "npm is not available")
	}
}

func TestInstallBrewNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.brew.IsAvailable() {
		t.Skip("brew is available, skipping unavailable test")
	}

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method:  "brew",
		Package: "test-package",
	}

	_, err := m.Install(nil, agentDef, method, false)
	if err == nil {
		t.Error("Install(brew) should fail when brew is not available")
	}
	if err.Error() != "brew is not available" {
		t.Errorf("Install(brew) error = %q, want %q", err.Error(), "brew is not available")
	}
}

func TestUpdateBrewNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.brew.IsAvailable() {
		t.Skip("brew is available, skipping unavailable test")
	}

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method:  "brew",
		Package: "test-package",
	}

	_, err := m.Update(nil, nil, agentDef, method)
	if err == nil {
		t.Error("Update(brew) should fail when brew is not available")
	}
	if err.Error() != "brew is not available" {
		t.Errorf("Update(brew) error = %q, want %q", err.Error(), "brew is not available")
	}
}

func TestUninstallBrewNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.brew.IsAvailable() {
		t.Skip("brew is available, skipping unavailable test")
	}

	method := catalog.InstallMethodDef{
		Method:  "brew",
		Package: "test-package",
	}

	err := m.Uninstall(nil, nil, method)
	if err == nil {
		t.Error("Uninstall(brew) should fail when brew is not available")
	}
	if err.Error() != "brew is not available" {
		t.Errorf("Uninstall(brew) error = %q, want %q", err.Error(), "brew is not available")
	}
}

func TestGetAvailableMethodsWithPipVariants(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	platformID := string(p.ID())

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"pip": {
				Method:    "pip",
				Package:   "test-package",
				Platforms: []string{platformID},
			},
			"pipx": {
				Method:    "pipx",
				Package:   "test-package",
				Platforms: []string{platformID},
			},
			"uv": {
				Method:    "uv",
				Package:   "test-package",
				Platforms: []string{platformID},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	// If pip is available, all three should be available
	// If pip is not available, none should be available
	pipAvail := m.pip.IsAvailable()

	if pipAvail && len(methods) != 3 {
		t.Errorf("GetAvailableMethods() with pip available returned %d methods, want 3", len(methods))
	}

	if !pipAvail && len(methods) != 0 {
		t.Errorf("GetAvailableMethods() without pip available returned %d methods, want 0", len(methods))
	}
}
