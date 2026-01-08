package agent

import (
	"testing"
	"time"
)

func TestInstallMethodString(t *testing.T) {
	tests := []struct {
		method   InstallMethod
		expected string
	}{
		{InstallMethodNPM, "npm"},
		{InstallMethodBrew, "brew"},
		{InstallMethodPip, "pip"},
		{InstallMethodPipx, "pipx"},
		{InstallMethodUV, "uv"},
		{InstallMethodScoop, "scoop"},
		{InstallMethodWinget, "winget"},
		{InstallMethodChocolatey, "chocolatey"},
		{InstallMethodNative, "native"},
		{InstallMethodCurl, "curl"},
		{InstallMethodBinary, "binary"},
	}

	for _, tt := range tests {
		t.Run(string(tt.method), func(t *testing.T) {
			got := tt.method.String()
			if got != tt.expected {
				t.Errorf("InstallMethod.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInstallMethodDisplayName(t *testing.T) {
	tests := []struct {
		method   InstallMethod
		expected string
	}{
		{InstallMethodNPM, "npm"},
		{InstallMethodBrew, "Homebrew"},
		{InstallMethodPip, "pip"},
		{InstallMethodPipx, "pipx"},
		{InstallMethodUV, "uv"},
		{InstallMethodScoop, "Scoop"},
		{InstallMethodWinget, "winget"},
		{InstallMethodChocolatey, "Chocolatey"},
		{InstallMethodNative, "Native Installer"},
		{InstallMethodCurl, "curl"},
		{InstallMethodBinary, "Binary"},
		{InstallMethod("unknown"), "unknown"}, // Unknown method returns string form
	}

	for _, tt := range tests {
		t.Run(string(tt.method), func(t *testing.T) {
			got := tt.method.DisplayName()
			if got != tt.expected {
				t.Errorf("InstallMethod.DisplayName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInstallationKey(t *testing.T) {
	tests := []struct {
		name           string
		agentID        string
		method         InstallMethod
		executablePath string
		expected       string
	}{
		{
			name:           "basic key",
			agentID:        "claude-code",
			method:         InstallMethodNPM,
			executablePath: "/usr/local/bin/claude",
			expected:       "claude-code:npm:/usr/local/bin/claude",
		},
		{
			name:           "pip method",
			agentID:        "aider",
			method:         InstallMethodPip,
			executablePath: "/home/user/.local/bin/aider",
			expected:       "aider:pip:/home/user/.local/bin/aider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := Installation{
				AgentID:        tt.agentID,
				Method:         tt.method,
				ExecutablePath: tt.executablePath,
			}
			got := inst.Key()
			if got != tt.expected {
				t.Errorf("Installation.Key() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInstallationHasUpdate(t *testing.T) {
	tests := []struct {
		name           string
		installedVer   string
		latestVer      *string
		expectedUpdate bool
	}{
		{
			name:           "no latest version",
			installedVer:   "1.0.0",
			latestVer:      nil,
			expectedUpdate: false,
		},
		{
			name:           "update available",
			installedVer:   "1.0.0",
			latestVer:      strPtr("2.0.0"),
			expectedUpdate: true,
		},
		{
			name:           "same version",
			installedVer:   "1.0.0",
			latestVer:      strPtr("1.0.0"),
			expectedUpdate: false,
		},
		{
			name:           "installed newer",
			installedVer:   "2.0.0",
			latestVer:      strPtr("1.0.0"),
			expectedUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := Installation{
				InstalledVersion: MustParseVersion(tt.installedVer),
			}
			if tt.latestVer != nil {
				v := MustParseVersion(*tt.latestVer)
				inst.LatestVersion = &v
			}
			got := inst.HasUpdate()
			if got != tt.expectedUpdate {
				t.Errorf("Installation.HasUpdate() = %v, want %v", got, tt.expectedUpdate)
			}
		})
	}
}

func TestInstallationGetStatus(t *testing.T) {
	tests := []struct {
		name         string
		installedVer string
		latestVer    *string
		expected     Status
	}{
		{
			name:         "no latest version - unknown",
			installedVer: "1.0.0",
			latestVer:    nil,
			expected:     StatusUnknown,
		},
		{
			name:         "update available - outdated",
			installedVer: "1.0.0",
			latestVer:    strPtr("2.0.0"),
			expected:     StatusOutdated,
		},
		{
			name:         "same version - current",
			installedVer: "1.0.0",
			latestVer:    strPtr("1.0.0"),
			expected:     StatusCurrent,
		},
		{
			name:         "installed newer - current",
			installedVer: "2.0.0",
			latestVer:    strPtr("1.0.0"),
			expected:     StatusCurrent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := Installation{
				InstalledVersion: MustParseVersion(tt.installedVer),
			}
			if tt.latestVer != nil {
				v := MustParseVersion(*tt.latestVer)
				inst.LatestVersion = &v
			}
			got := inst.GetStatus()
			if got != tt.expected {
				t.Errorf("Installation.GetStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInstallationFull(t *testing.T) {
	now := time.Now()
	latestVer := MustParseVersion("2.0.0")

	inst := Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           InstallMethodNPM,
		InstalledVersion: MustParseVersion("1.5.0"),
		LatestVersion:    &latestVer,
		ExecutablePath:   "/usr/local/bin/claude",
		InstallPath:      "/usr/local/lib/node_modules/@anthropic-ai/claude-code",
		IsGlobal:         true,
		DetectedAt:       now,
		LastChecked:      now,
		Metadata: map[string]string{
			"npm_version": "10.0.0",
		},
	}

	// Test key generation
	expectedKey := "claude-code:npm:/usr/local/bin/claude"
	if inst.Key() != expectedKey {
		t.Errorf("Key() = %q, want %q", inst.Key(), expectedKey)
	}

	// Test update detection
	if !inst.HasUpdate() {
		t.Error("HasUpdate() should return true")
	}

	// Test status
	if inst.GetStatus() != StatusOutdated {
		t.Errorf("GetStatus() = %v, want %v", inst.GetStatus(), StatusOutdated)
	}
}

// Helper function
func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
