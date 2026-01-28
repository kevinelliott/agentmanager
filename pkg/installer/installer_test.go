package installer

import (
	"testing"

	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/platform"
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

func TestGetLatestVersionUnsupportedMethod(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	method := catalog.InstallMethodDef{
		Method: "unsupported-method",
	}

	_, err := m.GetLatestVersion(nil, method)
	if err == nil {
		t.Error("GetLatestVersion() should return error for unsupported method")
	}
	if err.Error() != "unsupported install method: unsupported-method" {
		t.Errorf("GetLatestVersion() error = %q, want %q", err.Error(), "unsupported install method: unsupported-method")
	}
}

func TestGetLatestVersionNativeVariants(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	variants := []string{"native", "curl", "binary"}

	for _, variant := range variants {
		method := catalog.InstallMethodDef{
			Method: variant,
		}

		_, err := m.GetLatestVersion(nil, method)
		if err == nil {
			t.Errorf("GetLatestVersion(%q) should return error for native variants", variant)
		}
		expectedErr := "version checking not supported for " + variant
		if err.Error() != expectedErr {
			t.Errorf("GetLatestVersion(%q) error = %q, want %q", variant, err.Error(), expectedErr)
		}
	}
}

func TestGetLatestVersionNpmNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.npm.IsAvailable() {
		t.Skip("npm is available, skipping unavailable test")
	}

	method := catalog.InstallMethodDef{
		Method:  "npm",
		Package: "test-package",
	}

	_, err := m.GetLatestVersion(nil, method)
	if err == nil {
		t.Error("GetLatestVersion(npm) should fail when npm is not available")
	}
	if err.Error() != "npm is not available" {
		t.Errorf("GetLatestVersion(npm) error = %q, want %q", err.Error(), "npm is not available")
	}
}

func TestGetLatestVersionPipNotAvailable(t *testing.T) {
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

		_, err := m.GetLatestVersion(nil, method)
		if err == nil {
			t.Errorf("GetLatestVersion(%q) should fail when pip is not available", variant)
		}
		if err.Error() != "pip/pipx/uv is not available" {
			t.Errorf("GetLatestVersion(%q) error = %q, want %q", variant, err.Error(), "pip/pipx/uv is not available")
		}
	}
}

func TestGetLatestVersionBrewNotAvailable(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.brew.IsAvailable() {
		t.Skip("brew is available, skipping unavailable test")
	}

	method := catalog.InstallMethodDef{
		Method:  "brew",
		Package: "test-package",
	}

	_, err := m.GetLatestVersion(nil, method)
	if err == nil {
		t.Error("GetLatestVersion(brew) should fail when brew is not available")
	}
	if err.Error() != "brew is not available" {
		t.Errorf("GetLatestVersion(brew) error = %q, want %q", err.Error(), "brew is not available")
	}
}

func TestInstallUnsupportedMethodErrorMessage(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method: "invalid-method",
	}

	_, err := m.Install(nil, agentDef, method, false)
	if err == nil {
		t.Fatal("Install() should return error for invalid method")
	}
	expectedErr := "unsupported install method: invalid-method"
	if err.Error() != expectedErr {
		t.Errorf("Install() error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestUpdateUnsupportedMethodErrorMessage(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method: "invalid-method",
	}

	_, err := m.Update(nil, nil, agentDef, method)
	if err == nil {
		t.Fatal("Update() should return error for invalid method")
	}
	expectedErr := "unsupported install method: invalid-method"
	if err.Error() != expectedErr {
		t.Errorf("Update() error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestUninstallUnsupportedMethodErrorMessage(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	method := catalog.InstallMethodDef{
		Method: "invalid-method",
	}

	err := m.Uninstall(nil, nil, method)
	if err == nil {
		t.Fatal("Uninstall() should return error for invalid method")
	}
	expectedErr := "unsupported install method: invalid-method"
	if err.Error() != expectedErr {
		t.Errorf("Uninstall() error = %q, want %q", err.Error(), expectedErr)
	}
}

func TestGetAvailableMethodsEmptyInstallMethods(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	agentDef := catalog.AgentDef{
		ID:             "test-agent",
		Name:           "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{},
	}

	methods := m.GetAvailableMethods(agentDef)
	if len(methods) != 0 {
		t.Errorf("GetAvailableMethods() with empty InstallMethods returned %d, want 0", len(methods))
	}
}

func TestGetAvailableMethodsNilInstallMethods(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	agentDef := catalog.AgentDef{
		ID:             "test-agent",
		Name:           "Test Agent",
		InstallMethods: nil,
	}

	methods := m.GetAvailableMethods(agentDef)
	if len(methods) != 0 {
		t.Errorf("GetAvailableMethods() with nil InstallMethods returned %d, want 0", len(methods))
	}
}

func TestGetAvailableMethodsMultiplePlatforms(t *testing.T) {
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
				Platforms: []string{"linux", "darwin", "windows", platformID},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	if len(methods) != 1 {
		t.Errorf("GetAvailableMethods() returned %d methods, want 1", len(methods))
	}
}

func TestGetAvailableMethodsPlatformAtEnd(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	platformID := string(p.ID())

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"native": {
				Method:    "native",
				Command:   "install command",
				Platforms: []string{"other1", "other2", platformID},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	if len(methods) != 1 {
		t.Errorf("GetAvailableMethods() with platform at end returned %d methods, want 1", len(methods))
	}
}

func TestGetAvailableMethodsUnknownMethod(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	platformID := string(p.ID())

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"unknown": {
				Method:    "totally-unknown",
				Command:   "some command",
				Platforms: []string{platformID},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	if len(methods) != 0 {
		t.Errorf("GetAvailableMethods() with unknown method returned %d methods, want 0", len(methods))
	}
}

func TestNewManagerStoresPlatform(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.plat != p {
		t.Error("NewManager() should store the provided platform")
	}
}

func TestIsMethodAvailableEmptyString(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.IsMethodAvailable("") {
		t.Error("IsMethodAvailable(\"\") should return false")
	}
}

func TestGetAvailableMethodsMixedAvailability(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	platformID := string(p.ID())

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"native": {
				Method:    "native",
				Command:   "native install",
				Platforms: []string{platformID},
			},
			"npm": {
				Method:    "npm",
				Package:   "test-package",
				Platforms: []string{platformID},
			},
			"pip": {
				Method:    "pip",
				Package:   "test-package",
				Platforms: []string{platformID},
			},
			"brew": {
				Method:    "brew",
				Package:   "test-package",
				Platforms: []string{platformID},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	hasNative := false
	for _, method := range methods {
		if method.Method == "native" {
			hasNative = true
			break
		}
	}

	if !hasNative {
		t.Error("GetAvailableMethods() should always include native method when platform matches")
	}

	npmExpected := m.npm.IsAvailable()
	pipExpected := m.pip.IsAvailable()
	brewExpected := m.brew.IsAvailable()

	hasNpm, hasPip, hasBrew := false, false, false
	for _, method := range methods {
		switch method.Method {
		case "npm":
			hasNpm = true
		case "pip":
			hasPip = true
		case "brew":
			hasBrew = true
		}
	}

	if hasNpm != npmExpected {
		t.Errorf("npm method presence = %v, npm.IsAvailable() = %v", hasNpm, npmExpected)
	}
	if hasPip != pipExpected {
		t.Errorf("pip method presence = %v, pip.IsAvailable() = %v", hasPip, pipExpected)
	}
	if hasBrew != brewExpected {
		t.Errorf("brew method presence = %v, brew.IsAvailable() = %v", hasBrew, brewExpected)
	}
}

func TestIsMethodAvailableTableDriven(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	tests := []struct {
		method   string
		checkFn  func() bool
		alwaysOn bool
	}{
		{"native", nil, true},
		{"curl", nil, true},
		{"binary", nil, true},
		{"npm", func() bool { return m.npm.IsAvailable() }, false},
		{"pip", func() bool { return m.pip.IsAvailable() }, false},
		{"pipx", func() bool { return m.pip.IsAvailable() }, false},
		{"uv", func() bool { return m.pip.IsAvailable() }, false},
		{"brew", func() bool { return m.brew.IsAvailable() }, false},
		{"", nil, false},
		{"unknown-method", nil, false},
		{"npm ", nil, false},
		{" npm", nil, false},
		{"NPM", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := m.IsMethodAvailable(tt.method)
			if tt.alwaysOn {
				if !got {
					t.Errorf("IsMethodAvailable(%q) = false, want true (always available)", tt.method)
				}
			} else if tt.checkFn != nil {
				expected := tt.checkFn()
				if got != expected {
					t.Errorf("IsMethodAvailable(%q) = %v, provider available = %v", tt.method, got, expected)
				}
			} else if got {
				t.Errorf("IsMethodAvailable(%q) = true, want false", tt.method)
			}
		})
	}
}

func TestGetAvailableMethodsEmptyPlatforms(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"native": {
				Method:    "native",
				Command:   "install command",
				Platforms: []string{},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	if len(methods) != 0 {
		t.Errorf("GetAvailableMethods() with empty platforms returned %d methods, want 0", len(methods))
	}
}

func TestGetAvailableMethodsOnlyCurrentPlatform(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	platformID := string(p.ID())

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"native": {
				Method:    "native",
				Command:   "install command",
				Platforms: []string{platformID},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	if len(methods) != 1 {
		t.Errorf("GetAvailableMethods() with only current platform returned %d methods, want 1", len(methods))
	}
}

func TestGetAvailableMethodsAllPlatformTypes(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	platformID := string(p.ID())

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
		InstallMethods: map[string]catalog.InstallMethodDef{
			"native": {
				Method:    "native",
				Command:   "native install",
				Platforms: []string{platformID},
			},
			"curl": {
				Method:    "curl",
				Command:   "curl install",
				Platforms: []string{platformID},
			},
			"binary": {
				Method:    "binary",
				Command:   "binary install",
				Platforms: []string{platformID},
			},
			"npm": {
				Method:    "npm",
				Package:   "test-pkg",
				Platforms: []string{platformID},
			},
			"pip": {
				Method:    "pip",
				Package:   "test-pkg",
				Platforms: []string{platformID},
			},
			"pipx": {
				Method:    "pipx",
				Package:   "test-pkg",
				Platforms: []string{platformID},
			},
			"uv": {
				Method:    "uv",
				Package:   "test-pkg",
				Platforms: []string{platformID},
			},
			"brew": {
				Method:    "brew",
				Package:   "test-pkg",
				Platforms: []string{platformID},
			},
		},
	}

	methods := m.GetAvailableMethods(agentDef)

	foundMethods := make(map[string]bool)
	for _, method := range methods {
		foundMethods[method.Method] = true
	}

	if !foundMethods["native"] {
		t.Error("native should always be available")
	}
	if !foundMethods["curl"] {
		t.Error("curl should always be available")
	}
	if !foundMethods["binary"] {
		t.Error("binary should always be available")
	}

	if m.npm.IsAvailable() && !foundMethods["npm"] {
		t.Error("npm should be available when npm provider is available")
	}
	if m.pip.IsAvailable() && !foundMethods["pip"] {
		t.Error("pip should be available when pip provider is available")
	}
	if m.pip.IsAvailable() && !foundMethods["pipx"] {
		t.Error("pipx should be available when pip provider is available")
	}
	if m.pip.IsAvailable() && !foundMethods["uv"] {
		t.Error("uv should be available when pip provider is available")
	}
	if m.brew.IsAvailable() && !foundMethods["brew"] {
		t.Error("brew should be available when brew provider is available")
	}
}

func TestInstallForceParameter(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	agentDef := catalog.AgentDef{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	method := catalog.InstallMethodDef{
		Method: "unsupported",
	}

	_, err1 := m.Install(nil, agentDef, method, false)
	_, err2 := m.Install(nil, agentDef, method, true)

	if err1 == nil || err2 == nil {
		t.Error("Both Install calls should return error for unsupported method")
	}
	if err1.Error() != err2.Error() {
		t.Errorf("Errors should match: got %q and %q", err1.Error(), err2.Error())
	}
}

func TestManagerFieldsInitialized(t *testing.T) {
	p := platform.Current()
	m := NewManager(p)

	if m.npm == nil {
		t.Error("npm provider should be initialized")
	}
	if m.pip == nil {
		t.Error("pip provider should be initialized")
	}
	if m.brew == nil {
		t.Error("brew provider should be initialized")
	}
	if m.native == nil {
		t.Error("native provider should be initialized")
	}
	if m.plat == nil {
		t.Error("platform should be initialized")
	}
	if m.plat.ID() != p.ID() {
		t.Error("platform ID should match")
	}
}
