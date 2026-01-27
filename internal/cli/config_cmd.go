package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/kevinelliott/agentmanager/pkg/config"
)

// NewConfigCommand creates the config management command group.
func NewConfigCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long: `View and modify AgentManager configuration settings.

Configuration is stored in a YAML file and can be overridden with
environment variables using the AGENTMGR_ prefix.`,
	}

	cmd.AddCommand(
		newConfigShowCommand(cfg),
		newConfigGetCommand(cfg),
		newConfigSetCommand(cfg),
		newConfigPathCommand(cfg),
		newConfigInitCommand(cfg),
		newConfigExportCommand(cfg),
		newConfigImportCommand(cfg),
	)

	return cmd
}

func newConfigShowCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long:  `Display the current configuration settings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("failed to serialize config: %w", err)
			}
			fmt.Println(string(data))
			return nil
		},
	}
}

func newConfigGetCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get a configuration value by key path.

Examples:
  agentmgr config get ui.theme
  agentmgr config get updates.auto_check
  agentmgr config get logging.level`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			// Create a loader to read config
			loader := config.NewLoader()

			// Load current config
			_, err := loader.Load("")
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get the value
			value := loader.Get(key)
			if value == nil {
				return fmt.Errorf("key %q not found in configuration", key)
			}

			fmt.Printf("%s = %v\n", key, value)
			return nil
		},
	}
}

func newConfigSetCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value by key path.

Examples:
  agentmgr config set ui.theme dark
  agentmgr config set updates.auto_check false
  agentmgr config set logging.level debug
  agentmgr config set ui.page_size 50
  agentmgr config set catalog.refresh_interval 2h`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			valueStr := args[1]

			// Create a loader to manage config
			loader := config.NewLoader()

			// Load current config (this loads the file into viper)
			_, err := loader.Load("")
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Parse value based on known types for common keys
			value := parseConfigValue(key, valueStr)

			// Set the value in viper and save
			if err := loader.SetAndSave(key, value); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			printSuccess("Set %s = %s", key, valueStr)
			printInfo("Config saved to %s", config.GetConfigPath())
			return nil
		},
	}
}

// parseConfigValue parses a string value into the appropriate type based on the key.
func parseConfigValue(key, value string) interface{} {
	key = strings.ToLower(key)

	// Boolean keys
	boolKeys := []string{
		"catalog.refresh_on_start",
		"updates.auto_check", "updates.notify", "updates.auto_update",
		"ui.show_hidden", "ui.use_colors", "ui.compact_mode",
		"api.enable_grpc", "api.enable_rest", "api.require_auth",
	}
	for _, k := range boolKeys {
		if key == k {
			return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
		}
	}

	// Integer keys
	intKeys := []string{
		"ui.page_size",
		"api.grpc_port", "api.rest_port",
		"logging.max_size", "logging.max_age",
	}
	for _, k := range intKeys {
		if key == k {
			if i, err := strconv.Atoi(value); err == nil {
				return i
			}
			return value
		}
	}

	// Duration keys
	durationKeys := []string{
		"catalog.refresh_interval",
		"updates.check_interval",
	}
	for _, k := range durationKeys {
		if key == k {
			if d, err := time.ParseDuration(value); err == nil {
				return d
			}
			return value
		}
	}

	// Default: return as string
	return value
}

func newConfigPathCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show configuration file path",
		Long:  `Display the path to the configuration file.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.GetConfigPath())
		},
	}
}

func newConfigInitCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize configuration",
		Long: `Create the configuration directory and default configuration file
if they don't exist.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.InitConfig(); err != nil {
				return err
			}
			printSuccess("Configuration initialized at %s", config.GetConfigPath())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "F", false, "overwrite existing config")

	return cmd
}

func newConfigExportCommand(cfg *config.Config) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "export [file]",
		Short: "Export configuration to a file",
		Long: `Export the current configuration to a file.

If no file is specified, outputs to stdout.

Examples:
  agentmgr config export                    # Output to stdout
  agentmgr config export config.yaml        # Export to YAML file
  agentmgr config export config.json -f json # Export to JSON file`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error

			if format == "json" {
				data, err = json.MarshalIndent(cfg, "", "  ")
			} else {
				data, err = yaml.Marshal(cfg)
			}

			if err != nil {
				return fmt.Errorf("failed to serialize config: %w", err)
			}

			if len(args) == 0 {
				fmt.Println(string(data))
				return nil
			}

			filename := args[0]
			if err := os.WriteFile(filename, data, 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			printSuccess("Configuration exported to %s", filename)
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "F", "yaml", "output format (yaml, json)")

	return cmd
}

func newConfigImportCommand(cfg *config.Config) *cobra.Command {
	var merge bool

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import configuration from a file",
		Long: `Import configuration from a YAML or JSON file.

By default, this replaces the current configuration. Use --merge to
merge with existing settings.

Examples:
  agentmgr config import config.yaml        # Replace with imported config
  agentmgr config import config.yaml --merge # Merge with existing config`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filename := args[0]

			data, err := os.ReadFile(filename)
			if err != nil {
				return fmt.Errorf("failed to read config file: %w", err)
			}

			var importedCfg config.Config

			// Detect format from extension
			ext := strings.ToLower(filepath.Ext(filename))
			if ext == ".json" {
				if err := json.Unmarshal(data, &importedCfg); err != nil {
					return fmt.Errorf("failed to parse JSON config: %w", err)
				}
			} else {
				if err := yaml.Unmarshal(data, &importedCfg); err != nil {
					return fmt.Errorf("failed to parse YAML config: %w", err)
				}
			}

			// Validate imported config
			if err := importedCfg.Validate(); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			// Save to config file
			loader := config.NewLoader()
			if merge {
				// Merge with existing config
				*cfg = mergeConfigs(*cfg, importedCfg)
			} else {
				*cfg = importedCfg
			}

			if err := loader.Save(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			if merge {
				printSuccess("Configuration merged from %s", filename)
			} else {
				printSuccess("Configuration imported from %s", filename)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&merge, "merge", "m", false, "merge with existing configuration")

	return cmd
}

// mergeConfigs merges imported config into base config.
// Non-zero values from imported overwrite base values.
func mergeConfigs(base, imported config.Config) config.Config {
	// Catalog settings
	if imported.Catalog.SourceURL != "" {
		base.Catalog.SourceURL = imported.Catalog.SourceURL
	}
	if imported.Catalog.RefreshInterval != 0 {
		base.Catalog.RefreshInterval = imported.Catalog.RefreshInterval
	}
	if imported.Catalog.GitHubToken != "" {
		base.Catalog.GitHubToken = imported.Catalog.GitHubToken
	}

	// Detection settings
	if imported.Detection.CacheDuration != 0 {
		base.Detection.CacheDuration = imported.Detection.CacheDuration
	}
	if imported.Detection.UpdateCheckCacheDuration != 0 {
		base.Detection.UpdateCheckCacheDuration = imported.Detection.UpdateCheckCacheDuration
	}

	// Updates settings
	if imported.Updates.CheckInterval != 0 {
		base.Updates.CheckInterval = imported.Updates.CheckInterval
	}
	if len(imported.Updates.ExcludeAgents) > 0 {
		base.Updates.ExcludeAgents = imported.Updates.ExcludeAgents
	}

	// UI settings
	if imported.UI.Theme != "" {
		base.UI.Theme = imported.UI.Theme
	}
	if imported.UI.PageSize != 0 {
		base.UI.PageSize = imported.UI.PageSize
	}

	// Logging settings
	if imported.Logging.Level != "" {
		base.Logging.Level = imported.Logging.Level
	}
	if imported.Logging.Format != "" {
		base.Logging.Format = imported.Logging.Format
	}
	if imported.Logging.File != "" {
		base.Logging.File = imported.Logging.File
	}

	// Merge agent configs
	if len(imported.Agents) > 0 {
		if base.Agents == nil {
			base.Agents = make(map[string]config.AgentConfig)
		}
		for id, agentCfg := range imported.Agents {
			base.Agents[id] = agentCfg
		}
	}

	return base
}
