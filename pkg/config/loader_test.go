package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader()
	if loader == nil {
		t.Fatal("NewLoader() returned nil")
	}
	if loader.v == nil {
		t.Error("Loader.v should not be nil")
	}
	if loader.platform == nil {
		t.Error("Loader.platform should not be nil")
	}
}

func TestLoaderLoadDefaults(t *testing.T) {
	// Create a temp directory for config
	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an empty config file (uses defaults for all values)
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("# empty config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load from empty file (should use defaults)
	loader := NewLoader()
	cfg, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Verify defaults
	if cfg.Catalog.SourceURL == "" {
		t.Error("Should have default SourceURL")
	}
	if cfg.UI.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20", cfg.UI.PageSize)
	}
}

func TestLoaderLoadFromFile(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a config file
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
catalog:
  source_url: "https://custom.example.com/catalog.json"
  refresh_on_start: false
ui:
  theme: "dark"
  page_size: 50
  use_colors: false
api:
  enable_rest: true
  rest_port: 3000
logging:
  level: "debug"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load from file
	loader := NewLoader()
	cfg, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Verify loaded values
	if cfg.Catalog.SourceURL != "https://custom.example.com/catalog.json" {
		t.Errorf("SourceURL = %q, want %q", cfg.Catalog.SourceURL, "https://custom.example.com/catalog.json")
	}
	if cfg.Catalog.RefreshOnStart {
		t.Error("RefreshOnStart should be false")
	}
	if cfg.UI.Theme != "dark" {
		t.Errorf("Theme = %q, want %q", cfg.UI.Theme, "dark")
	}
	if cfg.UI.PageSize != 50 {
		t.Errorf("PageSize = %d, want 50", cfg.UI.PageSize)
	}
	if cfg.UI.UseColors {
		t.Error("UseColors should be false")
	}
	if !cfg.API.EnableREST {
		t.Error("EnableREST should be true")
	}
	if cfg.API.RESTPort != 3000 {
		t.Errorf("RESTPort = %d, want 3000", cfg.API.RESTPort)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Level = %q, want %q", cfg.Logging.Level, "debug")
	}
}

func TestLoaderSave(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create an empty config file first
	if err := os.WriteFile(configPath, []byte("# initial config\n"), 0644); err != nil {
		t.Fatalf("Failed to write initial config file: %v", err)
	}

	// Create loader and load defaults
	loader := NewLoader()
	cfg, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Modify config
	cfg.UI.Theme = "dark"
	cfg.UI.PageSize = 100
	cfg.Agents = map[string]AgentConfig{
		"test-agent": {
			PreferredMethod: "npm",
			Hidden:          true,
		},
	}

	// Save
	if err := loader.Save(cfg); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load again and verify
	loader2 := NewLoader()
	cfg2, err := loader2.Load(configPath)
	if err != nil {
		t.Fatalf("Second Load() returned error: %v", err)
	}

	if cfg2.UI.Theme != "dark" {
		t.Errorf("Theme = %q, want %q", cfg2.UI.Theme, "dark")
	}
	if cfg2.UI.PageSize != 100 {
		t.Errorf("PageSize = %d, want 100", cfg2.UI.PageSize)
	}
}

func TestLoaderGetFilePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "test-config.yaml")

	// Create the config file first
	if err := os.WriteFile(configPath, []byte("# test config\n"), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	_, err = loader.Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	gotPath := loader.GetFilePath()
	if gotPath != configPath {
		t.Errorf("GetFilePath() = %q, want %q", gotPath, configPath)
	}
}

func TestLoaderSetAndGet(t *testing.T) {
	loader := NewLoader()

	// Set values
	loader.Set("ui.theme", "dracula")
	loader.Set("ui.page_size", 75)
	loader.Set("api.enable_rest", true)

	// Get values
	if v := loader.GetString("ui.theme"); v != "dracula" {
		t.Errorf("GetString(ui.theme) = %q, want %q", v, "dracula")
	}
	if v := loader.GetInt("ui.page_size"); v != 75 {
		t.Errorf("GetInt(ui.page_size) = %d, want 75", v)
	}
	if v := loader.GetBool("api.enable_rest"); !v {
		t.Error("GetBool(api.enable_rest) should be true")
	}

	// Get generic
	if v := loader.Get("ui.theme"); v != "dracula" {
		t.Errorf("Get(ui.theme) = %v, want %q", v, "dracula")
	}
}

func TestLoaderLoadInvalidYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	invalidYAML := `
catalog:
  source_url: [this is invalid yaml
  not closed properly
`
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	_, err = loader.Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoaderValidationFixes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	// Config with invalid values that should be corrected
	configContent := `
catalog:
  refresh_interval: 1s  # Too short
updates:
  check_interval: 5s    # Too short
ui:
  page_size: 0          # Invalid
api:
  grpc_port: 0          # Invalid
  rest_port: 99999      # Invalid
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// All invalid values should be corrected
	if cfg.UI.PageSize != 20 {
		t.Errorf("PageSize should be corrected to 20, got %d", cfg.UI.PageSize)
	}
	if cfg.API.GRPCPort != 50051 {
		t.Errorf("GRPCPort should be corrected to 50051, got %d", cfg.API.GRPCPort)
	}
	if cfg.API.RESTPort != 8080 {
		t.Errorf("RESTPort should be corrected to 8080, got %d", cfg.API.RESTPort)
	}
}

func TestConstants(t *testing.T) {
	if ConfigFileName != "config" {
		t.Errorf("ConfigFileName = %q, want %q", ConfigFileName, "config")
	}
	if EnvPrefix != "AGENTMGR" {
		t.Errorf("EnvPrefix = %q, want %q", EnvPrefix, "AGENTMGR")
	}
}

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()
	if path == "" {
		t.Error("GetConfigPath() returned empty string")
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("GetConfigPath() should end with config.yaml, got %q", filepath.Base(path))
	}
}

func TestGetDataPath(t *testing.T) {
	path := GetDataPath()
	if path == "" {
		t.Error("GetDataPath() returned empty string")
	}
}

func TestGetCachePath(t *testing.T) {
	path := GetCachePath()
	if path == "" {
		t.Error("GetCachePath() returned empty string")
	}
}

func TestGetLogPath(t *testing.T) {
	path := GetLogPath()
	if path == "" {
		t.Error("GetLogPath() returned empty string")
	}
}

func TestLoaderWithAgentConfigs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agentmgr-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
agents:
  claude-code:
    preferred_method: "npm"
    hidden: true
    pinned_version: "1.0.0"
  aider:
    preferred_method: "pipx"
    disabled: true
    custom_paths:
      - "/custom/aider/path"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	loader := NewLoader()
	cfg, err := loader.Load(configPath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	// Verify claude-code config
	claudeCfg := cfg.GetAgentConfig("claude-code")
	if claudeCfg.PreferredMethod != "npm" {
		t.Errorf("claude-code.PreferredMethod = %q, want %q", claudeCfg.PreferredMethod, "npm")
	}
	if !claudeCfg.Hidden {
		t.Error("claude-code should be hidden")
	}
	if claudeCfg.PinnedVersion != "1.0.0" {
		t.Errorf("claude-code.PinnedVersion = %q, want %q", claudeCfg.PinnedVersion, "1.0.0")
	}

	// Verify aider config
	aiderCfg := cfg.GetAgentConfig("aider")
	if aiderCfg.PreferredMethod != "pipx" {
		t.Errorf("aider.PreferredMethod = %q, want %q", aiderCfg.PreferredMethod, "pipx")
	}
	if !aiderCfg.Disabled {
		t.Error("aider should be disabled")
	}
	if len(aiderCfg.CustomPaths) != 1 || aiderCfg.CustomPaths[0] != "/custom/aider/path" {
		t.Errorf("aider.CustomPaths = %v, want [/custom/aider/path]", aiderCfg.CustomPaths)
	}
}
