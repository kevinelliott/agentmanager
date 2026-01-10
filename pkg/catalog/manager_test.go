package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/storage"
)

// mockStore implements storage.Store for testing
type mockStore struct {
	catalogData []byte
	catalogEtag string
	err         error
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
	if m.err != nil {
		return nil, "", time.Time{}, m.err
	}
	return m.catalogData, m.catalogEtag, time.Now(), nil
}

func (m *mockStore) SaveCatalogCache(ctx context.Context, data []byte, etag string) error {
	m.catalogData = data
	m.catalogEtag = etag
	return m.err
}

func (m *mockStore) GetSetting(ctx context.Context, key string) (string, error) { return "", nil }
func (m *mockStore) SetSetting(ctx context.Context, key, value string) error    { return nil }
func (m *mockStore) DeleteSetting(ctx context.Context, key string) error        { return nil }

func newTestConfig() *config.Config {
	return &config.Config{
		Catalog: config.CatalogConfig{
			SourceURL:       "http://example.com/catalog.json",
			RefreshInterval: time.Hour,
			RefreshOnStart:  true,
		},
	}
}

func TestNewManager(t *testing.T) {
	cfg := newTestConfig()
	store := &mockStore{}

	mgr := NewManager(cfg, store)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.config != cfg {
		t.Error("config not set correctly")
	}
	if mgr.store != store {
		t.Error("store not set correctly")
	}
	if mgr.httpClient == nil {
		t.Error("httpClient should be initialized")
	}
}

func TestManagerGetFromCache(t *testing.T) {
	catalog := createTestCatalog()
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig()
	store := &mockStore{catalogData: data}
	mgr := NewManager(cfg, store)

	ctx := context.Background()
	result, err := mgr.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if result.Version != catalog.Version {
		t.Errorf("Version = %q, want %q", result.Version, catalog.Version)
	}
	if len(result.Agents) != len(catalog.Agents) {
		t.Errorf("Agents count = %d, want %d", len(result.Agents), len(catalog.Agents))
	}
}

func TestManagerGetCached(t *testing.T) {
	catalog := createTestCatalog()
	data, _ := json.Marshal(catalog)

	cfg := newTestConfig()
	store := &mockStore{catalogData: data}
	mgr := NewManager(cfg, store)

	ctx := context.Background()

	// First call loads from cache
	result1, err := mgr.Get(ctx)
	if err != nil {
		t.Fatalf("First Get() error = %v", err)
	}

	// Modify store to return different data
	newCatalog := createTestCatalog()
	newCatalog.Version = "2.0.0"
	store.catalogData, _ = json.Marshal(newCatalog)

	// Second call should return cached data (same version)
	result2, err := mgr.Get(ctx)
	if err != nil {
		t.Fatalf("Second Get() error = %v", err)
	}

	if result1.Version != result2.Version {
		t.Errorf("Cached catalog not returned: got %q, want %q", result2.Version, result1.Version)
	}
}

func TestManagerGetAgent(t *testing.T) {
	catalog := createTestCatalog()
	data, _ := json.Marshal(catalog)

	cfg := newTestConfig()
	store := &mockStore{catalogData: data}
	mgr := NewManager(cfg, store)

	ctx := context.Background()

	// Get existing agent
	agent, err := mgr.GetAgent(ctx, "claude-code")
	if err != nil {
		t.Fatalf("GetAgent(claude-code) error = %v", err)
	}
	if agent.Name != "Claude Code" {
		t.Errorf("Name = %q, want %q", agent.Name, "Claude Code")
	}

	// Get non-existing agent
	_, err = mgr.GetAgent(ctx, "nonexistent")
	if err == nil {
		t.Error("GetAgent(nonexistent) should return error")
	}
}

func TestManagerSearch(t *testing.T) {
	catalog := createTestCatalog()
	data, _ := json.Marshal(catalog)

	cfg := newTestConfig()
	store := &mockStore{catalogData: data}
	mgr := NewManager(cfg, store)

	ctx := context.Background()

	tests := []struct {
		query    string
		expected int
	}{
		{"claude", 1},
		{"aider", 1},
		{"cli", 2},
		{"", 3},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results, err := mgr.Search(ctx, tt.query)
			if err != nil {
				t.Fatalf("Search(%q) error = %v", tt.query, err)
			}
			if len(results) != tt.expected {
				t.Errorf("Search(%q) returned %d results, want %d", tt.query, len(results), tt.expected)
			}
		})
	}
}

func TestManagerGetAgentsForPlatform(t *testing.T) {
	catalog := createTestCatalog()
	data, _ := json.Marshal(catalog)

	cfg := newTestConfig()
	store := &mockStore{catalogData: data}
	mgr := NewManager(cfg, store)

	ctx := context.Background()

	tests := []struct {
		platform string
		expected int
	}{
		{"darwin", 3},
		{"linux", 3},
		{"windows", 3},
		{"freebsd", 0},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			results, err := mgr.GetAgentsForPlatform(ctx, tt.platform)
			if err != nil {
				t.Fatalf("GetAgentsForPlatform(%q) error = %v", tt.platform, err)
			}
			if len(results) != tt.expected {
				t.Errorf("GetAgentsForPlatform(%q) returned %d agents, want %d", tt.platform, len(results), tt.expected)
			}
		})
	}
}

func TestManagerRefresh(t *testing.T) {
	catalog := createTestCatalog()
	catalogJSON, _ := json.Marshal(catalog)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(catalogJSON)
	}))
	defer server.Close()

	cfg := newTestConfig()
	cfg.Catalog.SourceURL = server.URL + "/catalog.json"
	store := &mockStore{}
	mgr := NewManager(cfg, store)

	ctx := context.Background()
	_, err := mgr.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Verify catalog was saved to cache
	if store.catalogData == nil {
		t.Error("Catalog should be saved to cache")
	}

	// Verify we can get the refreshed catalog
	result, err := mgr.Get(ctx)
	if err != nil {
		t.Fatalf("Get() after Refresh() error = %v", err)
	}
	if result.Version != catalog.Version {
		t.Errorf("Version = %q, want %q", result.Version, catalog.Version)
	}
}

func TestManagerRefreshInvalidCatalog(t *testing.T) {
	// Create test server that returns invalid catalog
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version": "", "agents": {}}`))
	}))
	defer server.Close()

	cfg := newTestConfig()
	cfg.Catalog.SourceURL = server.URL + "/catalog.json"
	store := &mockStore{}
	mgr := NewManager(cfg, store)

	ctx := context.Background()
	_, err := mgr.Refresh(ctx)
	if err == nil {
		t.Error("Refresh() should return error for invalid catalog")
	}
}

func TestManagerRefreshHTTPError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := newTestConfig()
	cfg.Catalog.SourceURL = server.URL + "/catalog.json"
	store := &mockStore{}
	mgr := NewManager(cfg, store)

	ctx := context.Background()
	_, err := mgr.Refresh(ctx)
	if err == nil {
		t.Error("Refresh() should return error on HTTP error")
	}
}

func TestManagerGetLatestVersion(t *testing.T) {
	// Create mock GitHub releases response
	releases := []struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
	}{
		{TagName: "v1.2.3", Name: "Release 1.2.3"},
		{TagName: "v1.2.2", Name: "Release 1.2.2"},
	}
	releasesJSON, _ := json.Marshal(releases)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/anthropics/claude-code/releases" {
			w.Header().Set("Content-Type", "application/json")
			w.Write(releasesJSON)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	catalog := createTestCatalog()
	catalog.Agents["claude-code"] = AgentDef{
		ID:   "claude-code",
		Name: "Claude Code",
		InstallMethods: map[string]InstallMethodDef{
			"npm": {Method: "npm", Platforms: []string{"darwin"}},
		},
		Detection: DetectionDef{Executables: []string{"claude"}},
		Changelog: ChangelogDef{
			Type: "github_releases",
			URL:  server.URL + "/repos/anthropics/claude-code/releases",
		},
	}
	data, _ := json.Marshal(catalog)

	cfg := newTestConfig()
	store := &mockStore{catalogData: data}
	mgr := NewManager(cfg, store)

	ctx := context.Background()
	version, err := mgr.GetLatestVersion(ctx, "claude-code", "npm")
	if err != nil {
		t.Fatalf("GetLatestVersion() error = %v", err)
	}

	if version.String() != "1.2.3" {
		t.Errorf("Version = %q, want %q", version.String(), "1.2.3")
	}
}

func TestManagerGetChangelog(t *testing.T) {
	// Create mock GitHub releases response
	releases := []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		Body        string    `json:"body"`
		PublishedAt time.Time `json:"published_at"`
	}{
		{TagName: "v1.2.3", Name: "Release 1.2.3", Body: "Bug fixes", PublishedAt: time.Now()},
		{TagName: "v1.2.2", Name: "Release 1.2.2", Body: "New features", PublishedAt: time.Now().Add(-24 * time.Hour)},
		{TagName: "v1.2.1", Name: "Release 1.2.1", Body: "Initial release", PublishedAt: time.Now().Add(-48 * time.Hour)},
	}
	releasesJSON, _ := json.Marshal(releases)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(releasesJSON)
	}))
	defer server.Close()

	catalog := createTestCatalog()
	catalog.Agents["claude-code"] = AgentDef{
		ID:   "claude-code",
		Name: "Claude Code",
		InstallMethods: map[string]InstallMethodDef{
			"npm": {Method: "npm", Platforms: []string{"darwin"}},
		},
		Detection: DetectionDef{Executables: []string{"claude"}},
		Changelog: ChangelogDef{
			Type: "github_releases",
			URL:  server.URL + "/repos/anthropics/claude-code/releases",
		},
	}
	data, _ := json.Marshal(catalog)

	cfg := newTestConfig()
	store := &mockStore{catalogData: data}
	mgr := NewManager(cfg, store)

	ctx := context.Background()

	// Get changelog from 1.2.1 to 1.2.3
	from, _ := agent.ParseVersion("1.2.1")
	to, _ := agent.ParseVersion("1.2.3")
	changelog, err := mgr.GetChangelog(ctx, "claude-code", from, to)
	if err != nil {
		t.Fatalf("GetChangelog() error = %v", err)
	}

	if changelog == "" {
		t.Error("Changelog should not be empty")
	}
	if !contains(changelog, "Bug fixes") {
		t.Error("Changelog should contain 'Bug fixes'")
	}
	if !contains(changelog, "New features") {
		t.Error("Changelog should contain 'New features'")
	}
}

func TestManagerLoadEmbedded(t *testing.T) {
	// Create a temp directory with a catalog.json
	tmpDir := t.TempDir()
	catalog := createTestCatalog()
	data, _ := json.Marshal(catalog)

	// Change to temp directory so loadEmbedded can find catalog.json
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Write catalog.json
	err := os.WriteFile(filepath.Join(tmpDir, "catalog.json"), data, 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig()
	store := &mockStore{} // Empty cache
	mgr := NewManager(cfg, store)

	ctx := context.Background()
	result, err := mgr.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if result.Version != catalog.Version {
		t.Errorf("Version = %q, want %q", result.Version, catalog.Version)
	}
}

func TestManagerWithGitHubToken(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		catalog := createTestCatalog()
		data, _ := json.Marshal(catalog)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer server.Close()

	cfg := newTestConfig()
	cfg.Catalog.SourceURL = server.URL + "/catalog.json"
	cfg.Catalog.GitHubToken = "test-token-123"
	store := &mockStore{}
	mgr := NewManager(cfg, store)

	ctx := context.Background()
	_, err := mgr.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if receivedAuth != "token test-token-123" {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, "token test-token-123")
	}
}

// Helper to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
