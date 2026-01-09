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

			// Create installer manager
			inst := installer.NewManager(plat)

			// Determine installation method
			if method == "" {
				// Use preferred method from config or first available
				if preferred := cfg.GetAgentConfig(agentID).PreferredMethod; preferred != "" {
					method = preferred
				} else {
					// Get methods that are both supported on this platform AND have available tools
					methods := inst.GetAvailableMethods(agentDef)
					if len(methods) == 0 {
						return fmt.Errorf("no installation methods available for %q on %s (tools like npm, brew, pip may not be installed)", agentID, plat.ID())
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

			// Install
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			plat := platform.Current()

			store, err := storage.NewSQLiteStore(plat.GetDataDir())
			if err != nil {
				return fmt.Errorf("failed to create storage: %w", err)
			}
			defer store.Close()

			if err := store.Initialize(ctx); err != nil {
				return fmt.Errorf("failed to initialize storage: %w", err)
			}

			catMgr := catalog.NewManager(cfg, store)
			agentDefs, err := catMgr.GetAgentsForPlatform(ctx, string(plat.ID()))
			if err != nil {
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			det := detector.New(plat)
			installations, err := det.DetectAll(ctx, agentDefs)
			if err != nil {
				return fmt.Errorf("detection failed: %w", err)
			}

			inst := installer.NewManager(plat)
			cat, err := catMgr.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			if all {
				return updateAllAgents(ctx, installations, cat, inst, dryRun)
			}

			if len(args) == 0 {
				return fmt.Errorf("agent name required (or use --all)")
			}

			return updateSingleAgent(ctx, args[0], installations, cat, inst, force, dryRun)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "update all agents")
	cmd.Flags().BoolVarP(&force, "force", "F", false, "force update")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be updated")

	return cmd
}

// updateAllAgents handles the --all flag to update all agents with available updates.
func updateAllAgents(ctx context.Context, installations []*agent.Installation, cat *catalog.Catalog, inst *installer.Manager, dryRun bool) error {
	fmt.Println("Checking for updates...")

	var toUpdate []*agent.Installation
	for _, installation := range installations {
		if installation.HasUpdate() {
			toUpdate = append(toUpdate, installation)
		}
	}

	if len(toUpdate) == 0 {
		printInfo("No updates available")
		return nil
	}

	fmt.Printf("Found %d agent(s) with updates:\n", len(toUpdate))
	for _, installation := range toUpdate {
		latestVer := "unknown"
		if installation.LatestVersion != nil {
			latestVer = installation.LatestVersion.String()
		}
		fmt.Printf("  - %s: %s -> %s\n",
			installation.AgentName,
			installation.InstalledVersion.String(),
			latestVer)
	}

	if dryRun {
		printInfo("Dry run - no changes made")
		return nil
	}

	for _, installation := range toUpdate {
		agentDef, ok := cat.GetAgent(installation.AgentID)
		if !ok {
			printWarning("Skipping %s: not found in catalog", installation.AgentName)
			continue
		}

		methodDef, ok := agentDef.GetInstallMethod(string(installation.Method))
		if !ok {
			printWarning("Skipping %s: install method %s not found", installation.AgentName, installation.Method)
			continue
		}

		fmt.Printf("Updating %s via %s...\n", installation.AgentName, installation.Method)
		result, err := inst.Update(ctx, installation, agentDef, methodDef)
		if err != nil {
			printError("Failed to update %s: %v", installation.AgentName, err)
			continue
		}
		printSuccess("Updated %s to %s", installation.AgentName, result.Version.String())
	}

	return nil
}

// updateSingleAgent handles updating a specific agent by ID.
func updateSingleAgent(ctx context.Context, agentID string, installations []*agent.Installation, cat *catalog.Catalog, inst *installer.Manager, force, dryRun bool) error {
	var agentInstallations []*agent.Installation
	for _, installation := range installations {
		if installation.AgentID == agentID {
			agentInstallations = append(agentInstallations, installation)
		}
	}

	if len(agentInstallations) == 0 {
		return fmt.Errorf("agent %q not installed", agentID)
	}

	agentDef, ok := cat.GetAgent(agentID)
	if !ok {
		return fmt.Errorf("agent %q not found in catalog", agentID)
	}

	var hasUpdate bool
	for _, installation := range agentInstallations {
		if installation.HasUpdate() || force {
			hasUpdate = true
			break
		}
	}

	if !hasUpdate {
		printInfo("%s is already up to date", agentDef.Name)
		return nil
	}

	if dryRun {
		fmt.Printf("Would update %s (dry run)\n", agentDef.Name)
		for _, installation := range agentInstallations {
			if installation.HasUpdate() || force {
				latestVer := "latest"
				if installation.LatestVersion != nil {
					latestVer = installation.LatestVersion.String()
				}
				fmt.Printf("  - %s via %s: %s -> %s\n",
					installation.AgentName,
					installation.Method,
					installation.InstalledVersion.String(),
					latestVer)
			}
		}
		return nil
	}

	for _, installation := range agentInstallations {
		if !installation.HasUpdate() && !force {
			continue
		}

		methodDef, ok := agentDef.GetInstallMethod(string(installation.Method))
		if !ok {
			printWarning("Skipping %s via %s: method not in catalog", installation.AgentName, installation.Method)
			continue
		}

		fmt.Printf("Updating %s via %s...\n", installation.AgentName, installation.Method)
		result, err := inst.Update(ctx, installation, agentDef, methodDef)
		if err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		printSuccess("Updated %s to %s", agentDef.Name, result.Version.String())
	}

	return nil
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
			agentID := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get current platform
			plat := platform.Current()

			// Load catalog and storage
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

			// Get agent from catalog
			agentDef, ok := cat.GetAgent(agentID)
			if !ok {
				return fmt.Errorf("agent %q not found in catalog", agentID)
			}

			// Detect installations of this agent
			agentDefs, err := catMgr.GetAgentsForPlatform(ctx, string(plat.ID()))
			if err != nil {
				return fmt.Errorf("failed to get agents: %w", err)
			}
			det := detector.New(plat)
			allInstallations, err := det.DetectAll(ctx, agentDefs)
			if err != nil {
				return fmt.Errorf("detection failed: %w", err)
			}

			// Filter for this agent
			var installations []*agent.Installation
			for _, inst := range allInstallations {
				if inst.AgentID == agentID {
					installations = append(installations, inst)
				}
			}

			if format == "json" {
				return outputAgentInfoJSON(agentDef, installations)
			}

			return outputAgentInfoText(agentDef, installations, plat)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format (text, json)")

	return cmd
}

func outputAgentInfoText(agentDef catalog.AgentDef, installations []*agent.Installation, plat platform.Platform) error {
	fmt.Printf("Agent: %s\n", agentDef.Name)
	fmt.Printf("ID: %s\n", agentDef.ID)
	fmt.Printf("Description: %s\n", agentDef.Description)
	if agentDef.Homepage != "" {
		fmt.Printf("Homepage: %s\n", agentDef.Homepage)
	}
	if agentDef.Repository != "" {
		fmt.Printf("Repository: %s\n", agentDef.Repository)
	}

	// Installation methods
	methods := agentDef.GetSupportedMethods(string(plat.ID()))
	if len(methods) > 0 {
		fmt.Printf("\nInstallation Methods:\n")
		for _, m := range methods {
			fmt.Printf("  - %s: %s\n", m.Method, m.Command)
		}
	}

	// Installed versions
	if len(installations) > 0 {
		fmt.Printf("\nInstalled (%d):\n", len(installations))
		for _, inst := range installations {
			status := "up-to-date"
			if inst.HasUpdate() {
				status = fmt.Sprintf("update available: %s", inst.LatestVersion.String())
			}
			fmt.Printf("  - %s via %s: %s (%s)\n",
				inst.InstalledVersion.String(),
				inst.Method,
				inst.ExecutablePath,
				status,
			)
		}
	} else {
		fmt.Printf("\nNot installed\n")
	}

	return nil
}

func outputAgentInfoJSON(agentDef catalog.AgentDef, installations []*agent.Installation) error {
	type installationInfo struct {
		Version   string `json:"version"`
		Method    string `json:"method"`
		Path      string `json:"path"`
		HasUpdate bool   `json:"has_update"`
		LatestVer string `json:"latest_version,omitempty"`
	}

	type agentInfo struct {
		ID            string             `json:"id"`
		Name          string             `json:"name"`
		Description   string             `json:"description"`
		Homepage      string             `json:"homepage,omitempty"`
		Repository    string             `json:"repository,omitempty"`
		Installations []installationInfo `json:"installations"`
	}

	info := agentInfo{
		ID:          agentDef.ID,
		Name:        agentDef.Name,
		Description: agentDef.Description,
		Homepage:    agentDef.Homepage,
		Repository:  agentDef.Repository,
	}

	for _, inst := range installations {
		latestVer := ""
		if inst.LatestVersion != nil {
			latestVer = inst.LatestVersion.String()
		}
		info.Installations = append(info.Installations, installationInfo{
			Version:   inst.InstalledVersion.String(),
			Method:    string(inst.Method),
			Path:      inst.ExecutablePath,
			HasUpdate: inst.HasUpdate(),
			LatestVer: latestVer,
		})
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(info)
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
			agentID := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()

			// Get current platform
			plat := platform.Current()

			// Load catalog and storage
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

			// Detect current installations
			det := detector.New(plat)
			installations, err := det.DetectAll(ctx, agentDefs)
			if err != nil {
				return fmt.Errorf("detection failed: %w", err)
			}

			// Get catalog for looking up agent definitions
			cat, err := catMgr.Get(ctx)
			if err != nil {
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			// Find installations for this agent
			var agentInstallations []*agent.Installation
			for _, installation := range installations {
				if installation.AgentID == agentID {
					// Filter by method if specified
					if method != "" && string(installation.Method) != method {
						continue
					}
					agentInstallations = append(agentInstallations, installation)
				}
			}

			if len(agentInstallations) == 0 {
				if method != "" {
					return fmt.Errorf("agent %q not installed via %s", agentID, method)
				}
				return fmt.Errorf("agent %q not installed", agentID)
			}

			// Get agent definition
			agentDef, ok := cat.GetAgent(agentID)
			if !ok {
				return fmt.Errorf("agent %q not found in catalog", agentID)
			}

			// If multiple installations and no method specified, list them
			if len(agentInstallations) > 1 && method == "" {
				fmt.Printf("Multiple installations of %s found:\n", agentDef.Name)
				for _, installation := range agentInstallations {
					fmt.Printf("  - %s via %s (%s)\n",
						installation.InstalledVersion.String(),
						installation.Method,
						installation.ExecutablePath)
				}
				fmt.Println("\nUse --method to specify which installation to remove.")
				return nil
			}

			installation := agentInstallations[0]

			if !force {
				fmt.Printf("Are you sure you want to remove %s (%s via %s)? [y/N] ",
					agentDef.Name, installation.InstalledVersion.String(), installation.Method)
				var response string
				fmt.Scanln(&response)
				if !strings.EqualFold(response, "y") {
					fmt.Println("Canceled")
					return nil
				}
			}

			// Get the install method definition
			methodDef, ok := agentDef.GetInstallMethod(string(installation.Method))
			if !ok {
				return fmt.Errorf("install method %s not found in catalog for %s", installation.Method, agentID)
			}

			// Create installer and uninstall
			inst := installer.NewManager(plat)
			fmt.Printf("Removing %s via %s...\n", agentDef.Name, installation.Method)

			if err := inst.Uninstall(ctx, installation, methodDef); err != nil {
				return fmt.Errorf("removal failed: %w", err)
			}

			printSuccess("Removed %s successfully", agentDef.Name)
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
