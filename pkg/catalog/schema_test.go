package catalog

import (
	"testing"
	"time"
)

func createTestCatalog() *Catalog {
	return &Catalog{
		Version:       "1.0.0",
		SchemaVersion: 1,
		LastUpdated:   time.Now(),
		Agents: map[string]AgentDef{
			"claude-code": {
				ID:          "claude-code",
				Name:        "Claude Code",
				Description: "Anthropic's CLI for Claude",
				Homepage:    "https://claude.ai/claude-code",
				Repository:  "https://github.com/anthropics/claude-code",
				InstallMethods: map[string]InstallMethodDef{
					"npm": {
						Method:    "npm",
						Package:   "@anthropic-ai/claude-code",
						Command:   "npm install -g @anthropic-ai/claude-code",
						Platforms: []string{"darwin", "linux", "windows"},
					},
					"native": {
						Method:    "native",
						Command:   "curl -fsSL https://claude.ai/install.sh | sh",
						Platforms: []string{"darwin", "linux"},
					},
				},
				Detection: DetectionDef{
					Executables:  []string{"claude", "claude-code"},
					VersionCmd:   "claude --version",
					VersionRegex: `claude-code version ([\d.]+)`,
				},
				Changelog: ChangelogDef{
					Type: "github_releases",
					URL:  "https://api.github.com/repos/anthropics/claude-code/releases",
				},
			},
			"aider": {
				ID:          "aider",
				Name:        "Aider",
				Description: "AI pair programming in your terminal",
				Homepage:    "https://aider.chat",
				InstallMethods: map[string]InstallMethodDef{
					"pip": {
						Method:    "pip",
						Package:   "aider-chat",
						Command:   "pip install aider-chat",
						Platforms: []string{"darwin", "linux", "windows"},
					},
					"pipx": {
						Method:    "pipx",
						Package:   "aider-chat",
						Command:   "pipx install aider-chat",
						Platforms: []string{"darwin", "linux", "windows"},
					},
				},
				Detection: DetectionDef{
					Executables:  []string{"aider"},
					VersionCmd:   "aider --version",
					VersionRegex: `aider ([\d.]+)`,
				},
			},
			"copilot": {
				ID:          "copilot",
				Name:        "GitHub Copilot CLI",
				Description: "Your AI command line tool",
				InstallMethods: map[string]InstallMethodDef{
					"npm": {
						Method:    "npm",
						Package:   "@githubnext/github-copilot-cli",
						Command:   "npm install -g @githubnext/github-copilot-cli",
						Platforms: []string{"darwin", "linux", "windows"},
					},
				},
				Detection: DetectionDef{
					Executables: []string{"copilot"},
					VersionCmd:  "copilot --version",
				},
			},
		},
	}
}

func TestAgentDefIsSupported(t *testing.T) {
	catalog := createTestCatalog()

	tests := []struct {
		agentID    string
		platformID string
		expected   bool
	}{
		{"claude-code", "darwin", true},
		{"claude-code", "linux", true},
		{"claude-code", "windows", true},
		{"claude-code", "freebsd", false},
		{"aider", "darwin", true},
		{"aider", "linux", true},
		{"aider", "windows", true},
	}

	for _, tt := range tests {
		t.Run(tt.agentID+"-"+tt.platformID, func(t *testing.T) {
			agent := catalog.Agents[tt.agentID]
			got := agent.IsSupported(tt.platformID)
			if got != tt.expected {
				t.Errorf("IsSupported(%q) = %v, want %v", tt.platformID, got, tt.expected)
			}
		})
	}
}

func TestAgentDefGetInstallMethod(t *testing.T) {
	catalog := createTestCatalog()
	agent := catalog.Agents["claude-code"]

	// Get existing method
	npm, ok := agent.GetInstallMethod("npm")
	if !ok {
		t.Error("GetInstallMethod(npm) should return true")
	}
	if npm.Package != "@anthropic-ai/claude-code" {
		t.Errorf("npm.Package = %q, want %q", npm.Package, "@anthropic-ai/claude-code")
	}

	// Get non-existing method
	_, ok = agent.GetInstallMethod("brew")
	if ok {
		t.Error("GetInstallMethod(brew) should return false")
	}
}

func TestAgentDefGetSupportedMethods(t *testing.T) {
	catalog := createTestCatalog()

	tests := []struct {
		agentID    string
		platformID string
		expected   int
	}{
		{"claude-code", "darwin", 2},  // npm and native
		{"claude-code", "windows", 1}, // npm only
		{"aider", "darwin", 2},        // pip and pipx
	}

	for _, tt := range tests {
		t.Run(tt.agentID+"-"+tt.platformID, func(t *testing.T) {
			agent := catalog.Agents[tt.agentID]
			methods := agent.GetSupportedMethods(tt.platformID)
			if len(methods) != tt.expected {
				t.Errorf("GetSupportedMethods(%q) returned %d methods, want %d", tt.platformID, len(methods), tt.expected)
			}
		})
	}
}

func TestAgentDefGetExecutable(t *testing.T) {
	catalog := createTestCatalog()

	tests := []struct {
		agentID  string
		expected string
	}{
		{"claude-code", "claude"},
		{"aider", "aider"},
		{"copilot", "copilot"},
	}

	for _, tt := range tests {
		t.Run(tt.agentID, func(t *testing.T) {
			agent := catalog.Agents[tt.agentID]
			got := agent.GetExecutable()
			if got != tt.expected {
				t.Errorf("GetExecutable() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAgentDefGetExecutableEmpty(t *testing.T) {
	agent := AgentDef{
		ID:   "test",
		Name: "Test Agent",
		Detection: DetectionDef{
			Executables: []string{}, // Empty
		},
	}

	got := agent.GetExecutable()
	if got != "" {
		t.Errorf("GetExecutable() = %q, want empty string", got)
	}
}

func TestCatalogGetAgents(t *testing.T) {
	catalog := createTestCatalog()

	agents := catalog.GetAgents()
	if len(agents) != 3 {
		t.Errorf("GetAgents() returned %d agents, want 3", len(agents))
	}
}

func TestCatalogGetAgent(t *testing.T) {
	catalog := createTestCatalog()

	// Get existing agent
	agent, ok := catalog.GetAgent("claude-code")
	if !ok {
		t.Error("GetAgent(claude-code) should return true")
	}
	if agent.Name != "Claude Code" {
		t.Errorf("agent.Name = %q, want %q", agent.Name, "Claude Code")
	}

	// Get non-existing agent
	_, ok = catalog.GetAgent("nonexistent")
	if ok {
		t.Error("GetAgent(nonexistent) should return false")
	}
}

func TestCatalogGetAgentsByPlatform(t *testing.T) {
	catalog := createTestCatalog()

	// All agents support darwin
	darwinAgents := catalog.GetAgentsByPlatform("darwin")
	if len(darwinAgents) != 3 {
		t.Errorf("GetAgentsByPlatform(darwin) returned %d agents, want 3", len(darwinAgents))
	}

	// All agents support windows
	windowsAgents := catalog.GetAgentsByPlatform("windows")
	if len(windowsAgents) != 3 {
		t.Errorf("GetAgentsByPlatform(windows) returned %d agents, want 3", len(windowsAgents))
	}

	// No agents support freebsd
	freebsdAgents := catalog.GetAgentsByPlatform("freebsd")
	if len(freebsdAgents) != 0 {
		t.Errorf("GetAgentsByPlatform(freebsd) returned %d agents, want 0", len(freebsdAgents))
	}
}

func TestCatalogSearch(t *testing.T) {
	catalog := createTestCatalog()

	tests := []struct {
		query    string
		expected int
	}{
		{"", 3},            // Empty query returns all
		{"claude", 1},      // Match by name
		{"CLAUDE", 1},      // Case insensitive
		{"cli", 2},         // Match in description (Claude and Copilot)
		{"aider", 1},       // Match by ID
		{"AI", 2},          // Match in description
		{"nonexistent", 0}, // No match
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := catalog.Search(tt.query)
			if len(results) != tt.expected {
				t.Errorf("Search(%q) returned %d results, want %d", tt.query, len(results), tt.expected)
			}
		})
	}
}

func TestCatalogValidate(t *testing.T) {
	tests := []struct {
		name      string
		catalog   *Catalog
		wantError bool
	}{
		{
			name:      "valid catalog",
			catalog:   createTestCatalog(),
			wantError: false,
		},
		{
			name: "missing version",
			catalog: &Catalog{
				Version: "",
				Agents: map[string]AgentDef{
					"test": {
						ID:   "test",
						Name: "Test",
						InstallMethods: map[string]InstallMethodDef{
							"npm": {Method: "npm", Platforms: []string{"darwin"}},
						},
						Detection: DetectionDef{Executables: []string{"test"}},
					},
				},
			},
			wantError: true,
		},
		{
			name: "no agents",
			catalog: &Catalog{
				Version: "1.0.0",
				Agents:  map[string]AgentDef{},
			},
			wantError: true,
		},
		{
			name: "agent ID mismatch",
			catalog: &Catalog{
				Version: "1.0.0",
				Agents: map[string]AgentDef{
					"test": {
						ID:   "different-id",
						Name: "Test",
						InstallMethods: map[string]InstallMethodDef{
							"npm": {Method: "npm", Platforms: []string{"darwin"}},
						},
						Detection: DetectionDef{Executables: []string{"test"}},
					},
				},
			},
			wantError: true,
		},
		{
			name: "agent missing name",
			catalog: &Catalog{
				Version: "1.0.0",
				Agents: map[string]AgentDef{
					"test": {
						ID:   "test",
						Name: "",
						InstallMethods: map[string]InstallMethodDef{
							"npm": {Method: "npm", Platforms: []string{"darwin"}},
						},
						Detection: DetectionDef{Executables: []string{"test"}},
					},
				},
			},
			wantError: true,
		},
		{
			name: "agent no install methods",
			catalog: &Catalog{
				Version: "1.0.0",
				Agents: map[string]AgentDef{
					"test": {
						ID:             "test",
						Name:           "Test",
						InstallMethods: map[string]InstallMethodDef{},
						Detection:      DetectionDef{Executables: []string{"test"}},
					},
				},
			},
			wantError: true,
		},
		{
			name: "agent no executables",
			catalog: &Catalog{
				Version: "1.0.0",
				Agents: map[string]AgentDef{
					"test": {
						ID:   "test",
						Name: "Test",
						InstallMethods: map[string]InstallMethodDef{
							"npm": {Method: "npm", Platforms: []string{"darwin"}},
						},
						Detection: DetectionDef{Executables: []string{}},
					},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.catalog.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInstallMethodDef(t *testing.T) {
	method := InstallMethodDef{
		Method:       "npm",
		Package:      "@anthropic-ai/claude-code",
		Command:      "npm install -g @anthropic-ai/claude-code",
		UpdateCmd:    "npm update -g @anthropic-ai/claude-code",
		UninstallCmd: "npm uninstall -g @anthropic-ai/claude-code",
		Platforms:    []string{"darwin", "linux", "windows"},
		GlobalFlag:   "-g",
		PreReqs:      []string{"node", "npm"},
		Metadata: map[string]string{
			"min_version": "18.0.0",
		},
	}

	if method.Method != "npm" {
		t.Errorf("Method = %q, want %q", method.Method, "npm")
	}
	if method.Package != "@anthropic-ai/claude-code" {
		t.Errorf("Package = %q, want %q", method.Package, "@anthropic-ai/claude-code")
	}
	if len(method.Platforms) != 3 {
		t.Errorf("Platforms count = %d, want 3", len(method.Platforms))
	}
	if len(method.PreReqs) != 2 {
		t.Errorf("PreReqs count = %d, want 2", len(method.PreReqs))
	}
	if method.Metadata["min_version"] != "18.0.0" {
		t.Errorf("Metadata[min_version] = %q, want %q", method.Metadata["min_version"], "18.0.0")
	}
}

func TestDetectionDef(t *testing.T) {
	detection := DetectionDef{
		Executables:  []string{"claude", "claude-code"},
		VersionCmd:   "claude --version",
		VersionRegex: `claude-code version ([\d.]+)`,
		Signatures: map[string]SignatureDef{
			"npm": {
				CheckCmd:    "npm list -g @anthropic-ai/claude-code",
				PathPattern: "*/node_modules/@anthropic-ai/claude-code",
			},
		},
	}

	if len(detection.Executables) != 2 {
		t.Errorf("Executables count = %d, want 2", len(detection.Executables))
	}
	if detection.VersionCmd != "claude --version" {
		t.Errorf("VersionCmd = %q, want %q", detection.VersionCmd, "claude --version")
	}
	if detection.VersionRegex != `claude-code version ([\d.]+)` {
		t.Errorf("VersionRegex = %q", detection.VersionRegex)
	}
	if _, ok := detection.Signatures["npm"]; !ok {
		t.Error("Signatures should contain npm key")
	}
}

func TestChangelogDef(t *testing.T) {
	tests := []struct {
		name      string
		changelog ChangelogDef
		checkType string
		checkURL  bool
	}{
		{
			name: "github releases",
			changelog: ChangelogDef{
				Type: "github_releases",
				URL:  "https://api.github.com/repos/anthropics/claude-code/releases",
			},
			checkType: "github_releases",
			checkURL:  true,
		},
		{
			name: "file changelog",
			changelog: ChangelogDef{
				Type:       "file",
				URL:        "https://example.com/CHANGELOG.md",
				FileFormat: "markdown",
			},
			checkType: "file",
			checkURL:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.changelog.Type != tt.checkType {
				t.Errorf("Type = %q, want %q", tt.changelog.Type, tt.checkType)
			}
			if tt.checkURL && tt.changelog.URL == "" {
				t.Error("URL should not be empty")
			}
		})
	}
}

func TestSignatureDef(t *testing.T) {
	sig := SignatureDef{
		CheckCmd:    "npm list -g @anthropic-ai/claude-code",
		PathPattern: "*/node_modules/@anthropic-ai/claude-code",
		Paths:       []string{"/usr/local/lib/node_modules", "/usr/lib/node_modules"},
	}

	if sig.CheckCmd == "" {
		t.Error("CheckCmd should not be empty")
	}
	if sig.PathPattern == "" {
		t.Error("PathPattern should not be empty")
	}
	if len(sig.Paths) != 2 {
		t.Errorf("Paths count = %d, want 2", len(sig.Paths))
	}
}
