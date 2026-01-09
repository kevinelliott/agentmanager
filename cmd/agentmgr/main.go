// AgentManager CLI - A tool for managing AI development CLI agents
package main

import (
	"fmt"
	"os"

	"github.com/kevinelliott/agentmgr/internal/cli"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/platform"
)

// Version information (set by build flags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Check for native Windows (not supported)
	if msg := platform.CheckWindowsSupport(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}

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
