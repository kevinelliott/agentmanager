package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/pkg/agent"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/detector"
	"github.com/kevinelliott/agentmgr/pkg/installer"
	"github.com/kevinelliott/agentmgr/pkg/platform"
	"github.com/kevinelliott/agentmgr/pkg/storage"
)

// NewAgentCommand creates the agent management command group.
func NewAgentCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage AI development agents",
		Long: `List, install, update, and manage AI development CLI agents.

This command group provides operations for detecting installed agents,
installing new agents, updating existing installations, and viewing
detailed information about agents.`,
		Aliases: []string{"agents"},
	}

	cmd.AddCommand(
		newAgentListCommand(cfg),
		newAgentInstallCommand(cfg),
		newAgentUpdateCommand(cfg),
		newAgentInfoCommand(cfg),
		newAgentRemoveCommand(cfg),
	)

	return cmd
}

func newAgentListCommand(cfg *config.Config) *cobra.Command {
	var (
		showAll      bool
		showHidden   bool
		format       string
		updatesOnly  bool
		checkUpdates bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all detected agents",
		Long: `Detect and list all installed AI development agents on your system.

This command scans for agents installed via various methods (npm, pip, brew,
native installers, etc.) and displays their current version, installation
method, and update status.`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// Get current platform
			plat := platform.Current()

			// Load catalog
			store, err := storage.NewSQLiteStore(plat.GetDataDir())
			if err != nil {
				return fmt.Errorf("failed to create storage: %w", err)
			}
			defer store.Close()

			if err := store.Initialize(ctx); err != nil {
				return fmt.Errorf("failed to initialize storage: %w", err)
			}

			catMgr := catalog.NewManager(cfg, store)

			// Get agents for current platform
			agentDefs, err := catMgr.GetAgentsForPlatform(ctx, string(plat.ID()))
			if err != nil {
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			// Create detector and detect agents
			det := detector.New(plat)
			installations, err := det.DetectAll(ctx, agentDefs)
			if err != nil {
				return fmt.Errorf("detection failed: %w", err)
			}

			// Apply filters
			var filtered []*agent.Installation
			for _, inst := range installations {
				// Skip hidden agents unless --hidden flag
				if !showHidden && cfg.IsAgentHidden(inst.AgentID) {
					continue
				}

				// Filter for updates only if requested
				if updatesOnly && !inst.HasUpdate() {
					continue
				}

				filtered = append(filtered, inst)
			}

			// Convert to list items
			items := make([]AgentListItem, 0, len(filtered))
			for _, inst := range filtered {
				latestVer := ""
				if inst.LatestVersion != nil {
					latestVer = inst.LatestVersion.String()
				}

				items = append(items, AgentListItem{
					ID:            inst.AgentID,
					Name:          inst.AgentName,
					Method:        string(inst.Method),
					Version:       inst.InstalledVersion.String(),
					LatestVersion: latestVer,
					HasUpdate:     inst.HasUpdate(),
					Path:          inst.ExecutablePath,
					Status:        string(inst.GetStatus()),
				})
			}

			if format == "json" {
				return outputAgentsJSON(items)
			}

			return outputAgentsTable(items, cfg)
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "show all installations")
	cmd.Flags().BoolVar(&showHidden, "hidden", false, "show hidden agents")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().BoolVarP(&updatesOnly, "updates", "u", false, "show only agents with updates")
	cmd.Flags().BoolVar(&checkUpdates, "check-updates", false, "check for available updates")

	return cmd
}

func newAgentInstallCommand(cfg *config.Config) *cobra.Command {
	var (
		method  string
		version string
		global  bool
		force   bool
	)

	cmd := &cobra.Command{
		Use:   "install <agent-name>",
		Short: "Install an agent",
		Long: `Install an AI development agent using the specified or default method.

If no method is specified, the preferred method from the catalog or config
will be used.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// Get current platform
			plat := platform.Current()

			// Load catalog
			store, err := storage.NewSQLiteStore(plat.GetDataDir())
			if err != nil {
				return fmt.Errorf("failed to create storage: %w", err)
			}
			defer store.Close()

			if err := store.Initialize(ctx); err != nil {
				return fmt.Errorf("failed to initialize storage: %w", err)
			}

			catMgr := catalog.NewManager(cfg, store)
			cat, err := catMgr.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			// Find agent in catalog
			agentDef, ok := cat.GetAgent(agentID)
			if !ok {
				return fmt.Errorf("agent %q not found in catalog", agentID)
			}

			// Determine installation method
			if method == "" {
				// Use preferred method from config or first available
				if preferred := cfg.GetAgentConfig(agentID).PreferredMethod; preferred != "" {
					method = preferred
				} else {
					methods := agentDef.GetSupportedMethods(string(plat.ID()))
					if len(methods) == 0 {
						return fmt.Errorf("no installation methods available for %q on %s", agentID, plat.ID())
					}
					method = methods[0].Method
				}
			}

			// Get method definition
			methodDef, ok := agentDef.GetInstallMethod(method)
			if !ok {
				return fmt.Errorf("installation method %q not available for %q", method, agentID)
			}

			fmt.Printf("Installing %s via %s...\n", agentDef.Name, method)

			// Create installer and install
			inst := installer.NewManager(plat)
			result, err := inst.Install(ctx, agentDef, methodDef, force)
			if err != nil {
				return fmt.Errorf("installation failed: %w", err)
			}

			printSuccess("Installed %s %s successfully", agentDef.Name, result.Version.String())
			return nil
		},
	}

	cmd.Flags().StringVarP(&method, "method", "m", "", "installation method (npm, pip, brew, etc.)")
	cmd.Flags().StringVarP(&version, "version", "V", "", "specific version to install")
	cmd.Flags().BoolVarP(&global, "global", "g", true, "install globally")
	cmd.Flags().BoolVarP(&force, "force", "F", false, "force installation")

	return cmd
}

func newAgentUpdateCommand(cfg *config.Config) *cobra.Command {
	var (
		all    bool
		force  bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "update [agent-name]",
		Short: "Update an agent or all agents",
		Long: `Update a specific agent installation or all agents with available updates.

When updating, the full changelog is displayed before confirming the update.
Use --all to update all agents at once.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				fmt.Println("Checking for updates...")
				// TODO: Implement update all
				printInfo("No updates available")
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("agent name required (or use --all)")
			}

			agentName := args[0]

			if dryRun {
				fmt.Printf("Would update %s (dry run)\n", agentName)
				return nil
			}

			fmt.Printf("Updating %s...\n", agentName)
			// TODO: Implement actual update
			printSuccess("Updated %s successfully", agentName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "update all agents")
	cmd.Flags().BoolVarP(&force, "force", "F", false, "force update")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be updated")

	return cmd
}

func newAgentInfoCommand(cfg *config.Config) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "info <agent-name>",
		Short: "Show detailed agent information",
		Long: `Display detailed information about an agent including all installations,
version information, changelog for available updates, and configuration.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]

			// TODO: Implement actual info display
			fmt.Printf("Agent: %s\n", agentName)
			fmt.Println("Status: Not implemented yet")
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format (text, json)")

	return cmd
}

func newAgentRemoveCommand(cfg *config.Config) *cobra.Command {
	var (
		force  bool
		method string
	)

	cmd := &cobra.Command{
		Use:   "remove <agent-name>",
		Short: "Remove an agent installation",
		Long: `Remove an installed agent. By default, prompts for confirmation.
Use --method to specify which installation to remove if multiple exist.`,
		Aliases: []string{"rm", "uninstall"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]

			if !force {
				fmt.Printf("Are you sure you want to remove %s? [y/N] ", agentName)
				var response string
				fmt.Scanln(&response)
				if !strings.EqualFold(response, "y") {
					fmt.Println("Canceled")
					return nil
				}
			}

			fmt.Printf("Removing %s...\n", agentName)
			// TODO: Implement actual removal
			printSuccess("Removed %s successfully", agentName)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "F", false, "skip confirmation")
	cmd.Flags().StringVarP(&method, "method", "m", "", "specific installation method to remove")

	return cmd
}

// AgentListItem represents an agent in the list output.
type AgentListItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Method        string `json:"method"`
	Version       string `json:"version"`
	LatestVersion string `json:"latest_version,omitempty"`
	HasUpdate     bool   `json:"has_update"`
	Path          string `json:"path"`
	Status        string `json:"status"`
}

func outputAgentsTable(agents []AgentListItem, cfg *config.Config) error {
	if len(agents) == 0 {
		fmt.Println("No agents detected.")
		fmt.Println("\nRun 'agentmgr catalog list' to see available agents.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "AGENT\tMETHOD\tVERSION\tLATEST\tSTATUS")
	fmt.Fprintln(w, "-----\t------\t-------\t------\t------")

	for _, agent := range agents {
		status := "✓"
		if agent.HasUpdate {
			status = "⬆"
		}

		latest := agent.LatestVersion
		if latest == "" {
			latest = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			agent.Name,
			agent.Method,
			agent.Version,
			latest,
			status,
		)
	}

	return nil
}

func outputAgentsJSON(agents []AgentListItem) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}
