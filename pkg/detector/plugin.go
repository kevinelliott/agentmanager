// Package detector provides agent detection capabilities.
package detector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// PluginConfig defines the configuration for a detection plugin.
type PluginConfig struct {
	// Name is the unique identifier for this plugin.
	Name string `json:"name" yaml:"name"`

	// Description describes what this plugin detects.
	Description string `json:"description" yaml:"description"`

	// Method is the installation method this plugin detects (e.g., "custom", "docker").
	Method string `json:"method" yaml:"method"`

	// Platforms specifies which platforms this plugin runs on (e.g., ["darwin", "linux"]).
	// If empty, the plugin runs on all platforms.
	Platforms []string `json:"platforms,omitempty" yaml:"platforms,omitempty"`

	// DetectCommand is the command to run to detect agents.
	// The command should output JSON with the detected agents.
	DetectCommand string `json:"detect_command" yaml:"detect_command"`

	// DetectScript is an inline script to run for detection (alternative to DetectCommand).
	DetectScript string `json:"detect_script,omitempty" yaml:"detect_script,omitempty"`

	// AgentFilter is a list of agent IDs this plugin handles.
	// If empty, the plugin is offered all agents.
	AgentFilter []string `json:"agent_filter,omitempty" yaml:"agent_filter,omitempty"`

	// Enabled controls whether this plugin is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// PluginDetectionResult is the expected output format from a detection plugin.
type PluginDetectionResult struct {
	// Agents is the list of detected agent installations.
	Agents []PluginAgentResult `json:"agents"`
}

// PluginAgentResult represents a single detected agent from a plugin.
type PluginAgentResult struct {
	// AgentID is the agent identifier (must match catalog).
	AgentID string `json:"agent_id"`

	// Version is the detected version string.
	Version string `json:"version"`

	// ExecutablePath is the path to the agent executable.
	ExecutablePath string `json:"executable_path,omitempty"`

	// InstallPath is the installation directory.
	InstallPath string `json:"install_path,omitempty"`

	// Metadata contains additional detection metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PluginStrategy implements the Strategy interface for script-based plugins.
type PluginStrategy struct {
	config   PluginConfig
	platform platform.Platform
}

// NewPluginStrategy creates a new plugin-based detection strategy.
func NewPluginStrategy(cfg PluginConfig, p platform.Platform) *PluginStrategy {
	return &PluginStrategy{
		config:   cfg,
		platform: p,
	}
}

// Name returns the strategy name.
func (s *PluginStrategy) Name() string {
	return s.config.Name
}

// Method returns the install method this strategy detects.
func (s *PluginStrategy) Method() agent.InstallMethod {
	return agent.InstallMethod(s.config.Method)
}

// IsApplicable returns true if this strategy can run on the given platform.
func (s *PluginStrategy) IsApplicable(p platform.Platform) bool {
	if !s.config.Enabled {
		return false
	}
	if len(s.config.Platforms) == 0 {
		return true
	}
	platformID := string(p.ID())
	for _, supported := range s.config.Platforms {
		if strings.EqualFold(supported, platformID) {
			return true
		}
	}
	return false
}

// Detect runs the plugin and returns found installations.
func (s *PluginStrategy) Detect(ctx context.Context, agents []catalog.AgentDef) ([]*agent.Installation, error) {
	// Filter agents if configured
	var filteredAgents []catalog.AgentDef
	if len(s.config.AgentFilter) > 0 {
		filterSet := make(map[string]bool)
		for _, id := range s.config.AgentFilter {
			filterSet[id] = true
		}
		for _, a := range agents {
			if filterSet[a.ID] {
				filteredAgents = append(filteredAgents, a)
			}
		}
	} else {
		filteredAgents = agents
	}

	if len(filteredAgents) == 0 {
		return nil, nil
	}

	// Prepare agent IDs for the script
	agentIDs := make([]string, len(filteredAgents))
	for i, a := range filteredAgents {
		agentIDs[i] = a.ID
	}

	// Build command
	var cmd *exec.Cmd
	if s.config.DetectScript != "" {
		// Run inline script
		shell := s.platform.GetShell()
		shellArg := s.platform.GetShellArg()
		cmd = exec.CommandContext(ctx, shell, shellArg, s.config.DetectScript)
	} else if s.config.DetectCommand != "" {
		// Run external command
		// Note: This intentionally executes user-provided commands from plugin config files.
		// Plugins are opt-in and stored in user-controlled directories.
		parts := strings.Fields(s.config.DetectCommand)
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty detect command")
		}
		cmd = exec.CommandContext(ctx, parts[0], parts[1:]...) //nolint:gosec // intentional: plugins execute user-defined commands
	} else {
		return nil, fmt.Errorf("plugin %s has no detect_command or detect_script", s.config.Name)
	}

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"AGENTMGR_AGENT_IDS="+strings.Join(agentIDs, ","),
		"AGENTMGR_PLATFORM="+string(s.platform.ID()),
	)

	// Run and capture output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plugin %s failed: %w", s.config.Name, err)
	}

	// Parse JSON output
	var result PluginDetectionResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("plugin %s: invalid JSON output: %w", s.config.Name, err)
	}

	// Build agent lookup map
	agentDefMap := make(map[string]catalog.AgentDef)
	for _, a := range filteredAgents {
		agentDefMap[a.ID] = a
	}

	// Convert results to installations
	var installations []*agent.Installation
	for _, r := range result.Agents {
		agentDef, ok := agentDefMap[r.AgentID]
		if !ok {
			continue
		}

		version, err := agent.ParseVersion(r.Version)
		if err != nil {
			continue
		}

		inst := &agent.Installation{
			AgentID:          r.AgentID,
			AgentName:        agentDef.Name,
			Method:           agent.InstallMethod(s.config.Method),
			InstalledVersion: version,
			ExecutablePath:   r.ExecutablePath,
			InstallPath:      r.InstallPath,
			Metadata:         r.Metadata,
		}
		installations = append(installations, inst)
	}

	return installations, nil
}

// PluginRegistry manages detection plugins.
type PluginRegistry struct {
	plugins  map[string]PluginConfig
	mu       sync.RWMutex
	platform platform.Platform
}

// NewPluginRegistry creates a new plugin registry.
func NewPluginRegistry(p platform.Platform) *PluginRegistry {
	return &PluginRegistry{
		plugins:  make(map[string]PluginConfig),
		platform: p,
	}
}

// Register adds a plugin to the registry.
func (r *PluginRegistry) Register(cfg PluginConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if cfg.Method == "" {
		return fmt.Errorf("plugin method is required")
	}
	if cfg.DetectCommand == "" && cfg.DetectScript == "" {
		return fmt.Errorf("plugin must have detect_command or detect_script")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[cfg.Name] = cfg
	return nil
}

// Unregister removes a plugin from the registry.
func (r *PluginRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, name)
}

// Get returns a plugin by name.
func (r *PluginRegistry) Get(name string) (PluginConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.plugins[name]
	return cfg, ok
}

// List returns all registered plugins.
func (r *PluginRegistry) List() []PluginConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	plugins := make([]PluginConfig, 0, len(r.plugins))
	for _, cfg := range r.plugins {
		plugins = append(plugins, cfg)
	}
	return plugins
}

// GetStrategies returns Strategy instances for all enabled plugins.
func (r *PluginRegistry) GetStrategies() []Strategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var strategies []Strategy
	for _, cfg := range r.plugins {
		if cfg.Enabled {
			strategies = append(strategies, NewPluginStrategy(cfg, r.platform))
		}
	}
	return strategies
}

// LoadPluginsFromDir loads plugin configurations from a directory.
// Plugin files should be JSON files with the .plugin.json extension.
func (r *PluginRegistry) LoadPluginsFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No plugins directory is fine
		}
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".plugin.json") {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var cfg PluginConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}

		if err := r.Register(cfg); err != nil {
			continue
		}
	}

	return nil
}

// ValidatePlugin validates a plugin configuration.
func ValidatePlugin(cfg PluginConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9_-]*$`).MatchString(cfg.Name) {
		return fmt.Errorf("plugin name must start with a letter and contain only lowercase letters, numbers, hyphens, and underscores")
	}
	if cfg.Method == "" {
		return fmt.Errorf("plugin method is required")
	}
	if cfg.DetectCommand == "" && cfg.DetectScript == "" {
		return fmt.Errorf("plugin must have detect_command or detect_script")
	}
	return nil
}
