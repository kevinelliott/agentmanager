// Package catalog provides catalog management for AI development agents.
package catalog

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Catalog represents the complete agent catalog.
type Catalog struct {
	Version       string              `json:"version"`
	SchemaVersion int                 `json:"schema_version"`
	LastUpdated   time.Time           `json:"last_updated"`
	Agents        map[string]AgentDef `json:"agents"`
}

// AgentDef defines an agent in the catalog.
type AgentDef struct {
	ID             string                      `json:"id"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description"`
	Homepage       string                      `json:"homepage,omitempty"`
	Repository     string                      `json:"repository,omitempty"`
	Documentation  string                      `json:"documentation,omitempty"`
	Icon           string                      `json:"icon,omitempty"`
	InstallMethods map[string]InstallMethodDef `json:"install_methods"`
	Detection      DetectionDef                `json:"detection"`
	Changelog      ChangelogDef                `json:"changelog,omitempty"`
	Metadata       map[string]string           `json:"metadata,omitempty"`
}

// InstallMethodDef defines how to install via a specific method.
type InstallMethodDef struct {
	Method       string            `json:"method"`
	Package      string            `json:"package,omitempty"`
	Command      string            `json:"command"`
	UpdateCmd    string            `json:"update_cmd,omitempty"`
	UninstallCmd string            `json:"uninstall_cmd,omitempty"`
	Platforms    []string          `json:"platforms"`
	GlobalFlag   string            `json:"global_flag,omitempty"`
	PreReqs      []string          `json:"prereqs,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// DetectionDef defines how to detect an agent.
type DetectionDef struct {
	Executables  []string                `json:"executables"`
	VersionCmd   string                  `json:"version_cmd"`
	VersionRegex string                  `json:"version_regex,omitempty"`
	Signatures   map[string]SignatureDef `json:"signatures,omitempty"`
}

// SignatureDef defines detection signatures for a specific install method.
type SignatureDef struct {
	CheckCmd    string   `json:"check_cmd,omitempty"`
	PathPattern string   `json:"path_pattern,omitempty"`
	Paths       []string `json:"paths,omitempty"`
}

// ChangelogDef defines where to fetch changelogs.
type ChangelogDef struct {
	Type       string `json:"type"` // "github_releases", "file", "api"
	URL        string `json:"url"`
	FileFormat string `json:"file_format,omitempty"` // "markdown", "json", "plain"
}

// IsSupported returns true if the agent is supported on the given platform.
func (a AgentDef) IsSupported(platformID string) bool {
	for _, method := range a.InstallMethods {
		for _, p := range method.Platforms {
			if p == platformID {
				return true
			}
		}
	}
	return false
}

// GetInstallMethod returns the install method definition for the given method.
func (a AgentDef) GetInstallMethod(method string) (InstallMethodDef, bool) {
	m, ok := a.InstallMethods[method]
	return m, ok
}

// methodPriority returns a sort priority for installation methods.
// Lower values are preferred. Package managers (npm, pip, brew) are preferred
// over native installers for easier management and updates.
func methodPriority(method string) int {
	priorities := map[string]int{
		"npm":        1,
		"pip":        2,
		"pipx":       3,
		"uv":         4,
		"brew":       5,
		"bun":        6,
		"go":         7,
		"scoop":      8,
		"winget":     9,
		"chocolatey": 10,
		"krew":       11,
		"binary":     12,
		"native":     20, // Native installers are less preferred
		"powershell": 21,
		"dmg":        22,
	}
	if p, ok := priorities[method]; ok {
		return p
	}
	return 15 // Unknown methods get medium priority
}

// GetSupportedMethods returns all installation methods supported on the given platform.
// Methods are sorted by preference, with package managers (npm, pip, brew) preferred
// over native installers for easier management and updates.
func (a AgentDef) GetSupportedMethods(platformID string) []InstallMethodDef {
	var methods []InstallMethodDef
	for _, method := range a.InstallMethods {
		for _, p := range method.Platforms {
			if p == platformID {
				methods = append(methods, method)
				break
			}
		}
	}

	// Sort methods by priority (prefer package managers over native)
	sort.Slice(methods, func(i, j int) bool {
		return methodPriority(methods[i].Method) < methodPriority(methods[j].Method)
	})

	return methods
}

// GetExecutable returns the primary executable name for this agent.
func (a AgentDef) GetExecutable() string {
	if len(a.Detection.Executables) > 0 {
		return a.Detection.Executables[0]
	}
	return ""
}

// GetAgents returns all agents in the catalog.
func (c *Catalog) GetAgents() []AgentDef {
	agents := make([]AgentDef, 0, len(c.Agents))
	for _, agent := range c.Agents {
		agents = append(agents, agent)
	}
	return agents
}

// GetAgent returns a specific agent by ID.
func (c *Catalog) GetAgent(id string) (AgentDef, bool) {
	agent, ok := c.Agents[id]
	return agent, ok
}

// GetAgentsByPlatform returns all agents supported on the given platform.
func (c *Catalog) GetAgentsByPlatform(platformID string) []AgentDef {
	var agents []AgentDef
	for _, agent := range c.Agents {
		if agent.IsSupported(platformID) {
			agents = append(agents, agent)
		}
	}
	return agents
}

// Search searches agents by name or description.
func (c *Catalog) Search(query string) []AgentDef {
	if query == "" {
		return c.GetAgents()
	}

	query = strings.ToLower(query)
	var results []AgentDef
	for _, agent := range c.Agents {
		if strings.Contains(strings.ToLower(agent.Name), query) ||
			strings.Contains(strings.ToLower(agent.Description), query) ||
			strings.Contains(strings.ToLower(agent.ID), query) {
			results = append(results, agent)
		}
	}
	return results
}

// Validate validates the catalog structure.
func (c *Catalog) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("catalog version is required")
	}
	if len(c.Agents) == 0 {
		return fmt.Errorf("catalog has no agents")
	}

	for id, agent := range c.Agents {
		if agent.ID != id {
			return fmt.Errorf("agent ID mismatch: %s != %s", agent.ID, id)
		}
		if agent.Name == "" {
			return fmt.Errorf("agent %s has no name", id)
		}
		if len(agent.InstallMethods) == 0 {
			return fmt.Errorf("agent %s has no install methods", id)
		}
		if len(agent.Detection.Executables) == 0 {
			return fmt.Errorf("agent %s has no executables defined", id)
		}
	}

	return nil
}
