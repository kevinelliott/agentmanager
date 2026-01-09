// AgentManager CLI - A tool for managing AI development CLI agents
package main

import (
	"fmt"
	"os"

	"github.com/kevinelliott/agentmgr/internal/cli"
	"github.com/kevinelliott/agentmgr/pkg/config"
)

// Version information (set by build flags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.Default()
	}

	// Create and execute root command
	rootCmd := cli.NewRootCommand(cfg, version, commit, date)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
