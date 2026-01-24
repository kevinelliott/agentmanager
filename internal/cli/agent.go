package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/internal/cli/output"
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
		newAgentRefreshCommand(cfg),
	)

	return cmd
}

//nolint:gocyclo // CLI command setup with many flags and options
func newAgentListCommand(cfg *config.Config) *cobra.Command {
	var (
		showAll      bool
		showHidden   bool
		format       string
		updatesOnly  bool
		checkUpdates bool
		refresh      bool
		verify       bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all detected agents",
		Long: `Detect and list all installed AI development agents on your system.

This command scans for agents installed via various methods (npm, pip, brew,
native installers, etc.) and displays their current version, installation
method, and update status.

Results are cached for 1 hour by default. Use --refresh to force re-detection.`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			// Create printer for colored output
			printer := output.NewPrinter(cfg, cmd.Flag("no-color").Changed && cmd.Flag("no-color").Value.String() == "true")

			// Get current platform
			plat := platform.Current()

			// Create spinner for loading
			spinner := output.NewSpinner(
				output.WithMessage("Loading..."),
				output.WithNoColor(!cfg.UI.UseColors),
			)
			spinner.Start()

			// Load storage
			store, err := storage.NewSQLiteStore(plat.GetDataDir())
			if err != nil {
				spinner.Error("Failed to create storage")
				return fmt.Errorf("failed to create storage: %w", err)
			}
			defer store.Close()

			if err := store.Initialize(ctx); err != nil {
				spinner.Error("Failed to initialize storage")
				return fmt.Errorf("failed to initialize storage: %w", err)
			}

			catMgr := catalog.NewManager(cfg, store)

			// Get agents for current platform
			agentDefs, err := catMgr.GetAgentsForPlatform(ctx, string(plat.ID()))
			if err != nil {
				spinner.Error("Failed to load catalog")
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			// Build a map of agent ID -> AgentDef for quick lookup
			agentDefMap := make(map[string]catalog.AgentDef)
			for _, def := range agentDefs {
				agentDefMap[def.ID] = def
			}

			var installations []*agent.Installation
			var usedDetectionCache bool
			var needUpdateCheck bool

			// Check if we can use cached detection results
			if cfg.Detection.CacheEnabled && !refresh {
				cachedInstallations, cachedAt, err := store.GetDetectionCache(ctx)
				if err == nil && cachedInstallations != nil {
					// Check if detection cache is still valid
					if time.Since(cachedAt) < cfg.Detection.CacheDuration {
						installations = cachedInstallations
						usedDetectionCache = true
						spinner.UpdateMessage("Loading from cache...")

						// Check if we need to refresh update check
						lastUpdateCheck, err := store.GetLastUpdateCheckTime(ctx)
						if err != nil || lastUpdateCheck.IsZero() || time.Since(lastUpdateCheck) >= cfg.Detection.UpdateCheckCacheDuration {
							needUpdateCheck = true
						}
					}
				}
			}

			// If no cache or cache expired, detect agents
			if !usedDetectionCache {
				spinner.UpdateMessage("Detecting agents...")

				// Create detector and detect agents
				det := detector.New(plat)
				installations, err = det.DetectAll(ctx, agentDefs)
				if err != nil {
					spinner.Error("Agent detection failed")
					return fmt.Errorf("detection failed: %w", err)
				}

				// Always check for updates on fresh detection
				needUpdateCheck = true
			}

			// Check for updates if needed
			if needUpdateCheck {
				// Create installer manager for version checking
				instMgr := installer.NewManager(plat)

				// Update spinner for version checking
				spinner.UpdateMessage("Checking for updates...")

				// Check for latest versions
				for _, inst := range installations {
					if agentDef, ok := agentDefMap[inst.AgentID]; ok {
						// Find the matching install method
						methodStr := string(inst.Method)
						if method, ok := agentDef.InstallMethods[methodStr]; ok {
							// Get latest version from package registry
							latestVer, err := instMgr.GetLatestVersion(ctx, method)
							if err == nil {
								inst.LatestVersion = &latestVer
							}
						}
					}
				}

				// Save last update check time
				_ = store.SetLastUpdateCheckTime(ctx, time.Now()) //nolint:errcheck // best-effort timestamp; non-critical if this fails

				// Save to cache if enabled (with updated version info)
				if cfg.Detection.CacheEnabled {
					if err := store.SaveDetectionCache(ctx, installations); err != nil {
						// Log but don't fail
						_ = err
					}
				}
			}

			// Stop spinner
			spinner.Stop()

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

			// Sort agents alphabetically by name (case-insensitive)
			sort.Slice(filtered, func(i, j int) bool {
				return strings.ToLower(filtered[i].AgentName) < strings.ToLower(filtered[j].AgentName)
			})

			// Convert to list items
			items := make([]AgentListItem, 0, len(filtered))
			for _, inst := range filtered {
				latestVer := ""
				if inst.LatestVersion != nil {
					latestVer = inst.LatestVersion.String()
				}

				item := AgentListItem{
					ID:            inst.AgentID,
					Name:          inst.AgentName,
					Method:        string(inst.Method),
					Version:       inst.InstalledVersion.String(),
					LatestVersion: latestVer,
					HasUpdate:     inst.HasUpdate(),
					Path:          inst.ExecutablePath,
					Status:        string(inst.GetStatus()),
				}

				// Verify agent health if requested
				if verify && inst.ExecutablePath != "" {
					healthy, healthErr := verifyAgentHealth(ctx, inst.ExecutablePath)
					item.Healthy = &healthy
					if healthErr != "" {
						item.HealthError = healthErr
					}
				}

				items = append(items, item)
			}

			if format == "json" {
				return outputAgentsJSON(items)
			}

			return outputAgentsTable(items, printer, verify)
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "show all installations")
	cmd.Flags().BoolVar(&showHidden, "hidden", false, "show hidden agents")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().BoolVarP(&updatesOnly, "updates", "u", false, "show only agents with updates")
	cmd.Flags().BoolVar(&checkUpdates, "check-updates", false, "check for available updates")
	cmd.Flags().BoolVarP(&refresh, "refresh", "r", false, "force re-detection (ignore cache)")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify agents can execute (health check)")

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

			// Create spinner
			spinner := output.NewSpinner(
				output.WithMessage("Loading catalog..."),
				output.WithNoColor(!cfg.UI.UseColors),
			)
			spinner.Start()

			// Load catalog
			store, err := storage.NewSQLiteStore(plat.GetDataDir())
			if err != nil {
				spinner.Error("Failed to create storage")
				return fmt.Errorf("failed to create storage: %w", err)
			}
			defer store.Close()

			if err := store.Initialize(ctx); err != nil {
				spinner.Error("Failed to initialize storage")
				return fmt.Errorf("failed to initialize storage: %w", err)
			}

			catMgr := catalog.NewManager(cfg, store)
			cat, err := catMgr.Get(ctx)
			if err != nil {
				spinner.Error("Failed to load catalog")
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			// Find agent in catalog
			agentDef, ok := cat.GetAgent(agentID)
			if !ok {
				spinner.Error(fmt.Sprintf("Agent %q not found in catalog", agentID))
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
						spinner.Error("No installation methods available")
						return fmt.Errorf("no installation methods available for %q on %s", agentID, plat.ID())
					}
					method = methods[0].Method
				}
			}

			// Get method definition
			methodDef, ok := agentDef.GetInstallMethod(method)
			if !ok {
				spinner.Error(fmt.Sprintf("Installation method %q not available", method))
				return fmt.Errorf("installation method %q not available for %q", method, agentID)
			}

			spinner.UpdateMessage(fmt.Sprintf("Installing %s via %s...", agentDef.Name, method))

			// Create installer and install
			inst := installer.NewManager(plat)
			result, err := inst.Install(ctx, agentDef, methodDef, force)
			if err != nil {
				spinner.Error(fmt.Sprintf("Failed to install %s", agentDef.Name))
				return fmt.Errorf("installation failed: %w", err)
			}

			spinner.Success(fmt.Sprintf("Installed %s %s successfully", agentDef.Name, result.Version.String()))
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

			// Create printer for colored output
			printer := output.NewPrinter(cfg, cmd.Flag("no-color").Changed && cmd.Flag("no-color").Value.String() == "true")

			plat := platform.Current()

			// Create spinner
			spinner := output.NewSpinner(
				output.WithMessage("Loading catalog..."),
				output.WithNoColor(!cfg.UI.UseColors),
			)
			spinner.Start()

			store, err := storage.NewSQLiteStore(plat.GetDataDir())
			if err != nil {
				spinner.Error("Failed to create storage")
				return fmt.Errorf("failed to create storage: %w", err)
			}
			defer store.Close()

			if err := store.Initialize(ctx); err != nil {
				spinner.Error("Failed to initialize storage")
				return fmt.Errorf("failed to initialize storage: %w", err)
			}

			catMgr := catalog.NewManager(cfg, store)
			agentDefs, err := catMgr.GetAgentsForPlatform(ctx, string(plat.ID()))
			if err != nil {
				spinner.Error("Failed to load catalog")
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			spinner.UpdateMessage("Detecting agents...")

			det := detector.New(plat)
			installations, err := det.DetectAll(ctx, agentDefs)
			if err != nil {
				spinner.Error("Detection failed")
				return fmt.Errorf("detection failed: %w", err)
			}

			inst := installer.NewManager(plat)
			cat, err := catMgr.Get(ctx)
			if err != nil {
				spinner.Error("Failed to load catalog")
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			// Build agent lookup map for version checking
			agentDefMap := make(map[string]catalog.AgentDef)
			for _, def := range agentDefs {
				agentDefMap[def.ID] = def
			}

			spinner.UpdateMessage("Checking for updates...")

			// Check for latest versions so HasUpdate() works correctly
			for _, installation := range installations {
				if agentDef, ok := agentDefMap[installation.AgentID]; ok {
					methodStr := string(installation.Method)
					if method, ok := agentDef.InstallMethods[methodStr]; ok {
						latestVer, err := inst.GetLatestVersion(ctx, method)
						if err == nil {
							installation.LatestVersion = &latestVer
						}
					}
				}
			}

			spinner.Stop()

			if all {
				return updateAllAgents(ctx, installations, cat, inst, dryRun, printer)
			}

			if len(args) == 0 {
				printer.Error("agent name required (or use --all)")
				return fmt.Errorf("agent name required (or use --all)")
			}

			return updateSingleAgent(ctx, args[0], installations, cat, inst, force, dryRun, printer)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "update all agents")
	cmd.Flags().BoolVarP(&force, "force", "F", false, "force update")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be updated")

	return cmd
}

// updateAllAgents handles the --all flag to update all agents with available updates.
func updateAllAgents(ctx context.Context, installations []*agent.Installation, cat *catalog.Catalog, inst *installer.Manager, dryRun bool, printer *output.Printer) error {
	styles := printer.Styles()

	spinner := output.NewSpinner(
		output.WithMessage("Checking for updates..."),
		output.WithNoColor(os.Getenv("NO_COLOR") != ""),
	)
	spinner.Start()

	var toUpdate []*agent.Installation
	for _, installation := range installations {
		if installation.HasUpdate() {
			toUpdate = append(toUpdate, installation)
		}
	}

	spinner.Stop()

	if len(toUpdate) == 0 {
		printer.Info("No updates available")
		return nil
	}

	printer.Print("\nFound %s with updates:", styles.Bold.Render(fmt.Sprintf("%d agent(s)", len(toUpdate))))
	for _, installation := range toUpdate {
		latestVer := "unknown"
		if installation.LatestVersion != nil {
			latestVer = installation.LatestVersion.String()
		}
		printer.Print("  - %s: %s -> %s",
			styles.FormatAgentName(installation.AgentName),
			styles.FormatVersion(installation.InstalledVersion.String(), true),
			styles.FormatVersion(latestVer, false))
	}

	if dryRun {
		printer.Info("\nDry run - no changes made")
		return nil
	}

	printer.Print("")

	for _, installation := range toUpdate {
		agentDef, ok := cat.GetAgent(installation.AgentID)
		if !ok {
			printer.Warning("Skipping %s: not found in catalog", installation.AgentName)
			continue
		}

		methodDef, ok := agentDef.GetInstallMethod(string(installation.Method))
		if !ok {
			printer.Warning("Skipping %s: install method %s not found", installation.AgentName, installation.Method)
			continue
		}

		spinner := output.NewSpinner(
			output.WithMessage(fmt.Sprintf("Updating %s via %s...", installation.AgentName, installation.Method)),
			output.WithNoColor(os.Getenv("NO_COLOR") != ""),
		)
		spinner.Start()

		result, err := inst.Update(ctx, installation, agentDef, methodDef)
		if err != nil {
			spinner.Error(fmt.Sprintf("Failed to update %s: %v", installation.AgentName, err))
			continue
		}
		spinner.Success(fmt.Sprintf("Updated %s to %s", installation.AgentName, result.Version.String()))
	}

	return nil
}

// updateSingleAgent handles updating a specific agent by ID.
func updateSingleAgent(ctx context.Context, agentID string, installations []*agent.Installation, cat *catalog.Catalog, inst *installer.Manager, force, dryRun bool, printer *output.Printer) error {
	styles := printer.Styles()

	var agentInstallations []*agent.Installation
	for _, installation := range installations {
		if installation.AgentID == agentID {
			agentInstallations = append(agentInstallations, installation)
		}
	}

	if len(agentInstallations) == 0 {
		printer.Error("Agent %q not installed", agentID)
		return fmt.Errorf("agent %q not installed", agentID)
	}

	agentDef, ok := cat.GetAgent(agentID)
	if !ok {
		printer.Error("Agent %q not found in catalog", agentID)
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
		printer.Info("%s is already up to date", agentDef.Name)
		return nil
	}

	if dryRun {
		printer.Info("Would update %s (dry run)", agentDef.Name)
		for _, installation := range agentInstallations {
			if installation.HasUpdate() || force {
				latestVer := "latest"
				if installation.LatestVersion != nil {
					latestVer = installation.LatestVersion.String()
				}
				printer.Print("  - %s via %s: %s -> %s",
					styles.FormatAgentName(installation.AgentName),
					styles.FormatMethod(string(installation.Method)),
					styles.FormatVersion(installation.InstalledVersion.String(), true),
					styles.FormatVersion(latestVer, false))
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
			printer.Warning("Skipping %s via %s: method not in catalog", installation.AgentName, installation.Method)
			continue
		}

		spinner := output.NewSpinner(
			output.WithMessage(fmt.Sprintf("Updating %s via %s...", installation.AgentName, installation.Method)),
			output.WithNoColor(os.Getenv("NO_COLOR") != ""),
		)
		spinner.Start()

		result, err := inst.Update(ctx, installation, agentDef, methodDef)
		if err != nil {
			spinner.Error(fmt.Sprintf("Failed to update %s", agentDef.Name))
			return fmt.Errorf("update failed: %w", err)
		}
		spinner.Success(fmt.Sprintf("Updated %s to %s", agentDef.Name, result.Version.String()))
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

func newAgentRefreshCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Force re-detection of all agents",
		Long: `Clear the agent detection cache and re-detect all installed agents.

This command forces a fresh scan of installed agents, ignoring any cached
results. The new detection results are then cached for future use.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			// Get current platform
			plat := platform.Current()

			// Create spinner
			spinner := output.NewSpinner(
				output.WithMessage("Clearing cache..."),
				output.WithNoColor(!cfg.UI.UseColors),
			)
			spinner.Start()

			// Load storage
			store, err := storage.NewSQLiteStore(plat.GetDataDir())
			if err != nil {
				spinner.Error("Failed to create storage")
				return fmt.Errorf("failed to create storage: %w", err)
			}
			defer store.Close()

			if err := store.Initialize(ctx); err != nil {
				spinner.Error("Failed to initialize storage")
				return fmt.Errorf("failed to initialize storage: %w", err)
			}

			// Clear the detection cache
			if err := store.ClearDetectionCache(ctx); err != nil {
				spinner.Error("Failed to clear cache")
				return fmt.Errorf("failed to clear cache: %w", err)
			}

			catMgr := catalog.NewManager(cfg, store)

			// Get agents for current platform
			agentDefs, err := catMgr.GetAgentsForPlatform(ctx, string(plat.ID()))
			if err != nil {
				spinner.Error("Failed to load catalog")
				return fmt.Errorf("failed to load catalog: %w", err)
			}

			// Build a map of agent ID -> AgentDef for quick lookup
			agentDefMap := make(map[string]catalog.AgentDef)
			for _, def := range agentDefs {
				agentDefMap[def.ID] = def
			}

			spinner.UpdateMessage("Detecting agents...")

			// Create detector and detect agents
			det := detector.New(plat)
			installations, err := det.DetectAll(ctx, agentDefs)
			if err != nil {
				spinner.Error("Agent detection failed")
				return fmt.Errorf("detection failed: %w", err)
			}

			// Create installer manager for version checking
			instMgr := installer.NewManager(plat)

			spinner.UpdateMessage("Checking for updates...")

			// Check for latest versions
			for _, inst := range installations {
				if agentDef, ok := agentDefMap[inst.AgentID]; ok {
					methodStr := string(inst.Method)
					if method, ok := agentDef.InstallMethods[methodStr]; ok {
						latestVer, err := instMgr.GetLatestVersion(ctx, method)
						if err == nil {
							inst.LatestVersion = &latestVer
						}
					}
				}
			}

			// Save to cache
			if cfg.Detection.CacheEnabled {
				if err := store.SaveDetectionCache(ctx, installations); err != nil {
					spinner.Error("Failed to save cache")
					return fmt.Errorf("failed to save cache: %w", err)
				}
			}

			// Count agents with updates
			updateCount := 0
			for _, inst := range installations {
				if inst.HasUpdate() {
					updateCount++
				}
			}

			spinner.Success(fmt.Sprintf("Detected %d agents (%d with updates available)", len(installations), updateCount))
			return nil
		},
	}

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
	Healthy       *bool  `json:"healthy,omitempty"`
	HealthError   string `json:"health_error,omitempty"`
}

func outputAgentsTable(agents []AgentListItem, printer *output.Printer, showHealth bool) error {
	if len(agents) == 0 {
		printer.Info("No agents detected")
		printer.Print("")
		printer.Print("Run 'agentmgr catalog list' to see available agents.")
		return nil
	}

	styles := printer.Styles()
	table := output.NewTable()

	// Set headers
	if showHealth {
		table.SetHeaders(
			styles.FormatHeader("ID"),
			styles.FormatHeader("AGENT"),
			styles.FormatHeader("METHOD"),
			styles.FormatHeader("VERSION"),
			styles.FormatHeader("LATEST"),
			styles.FormatHeader("STATUS"),
			styles.FormatHeader("HEALTH"),
		)
	} else {
		table.SetHeaders(
			styles.FormatHeader("ID"),
			styles.FormatHeader("AGENT"),
			styles.FormatHeader("METHOD"),
			styles.FormatHeader("VERSION"),
			styles.FormatHeader("LATEST"),
			styles.FormatHeader("STATUS"),
		)
	}

	// Add rows
	for _, agent := range agents {
		var statusIcon string
		if agent.HasUpdate {
			statusIcon = styles.UpdateIcon()
		} else {
			statusIcon = styles.InstalledIcon()
		}

		latest := agent.LatestVersion
		if latest == "" {
			latest = "-"
		} else {
			latest = styles.FormatVersion(latest, false)
		}

		if showHealth {
			healthIcon := "-"
			if agent.Healthy != nil {
				if *agent.Healthy {
					healthIcon = styles.SuccessIcon()
				} else {
					healthIcon = styles.ErrorIcon()
				}
			}
			table.AddRow(
				styles.Info.Render(agent.ID),
				styles.FormatAgentName(agent.Name),
				styles.FormatMethod(agent.Method),
				styles.FormatVersion(agent.Version, agent.HasUpdate),
				latest,
				statusIcon,
				healthIcon,
			)
		} else {
			table.AddRow(
				styles.Info.Render(agent.ID),
				styles.FormatAgentName(agent.Name),
				styles.FormatMethod(agent.Method),
				styles.FormatVersion(agent.Version, agent.HasUpdate),
				latest,
				statusIcon,
			)
		}
	}

	table.Render()
	return nil
}

// verifyAgentHealth checks if an agent executable can run by trying --version or --help.
func verifyAgentHealth(ctx context.Context, execPath string) (bool, string) {
	// Try common version/help flags
	flags := []string{"--version", "-v", "--help", "-h"}

	for _, flag := range flags {
		cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cmd := exec.CommandContext(cmdCtx, execPath, flag)
		err := cmd.Run()
		cancel()

		if err == nil {
			return true, ""
		}

		// Check if it's just a non-zero exit (some tools exit non-zero for --help)
		if exitErr, ok := err.(*exec.ExitError); ok {
			// If it ran but exited non-zero, it's still "healthy" (executable works)
			if exitErr.ExitCode() != 0 {
				return true, ""
			}
		}
	}

	// Try running with no arguments (some tools show help this way)
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	cmd := exec.CommandContext(cmdCtx, execPath)
	err := cmd.Run()
	cancel()

	if err == nil {
		return true, ""
	}

	// Check if it ran but exited non-zero
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 0 {
			return true, ""
		}
	}

	return false, err.Error()
}

func outputAgentsJSON(agents []AgentListItem) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}
