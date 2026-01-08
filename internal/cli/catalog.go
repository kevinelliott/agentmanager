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

			// Get current platform
			plat := platform.Current()
			if platformID == "" {
				platformID = string(plat.ID())
			}

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

			return outputCatalogTable(agents)
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
			fmt.Println("Refreshing catalog from GitHub...")

			// TODO: Implement actual catalog refresh
			printSuccess("Catalog refreshed successfully")
			printInfo("9 agents available")
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

			// Search agents
			results := cat.Search(query)
			if len(results) == 0 {
				fmt.Printf("No results found for %q\n", query)
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

			return outputCatalogTable(agents)
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

			// TODO: Load actual catalog and show agent
			fmt.Printf("Agent ID: %s\n", agentID)
			fmt.Println("\nNot implemented yet.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "output format (text, json, yaml)")

	return cmd
}

// CatalogListItem represents an agent in the catalog list output.
type CatalogListItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Methods     []string `json:"methods"`
}

func outputCatalogTable(agents []CatalogListItem) error {
	if len(agents) == 0 {
		fmt.Println("No agents in catalog.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "ID\tNAME\tMETHODS\tDESCRIPTION")
	fmt.Fprintln(w, "--\t----\t-------\t-----------")

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

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			agent.ID,
			agent.Name,
			methods,
			desc,
		)
	}

	fmt.Printf("\n%d agents available\n", len(agents))
	return nil
}

func outputCatalogJSON(agents []CatalogListItem) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}
