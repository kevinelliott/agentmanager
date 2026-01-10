package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/storage"
)

// Manager manages the agent catalog.
type Manager struct {
	config  *config.Config
	store   storage.Store
	catalog *Catalog
	mu      sync.RWMutex

	// HTTP client for fetching remote catalog
	httpClient *http.Client
}

// NewManager creates a new catalog manager.
func NewManager(cfg *config.Config, store storage.Store) *Manager {
	return &Manager{
		config: cfg,
		store:  store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get returns the current catalog, loading from cache or embedded if needed.
func (m *Manager) Get(ctx context.Context) (*Catalog, error) {
	m.mu.RLock()
	if m.catalog != nil {
		defer m.mu.RUnlock()
		return m.catalog, nil
	}
	m.mu.RUnlock()

	// Try to load from cache
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if m.catalog != nil {
		return m.catalog, nil
	}

	// Try cached catalog first
	if cached, err := m.loadFromCache(ctx); err == nil && cached != nil {
		m.catalog = cached
		return m.catalog, nil
	}

	// Fall back to embedded catalog
	if embedded, err := m.loadEmbedded(); err == nil && embedded != nil {
		m.catalog = embedded
		return m.catalog, nil
	}

	return nil, fmt.Errorf("no catalog available")
}

// RefreshResult contains the result of a catalog refresh operation.
type RefreshResult struct {
	Updated        bool   // Whether the catalog was updated
	CurrentVersion string // The current catalog version after refresh
	RemoteVersion  string // The remote catalog version that was fetched
}

// Refresh fetches the latest catalog from the remote source.
// It only updates if the remote version is newer than the current version.
// Returns a RefreshResult indicating whether an update occurred.
func (m *Manager) Refresh(ctx context.Context) (*RefreshResult, error) {
	remoteCatalog, err := m.fetchRemote(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote catalog: %w", err)
	}

	// Validate the remote catalog
	if err := remoteCatalog.Validate(); err != nil {
		return nil, fmt.Errorf("invalid remote catalog: %w", err)
	}

	result := &RefreshResult{
		RemoteVersion: remoteCatalog.Version,
	}

	// Get current catalog (if available) and compare versions
	currentCatalog, _ := m.Get(ctx)
	if currentCatalog != nil {
		result.CurrentVersion = currentCatalog.Version

		if currentCatalog.Version != "" && remoteCatalog.Version != "" {
			currentVersion, currentErr := agent.ParseVersion(currentCatalog.Version)
			remoteVersion, remoteErr := agent.ParseVersion(remoteCatalog.Version)

			// Only skip update if both versions parse successfully and current is >= remote
			if currentErr == nil && remoteErr == nil {
				if !remoteVersion.IsNewerThan(currentVersion) {
					// Remote is not newer, no update needed
					result.Updated = false
					return result, nil
				}
			}
		}
	}

	// Save to cache
	if err := m.saveToCache(ctx, remoteCatalog); err != nil {
		// Log but don't fail - we have the catalog in memory
	}

	m.mu.Lock()
	m.catalog = remoteCatalog
	m.mu.Unlock()

	result.Updated = true
	result.CurrentVersion = remoteCatalog.Version
	return result, nil
}

// GetAgent returns a specific agent definition.
func (m *Manager) GetAgent(ctx context.Context, id string) (*AgentDef, error) {
	catalog, err := m.Get(ctx)
	if err != nil {
		return nil, err
	}

	agent, ok := catalog.GetAgent(id)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", id)
	}

	return &agent, nil
}

// GetLatestVersion returns the latest version for an agent and installation method.
func (m *Manager) GetLatestVersion(ctx context.Context, agentID, method string) (*agent.Version, error) {
	agentDef, err := m.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Get version from changelog source
	if agentDef.Changelog.Type == "github_releases" {
		return m.getLatestGitHubVersion(ctx, agentDef.Changelog.URL)
	}

	// For other types, we'd need to implement specific handlers
	return nil, fmt.Errorf("unsupported changelog type: %s", agentDef.Changelog.Type)
}

// GetChangelog fetches the changelog between two versions.
func (m *Manager) GetChangelog(ctx context.Context, agentID string, from, to agent.Version) (string, error) {
	agentDef, err := m.GetAgent(ctx, agentID)
	if err != nil {
		return "", err
	}

	if agentDef.Changelog.Type == "github_releases" {
		return m.getGitHubChangelog(ctx, agentDef.Changelog.URL, from, to)
	}

	return "", fmt.Errorf("unsupported changelog type: %s", agentDef.Changelog.Type)
}

// Search searches the catalog for agents matching the query.
func (m *Manager) Search(ctx context.Context, query string) ([]AgentDef, error) {
	catalog, err := m.Get(ctx)
	if err != nil {
		return nil, err
	}

	return catalog.Search(query), nil
}

// GetAgentsForPlatform returns all agents supported on the given platform.
func (m *Manager) GetAgentsForPlatform(ctx context.Context, platformID string) ([]AgentDef, error) {
	catalog, err := m.Get(ctx)
	if err != nil {
		return nil, err
	}

	return catalog.GetAgentsByPlatform(platformID), nil
}

// loadFromCache loads the catalog from storage cache.
func (m *Manager) loadFromCache(ctx context.Context) (*Catalog, error) {
	data, _, _, err := m.store.GetCatalogCache(ctx)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("no cached catalog")
	}

	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}

	return &catalog, nil
}

// saveToCache saves the catalog to storage cache.
func (m *Manager) saveToCache(ctx context.Context, catalog *Catalog) error {
	data, err := json.Marshal(catalog)
	if err != nil {
		return err
	}

	// Use version as etag for cache validation
	return m.store.SaveCatalogCache(ctx, data, catalog.Version)
}

// loadEmbedded loads the embedded default catalog.
func (m *Manager) loadEmbedded() (*Catalog, error) {
	// Try to read from file in current directory or known locations
	paths := []string{
		"catalog.json",
		"/usr/local/share/agentmgr/catalog.json",
		"/etc/agentmgr/catalog.json",
	}

	// Add user home directory paths
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			home+"/.agentmgr/catalog.json",
			home+"/.config/agentmgr/catalog.json",
		)
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var catalog Catalog
		if err := json.Unmarshal(data, &catalog); err != nil {
			continue
		}

		return &catalog, nil
	}

	return nil, fmt.Errorf("no embedded catalog found")
}

// fetchRemote fetches the catalog from the remote URL.
func (m *Manager) fetchRemote(ctx context.Context) (*Catalog, error) {
	url := m.config.Catalog.SourceURL
	if url == "" {
		return nil, fmt.Errorf("no catalog source URL configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "AgentManager/1.0")
	req.Header.Set("Accept", "application/json")

	// Add GitHub token if configured
	if m.config.Catalog.GitHubToken != "" {
		req.Header.Set("Authorization", "token "+m.config.Catalog.GitHubToken)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, err
	}

	return &catalog, nil
}

// getLatestGitHubVersion fetches the latest version from GitHub releases.
func (m *Manager) getLatestGitHubVersion(ctx context.Context, apiURL string) (*agent.Version, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "AgentManager/1.0")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	if m.config.Catalog.GitHubToken != "" {
		req.Header.Set("Authorization", "token "+m.config.Catalog.GitHubToken)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var releases []struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	// Parse the latest release tag
	tag := releases[0].TagName
	// Remove 'v' prefix if present
	if len(tag) > 0 && tag[0] == 'v' {
		tag = tag[1:]
	}

	version, err := agent.ParseVersion(tag)
	if err != nil {
		return nil, fmt.Errorf("failed to parse version %s: %w", tag, err)
	}

	return &version, nil
}

// getGitHubChangelog fetches changelog from GitHub releases between two versions.
func (m *Manager) getGitHubChangelog(ctx context.Context, apiURL string, from, to agent.Version) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "AgentManager/1.0")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	if m.config.Catalog.GitHubToken != "" {
		req.Header.Set("Authorization", "token "+m.config.Catalog.GitHubToken)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var releases []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		Body        string    `json:"body"`
		PublishedAt time.Time `json:"published_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", err
	}

	var changelog string
	for _, release := range releases {
		tag := release.TagName
		if len(tag) > 0 && tag[0] == 'v' {
			tag = tag[1:]
		}

		version, err := agent.ParseVersion(tag)
		if err != nil {
			continue
		}

		// Include releases between from and to versions
		if version.IsNewerThan(from) && !version.IsNewerThan(to) {
			if changelog != "" {
				changelog += "\n\n---\n\n"
			}
			changelog += fmt.Sprintf("## %s\n\n%s", release.Name, release.Body)
		}
	}

	return changelog, nil
}
