package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/kevinelliott/agentmgr/pkg/platform"
)

const (
	// ConfigFileName is the name of the config file (without extension)
	ConfigFileName = "config"

	// EnvPrefix is the prefix for environment variables
	EnvPrefix = "AGENTMGR"
)

// Loader handles configuration loading and saving.
type Loader struct {
	v        *viper.Viper
	platform platform.Platform
	filePath string
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	return &Loader{
		v:        viper.New(),
		platform: platform.Current(),
	}
}

// Load loads configuration from file, environment, and flags.
// Priority: flags > env > file > defaults
func (l *Loader) Load(customPath string) (*Config, error) {
	// Set defaults
	l.setDefaults()

	// Configure viper
	l.v.SetConfigName(ConfigFileName)
	l.v.SetConfigType("yaml")

	// Add config paths
	if customPath != "" {
		l.v.SetConfigFile(customPath)
		l.filePath = customPath
	} else {
		configDir := l.platform.GetConfigDir()
		l.v.AddConfigPath(configDir)
		l.filePath = filepath.Join(configDir, ConfigFileName+".yaml")

		// Also check current directory
		l.v.AddConfigPath(".")
	}

	// Environment variables
	l.v.SetEnvPrefix(EnvPrefix)
	l.v.AutomaticEnv()

	// Read config file (ignore not found)
	if err := l.v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal into struct
	cfg := Default()
	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to file.
func (l *Loader) Save(cfg *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(l.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Update viper with current config
	l.v.Set("catalog", cfg.Catalog)
	l.v.Set("updates", cfg.Updates)
	l.v.Set("ui", cfg.UI)
	l.v.Set("api", cfg.API)
	l.v.Set("logging", cfg.Logging)
	l.v.Set("agents", cfg.Agents)

	// Write to file
	if err := l.v.WriteConfigAs(l.filePath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetFilePath returns the path to the config file.
func (l *Loader) GetFilePath() string {
	return l.filePath
}

// Set sets a configuration value by key path.
func (l *Loader) Set(key string, value interface{}) {
	l.v.Set(key, value)
}

// Get gets a configuration value by key path.
func (l *Loader) Get(key string) interface{} {
	return l.v.Get(key)
}

// GetString gets a string configuration value by key path.
func (l *Loader) GetString(key string) string {
	return l.v.GetString(key)
}

// GetInt gets an int configuration value by key path.
func (l *Loader) GetInt(key string) int {
	return l.v.GetInt(key)
}

// GetBool gets a bool configuration value by key path.
func (l *Loader) GetBool(key string) bool {
	return l.v.GetBool(key)
}

// setDefaults sets the default values in viper.
func (l *Loader) setDefaults() {
	defaults := Default()

	// Catalog defaults
	l.v.SetDefault("catalog.source_url", defaults.Catalog.SourceURL)
	l.v.SetDefault("catalog.refresh_interval", defaults.Catalog.RefreshInterval)
	l.v.SetDefault("catalog.refresh_on_start", defaults.Catalog.RefreshOnStart)
	l.v.SetDefault("catalog.github_token", defaults.Catalog.GitHubToken)

	// Update defaults
	l.v.SetDefault("updates.auto_check", defaults.Updates.AutoCheck)
	l.v.SetDefault("updates.check_interval", defaults.Updates.CheckInterval)
	l.v.SetDefault("updates.notify", defaults.Updates.Notify)
	l.v.SetDefault("updates.auto_update", defaults.Updates.AutoUpdate)
	l.v.SetDefault("updates.exclude_agents", defaults.Updates.ExcludeAgents)

	// UI defaults
	l.v.SetDefault("ui.theme", defaults.UI.Theme)
	l.v.SetDefault("ui.show_hidden", defaults.UI.ShowHidden)
	l.v.SetDefault("ui.page_size", defaults.UI.PageSize)
	l.v.SetDefault("ui.use_colors", defaults.UI.UseColors)
	l.v.SetDefault("ui.compact_mode", defaults.UI.CompactMode)

	// API defaults
	l.v.SetDefault("api.enable_grpc", defaults.API.EnableGRPC)
	l.v.SetDefault("api.grpc_port", defaults.API.GRPCPort)
	l.v.SetDefault("api.enable_rest", defaults.API.EnableREST)
	l.v.SetDefault("api.rest_port", defaults.API.RESTPort)
	l.v.SetDefault("api.require_auth", defaults.API.RequireAuth)
	l.v.SetDefault("api.auth_token", defaults.API.AuthToken)

	// Logging defaults
	l.v.SetDefault("logging.level", defaults.Logging.Level)
	l.v.SetDefault("logging.format", defaults.Logging.Format)
	l.v.SetDefault("logging.file", defaults.Logging.File)
	l.v.SetDefault("logging.max_size", defaults.Logging.MaxSize)
	l.v.SetDefault("logging.max_age", defaults.Logging.MaxAge)
}

// InitConfig creates the config directory and default config file if they don't exist.
func InitConfig() error {
	p := platform.Current()
	configDir := p.GetConfigDir()

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create default config file if it doesn't exist
	configPath := filepath.Join(configDir, ConfigFileName+".yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		loader := NewLoader()
		cfg := Default()
		loader.filePath = configPath
		if err := loader.Save(cfg); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
	}

	return nil
}

// GetConfigPath returns the default config file path.
func GetConfigPath() string {
	p := platform.Current()
	return filepath.Join(p.GetConfigDir(), ConfigFileName+".yaml")
}

// GetDataPath returns the data directory path.
func GetDataPath() string {
	p := platform.Current()
	return p.GetDataDir()
}

// GetCachePath returns the cache directory path.
func GetCachePath() string {
	p := platform.Current()
	return p.GetCacheDir()
}

// GetLogPath returns the log directory path.
func GetLogPath() string {
	p := platform.Current()
	return p.GetLogDir()
}
