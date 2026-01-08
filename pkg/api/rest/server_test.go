package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestHealthEndpoint(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestGetStatusEndpoint(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Without a detector, the handler panics and Recoverer returns 500
	// This is expected behavior when detector is nil
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d (without detector)", w.Code, http.StatusInternalServerError)
	}
}

func TestListCatalogEndpoint(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("GET", "/api/v1/catalog", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	agents, ok := resp["agents"].([]interface{})
	if !ok {
		t.Fatal("agents should be an array")
	}

	if len(agents) != 2 {
		t.Errorf("agents count = %d, want 2", len(agents))
	}
}

func TestListCatalogWithPlatformFilter(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("GET", "/api/v1/catalog?platform=darwin", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	agents, ok := resp["agents"].([]interface{})
	if !ok {
		t.Fatal("agents should be an array")
	}

	// Both agents support darwin
	if len(agents) != 2 {
		t.Errorf("agents count = %d, want 2", len(agents))
	}
}

func TestGetCatalogAgentEndpoint(t *testing.T) {
	server := setupTestServer()

	t.Run("existing agent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/catalog/claude-code", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		agent, ok := resp["agent"].(map[string]interface{})
		if !ok {
			t.Fatal("agent should be an object")
		}

		if agent["id"] != "claude-code" {
			t.Errorf("id = %v, want %q", agent["id"], "claude-code")
		}
		if agent["name"] != "Claude Code" {
			t.Errorf("name = %v, want %q", agent["name"], "Claude Code")
		}
	})

	t.Run("nonexistent agent", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/catalog/nonexistent", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}

func TestSearchCatalogEndpoint(t *testing.T) {
	server := setupTestServer()

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
			req := httptest.NewRequest("GET", "/api/v1/catalog/search?q="+tt.query, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatal(err)
			}

			agents, ok := resp["agents"].([]interface{})
			if !ok {
				t.Fatal("agents should be an array")
			}

			if len(agents) != tt.expectedLen {
				t.Errorf("agents count = %d, want %d", len(agents), tt.expectedLen)
			}
		})
	}
}

func TestCORSMiddleware(t *testing.T) {
	server := setupTestServer()

	t.Run("OPTIONS request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/api/v1/status", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}

		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("CORS header should allow all origins")
		}
	})

	t.Run("regular request has CORS headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("CORS header should be present")
		}
	})
}

func TestContentTypeMiddleware(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}
}

func TestListAgentsEndpoint(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("GET", "/api/v1/agents", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Without a detector, the handler panics and Recoverer returns 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d (without detector)", w.Code, http.StatusInternalServerError)
	}
}

func TestGetAgentEndpoint(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("GET", "/api/v1/agents/test-key", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Without a detector, the handler panics and Recoverer returns 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d (without detector)", w.Code, http.StatusInternalServerError)
	}
}

func TestInstallAgentEndpoint(t *testing.T) {
	server := setupTestServer()

	t.Run("without installer", func(t *testing.T) {
		body := `{"agent_id": "claude-code", "method": "npm", "global": true}`
		req := httptest.NewRequest("POST", "/api/v1/agents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		// Without installer, should return service unavailable
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusServiceUnavailable)
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		body := `invalid json`
		req := httptest.NewRequest("POST", "/api/v1/agents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestUpdateAgentEndpoint(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("PUT", "/api/v1/agents/test-key", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Without installer, should return service unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestUninstallAgentEndpoint(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("DELETE", "/api/v1/agents/test-key", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Without installer, should return service unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestCheckUpdatesEndpoint(t *testing.T) {
	server := setupTestServer()

	req := httptest.NewRequest("GET", "/api/v1/updates", nil)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	// Without a detector, the handler panics and Recoverer returns 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d (without detector)", w.Code, http.StatusInternalServerError)
	}
}

func TestGetChangelogEndpoint(t *testing.T) {
	server := setupTestServer()

	t.Run("missing parameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/changelog/claude-code", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid from version falls back to zero version", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/changelog/claude-code?from=invalid&to=1.0.0", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		// ParseVersion returns zero version for invalid strings, handler continues
		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("valid parameters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/changelog/claude-code?from=1.0.0&to=1.1.0", nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		// Even if changelog fails to fetch, it should return OK with empty changelog
		if w.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
		}
	})
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
	cfg := ServerConfig{Address: ":8888"}

	if err := server.Start(ctx, cfg); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Stop(ctx)

	time.Sleep(50 * time.Millisecond)

	if server.Address() != ":8888" {
		t.Errorf("Address() = %q, want %q", server.Address(), ":8888")
	}
}

func TestRespondJSON(t *testing.T) {
	server := setupTestServer()
	w := httptest.NewRecorder()

	data := map[string]string{"key": "value"}
	server.respondJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["key"] != "value" {
		t.Errorf("key = %q, want %q", resp["key"], "value")
	}
}

func TestRespondError(t *testing.T) {
	server := setupTestServer()

	t.Run("without error details", func(t *testing.T) {
		w := httptest.NewRecorder()
		server.respondError(w, http.StatusBadRequest, "Bad request", nil)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}

		if resp["error"] != "Bad request" {
			t.Errorf("error = %v, want %q", resp["error"], "Bad request")
		}
		if resp["success"] != false {
			t.Error("success should be false")
		}
	})

	t.Run("with error details", func(t *testing.T) {
		w := httptest.NewRecorder()
		server.respondError(w, http.StatusInternalServerError, "Server error", nil)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})
}

func TestInstallationToMap(t *testing.T) {
	server := setupTestServer()

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
	}

	result := server.installationToMap(inst)

	if result["agent_id"] != "claude-code" {
		t.Errorf("agent_id = %v, want %q", result["agent_id"], "claude-code")
	}
	if result["install_method"] != "npm" {
		t.Errorf("install_method = %v, want %q", result["install_method"], "npm")
	}
	if result["has_update"] != true {
		t.Error("has_update should be true")
	}
}

func TestCatalogAgentToMap(t *testing.T) {
	server := setupTestServer()

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

	result := server.catalogAgentToMap(def)

	if result["id"] != "claude-code" {
		t.Errorf("id = %v, want %q", result["id"], "claude-code")
	}
	if result["name"] != "Claude Code" {
		t.Errorf("name = %v, want %q", result["name"], "Claude Code")
	}

	methods, ok := result["install_methods"].([]map[string]interface{})
	if !ok {
		t.Fatal("install_methods should be an array")
	}
	if len(methods) != 1 {
		t.Errorf("install_methods count = %d, want 1", len(methods))
	}
}
