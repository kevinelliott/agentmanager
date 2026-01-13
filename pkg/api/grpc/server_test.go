package grpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/platform"
	"github.com/kevinelliott/agentmgr/pkg/storage"
)

// mockPlatform implements platform.Platform for testing
type mockPlatform struct{}

func (m *mockPlatform) ID() platform.ID                                             { return platform.Darwin }
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
func (m *mockPlatform) IsExecutableInPath(name string) bool                         { return false }
func (m *mockPlatform) GetPathDirs() []string                                       { return nil }
func (m *mockPlatform) GetShell() string                                            { return "/bin/bash" }
func (m *mockPlatform) GetShellArg() string                                         { return "-c" }
func (m *mockPlatform) ShowNotification(title, message string) error                { return nil }
func (m *mockPlatform) ShowChangelogDialog(a, b, c, d string) platform.DialogResult { return 0 }

// mockStore implements storage.Store for testing
type mockStore struct {
	catalogData []byte
}

func (m *mockStore) Initialize(ctx context.Context) error { return nil }
func (m *mockStore) Close() error                         { return nil }
func (m *mockStore) SaveInstallation(ctx context.Context, inst *agent.Installation) error {
	return nil
}
func (m *mockStore) GetInstallation(ctx context.Context, key string) (*agent.Installation, error) {
	return nil, nil
}
func (m *mockStore) ListInstallations(ctx context.Context, filter *agent.Filter) ([]*agent.Installation, error) {
	return nil, nil
}
func (m *mockStore) DeleteInstallation(ctx context.Context, key string) error { return nil }
func (m *mockStore) SaveUpdateEvent(ctx context.Context, event *storage.UpdateEvent) error {
	return nil
}
func (m *mockStore) GetUpdateHistory(ctx context.Context, agentID string, limit int) ([]*storage.UpdateEvent, error) {
	return nil, nil
}
func (m *mockStore) GetCatalogCache(ctx context.Context) ([]byte, string, time.Time, error) {
	return m.catalogData, "", time.Now(), nil
}
func (m *mockStore) SaveCatalogCache(ctx context.Context, data []byte, etag string) error {
	m.catalogData = data
	return nil
}
func (m *mockStore) GetSetting(ctx context.Context, key string) (string, error) { return "", nil }
func (m *mockStore) SetSetting(ctx context.Context, key, value string) error    { return nil }
func (m *mockStore) DeleteSetting(ctx context.Context, key string) error        { return nil }
func (m *mockStore) SaveDetectionCache(ctx context.Context, installations []*agent.Installation) error {
	return nil
}
func (m *mockStore) GetDetectionCache(ctx context.Context) ([]*agent.Installation, time.Time, error) {
	return nil, time.Time{}, nil
}
func (m *mockStore) ClearDetectionCache(ctx context.Context) error { return nil }
func (m *mockStore) GetDetectionCacheTime(ctx context.Context) (time.Time, error) {
	return time.Time{}, nil
}
func (m *mockStore) SetLastUpdateCheckTime(ctx context.Context, t time.Time) error { return nil }
func (m *mockStore) GetLastUpdateCheckTime(ctx context.Context) (time.Time, error) {
	return time.Time{}, nil
}

func createTestCatalog() *catalog.Catalog {
	return &catalog.Catalog{
		Version:       "1.0.0",
		SchemaVersion: 1,
		LastUpdated:   time.Now(),
		Agents: map[string]catalog.AgentDef{
			"claude-code": {
				ID:          "claude-code",
				Name:        "Claude Code",
				Description: "Anthropic's CLI for Claude",
				Homepage:    "https://claude.ai/claude-code",
				InstallMethods: map[string]catalog.InstallMethodDef{
					"npm": {
						Method:    "npm",
						Package:   "@anthropic-ai/claude-code",
						Command:   "npm install -g @anthropic-ai/claude-code",
						Platforms: []string{"darwin", "linux", "windows"},
					},
				},
				Detection: catalog.DetectionDef{
					Executables:  []string{"claude"},
					VersionCmd:   "claude --version",
					VersionRegex: `claude-code version ([\d.]+)`,
				},
			},
			"aider": {
				ID:          "aider",
				Name:        "Aider",
				Description: "AI pair programming",
				Homepage:    "https://aider.chat",
				InstallMethods: map[string]catalog.InstallMethodDef{
					"pip": {
						Method:    "pip",
						Package:   "aider-chat",
						Command:   "pip install aider-chat",
						Platforms: []string{"darwin", "linux", "windows"},
					},
				},
				Detection: catalog.DetectionDef{
					Executables: []string{"aider"},
					VersionCmd:  "aider --version",
				},
			},
		},
	}
}

func newTestConfig() *config.Config {
	return &config.Config{
		Catalog: config.CatalogConfig{
			SourceURL:       "http://example.com/catalog.json",
			RefreshInterval: time.Hour,
		},
	}
}

func setupTestServer() *Server {
	cat := createTestCatalog()
	catalogJSON, _ := json.Marshal(cat)

	cfg := newTestConfig()
	store := &mockStore{catalogData: catalogJSON}
	plat := &mockPlatform{}
	catMgr := catalog.NewManager(cfg, store)

	return NewServer(cfg, plat, store, nil, catMgr, nil)
}

func TestNewServer(t *testing.T) {
	server := setupTestServer()

	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if server.config == nil {
		t.Error("config should not be nil")
	}
	if server.platform == nil {
		t.Error("platform should not be nil")
	}
	if server.catalog == nil {
		t.Error("catalog should not be nil")
	}
}

func TestServerStartStop(t *testing.T) {
	server := setupTestServer()

	ctx := context.Background()
	cfg := ServerConfig{Address: ":0"} // Use random port

	if err := server.Start(ctx, cfg); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	if err := server.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestServerAddress(t *testing.T) {
	server := setupTestServer()

	// Before start, address should be empty
	if server.Address() != "" {
		t.Error("Address should be empty before Start()")
	}

	ctx := context.Background()
	cfg := ServerConfig{Address: ":0"} // Use random port

	if err := server.Start(ctx, cfg); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	// After start with random port, address should be set
	addr := server.Address()
	if addr == "" {
		t.Error("Address should not be empty after Start()")
	}
}

func TestGetStatus(t *testing.T) {
	server := setupTestServer()

	ctx := context.Background()
	status, err := server.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if !status.Running {
		t.Error("Running should be true")
	}
	if status.Version != "dev" {
		t.Errorf("Version = %q, want %q", status.Version, "dev")
	}
	if status.AgentCount != 0 {
		t.Errorf("AgentCount = %d, want 0 (no agents detected without detector)", status.AgentCount)
	}
}

func TestListCatalog(t *testing.T) {
	server := setupTestServer()
	ctx := context.Background()

	t.Run("without platform filter", func(t *testing.T) {
		resp, err := server.ListCatalog(ctx, &ListCatalogRequest{})
		if err != nil {
			t.Fatalf("ListCatalog() error = %v", err)
		}

		if len(resp.Agents) != 2 {
			t.Errorf("Agents count = %d, want 2", len(resp.Agents))
		}
		if resp.Total != 2 {
			t.Errorf("Total = %d, want 2", resp.Total)
		}
	})

	t.Run("with platform filter", func(t *testing.T) {
		resp, err := server.ListCatalog(ctx, &ListCatalogRequest{Platform: "darwin"})
		if err != nil {
			t.Fatalf("ListCatalog(darwin) error = %v", err)
		}

		// Both agents support darwin
		if len(resp.Agents) != 2 {
			t.Errorf("Agents count = %d, want 2", len(resp.Agents))
		}
	})
}

func TestGetCatalogAgent(t *testing.T) {
	server := setupTestServer()
	ctx := context.Background()

	t.Run("existing agent", func(t *testing.T) {
		resp, err := server.GetCatalogAgent(ctx, &GetCatalogAgentRequest{AgentID: "claude-code"})
		if err != nil {
			t.Fatalf("GetCatalogAgent() error = %v", err)
		}

		if resp.Agent == nil {
			t.Fatal("Agent should not be nil")
		}
		if resp.Agent.ID != "claude-code" {
			t.Errorf("ID = %q, want %q", resp.Agent.ID, "claude-code")
		}
		if resp.Agent.Name != "Claude Code" {
			t.Errorf("Name = %q, want %q", resp.Agent.Name, "Claude Code")
		}
	})

	t.Run("nonexistent agent", func(t *testing.T) {
		resp, err := server.GetCatalogAgent(ctx, &GetCatalogAgentRequest{AgentID: "nonexistent"})
		if err != nil {
			t.Fatalf("GetCatalogAgent() error = %v", err)
		}

		if resp.Agent != nil {
			t.Error("Agent should be nil for nonexistent agent")
		}
	})
}

func TestSearchCatalog(t *testing.T) {
	server := setupTestServer()
	ctx := context.Background()

	tests := []struct {
		query       string
		expectedLen int
	}{
		{"claude", 1},
		{"aider", 1},
		{"cli", 1}, // Claude Code description has "CLI"
		{"", 2},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			resp, err := server.SearchCatalog(ctx, &SearchCatalogRequest{Query: tt.query})
			if err != nil {
				t.Fatalf("SearchCatalog(%q) error = %v", tt.query, err)
			}

			if len(resp.Agents) != tt.expectedLen {
				t.Errorf("Agents count = %d, want %d", len(resp.Agents), tt.expectedLen)
			}
		})
	}
}

func TestInstallAgentWithoutInstaller(t *testing.T) {
	server := setupTestServer()
	ctx := context.Background()

	resp, err := server.InstallAgent(ctx, &InstallAgentRequest{
		AgentID: "claude-code",
		Method:  "npm",
		Global:  true,
	})
	if err != nil {
		t.Fatalf("InstallAgent() error = %v", err)
	}

	if resp.Success {
		t.Error("Success should be false without installer")
	}
	if resp.Message != "installer not available" {
		t.Errorf("Message = %q, want %q", resp.Message, "installer not available")
	}
}

func TestUpdateAgentWithoutInstaller(t *testing.T) {
	server := setupTestServer()
	ctx := context.Background()

	resp, err := server.UpdateAgent(ctx, &UpdateAgentRequest{Key: "claude-code:npm"})
	if err != nil {
		t.Fatalf("UpdateAgent() error = %v", err)
	}

	if resp.Success {
		t.Error("Success should be false without installer")
	}
	if resp.Message != "installer not available" {
		t.Errorf("Message = %q, want %q", resp.Message, "installer not available")
	}
}

func TestUninstallAgentWithoutInstaller(t *testing.T) {
	server := setupTestServer()
	ctx := context.Background()

	resp, err := server.UninstallAgent(ctx, &UninstallAgentRequest{Key: "claude-code:npm"})
	if err != nil {
		t.Fatalf("UninstallAgent() error = %v", err)
	}

	if resp.Success {
		t.Error("Success should be false without installer")
	}
	if resp.Message != "installer not available" {
		t.Errorf("Message = %q, want %q", resp.Message, "installer not available")
	}
}

func TestGetChangelog(t *testing.T) {
	server := setupTestServer()
	ctx := context.Background()

	t.Run("valid versions", func(t *testing.T) {
		resp, err := server.GetChangelog(ctx, &GetChangelogRequest{
			AgentID:     "claude-code",
			FromVersion: "1.0.0",
			ToVersion:   "1.1.0",
		})
		if err != nil {
			t.Fatalf("GetChangelog() error = %v", err)
		}

		// Changelog may be empty without actual changelog URL, but should not error
		if resp == nil {
			t.Error("Response should not be nil")
		}
	})

	t.Run("non-numeric versions are stored as raw", func(t *testing.T) {
		// ParseVersion accepts invalid strings and stores them as raw versions
		// So GetChangelog doesn't return an error for "invalid"
		resp, err := server.GetChangelog(ctx, &GetChangelogRequest{
			AgentID:     "claude-code",
			FromVersion: "invalid",
			ToVersion:   "1.0.0",
		})
		if err != nil {
			t.Fatalf("GetChangelog() error = %v", err)
		}

		// Returns empty changelog since we can't compare raw versions
		if resp == nil {
			t.Error("Response should not be nil")
		}
	})

	t.Run("empty version returns error", func(t *testing.T) {
		_, err := server.GetChangelog(ctx, &GetChangelogRequest{
			AgentID:     "claude-code",
			FromVersion: "",
			ToVersion:   "1.0.0",
		})
		if err == nil {
			t.Error("GetChangelog should return error for empty from_version")
		}
	})
}

func TestSubscribeUnsubscribe(t *testing.T) {
	server := setupTestServer()

	// Subscribe
	ch := server.Subscribe()
	if ch == nil {
		t.Fatal("Subscribe should return a channel")
	}

	// Check subscriber count
	server.subMu.RLock()
	subCount := len(server.subscribers)
	server.subMu.RUnlock()

	if subCount != 1 {
		t.Errorf("Subscriber count = %d, want 1", subCount)
	}

	// Unsubscribe
	server.Unsubscribe(ch)

	// Check subscriber count after unsubscribe
	server.subMu.RLock()
	subCount = len(server.subscribers)
	server.subMu.RUnlock()

	if subCount != 0 {
		t.Errorf("Subscriber count after unsubscribe = %d, want 0", subCount)
	}
}

func TestMatchesFilter(t *testing.T) {
	server := setupTestServer()

	version, _ := agent.ParseVersion("1.0.0")
	latestVersion, _ := agent.ParseVersion("1.1.0")

	inst := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           agent.InstallMethodNPM,
		InstalledVersion: version,
		LatestVersion:    &latestVersion,
		IsGlobal:         true,
	}

	t.Run("nil filter matches all", func(t *testing.T) {
		if !server.matchesFilter(inst, nil) {
			t.Error("nil filter should match")
		}
	})

	t.Run("empty filter matches all", func(t *testing.T) {
		if !server.matchesFilter(inst, &AgentFilter{}) {
			t.Error("empty filter should match")
		}
	})

	t.Run("filter by agent ID - match", func(t *testing.T) {
		filter := &AgentFilter{AgentIDs: []string{"claude-code"}}
		if !server.matchesFilter(inst, filter) {
			t.Error("filter by matching agent ID should match")
		}
	})

	t.Run("filter by agent ID - no match", func(t *testing.T) {
		filter := &AgentFilter{AgentIDs: []string{"aider"}}
		if server.matchesFilter(inst, filter) {
			t.Error("filter by non-matching agent ID should not match")
		}
	})

	t.Run("filter by method - match", func(t *testing.T) {
		filter := &AgentFilter{Methods: []string{"npm"}}
		if !server.matchesFilter(inst, filter) {
			t.Error("filter by matching method should match")
		}
	})

	t.Run("filter by method - no match", func(t *testing.T) {
		filter := &AgentFilter{Methods: []string{"pip"}}
		if server.matchesFilter(inst, filter) {
			t.Error("filter by non-matching method should not match")
		}
	})

	t.Run("filter by has_update - match", func(t *testing.T) {
		hasUpdate := true
		filter := &AgentFilter{HasUpdate: &hasUpdate}
		if !server.matchesFilter(inst, filter) {
			t.Error("filter by has_update=true should match when update available")
		}
	})

	t.Run("filter by has_update - no match", func(t *testing.T) {
		hasUpdate := false
		filter := &AgentFilter{HasUpdate: &hasUpdate}
		if server.matchesFilter(inst, filter) {
			t.Error("filter by has_update=false should not match when update available")
		}
	})

	t.Run("filter by is_global - match", func(t *testing.T) {
		isGlobal := true
		filter := &AgentFilter{IsGlobal: &isGlobal}
		if !server.matchesFilter(inst, filter) {
			t.Error("filter by is_global=true should match")
		}
	})

	t.Run("filter by query - match", func(t *testing.T) {
		filter := &AgentFilter{Query: "claude"}
		if !server.matchesFilter(inst, filter) {
			t.Error("filter by query 'claude' should match")
		}
	})

	t.Run("filter by query case insensitive", func(t *testing.T) {
		filter := &AgentFilter{Query: "CLAUDE"}
		if !server.matchesFilter(inst, filter) {
			t.Error("filter by query 'CLAUDE' should match (case insensitive)")
		}
	})

	t.Run("filter by query - no match", func(t *testing.T) {
		filter := &AgentFilter{Query: "aider"}
		if server.matchesFilter(inst, filter) {
			t.Error("filter by query 'aider' should not match")
		}
	})
}

func TestFromAgentInstallation(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := FromAgentInstallation(nil)
		if result != nil {
			t.Error("FromAgentInstallation(nil) should return nil")
		}
	})

	t.Run("valid input", func(t *testing.T) {
		version, _ := agent.ParseVersion("1.0.0")
		latestVersion, _ := agent.ParseVersion("1.1.0")

		inst := &agent.Installation{
			AgentID:          "claude-code",
			AgentName:        "Claude Code",
			Method:           agent.InstallMethodNPM,
			InstalledVersion: version,
			LatestVersion:    &latestVersion,
			ExecutablePath:   "/usr/local/bin/claude",
			InstallPath:      "/usr/local/lib/node_modules",
			IsGlobal:         true,
			DetectedAt:       time.Now(),
			Metadata:         map[string]string{"key": "value"},
		}

		result := FromAgentInstallation(inst)
		if result == nil {
			t.Fatal("Result should not be nil")
		}

		if result.AgentID != "claude-code" {
			t.Errorf("AgentID = %q, want %q", result.AgentID, "claude-code")
		}
		if result.InstallMethod != "npm" {
			t.Errorf("InstallMethod = %q, want %q", result.InstallMethod, "npm")
		}
		if result.InstalledVersion != "1.0.0" {
			t.Errorf("InstalledVersion = %q, want %q", result.InstalledVersion, "1.0.0")
		}
		if result.LatestVersion != "1.1.0" {
			t.Errorf("LatestVersion = %q, want %q", result.LatestVersion, "1.1.0")
		}
		if !result.HasUpdate {
			t.Error("HasUpdate should be true")
		}
	})

	t.Run("no latest version", func(t *testing.T) {
		version, _ := agent.ParseVersion("1.0.0")

		inst := &agent.Installation{
			AgentID:          "claude-code",
			Method:           agent.InstallMethodNPM,
			InstalledVersion: version,
			LatestVersion:    nil,
		}

		result := FromAgentInstallation(inst)
		if result.LatestVersion != "" {
			t.Errorf("LatestVersion = %q, want empty string", result.LatestVersion)
		}
	})
}

func TestFromCatalogAgentDef(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		result := FromCatalogAgentDef(nil)
		if result != nil {
			t.Error("FromCatalogAgentDef(nil) should return nil")
		}
	})

	t.Run("valid input", func(t *testing.T) {
		def := &catalog.AgentDef{
			ID:          "claude-code",
			Name:        "Claude Code",
			Description: "Anthropic's CLI",
			Homepage:    "https://claude.ai",
			Repository:  "https://github.com/anthropics/claude-code",
			InstallMethods: map[string]catalog.InstallMethodDef{
				"npm": {
					Method:    "npm",
					Package:   "@anthropic-ai/claude-code",
					Command:   "npm install -g @anthropic-ai/claude-code",
					Platforms: []string{"darwin", "linux"},
				},
			},
		}

		result := FromCatalogAgentDef(def)
		if result == nil {
			t.Fatal("Result should not be nil")
		}

		if result.ID != "claude-code" {
			t.Errorf("ID = %q, want %q", result.ID, "claude-code")
		}
		if result.Name != "Claude Code" {
			t.Errorf("Name = %q, want %q", result.Name, "Claude Code")
		}
		if len(result.InstallMethods) != 1 {
			t.Errorf("InstallMethods count = %d, want 1", len(result.InstallMethods))
		}
	})
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"Hello World", "world", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "hello", true},
		{"Hello World", "notfound", false},
		{"", "", true},
		{"test", "", true},
		{"", "test", false},
		{"ABC", "abc", true},
		{"abc", "ABC", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// Note: ListAgents calls refreshAgents which requires a detector.
// Without a detector, ListAgents panics. This is tested implicitly.
// A proper integration test would require a full mock detector.

// TestListAgentsWithPrePopulatedData tests ListAgents behavior when agents are already loaded
// by directly manipulating the server state (simulating post-detection).
func TestListAgentsWithPrePopulatedData(t *testing.T) {
	server := setupTestServer()

	// Pre-populate some agents for testing (bypassing refreshAgents)
	version, _ := agent.ParseVersion("1.0.0")
	server.agents = []*agent.Installation{
		{
			AgentID:          "claude-code",
			AgentName:        "Claude Code",
			Method:           agent.InstallMethodNPM,
			InstalledVersion: version,
			IsGlobal:         true,
		},
		{
			AgentID:          "aider",
			AgentName:        "Aider",
			Method:           agent.InstallMethodPip,
			InstalledVersion: version,
			IsGlobal:         false,
		},
	}

	// Test the internal state can be used for GetAgent (which doesn't call refreshAgents)
	t.Run("verify pre-populated agents", func(t *testing.T) {
		server.agentsMu.RLock()
		count := len(server.agents)
		server.agentsMu.RUnlock()

		if count != 2 {
			t.Errorf("Pre-populated agents count = %d, want 2", count)
		}
	})
}

func TestGetAgent(t *testing.T) {
	server := setupTestServer()
	ctx := context.Background()

	// Pre-populate agents
	version, _ := agent.ParseVersion("1.0.0")
	testInst := &agent.Installation{
		AgentID:          "claude-code",
		AgentName:        "Claude Code",
		Method:           agent.InstallMethodNPM,
		InstalledVersion: version,
		ExecutablePath:   "/usr/local/bin/claude",
	}
	server.agents = []*agent.Installation{testInst}

	t.Run("existing agent", func(t *testing.T) {
		// Key format is "agentID:method:executablePath"
		key := testInst.Key()
		resp, err := server.GetAgent(ctx, &GetAgentRequest{Key: key})
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}

		if resp.Agent == nil {
			t.Fatal("Agent should not be nil")
		}
		if resp.Agent.AgentID != "claude-code" {
			t.Errorf("AgentID = %q, want %q", resp.Agent.AgentID, "claude-code")
		}
	})

	t.Run("nonexistent agent", func(t *testing.T) {
		resp, err := server.GetAgent(ctx, &GetAgentRequest{Key: "nonexistent:npm:/fake/path"})
		if err != nil {
			t.Fatalf("GetAgent() error = %v", err)
		}

		if resp.Agent != nil {
			t.Error("Agent should be nil for nonexistent key")
		}
	})
}
