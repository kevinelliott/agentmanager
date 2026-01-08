// AgentManager Helper - System tray helper for managing AI development agents
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kevinelliott/agentmgr/internal/systray"
	"github.com/kevinelliott/agentmgr/pkg/catalog"
	"github.com/kevinelliott/agentmgr/pkg/config"
	"github.com/kevinelliott/agentmgr/pkg/detector"
	"github.com/kevinelliott/agentmgr/pkg/installer"
	"github.com/kevinelliott/agentmgr/pkg/platform"
	"github.com/kevinelliott/agentmgr/pkg/storage"
)

// Version information (set by build flags)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Get current platform
	plat := platform.Current()

	// Load configuration
	loader := config.NewLoader()
	cfg, err := loader.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize storage
	dataDir := plat.GetDataDir()
	store, err := storage.NewSQLiteStore(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Initialize detector with strategies
	det := detector.New(plat)

	// Initialize catalog manager
	cat := catalog.NewManager(cfg, store)

	// Initialize installer manager
	inst := installer.NewManager(plat)

	// Create and run systray app
	app := systray.New(cfg, plat, store, det, cat, inst)

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run in a goroutine so we can handle signals
	errChan := make(chan error, 1)
	go func() {
		errChan <- app.Run()
	}()

	// Wait for either signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
		// Give systray time to clean up
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		<-ctx.Done()
	case err := <-errChan:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return // Exit main() normally, letting defers run
		}
	}
}
