// Package grpc provides the gRPC API server for AgentManager.
package grpc

import (
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
)

// Installation represents an installed agent in API format.
type Installation struct {
	Key              string            `json:"key"`
	AgentID          string            `json:"agent_id"`
	AgentName        string            `json:"agent_name"`
	InstallMethod    string            `json:"install_method"`
	InstalledVersion string            `json:"installed_version"`
	LatestVersion    string            `json:"latest_version,omitempty"`
	ExecutablePath   string            `json:"executable_path"`
	InstallPath      string            `json:"install_path,omitempty"`
	IsGlobal         bool              `json:"is_global"`
	DetectedAt       time.Time         `json:"detected_at"`
	LastChecked      time.Time         `json:"last_checked"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	HasUpdate        bool              `json:"has_update"`
	Status           string            `json:"status"`
}

// FromAgentInstallation converts from pkg/agent.Installation to API format.
func FromAgentInstallation(inst *agent.Installation) *Installation {
	if inst == nil {
		return nil
	}

	latestVer := ""
	if inst.LatestVersion != nil {
		latestVer = inst.LatestVersion.String()
	}

	return &Installation{
		Key:              inst.Key(),
		AgentID:          inst.AgentID,
		AgentName:        inst.AgentName,
		InstallMethod:    string(inst.Method),
		InstalledVersion: inst.InstalledVersion.String(),
		LatestVersion:    latestVer,
		ExecutablePath:   inst.ExecutablePath,
		InstallPath:      inst.InstallPath,
		IsGlobal:         inst.IsGlobal,
		DetectedAt:       inst.DetectedAt,
		LastChecked:      inst.LastChecked,
		Metadata:         inst.Metadata,
		HasUpdate:        inst.HasUpdate(),
		Status:           string(inst.GetStatus()),
	}
}

// CatalogAgent represents an agent in the catalog in API format.
type CatalogAgent struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Homepage       string             `json:"homepage,omitempty"`
	Repository     string             `json:"repository,omitempty"`
	InstallMethods []InstallMethodDef `json:"install_methods"`
}

// InstallMethodDef defines an installation method in API format.
type InstallMethodDef struct {
	Method    string   `json:"method"`
	Package   string   `json:"package,omitempty"`
	Command   string   `json:"command"`
	Platforms []string `json:"platforms"`
}

// FromCatalogAgentDef converts from pkg/catalog.AgentDef to API format.
func FromCatalogAgentDef(def *catalog.AgentDef) *CatalogAgent {
	if def == nil {
		return nil
	}

	methods := make([]InstallMethodDef, 0, len(def.InstallMethods))
	for _, m := range def.InstallMethods {
		methods = append(methods, InstallMethodDef{
			Method:    m.Method,
			Package:   m.Package,
			Command:   m.Command,
			Platforms: m.Platforms,
		})
	}

	return &CatalogAgent{
		ID:             def.ID,
		Name:           def.Name,
		Description:    def.Description,
		Homepage:       def.Homepage,
		Repository:     def.Repository,
		InstallMethods: methods,
	}
}

// AgentFilter for listing agents.
type AgentFilter struct {
	AgentIDs  []string `json:"agent_ids,omitempty"`
	Methods   []string `json:"methods,omitempty"`
	HasUpdate *bool    `json:"has_update,omitempty"`
	IsGlobal  *bool    `json:"is_global,omitempty"`
	Query     string   `json:"query,omitempty"`
}

// ListAgentsRequest requests a list of agents.
type ListAgentsRequest struct {
	Filter    *AgentFilter `json:"filter,omitempty"`
	SortBy    string       `json:"sort_by,omitempty"`
	SortOrder string       `json:"sort_order,omitempty"`
	Limit     int          `json:"limit,omitempty"`
	Offset    int          `json:"offset,omitempty"`
}

// ListAgentsResponse contains the list of agents.
type ListAgentsResponse struct {
	Agents []*Installation `json:"agents"`
	Total  int             `json:"total"`
}

// GetAgentRequest requests a specific agent.
type GetAgentRequest struct {
	Key string `json:"key"`
}

// GetAgentResponse contains the agent details.
type GetAgentResponse struct {
	Agent *Installation `json:"agent,omitempty"`
}

// InstallAgentRequest requests agent installation.
type InstallAgentRequest struct {
	AgentID string `json:"agent_id"`
	Method  string `json:"method"`
	Global  bool   `json:"global"`
}

// InstallAgentResponse contains the installation result.
type InstallAgentResponse struct {
	Installation *Installation `json:"installation,omitempty"`
	Success      bool          `json:"success"`
	Message      string        `json:"message,omitempty"`
}

// UpdateAgentRequest requests an agent update.
type UpdateAgentRequest struct {
	Key string `json:"key"`
}

// UpdateAgentResponse contains the update result.
type UpdateAgentResponse struct {
	Installation *Installation `json:"installation,omitempty"`
	FromVersion  string        `json:"from_version"`
	ToVersion    string        `json:"to_version"`
	Success      bool          `json:"success"`
	Message      string        `json:"message,omitempty"`
}

// UninstallAgentRequest requests agent uninstallation.
type UninstallAgentRequest struct {
	Key string `json:"key"`
}

// UninstallAgentResponse contains the uninstallation result.
type UninstallAgentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ListCatalogRequest requests the catalog list.
type ListCatalogRequest struct {
	Platform string `json:"platform,omitempty"`
}

// ListCatalogResponse contains the catalog list.
type ListCatalogResponse struct {
	Agents []*CatalogAgent `json:"agents"`
	Total  int             `json:"total"`
}

// GetCatalogAgentRequest requests a catalog agent.
type GetCatalogAgentRequest struct {
	AgentID string `json:"agent_id"`
}

// GetCatalogAgentResponse contains the catalog agent.
type GetCatalogAgentResponse struct {
	Agent *CatalogAgent `json:"agent,omitempty"`
}

// RefreshCatalogResponse contains the refresh result.
type RefreshCatalogResponse struct {
	Success    bool   `json:"success"`
	Updated    bool   `json:"updated"`
	Message    string `json:"message,omitempty"`
	Version    string `json:"version,omitempty"`
	AgentCount int    `json:"agent_count"`
}

// SearchCatalogRequest requests a catalog search.
type SearchCatalogRequest struct {
	Query    string `json:"query"`
	Platform string `json:"platform,omitempty"`
}

// SearchCatalogResponse contains the search results.
type SearchCatalogResponse struct {
	Agents []*CatalogAgent `json:"agents"`
	Total  int             `json:"total"`
}

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	Installation *Installation `json:"installation"`
	FromVersion  string        `json:"from_version"`
	ToVersion    string        `json:"to_version"`
}

// CheckUpdatesResponse contains update check results.
type CheckUpdatesResponse struct {
	Updates []*UpdateInfo `json:"updates"`
	Total   int           `json:"total"`
}

// GetChangelogRequest requests a changelog.
type GetChangelogRequest struct {
	AgentID     string `json:"agent_id"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
}

// Release represents a single release.
type Release struct {
	Version     string    `json:"version"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	Highlights  []string  `json:"highlights,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	URL         string    `json:"url,omitempty"`
}

// GetChangelogResponse contains the changelog.
type GetChangelogResponse struct {
	Changelog string     `json:"changelog"`
	Releases  []*Release `json:"releases,omitempty"`
}

// StatusResponse contains the service status.
type StatusResponse struct {
	Running            bool      `json:"running"`
	UptimeSeconds      int64     `json:"uptime_seconds"`
	AgentCount         int       `json:"agent_count"`
	UpdatesAvailable   int       `json:"updates_available"`
	LastCatalogRefresh time.Time `json:"last_catalog_refresh"`
	LastUpdateCheck    time.Time `json:"last_update_check"`
	Version            string    `json:"version"`
}

// AgentEvent represents an agent-related event.
type AgentEvent struct {
	Type         string        `json:"type"` // "added", "updated", "removed", "update_available"
	Installation *Installation `json:"installation,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
}
