package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/internal/cli/output"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/platform"
	"github.com/kevinelliott/agentmgr/pkg/storage"
)

// NewCatalogCommand creates the catalog management command group.
func NewCatalogCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage the agent catalog",
		Long: `List available agents from the catalog, refresh the catalog from GitHub,
and search for specific agents.

The catalog contains definitions for all supported AI development agents
including their installation methods, detection signatures, and changelog
sources.`,
	}

	cmd.AddCommand(
		newCatalogListCommand(cfg),
		newCatalogRefreshCommand(cfg),
		newCatalogSearchCommand(cfg),
		newCatalogShowCommand(cfg),
	)

	return cmd
}

func newCatalogListCommand(cfg *config.Config) *cobra.Command {
	var (
		format     string
		platformID string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available agents in the catalog",
		Long: `Display all agents available in the catalog. Use --platform to filter
by platform compatibility.`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Create printer for colored output
			printer := output.NewPrinter(cfg, cmd.Flag("no-color").Changed && cmd.Flag("no-color").Value.String() == "true")

			// Get current platform
			plat := platform.Current()
			if platformID == "" {
				platformID = string(plat.ID())
			}

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

			spinner.Stop()

			// Get agents for platform
			var agents []CatalogListItem
			for _, agentDef := range cat.Agents {
				if !agentDef.IsSupported(platformID) {
					continue
				}

				methods := agentDef.GetSupportedMethods(platformID)
				methodNames := make([]string, 0, len(methods))
				for _, m := range methods {
					methodNames = append(methodNames, m.Method)
				}

				agents = append(agents, CatalogListItem{
					ID:          agentDef.ID,
					Name:        agentDef.Name,
					Description: agentDef.Description,
					Methods:     methodNames,
				})
			}

			if format == "json" {
				return outputCatalogJSON(agents)
			}

			return outputCatalogTable(agents, printer)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().StringVarP(&platformID, "platform", "p", "", "filter by platform (darwin, linux, windows)")

	return cmd
}

func newCatalogRefreshCommand(cfg *config.Config) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the catalog from GitHub",
		Long: `Fetch the latest catalog from the GitHub repository and update
the local cache. This is done automatically on startup if enabled
in configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// Create spinner
			spinner := output.NewSpinner(
				output.WithMessage("Refreshing catalog from GitHub..."),
				output.WithNoColor(!cfg.UI.UseColors),
			)
			spinner.Start()

			// Get current platform
			plat := platform.Current()

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

			// Refresh catalog from remote
			if err := catMgr.Refresh(ctx); err != nil {
				spinner.Error("Failed to refresh catalog")
				return fmt.Errorf("failed to refresh catalog: %w", err)
			}

			// Get refreshed catalog to show stats
			cat, err := catMgr.Get(ctx)
			if err != nil {
				spinner.Error("Failed to get catalog")
				return fmt.Errorf("failed to get catalog: %w", err)
			}

			spinner.Success(fmt.Sprintf("Catalog refreshed - %d agents available (version %s)", len(cat.Agents), cat.Version))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "F", false, "force refresh even if recently updated")

	return cmd
}

func newCatalogSearchCommand(cfg *config.Config) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the catalog",
		Long:  `Search for agents in the catalog by name or description.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Create printer for colored output
			printer := output.NewPrinter(cfg, cmd.Flag("no-color").Changed && cmd.Flag("no-color").Value.String() == "true")

			// Get current platform
			plat := platform.Current()

			// Create spinner
			spinner := output.NewSpinner(
				output.WithMessage("Searching catalog..."),
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

			// Search agents
			results := cat.Search(query)
			spinner.Stop()

			if len(results) == 0 {
				printer.Info("No results found for %q", query)
				return nil
			}

			// Convert to list items
			var agents []CatalogListItem
			for _, agentDef := range results {
				methods := agentDef.GetSupportedMethods(string(plat.ID()))
				methodNames := make([]string, 0, len(methods))
				for _, m := range methods {
					methodNames = append(methodNames, m.Method)
				}

				agents = append(agents, CatalogListItem{
					ID:          agentDef.ID,
					Name:        agentDef.Name,
					Description: agentDef.Description,
					Methods:     methodNames,
				})
			}

			if format == "json" {
				return outputCatalogJSON(agents)
			}

			return outputCatalogTable(agents, printer)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")

	return cmd
}

func newCatalogShowCommand(cfg *config.Config) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show <agent-id>",
		Short: "Show detailed catalog entry for an agent",
		Long: `Display the full catalog entry for an agent, including all
installation methods, detection signatures, and changelog sources.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentID := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

			// Get agent from catalog
			agentDef, ok := cat.GetAgent(agentID)
			if !ok {
				return fmt.Errorf("agent %q not found in catalog", agentID)
			}

			if format == "json" {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(agentDef)
			}

			// Text output
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
			fmt.Printf("\nInstallation Methods:\n")
			for _, m := range agentDef.InstallMethods {
				platforms := "all"
				if len(m.Platforms) > 0 {
					platforms = fmt.Sprintf("%v", m.Platforms)
				}
				fmt.Printf("  %s:\n", m.Method)
				fmt.Printf("    Command: %s\n", m.Command)
				if m.Package != "" {
					fmt.Printf("    Package: %s\n", m.Package)
				}
				fmt.Printf("    Platforms: %s\n", platforms)
			}

			// Detection info
			if agentDef.Detection.VersionCmd != "" {
				fmt.Printf("\nDetection:\n")
				fmt.Printf("  Executables: %v\n", agentDef.Detection.Executables)
				fmt.Printf("  Version Command: %s\n", agentDef.Detection.VersionCmd)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format (text, json)")

	return cmd
}

// CatalogListItem represents an agent in the catalog list output.
type CatalogListItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Methods     []string `json:"methods"`
}

func outputCatalogTable(agents []CatalogListItem, printer *output.Printer) error {
	if len(agents) == 0 {
		printer.Info("No agents in catalog.")
		return nil
	}

	styles := printer.Styles()
	table := output.NewTable()

	// Set headers
	table.SetHeaders(
		styles.FormatHeader("ID"),
		styles.FormatHeader("NAME"),
		styles.FormatHeader("METHODS"),
		styles.FormatHeader("DESCRIPTION"),
	)

	// Add rows
	for _, agent := range agents {
		methods := ""
		if len(agent.Methods) > 0 {
			methods = agent.Methods[0]
			if len(agent.Methods) > 1 {
				methods += fmt.Sprintf(" +%d", len(agent.Methods)-1)
			}
		}

		desc := agent.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		table.AddRow(
			styles.Info.Render(agent.ID),
			styles.FormatAgentName(agent.Name),
			styles.FormatMethod(methods),
			styles.Muted.Render(desc),
		)
	}

	table.Render()
	printer.Print("")
	printer.Print("%s agents available", styles.Bold.Render(fmt.Sprintf("%d", len(agents))))
	return nil
}

func outputCatalogJSON(agents []CatalogListItem) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}
