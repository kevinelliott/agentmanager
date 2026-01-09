// Package cli implements the command-line interface for AgentManager.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/pkg/config"
)

// NewRootCommand creates the root command for agentmgr.
func NewRootCommand(cfg *config.Config, version, commit, date string) *cobra.Command {
	var (
		configFile string
		verbose    bool
		format     string
		noColor    bool
	)

	root := &cobra.Command{
		Use:   "agentmgr",
		Short: "AI Development Agent Manager",
		Long: `AgentManager is a comprehensive tool for discovering, installing,
updating, and managing AI development CLI agents across your system.

It supports multiple installation methods (npm, pip, brew, etc.) and
can detect all installations of the same agent separately.

Examples:
  agentmgr agent list              # List all detected agents
  agentmgr agent install aider     # Install aider
  agentmgr agent update --all      # Update all agents
  agentmgr tui                     # Launch TUI interface`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Reload config if custom path specified
			if configFile != "" {
				loader := config.NewLoader()
				newCfg, err := loader.Load(configFile)
				if err != nil {
					return fmt.Errorf("failed to load config from %s: %w", configFile, err)
				}
				*cfg = *newCfg
			}

			// Set verbose mode
			if verbose {
				cfg.Logging.Level = "debug"
			}

			// Override color config with flag if set
			if noColor {
				cfg.UI.UseColors = false
			}

			return nil
		},
	}

	// Global flags
	root.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	root.PersistentFlags().StringVarP(&format, "format", "f", "table", "output format (table, json, yaml)")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	// Add subcommands
	root.AddCommand(
		NewAgentCommand(cfg),
		NewCatalogCommand(cfg),
		NewConfigCommand(cfg),
		NewHelperCommand(cfg),
		NewTUICommand(cfg),
		NewVersionCommand(version, commit, date),
	)

	return root
}

// These helper functions are deprecated in favor of using output.Printer.
// They are kept for backwards compatibility with existing code.

// checkError prints an error message and exits if err is not nil.
//
//nolint:unused // Reserved for future use in command implementations
func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// printSuccess prints a success message with a checkmark.
//
// Deprecated: Use output.Printer.Success instead.
func printSuccess(format string, args ...interface{}) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// printInfo prints an info message.
//
// Deprecated: Use output.Printer.Info instead.
func printInfo(format string, args ...interface{}) {
	fmt.Printf("ℹ "+format+"\n", args...)
}

// printWarning prints a warning message.
//
// Deprecated: Use output.Printer.Warning instead.
func printWarning(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "⚠ "+format+"\n", args...)
}
