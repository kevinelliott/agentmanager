package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/kevinelliott/agentmanager/pkg/config"
)

// NewAPICommand creates the API documentation command group.
func NewAPICommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "API documentation and tools",
		Long: `View and interact with the AgentManager REST API documentation.

This command group provides tools for working with the REST API,
including viewing the OpenAPI specification and endpoint information.`,
	}

	cmd.AddCommand(
		newAPIDocsCommand(cfg),
		newAPISpecCommand(cfg),
		newAPIEndpointsCommand(cfg),
	)

	return cmd
}

func newAPIDocsCommand(cfg *config.Config) *cobra.Command {
	var open bool

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Show API documentation",
		Long: `Display information about the AgentManager REST API.

The full OpenAPI specification is available at:
  - File: api/openapi.yaml (in the source repository)
  - Endpoint: http://localhost:<port>/openapi.yaml (when server is running)

You can use tools like Swagger UI, Redoc, or Postman to explore the API.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("AgentManager REST API Documentation")
			fmt.Println("====================================")
			fmt.Println()
			fmt.Println("Base URL: http://localhost:<port>/api/v1")
			fmt.Println()
			fmt.Println("OpenAPI Specification:")
			fmt.Println("  • YAML: /openapi.yaml")
			fmt.Println("  • JSON: /openapi.json (redirects to YAML)")
			fmt.Println()
			fmt.Println("Available Endpoints:")
			fmt.Println()
			fmt.Println("  Status:")
			fmt.Println("    GET  /health              Health check")
			fmt.Println("    GET  /api/v1/status       Server status")
			fmt.Println()
			fmt.Println("  Agents:")
			fmt.Println("    GET  /api/v1/agents       List installed agents")
			fmt.Println("    POST /api/v1/agents       Install an agent")
			fmt.Println("    GET  /api/v1/agents/{key} Get agent details")
			fmt.Println("    PUT  /api/v1/agents/{key} Update an agent")
			fmt.Println("    DEL  /api/v1/agents/{key} Uninstall an agent")
			fmt.Println()
			fmt.Println("  Catalog:")
			fmt.Println("    GET  /api/v1/catalog         List catalog agents")
			fmt.Println("    GET  /api/v1/catalog/{id}    Get catalog agent")
			fmt.Println("    POST /api/v1/catalog/refresh Refresh catalog")
			fmt.Println("    GET  /api/v1/catalog/search  Search catalog")
			fmt.Println()
			fmt.Println("  Updates:")
			fmt.Println("    GET  /api/v1/updates           Check for updates")
			fmt.Println("    GET  /api/v1/changelog/{id}    Get changelog")
			fmt.Println()

			if open {
				specPath := filepath.Join("api", "openapi.yaml")
				if _, err := os.Stat(specPath); err == nil {
					var openCmd string
					switch runtime.GOOS {
					case "darwin":
						openCmd = "open"
					case "linux":
						openCmd = "xdg-open"
					case "windows":
						openCmd = "start"
					}
					if openCmd != "" {
						fmt.Printf("Opening %s...\n", specPath)
					}
				}
			}

			fmt.Println("For the full OpenAPI specification, see: api/openapi.yaml")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&open, "open", "o", false, "open the OpenAPI spec file")

	return cmd
}

func newAPISpecCommand(cfg *config.Config) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Output the OpenAPI specification",
		Long:  `Output the embedded OpenAPI specification to stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read the full spec from the api directory if available
			specPath := filepath.Join("api", "openapi.yaml")
			if data, err := os.ReadFile(specPath); err == nil {
				fmt.Print(string(data))
				return nil
			}

			// Fall back to embedded minimal spec
			fmt.Print(embeddedOpenAPISpec)
			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "yaml", "output format (yaml)")

	return cmd
}

func newAPIEndpointsCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "endpoints",
		Short: "List all API endpoints",
		Long:  `List all available REST API endpoints with their HTTP methods.`,
		Run: func(cmd *cobra.Command, args []string) {
			endpoints := []struct {
				Method      string
				Path        string
				Description string
			}{
				{"GET", "/health", "Health check"},
				{"GET", "/api/v1/status", "Server status and uptime"},
				{"GET", "/api/v1/agents", "List installed agents"},
				{"POST", "/api/v1/agents", "Install a new agent"},
				{"GET", "/api/v1/agents/{key}", "Get agent installation details"},
				{"PUT", "/api/v1/agents/{key}", "Update an agent"},
				{"DELETE", "/api/v1/agents/{key}", "Uninstall an agent"},
				{"GET", "/api/v1/catalog", "List agents in catalog"},
				{"GET", "/api/v1/catalog/{agentID}", "Get catalog agent details"},
				{"POST", "/api/v1/catalog/refresh", "Refresh catalog from remote"},
				{"GET", "/api/v1/catalog/search", "Search catalog (query: q)"},
				{"GET", "/api/v1/updates", "Check for available updates"},
				{"GET", "/api/v1/changelog/{agentID}", "Get changelog (query: from, to)"},
				{"GET", "/openapi.yaml", "OpenAPI specification (YAML)"},
				{"GET", "/openapi.json", "OpenAPI specification info"},
			}

			fmt.Println("AgentManager REST API Endpoints")
			fmt.Println("================================")
			fmt.Println()
			fmt.Printf("%-8s %-35s %s\n", "METHOD", "PATH", "DESCRIPTION")
			fmt.Printf("%-8s %-35s %s\n", "------", "----", "-----------")
			for _, ep := range endpoints {
				fmt.Printf("%-8s %-35s %s\n", ep.Method, ep.Path, ep.Description)
			}
		},
	}
}

const embeddedOpenAPISpec = `openapi: 3.0.3
info:
  title: AgentManager REST API
  description: REST API for managing AI development agents.
  version: 1.0.0
  contact:
    name: Kevin Elliott
    url: https://github.com/kevinelliott/agentmanager
  license:
    name: MIT
servers:
  - url: http://localhost:8080/api/v1
    description: Local development server
paths:
  /health:
    get:
      summary: Health check
      responses:
        "200":
          description: Server is healthy
  /status:
    get:
      summary: Get server status
      responses:
        "200":
          description: Server status
  /agents:
    get:
      summary: List installed agents
      responses:
        "200":
          description: List of agents
    post:
      summary: Install an agent
      responses:
        "200":
          description: Agent installed
  /agents/{key}:
    get:
      summary: Get agent details
      responses:
        "200":
          description: Agent details
    put:
      summary: Update an agent
      responses:
        "200":
          description: Agent updated
    delete:
      summary: Uninstall an agent
      responses:
        "200":
          description: Agent uninstalled
  /catalog:
    get:
      summary: List catalog agents
      responses:
        "200":
          description: Catalog agents
  /catalog/{agentID}:
    get:
      summary: Get catalog agent
      responses:
        "200":
          description: Catalog agent details
  /catalog/refresh:
    post:
      summary: Refresh catalog
      responses:
        "200":
          description: Catalog refreshed
  /catalog/search:
    get:
      summary: Search catalog
      responses:
        "200":
          description: Search results
  /updates:
    get:
      summary: Check for updates
      responses:
        "200":
          description: Available updates
  /changelog/{agentID}:
    get:
      summary: Get changelog
      responses:
        "200":
          description: Changelog
`
