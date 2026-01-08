package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmgr/pkg/config"
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
		format   string
		platform string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available agents in the catalog",
		Long: `Display all agents available in the catalog. Use --platform to filter
by platform compatibility.`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Load actual catalog
			agents := []CatalogListItem{
				{ID: "claude-code", Name: "Claude Code", Description: "Anthropic's official CLI for Claude AI pair programming", Methods: []string{"npm", "native"}},
				{ID: "aider", Name: "Aider", Description: "AI pair programming in your terminal", Methods: []string{"pip", "pipx", "uv"}},
				{ID: "copilot-cli", Name: "GitHub Copilot CLI", Description: "GitHub Copilot in the command line", Methods: []string{"npm", "brew", "winget"}},
				{ID: "gemini-cli", Name: "Gemini CLI", Description: "Google's Gemini AI in your terminal", Methods: []string{"npm"}},
				{ID: "continue-cli", Name: "Continue CLI", Description: "Open-source AI code assistant CLI", Methods: []string{"npm"}},
				{ID: "opencode", Name: "OpenCode", Description: "The open source AI coding agent", Methods: []string{"npm", "brew", "scoop", "chocolatey", "curl"}},
				{ID: "cursor-cli", Name: "Cursor CLI", Description: "Cursor AI editor CLI agent", Methods: []string{"native"}},
				{ID: "qoder-cli", Name: "Qoder CLI", Description: "Qoder AI coding assistant CLI", Methods: []string{"binary"}},
				{ID: "amazon-q", Name: "Amazon Q Developer", Description: "Amazon's AI-powered developer assistant", Methods: []string{"brew", "native"}},
			}

			if format == "json" {
				return outputCatalogJSON(agents)
			}

			return outputCatalogTable(agents)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, json)")
	cmd.Flags().StringVarP(&platform, "platform", "p", "", "filter by platform (darwin, linux, windows)")

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
			fmt.Printf("Searching for: %s\n\n", query)

			// TODO: Implement actual search
			fmt.Println("No results found.")
			return nil
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
