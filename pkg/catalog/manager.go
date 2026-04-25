package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/logging"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

// Manager manages the agent catalog.
type Manager struct {
	config  *config.Config
	store   storage.Store
	catalog *Catalog
	mu      sync.RWMutex

	// HTTP client for fetching remote catalog
	httpClient *http.Client

	// refreshGroup serializes concurrent Refresh() calls. Without it, two
	// concurrent refreshes would both hit the network and race on the cache
	// write — last writer wins.
	refreshGroup singleflight.Group
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
//
// Concurrent calls to Refresh coalesce via a singleflight group — only one
// HTTP fetch runs at a time; other callers receive the same result.
func (m *Manager) Refresh(ctx context.Context) (*RefreshResult, error) {
	v, err, _ := m.refreshGroup.Do("catalog-refresh", func() (interface{}, error) {
		return m.doRefresh(ctx)
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	result, ok := v.(*RefreshResult)
	if !ok {
		return nil, fmt.Errorf("unexpected singleflight result type %T", v)
	}
	return result, nil
}

// doRefresh is the un-coalesced Refresh implementation. It must only be called
// via the singleflight group (see Refresh).
func (m *Manager) doRefresh(ctx context.Context) (*RefreshResult, error) {
	// Load the previous etag (if any) so we can send If-None-Match.
	prevEtag := m.previousEtag(ctx)

	remoteCatalog, newEtag, notModified, err := m.fetchRemote(ctx, prevEtag)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote catalog: %w", err)
	}

	// If the server responded 304, our cached copy is still fresh.
	if notModified {
		currentCatalog, _ := m.Get(ctx) //nolint:errcheck
		result := &RefreshResult{Updated: false}
		if currentCatalog != nil {
			result.CurrentVersion = currentCatalog.Version
			result.RemoteVersion = currentCatalog.Version
		}
		return result, nil
	}

	// Validate the remote catalog
	if err := remoteCatalog.Validate(); err != nil {
		return nil, fmt.Errorf("invalid remote catalog: %w", err)
	}

	result := &RefreshResult{
		RemoteVersion: remoteCatalog.Version,
	}

	// Get current catalog (if available) and compare versions
	currentCatalog, _ := m.Get(ctx) //nolint:errcheck // best-effort; nil catalog is handled below
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

	// Save to cache. Prefer the server-provided ETag (so subsequent Refresh
	// calls can send If-None-Match); fall back to the catalog version to
	// preserve the pre-existing behavior.
	etagToStore := newEtag
	if etagToStore == "" {
		etagToStore = remoteCatalog.Version
	}
	if err := m.store.SaveCatalogCache(ctx, mustMarshal(remoteCatalog), etagToStore); err != nil {
		// Non-fatal: we still have the catalog in memory. Use the
		// request-scoped logger so operators can tag refresh events
		// (e.g. with a trigger source) at the call site.
		logging.FromContext(ctx).Warn("catalog: failed to persist cache",
			"err", err,
			"version", remoteCatalog.Version,
		)
	}

	m.mu.Lock()
	m.catalog = remoteCatalog
	m.mu.Unlock()

	result.Updated = true
	result.CurrentVersion = remoteCatalog.Version
	return result, nil
}

// previousEtag best-effort reads the etag from the previous cache entry.
// Missing / unreadable cache yields the empty string.
func (m *Manager) previousEtag(ctx context.Context) string {
	_, etag, _, err := m.store.GetCatalogCache(ctx)
	if err != nil {
		return ""
	}
	return etag
}

// mustMarshal is saveToCache's marshal step surfaced inline so we can log on
// persistence failure while still writing the same bytes.
func mustMarshal(c *Catalog) []byte {
	data, err := json.Marshal(c)
	if err != nil {
		// Marshal can only fail on cycles / unsupported types, which Catalog
		// is not. Log and return empty — the store layer will treat as empty.
		slog.Warn("catalog: failed to marshal catalog", "err", err)
		return nil
	}
	return data
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

// loadEmbedded returns the baseline catalog that ships with the binary,
// allowing user-scoped on-disk overrides to win when present.
//
// Resolution order:
//  1. User-override paths ($HOME/.agentmgr/catalog.json,
//     $HOME/.config/agentmgr/catalog.json) — lets power users pin a
//     modified catalog without rebuilding.
//  2. System-wide install share (/usr/local/share/agentmgr/catalog.json,
//     /etc/agentmgr/catalog.json) — populated by goreleaser packaging.
//  3. The go:embed'd catalog compiled into the binary (see embed.go).
//
// The current working directory is intentionally NOT probed. A stray
// catalog.json in whatever directory the user happened to invoke agentmgr
// from would silently shadow the real catalog.
func (m *Manager) loadEmbedded() (*Catalog, error) {
	paths := make([]string, 0, 4)

	// 1. User-scoped overrides.
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			home+"/.agentmgr/catalog.json",
			home+"/.config/agentmgr/catalog.json",
		)
	}

	// 2. System-wide install share. `/usr/share/agentmgr/catalog.json` is
	// where the goreleaser nfpm config installs the catalog for .deb/.rpm
	// users; `/usr/local/share` is the common install prefix for manual
	// installs; `/etc/agentmgr` is the system-wide override location.
	paths = append(paths,
		"/usr/share/agentmgr/catalog.json",
		"/usr/local/share/agentmgr/catalog.json",
		"/etc/agentmgr/catalog.json",
	)

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

	// 3. Baseline: the catalog compiled into the binary at build time.
	if len(embeddedCatalogJSON) > 0 {
		var catalog Catalog
		if err := json.Unmarshal(embeddedCatalogJSON, &catalog); err != nil {
			return nil, fmt.Errorf("invalid embedded catalog: %w", err)
		}
		return &catalog, nil
	}

	return nil, fmt.Errorf("no embedded catalog found")
}

// fetchRemote fetches the catalog from the remote URL. If prevEtag is
// non-empty it is sent as If-None-Match so the server may respond 304 Not
// Modified — in that case the returned catalog is nil and notModified=true.
// On 200 the returned etag is the server's current ETag header (may be empty).
func (m *Manager) fetchRemote(ctx context.Context, prevEtag string) (*Catalog, string, bool, error) {
	url := m.config.Catalog.SourceURL
	if url == "" {
		return nil, "", false, fmt.Errorf("no catalog source URL configured")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", false, err
	}

	req.Header.Set("User-Agent", "AgentManager/1.0")
	req.Header.Set("Accept", "application/json")
	if prevEtag != "" {
		req.Header.Set("If-None-Match", prevEtag)
	}

	// Add GitHub token if configured
	if m.config.Catalog.GitHubToken != "" {
		req.Header.Set("Authorization", "token "+m.config.Catalog.GitHubToken)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()

	// 304 Not Modified: caller should keep using its cached catalog.
	if resp.StatusCode == http.StatusNotModified {
		return nil, prevEtag, true, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", false, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", false, err
	}

	var catalog Catalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, "", false, err
	}

	return &catalog, resp.Header.Get("ETag"), false, nil
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
