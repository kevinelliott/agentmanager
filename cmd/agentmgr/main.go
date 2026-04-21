// AgentManager CLI - A tool for managing AI development CLI agents
package main

import (
	"fmt"
	"os"

	"github.com/kevinelliott/agentmanager/internal/cli"
	"github.com/kevinelliott/agentmanager/pkg/config"
	"github.com/kevinelliott/agentmanager/pkg/logging"
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

	// Configure the default logger from cfg.Logging so any library (including
	// our own packages) that reaches for slog.Default picks up the level,
	// format, and destination the operator configured.
	logging.Install(logging.New(cfg))

	// Create and execute root command
	rootCmd := cli.NewRootCommand(cfg, version, commit, date)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
