// Package config provides configuration management for AgentManager.
package config

import (
	"time"
)

// Config represents the application configuration.
type Config struct {
	// Catalog settings
	Catalog CatalogConfig `yaml:"catalog" json:"catalog" mapstructure:"catalog"`

	// Update settings
	Updates UpdateConfig `yaml:"updates" json:"updates" mapstructure:"updates"`

	// UI settings
	UI UIConfig `yaml:"ui" json:"ui" mapstructure:"ui"`

	// API settings
	API APIConfig `yaml:"api" json:"api" mapstructure:"api"`

	// Logging settings
	Logging LoggingConfig `yaml:"logging" json:"logging" mapstructure:"logging"`

	// Agent-specific overrides
	Agents map[string]AgentConfig `yaml:"agents" json:"agents" mapstructure:"agents"`
}

// CatalogConfig contains catalog-related settings.
type CatalogConfig struct {
	// SourceURL is the URL to fetch the catalog from
	SourceURL string `yaml:"source_url" json:"source_url" mapstructure:"source_url"`

	// RefreshInterval is how often to refresh in background
	RefreshInterval time.Duration `yaml:"refresh_interval" json:"refresh_interval" mapstructure:"refresh_interval"`

	// RefreshOnStart enables auto-refresh when the app starts
	RefreshOnStart bool `yaml:"refresh_on_start" json:"refresh_on_start" mapstructure:"refresh_on_start"`

	// GitHubToken is an optional token for higher API rate limits
	GitHubToken string `yaml:"github_token" json:"github_token" mapstructure:"github_token"`
}

// UpdateConfig contains update-related settings.
type UpdateConfig struct {
	// AutoCheck enables automatic update checking
	AutoCheck bool `yaml:"auto_check" json:"auto_check" mapstructure:"auto_check"`

	// CheckInterval is how often to check for updates
	CheckInterval time.Duration `yaml:"check_interval" json:"check_interval" mapstructure:"check_interval"`

	// Notify enables desktop notifications for updates
	Notify bool `yaml:"notify" json:"notify" mapstructure:"notify"`

	// AutoUpdate enables automatic updating (use with caution)
	AutoUpdate bool `yaml:"auto_update" json:"auto_update" mapstructure:"auto_update"`

	// ExcludeAgents lists agents to exclude from auto-update
	ExcludeAgents []string `yaml:"exclude_agents" json:"exclude_agents" mapstructure:"exclude_agents"`
}

// UIConfig contains UI-related settings.
type UIConfig struct {
	// Theme is the TUI theme name
	Theme string `yaml:"theme" json:"theme" mapstructure:"theme"`

	// ShowHidden shows hidden agents in listings
	ShowHidden bool `yaml:"show_hidden" json:"show_hidden" mapstructure:"show_hidden"`

	// PageSize is the number of items per page in tables
	PageSize int `yaml:"page_size" json:"page_size" mapstructure:"page_size"`

	// UseColors enables colored output
	UseColors bool `yaml:"use_colors" json:"use_colors" mapstructure:"use_colors"`

	// CompactMode reduces whitespace in output
	CompactMode bool `yaml:"compact_mode" json:"compact_mode" mapstructure:"compact_mode"`
}

// APIConfig contains API server settings.
type APIConfig struct {
	// EnableGRPC enables the gRPC server
	EnableGRPC bool `yaml:"enable_grpc" json:"enable_grpc" mapstructure:"enable_grpc"`

	// GRPCPort is the port for the gRPC server
	GRPCPort int `yaml:"grpc_port" json:"grpc_port" mapstructure:"grpc_port"`

	// EnableREST enables the REST server
	EnableREST bool `yaml:"enable_rest" json:"enable_rest" mapstructure:"enable_rest"`

	// RESTPort is the port for the REST server
	RESTPort int `yaml:"rest_port" json:"rest_port" mapstructure:"rest_port"`

	// RequireAuth requires authentication for API calls
	RequireAuth bool `yaml:"require_auth" json:"require_auth" mapstructure:"require_auth"`

	// AuthToken is the authentication token
	AuthToken string `yaml:"auth_token" json:"auth_token" mapstructure:"auth_token"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	// Level is the log level (debug, info, warn, error)
	Level string `yaml:"level" json:"level" mapstructure:"level"`

	// Format is the log format (json, text)
	Format string `yaml:"format" json:"format" mapstructure:"format"`

	// File is an optional log file path
	File string `yaml:"file" json:"file" mapstructure:"file"`

	// MaxSize is the max size in MB before rotation
	MaxSize int `yaml:"max_size" json:"max_size" mapstructure:"max_size"`

	// MaxAge is the max days to keep old logs
	MaxAge int `yaml:"max_age" json:"max_age" mapstructure:"max_age"`
}

// AgentConfig contains per-agent configuration overrides.
type AgentConfig struct {
	// PreferredMethod is the preferred installation method
	PreferredMethod string `yaml:"preferred_method" json:"preferred_method" mapstructure:"preferred_method"`

	// CustomPaths are additional paths to check for this agent
	CustomPaths []string `yaml:"custom_paths" json:"custom_paths" mapstructure:"custom_paths"`

	// Hidden hides this agent from listings
	Hidden bool `yaml:"hidden" json:"hidden" mapstructure:"hidden"`

	// PinnedVersion prevents updates past this version
	PinnedVersion string `yaml:"pinned_version" json:"pinned_version" mapstructure:"pinned_version"`

	// Disabled prevents detection and management
	Disabled bool `yaml:"disabled" json:"disabled" mapstructure:"disabled"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Catalog: CatalogConfig{
			SourceURL:       "https://raw.githubusercontent.com/kevinelliott/agentmgr/main/catalog.json",
			RefreshInterval: time.Hour,
			RefreshOnStart:  true,
			GitHubToken:     "",
		},
		Updates: UpdateConfig{
			AutoCheck:     true,
			CheckInterval: 6 * time.Hour,
			Notify:        true,
			AutoUpdate:    false,
			ExcludeAgents: []string{},
		},
		UI: UIConfig{
			Theme:       "default",
			ShowHidden:  false,
			PageSize:    20,
			UseColors:   true,
			CompactMode: false,
		},
		API: APIConfig{
			EnableGRPC:  false,
			GRPCPort:    50051,
			EnableREST:  false,
			RESTPort:    8080,
			RequireAuth: false,
			AuthToken:   "",
		},
		Logging: LoggingConfig{
			Level:   "info",
			Format:  "text",
			File:    "",
			MaxSize: 10,
			MaxAge:  7,
		},
		Agents: map[string]AgentConfig{},
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Catalog.RefreshInterval < time.Minute {
		c.Catalog.RefreshInterval = time.Minute
	}
	if c.Updates.CheckInterval < time.Minute {
		c.Updates.CheckInterval = time.Minute
	}
	if c.UI.PageSize < 1 {
		c.UI.PageSize = 20
	}
	if c.API.GRPCPort < 1 || c.API.GRPCPort > 65535 {
		c.API.GRPCPort = 50051
	}
	if c.API.RESTPort < 1 || c.API.RESTPort > 65535 {
		c.API.RESTPort = 8080
	}
	return nil
}

// GetAgentConfig returns the configuration for a specific agent.
func (c *Config) GetAgentConfig(agentID string) AgentConfig {
	if cfg, ok := c.Agents[agentID]; ok {
		return cfg
	}
	return AgentConfig{}
}

// IsAgentHidden returns true if the agent is configured as hidden.
func (c *Config) IsAgentHidden(agentID string) bool {
	if cfg, ok := c.Agents[agentID]; ok {
		return cfg.Hidden
	}
	return false
}

// IsAgentDisabled returns true if the agent is disabled.
func (c *Config) IsAgentDisabled(agentID string) bool {
	if cfg, ok := c.Agents[agentID]; ok {
		return cfg.Disabled
	}
	return false
}

// GetPinnedVersion returns the pinned version for an agent, if any.
func (c *Config) GetPinnedVersion(agentID string) string {
	if cfg, ok := c.Agents[agentID]; ok {
		return cfg.PinnedVersion
	}
	return ""
}
