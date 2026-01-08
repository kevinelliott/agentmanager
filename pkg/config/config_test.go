package config

import (
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Test catalog defaults
	if cfg.Catalog.SourceURL == "" {
		t.Error("Catalog.SourceURL should not be empty")
	}
	if cfg.Catalog.RefreshInterval != time.Hour {
		t.Errorf("Catalog.RefreshInterval = %v, want %v", cfg.Catalog.RefreshInterval, time.Hour)
	}
	if !cfg.Catalog.RefreshOnStart {
		t.Error("Catalog.RefreshOnStart should be true")
	}

	// Test update defaults
	if !cfg.Updates.AutoCheck {
		t.Error("Updates.AutoCheck should be true")
	}
	if cfg.Updates.CheckInterval != 6*time.Hour {
		t.Errorf("Updates.CheckInterval = %v, want %v", cfg.Updates.CheckInterval, 6*time.Hour)
	}
	if !cfg.Updates.Notify {
		t.Error("Updates.Notify should be true")
	}
	if cfg.Updates.AutoUpdate {
		t.Error("Updates.AutoUpdate should be false")
	}

	// Test UI defaults
	if cfg.UI.Theme != "default" {
		t.Errorf("UI.Theme = %q, want %q", cfg.UI.Theme, "default")
	}
	if cfg.UI.ShowHidden {
		t.Error("UI.ShowHidden should be false")
	}
	if cfg.UI.PageSize != 20 {
		t.Errorf("UI.PageSize = %d, want 20", cfg.UI.PageSize)
	}
	if !cfg.UI.UseColors {
		t.Error("UI.UseColors should be true")
	}

	// Test API defaults
	if cfg.API.EnableGRPC {
		t.Error("API.EnableGRPC should be false")
	}
	if cfg.API.GRPCPort != 50051 {
		t.Errorf("API.GRPCPort = %d, want 50051", cfg.API.GRPCPort)
	}
	if cfg.API.EnableREST {
		t.Error("API.EnableREST should be false")
	}
	if cfg.API.RESTPort != 8080 {
		t.Errorf("API.RESTPort = %d, want 8080", cfg.API.RESTPort)
	}

	// Test logging defaults
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, "info")
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, "text")
	}
	if cfg.Logging.MaxSize != 10 {
		t.Errorf("Logging.MaxSize = %d, want 10", cfg.Logging.MaxSize)
	}
	if cfg.Logging.MaxAge != 7 {
		t.Errorf("Logging.MaxAge = %d, want 7", cfg.Logging.MaxAge)
	}

	// Test agents defaults
	if cfg.Agents == nil {
		t.Error("Agents should not be nil")
	}
	if len(cfg.Agents) != 0 {
		t.Errorf("Agents should be empty, got %d", len(cfg.Agents))
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Config)
		check  func(*Config) error
	}{
		{
			name: "fix short refresh interval",
			modify: func(c *Config) {
				c.Catalog.RefreshInterval = time.Second
			},
			check: func(c *Config) error {
				if c.Catalog.RefreshInterval < time.Minute {
					t.Errorf("RefreshInterval should be at least 1 minute, got %v", c.Catalog.RefreshInterval)
				}
				return nil
			},
		},
		{
			name: "fix short check interval",
			modify: func(c *Config) {
				c.Updates.CheckInterval = time.Second
			},
			check: func(c *Config) error {
				if c.Updates.CheckInterval < time.Minute {
					t.Errorf("CheckInterval should be at least 1 minute, got %v", c.Updates.CheckInterval)
				}
				return nil
			},
		},
		{
			name: "fix zero page size",
			modify: func(c *Config) {
				c.UI.PageSize = 0
			},
			check: func(c *Config) error {
				if c.UI.PageSize != 20 {
					t.Errorf("PageSize should be 20, got %d", c.UI.PageSize)
				}
				return nil
			},
		},
		{
			name: "fix negative page size",
			modify: func(c *Config) {
				c.UI.PageSize = -5
			},
			check: func(c *Config) error {
				if c.UI.PageSize != 20 {
					t.Errorf("PageSize should be 20, got %d", c.UI.PageSize)
				}
				return nil
			},
		},
		{
			name: "fix invalid grpc port low",
			modify: func(c *Config) {
				c.API.GRPCPort = 0
			},
			check: func(c *Config) error {
				if c.API.GRPCPort != 50051 {
					t.Errorf("GRPCPort should be 50051, got %d", c.API.GRPCPort)
				}
				return nil
			},
		},
		{
			name: "fix invalid grpc port high",
			modify: func(c *Config) {
				c.API.GRPCPort = 70000
			},
			check: func(c *Config) error {
				if c.API.GRPCPort != 50051 {
					t.Errorf("GRPCPort should be 50051, got %d", c.API.GRPCPort)
				}
				return nil
			},
		},
		{
			name: "fix invalid rest port low",
			modify: func(c *Config) {
				c.API.RESTPort = -1
			},
			check: func(c *Config) error {
				if c.API.RESTPort != 8080 {
					t.Errorf("RESTPort should be 8080, got %d", c.API.RESTPort)
				}
				return nil
			},
		},
		{
			name: "fix invalid rest port high",
			modify: func(c *Config) {
				c.API.RESTPort = 100000
			},
			check: func(c *Config) error {
				if c.API.RESTPort != 8080 {
					t.Errorf("RESTPort should be 8080, got %d", c.API.RESTPort)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.modify(cfg)
			err := cfg.Validate()
			if err != nil {
				t.Errorf("Validate() returned error: %v", err)
			}
			if err := tt.check(cfg); err != nil {
				t.Errorf("check() returned error: %v", err)
			}
		})
	}
}

func TestGetAgentConfig(t *testing.T) {
	cfg := Default()
	cfg.Agents = map[string]AgentConfig{
		"claude-code": {
			PreferredMethod: "npm",
			Hidden:          true,
			Disabled:        false,
			PinnedVersion:   "1.0.0",
		},
		"aider": {
			PreferredMethod: "pipx",
			Hidden:          false,
			Disabled:        true,
		},
	}

	tests := []struct {
		name            string
		agentID         string
		wantMethod      string
		wantHidden      bool
		wantDisabled    bool
		wantPinnedVer   string
		wantEmptyConfig bool
	}{
		{
			name:          "existing agent claude-code",
			agentID:       "claude-code",
			wantMethod:    "npm",
			wantHidden:    true,
			wantDisabled:  false,
			wantPinnedVer: "1.0.0",
		},
		{
			name:         "existing agent aider",
			agentID:      "aider",
			wantMethod:   "pipx",
			wantHidden:   false,
			wantDisabled: true,
		},
		{
			name:            "non-existing agent",
			agentID:         "unknown-agent",
			wantEmptyConfig: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentCfg := cfg.GetAgentConfig(tt.agentID)

			if tt.wantEmptyConfig {
				if agentCfg.PreferredMethod != "" || agentCfg.Hidden || agentCfg.Disabled || agentCfg.PinnedVersion != "" {
					t.Error("Expected empty AgentConfig")
				}
				return
			}

			if agentCfg.PreferredMethod != tt.wantMethod {
				t.Errorf("PreferredMethod = %q, want %q", agentCfg.PreferredMethod, tt.wantMethod)
			}
			if agentCfg.Hidden != tt.wantHidden {
				t.Errorf("Hidden = %v, want %v", agentCfg.Hidden, tt.wantHidden)
			}
			if agentCfg.Disabled != tt.wantDisabled {
				t.Errorf("Disabled = %v, want %v", agentCfg.Disabled, tt.wantDisabled)
			}
			if agentCfg.PinnedVersion != tt.wantPinnedVer {
				t.Errorf("PinnedVersion = %q, want %q", agentCfg.PinnedVersion, tt.wantPinnedVer)
			}
		})
	}
}

func TestIsAgentHidden(t *testing.T) {
	cfg := Default()
	cfg.Agents = map[string]AgentConfig{
		"hidden-agent":  {Hidden: true},
		"visible-agent": {Hidden: false},
	}

	tests := []struct {
		agentID  string
		expected bool
	}{
		{"hidden-agent", true},
		{"visible-agent", false},
		{"unknown-agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.agentID, func(t *testing.T) {
			got := cfg.IsAgentHidden(tt.agentID)
			if got != tt.expected {
				t.Errorf("IsAgentHidden(%q) = %v, want %v", tt.agentID, got, tt.expected)
			}
		})
	}
}

func TestIsAgentDisabled(t *testing.T) {
	cfg := Default()
	cfg.Agents = map[string]AgentConfig{
		"disabled-agent": {Disabled: true},
		"enabled-agent":  {Disabled: false},
	}

	tests := []struct {
		agentID  string
		expected bool
	}{
		{"disabled-agent", true},
		{"enabled-agent", false},
		{"unknown-agent", false},
	}

	for _, tt := range tests {
		t.Run(tt.agentID, func(t *testing.T) {
			got := cfg.IsAgentDisabled(tt.agentID)
			if got != tt.expected {
				t.Errorf("IsAgentDisabled(%q) = %v, want %v", tt.agentID, got, tt.expected)
			}
		})
	}
}

func TestGetPinnedVersion(t *testing.T) {
	cfg := Default()
	cfg.Agents = map[string]AgentConfig{
		"pinned-agent":   {PinnedVersion: "2.0.0"},
		"unpinned-agent": {PinnedVersion: ""},
	}

	tests := []struct {
		agentID  string
		expected string
	}{
		{"pinned-agent", "2.0.0"},
		{"unpinned-agent", ""},
		{"unknown-agent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.agentID, func(t *testing.T) {
			got := cfg.GetPinnedVersion(tt.agentID)
			if got != tt.expected {
				t.Errorf("GetPinnedVersion(%q) = %q, want %q", tt.agentID, got, tt.expected)
			}
		})
	}
}

func TestAgentConfigCustomPaths(t *testing.T) {
	cfg := Default()
	cfg.Agents = map[string]AgentConfig{
		"custom-paths-agent": {
			CustomPaths: []string{"/custom/path/1", "/custom/path/2"},
		},
	}

	agentCfg := cfg.GetAgentConfig("custom-paths-agent")
	if len(agentCfg.CustomPaths) != 2 {
		t.Errorf("CustomPaths length = %d, want 2", len(agentCfg.CustomPaths))
	}
	if agentCfg.CustomPaths[0] != "/custom/path/1" {
		t.Errorf("CustomPaths[0] = %q, want %q", agentCfg.CustomPaths[0], "/custom/path/1")
	}
	if agentCfg.CustomPaths[1] != "/custom/path/2" {
		t.Errorf("CustomPaths[1] = %q, want %q", agentCfg.CustomPaths[1], "/custom/path/2")
	}
}

func TestCatalogConfig(t *testing.T) {
	cfg := CatalogConfig{
		SourceURL:       "https://example.com/catalog.json",
		RefreshInterval: 2 * time.Hour,
		RefreshOnStart:  false,
		GitHubToken:     "test-token",
	}

	if cfg.SourceURL != "https://example.com/catalog.json" {
		t.Errorf("SourceURL = %q, want %q", cfg.SourceURL, "https://example.com/catalog.json")
	}
	if cfg.RefreshInterval != 2*time.Hour {
		t.Errorf("RefreshInterval = %v, want %v", cfg.RefreshInterval, 2*time.Hour)
	}
	if cfg.RefreshOnStart {
		t.Error("RefreshOnStart should be false")
	}
	if cfg.GitHubToken != "test-token" {
		t.Errorf("GitHubToken = %q, want %q", cfg.GitHubToken, "test-token")
	}
}

func TestUpdateConfig(t *testing.T) {
	cfg := UpdateConfig{
		AutoCheck:     false,
		CheckInterval: 12 * time.Hour,
		Notify:        false,
		AutoUpdate:    true,
		ExcludeAgents: []string{"agent1", "agent2"},
	}

	if cfg.AutoCheck {
		t.Error("AutoCheck should be false")
	}
	if cfg.CheckInterval != 12*time.Hour {
		t.Errorf("CheckInterval = %v, want %v", cfg.CheckInterval, 12*time.Hour)
	}
	if cfg.Notify {
		t.Error("Notify should be false")
	}
	if !cfg.AutoUpdate {
		t.Error("AutoUpdate should be true")
	}
	if len(cfg.ExcludeAgents) != 2 {
		t.Errorf("ExcludeAgents length = %d, want 2", len(cfg.ExcludeAgents))
	}
}

func TestUIConfig(t *testing.T) {
	cfg := UIConfig{
		Theme:       "dark",
		ShowHidden:  true,
		PageSize:    50,
		UseColors:   false,
		CompactMode: true,
	}

	if cfg.Theme != "dark" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "dark")
	}
	if !cfg.ShowHidden {
		t.Error("ShowHidden should be true")
	}
	if cfg.PageSize != 50 {
		t.Errorf("PageSize = %d, want 50", cfg.PageSize)
	}
	if cfg.UseColors {
		t.Error("UseColors should be false")
	}
	if !cfg.CompactMode {
		t.Error("CompactMode should be true")
	}
}

func TestAPIConfig(t *testing.T) {
	cfg := APIConfig{
		EnableGRPC:  true,
		GRPCPort:    9999,
		EnableREST:  true,
		RESTPort:    3000,
		RequireAuth: true,
		AuthToken:   "secret-token",
	}

	if !cfg.EnableGRPC {
		t.Error("EnableGRPC should be true")
	}
	if cfg.GRPCPort != 9999 {
		t.Errorf("GRPCPort = %d, want 9999", cfg.GRPCPort)
	}
	if !cfg.EnableREST {
		t.Error("EnableREST should be true")
	}
	if cfg.RESTPort != 3000 {
		t.Errorf("RESTPort = %d, want 3000", cfg.RESTPort)
	}
	if !cfg.RequireAuth {
		t.Error("RequireAuth should be true")
	}
	if cfg.AuthToken != "secret-token" {
		t.Errorf("AuthToken = %q, want %q", cfg.AuthToken, "secret-token")
	}
}

func TestLoggingConfig(t *testing.T) {
	cfg := LoggingConfig{
		Level:   "debug",
		Format:  "json",
		File:    "/var/log/agentmgr.log",
		MaxSize: 50,
		MaxAge:  30,
	}

	if cfg.Level != "debug" {
		t.Errorf("Level = %q, want %q", cfg.Level, "debug")
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %q, want %q", cfg.Format, "json")
	}
	if cfg.File != "/var/log/agentmgr.log" {
		t.Errorf("File = %q, want %q", cfg.File, "/var/log/agentmgr.log")
	}
	if cfg.MaxSize != 50 {
		t.Errorf("MaxSize = %d, want 50", cfg.MaxSize)
	}
	if cfg.MaxAge != 30 {
		t.Errorf("MaxAge = %d, want 30", cfg.MaxAge)
	}
}
