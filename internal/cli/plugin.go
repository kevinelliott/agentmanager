package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmanager/internal/cli/output"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/detector"
	"github.com/kevinelliott/agentmanager/pkg/platform"
)

// NewPluginCommand creates the plugin management command group.
func NewPluginCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage detection plugins",
		Long: `Manage custom detection plugins for agent discovery.

Detection plugins allow you to extend AgentManager with custom logic
for detecting agents installed via non-standard methods (e.g., Docker,
custom build systems, etc.).

Plugins are JSON files with the .plugin.json extension stored in:
  ~/.config/agentmgr/plugins/`,
		Aliases: []string{"plugins"},
	}

	cmd.AddCommand(
		newPluginListCommand(cfg),
		newPluginCreateCommand(cfg),
		newPluginValidateCommand(cfg),
		newPluginEnableCommand(cfg),
		newPluginDisableCommand(cfg),
	)

	return cmd
}

func newPluginListCommand(cfg *config.Config) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List installed detection plugins",
		Long:    `List all detection plugins installed in the plugins directory.`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			plat := platform.Current()
			registry := detector.NewPluginRegistry(plat)

			// Load plugins from config directory
			pluginsDir := filepath.Join(plat.GetConfigDir(), "plugins")
			if err := registry.LoadPluginsFromDir(pluginsDir); err != nil {
				return fmt.Errorf("failed to load plugins: %w", err)
			}

			plugins := registry.List()

			if format == "json" {
				data, err := json.MarshalIndent(plugins, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal plugins: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}

			printer := output.NewPrinter(cfg, false)
			styles := printer.Styles()

			if len(plugins) == 0 {
				printer.Info("No plugins installed")
				printer.Print("")
				printer.Print("Create a plugin with 'agentmgr plugin create <name>'")
				return nil
			}

			table := output.NewTable()
			table.SetHeaders(
				styles.FormatHeader("NAME"),
				styles.FormatHeader("METHOD"),
				styles.FormatHeader("ENABLED"),
				styles.FormatHeader("DESCRIPTION"),
			)

			for _, p := range plugins {
				enabled := "No"
				if p.Enabled {
					enabled = styles.SuccessIcon()
				}
				table.AddRow(
					styles.FormatAgentName(p.Name),
					styles.FormatMethod(p.Method),
					enabled,
					truncate(p.Description, 40),
				)
			}

			table.Render()
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

func newPluginCreateCommand(cfg *config.Config) *cobra.Command {
	var (
		method      string
		description string
		script      string
		command     string
		platforms   []string
		agentFilter []string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new detection plugin",
		Long: `Create a new detection plugin with the given name.

The plugin will be created as a JSON file in the plugins directory.
You must provide either --script for inline detection logic or
--command for an external detection command.

The detection script/command should output JSON in this format:
{
  "agents": [
    {
      "agent_id": "agent-name",
      "version": "1.0.0",
      "executable_path": "/path/to/executable"
    }
  ]
}

Environment variables available to the script:
  AGENTMGR_AGENT_IDS  - Comma-separated list of agent IDs to detect
  AGENTMGR_PLATFORM   - Current platform (darwin, linux, windows)`,
		Example: `  # Create a Docker-based detection plugin
  agentmgr plugin create docker-agents \
    --method docker \
    --description "Detect agents running in Docker containers" \
    --script 'docker ps --format json | jq ...'

  # Create a plugin using an external script
  agentmgr plugin create custom-detector \
    --method custom \
    --command "/usr/local/bin/detect-agents.sh"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			pluginCfg := detector.PluginConfig{
				Name:          name,
				Description:   description,
				Method:        method,
				Platforms:     platforms,
				DetectCommand: command,
				DetectScript:  script,
				AgentFilter:   agentFilter,
				Enabled:       true,
			}

			if err := detector.ValidatePlugin(pluginCfg); err != nil {
				return fmt.Errorf("invalid plugin configuration: %w", err)
			}

			// Create plugins directory if needed
			plat := platform.Current()
			pluginsDir := filepath.Join(plat.GetConfigDir(), "plugins")
			if err := os.MkdirAll(pluginsDir, 0755); err != nil {
				return fmt.Errorf("failed to create plugins directory: %w", err)
			}

			// Write plugin file
			pluginPath := filepath.Join(pluginsDir, name+".plugin.json")
			data, err := json.MarshalIndent(pluginCfg, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal plugin config: %w", err)
			}
			if err := os.WriteFile(pluginPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write plugin file: %w", err)
			}

			printSuccess("Created plugin: %s", pluginPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&method, "method", "m", "custom", "installation method this plugin detects")
	cmd.Flags().StringVarP(&description, "description", "d", "", "plugin description")
	cmd.Flags().StringVarP(&script, "script", "s", "", "inline detection script")
	cmd.Flags().StringVarP(&command, "command", "x", "", "external detection command")
	cmd.Flags().StringSliceVar(&platforms, "platforms", nil, "supported platforms (darwin, linux, windows)")
	cmd.Flags().StringSliceVar(&agentFilter, "agents", nil, "agent IDs to handle (empty = all)")

	return cmd
}

func newPluginValidateCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a plugin configuration file",
		Long:  `Validate the structure and configuration of a plugin JSON file.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read plugin file: %w", err)
			}

			var pluginCfg detector.PluginConfig
			if err := json.Unmarshal(data, &pluginCfg); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}

			if err := detector.ValidatePlugin(pluginCfg); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			printSuccess("Plugin %q is valid", pluginCfg.Name)
			return nil
		},
	}
}

func newPluginEnableCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a detection plugin",
		Long:  `Enable a detection plugin so it runs during agent detection.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setPluginEnabled(args[0], true)
		},
	}
}

func newPluginDisableCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a detection plugin",
		Long:  `Disable a detection plugin to prevent it from running.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setPluginEnabled(args[0], false)
		},
	}
}

func setPluginEnabled(name string, enabled bool) error {
	plat := platform.Current()
	pluginsDir := filepath.Join(plat.GetConfigDir(), "plugins")
	pluginPath := filepath.Join(pluginsDir, name+".plugin.json")

	data, err := os.ReadFile(pluginPath)
	if err != nil {
		return fmt.Errorf("plugin not found: %s", name)
	}

	var pluginCfg detector.PluginConfig
	if err := json.Unmarshal(data, &pluginCfg); err != nil {
		return fmt.Errorf("invalid plugin file: %w", err)
	}

	pluginCfg.Enabled = enabled

	data, err = json.MarshalIndent(pluginCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plugin config: %w", err)
	}
	if err := os.WriteFile(pluginPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write plugin file: %w", err)
	}

	if enabled {
		printSuccess("Enabled plugin: %s", name)
	} else {
		printSuccess("Disabled plugin: %s", name)
	}
	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
