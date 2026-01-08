package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/kevinelliott/agentmgr/pkg/config"
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
		newConfigSetCommand(cfg),
		newConfigPathCommand(cfg),
		newConfigInitCommand(cfg),
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

func newConfigSetCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value by key path.

Examples:
  agentmgr config set ui.theme dark
  agentmgr config set updates.auto_check false
  agentmgr config set logging.level debug`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			// TODO: Implement actual config set
			printSuccess("Set %s = %s", key, value)
			return nil
		},
	}
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
